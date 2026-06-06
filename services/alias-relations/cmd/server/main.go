// cmd/server/main.go — Entry point for Alias Relations Service (Enterprise Edition)
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/nats-io/nats.go"
	natsjs "github.com/nats-io/nats.go/jetstream"
	mergehandler "github.com/osv/alias-relations/internal/application/command/merge_alias_group"
	detecthandler "github.com/osv/alias-relations/internal/application/command/detect_new_aliases"
	resolvehandler "github.com/osv/alias-relations/internal/application/query/resolve_alias"
	"github.com/osv/alias-relations/internal/domain/service"
	firestorepkg "github.com/osv/alias-relations/internal/infra/persistence/firestore"
	natspkg "github.com/osv/alias-relations/internal/infra/messaging/nats"
	httphandler "github.com/osv/alias-relations/interface/http/handler"
	"github.com/osv/alias-relations/config"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"github.com/osv/pkg/grpcutil"
	"github.com/osv/pkg/health"
	"github.com/osv/pkg/observability"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── OTel ────────────────────────────────────────────────────────────
	log = log.With().Str("service", "alias-relations").Logger()
	otelShutdown := observability.MustSetup("alias-relations", "1.0.0", log)
	defer otelShutdown()

	// ── Firestore ──────────────────────────────────────────────────────────────
	fsClient, err := firestore.NewClient(ctx, cfg.Firestore.ProjectID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create Firestore client")
	}
	defer fsClient.Close()

	// ── NATS ───────────────────────────────────────────────────────────────────
	nc, err := nats.Connect(cfg.NATS.URL,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Warn().Err(err).Msg("NATS disconnected")
		}),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to NATS")
	}
	defer nc.Drain() //nolint:errcheck

	js, err := natsjs.New(nc)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create JetStream context")
	}

	// ── Repositories ──────────────────────────────────────────────────────────
	groupRepo := firestorepkg.NewAliasGroupRepo(fsClient)

	// ── Publisher ─────────────────────────────────────────────────────────────
	publisher := natspkg.NewPublisher(js, log)

	// ── Domain Services ───────────────────────────────────────────────────────
	aliasMerger := service.NewAliasMerger(groupRepo, log)

	// SimilarityDetector requires AI Enrichment gRPC client (optional)
	// TODO: wire AI enrichment gRPC client when available
	var similarityDetector *service.SimilarityDetector

	// ── Application Handlers ──────────────────────────────────────────────────
	mergeH := mergehandler.NewHandler(aliasMerger, groupRepo, publisher, log)
	resolveH := resolvehandler.NewHandler(groupRepo)

	var detectH *detecthandler.Handler
	if similarityDetector != nil {
		detectH = detecthandler.NewHandler(similarityDetector, mergeH, groupRepo, log)
	}

	// ── NATS Consumers ────────────────────────────────────────────────────────
	vulnConsumer := natspkg.NewVulnImportedConsumer(js, mergeH, log)
	go func() {
		if err := vulnConsumer.Start(ctx); err != nil {
			log.Error().Err(err).Msg("VulnImported consumer stopped")
		}
	}()

	if detectH != nil {
		aiConsumer := natspkg.NewAIEnrichmentConsumer(js, detectH, log)
		go func() {
			if err := aiConsumer.Start(ctx); err != nil {
				log.Error().Err(err).Msg("AIEnrichment consumer stopped")
			}
		}()
	}

	// ── HTTP Server (pkg/health MultiChecker) ────────────────────────────────
	checker := health.NewMultiChecker(5*time.Second,
		health.NATSProber(nc),
		health.NewFirestoreProber(func(ctx context.Context) error {
			_, err := fsClient.Collection("health").Doc("probe").Get(ctx)
			if err != nil && err.Error() != "rpc error: code = NotFound" {
				return err
			}
			return nil
		}),
	)
	_ = httphandler.NewHealthHandler() // keep reference, replaced by pkg/health
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", health.LiveHandler())
	mux.Handle("/health/ready", health.ReadyHandler(checker))

	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		log.Info().Int("port", cfg.Server.HTTPPort).Msg("HTTP server starting")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP server error")
		}
	}()

	// ── gRPC Server (Enterprise interceptor chain) ───────────────────────────
	grpcSrv := grpc.NewServer(grpcutil.ServerOptions("alias-relations", log, 10*time.Second)...)
	// TODO: register AliasRelationsServiceServer when proto is generated
	_ = resolveH
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen on gRPC port")
	}
	go func() {
		log.Info().Int("port", cfg.Server.GRPCPort).Msg("gRPC server starting")
		if err := grpcSrv.Serve(lis); err != nil {
			log.Error().Err(err).Msg("gRPC server error")
		}
	}()

	// ── Graceful Shutdown ──────────────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	log.Info().Msg("shutting down...")
	cancel()
	grpcSrv.GracefulStop()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	httpSrv.Shutdown(shutCtx) //nolint:errcheck

	log.Info().Msg("shutdown complete")
}
