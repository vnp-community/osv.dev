// cmd/server/main.go — Search Service entry point (Enterprise Edition)
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

	natsgo "github.com/nats-io/nats.go"
	natsjs "github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"

	"github.com/osv/search/internal/application/command/index_vulnerability"
	"github.com/osv/search/internal/application/query/search_vulnerabilities"
	rediscache "github.com/osv/search/internal/infra/cache/redis"
	natsconsumer "github.com/osv/search/internal/infra/messaging/nats"
	"github.com/osv/search/internal/infra/opensearch"
	"github.com/osv/pkg/grpcutil"
	"github.com/osv/pkg/health"
	"github.com/osv/pkg/observability"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Str("service", "search").Logger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── OTel setup ────────────────────────────────────────────────────────────
	otelShutdown := observability.MustSetup("search", "1.0.0", log)
	defer otelShutdown()

	osURL := envStr("OPENSEARCH_URL", "http://localhost:9200")
	osIndex := envStr("OPENSEARCH_INDEX", "vulnerabilities")
	redisAddr := envStr("REDIS_ADDR", "localhost:6379")
	natsURL := envStr("NATS_URL", "nats://localhost:4222")
	grpcPort := envStr("GRPC_PORT", "50051")
	httpPort := envStr("HTTP_PORT", "8080")

	// ── OpenSearch ────────────────────────────────────────────────────────────
	osClient := opensearch.NewClient(osURL, osIndex, log)
	if err := osClient.EnsureIndex(ctx); err != nil {
		log.Warn().Err(err).Msg("OpenSearch index ensure failed (may not be running yet)")
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	rc := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rc.Close()

	// ── NATS ─────────────────────────────────────────────────────────────────
	nc, err := natsgo.Connect(natsURL, natsgo.MaxReconnects(-1), natsgo.ReconnectWait(2*time.Second))
	if err != nil {
		log.Fatal().Err(err).Msg("NATS connect failed")
	}
	defer nc.Drain() //nolint:errcheck

	js, err := natsjs.New(nc)
	if err != nil {
		log.Fatal().Err(err).Msg("JetStream init failed")
	}

	// ── Wiring ───────────────────────────────────────────────────────────────
	tracer := otel.Tracer("search")
	cache := rediscache.NewSearchCache(rc, log)

	indexHandler := index_vulnerability.NewHandler(osClient, log)
	searchHandler := search_vulnerabilities.NewHandler(osClient, cache, tracer, log)

	// ── NATS Consumer ─────────────────────────────────────────────────────────
	consumer := natsconsumer.NewVulnEventConsumer(js, indexHandler, log)
	go func() {
		if err := consumer.Start(ctx); err != nil && ctx.Err() == nil {
			log.Error().Err(err).Msg("NATS consumer error")
		}
	}()

	// ── gRPC Server (Enterprise interceptor chain) ───────────────────────────
	grpcSrv := grpc.NewServer(grpcutil.ServerOptions("search", log, 10*time.Second)...)
	_ = searchHandler // wire to generated server once proto-gen set up
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Fatal().Err(err).Msg("gRPC listen failed")
	}
	go func() {
		log.Info().Str("port", grpcPort).Msg("gRPC starting")
		if err := grpcSrv.Serve(lis); err != nil {
			log.Error().Err(err).Msg("gRPC error")
		}
	}()

	// ── HTTP Health (pkg/health MultiChecker) ────────────────────────────────
	checker := health.NewMultiChecker(5*time.Second,
		health.RedisProber(rc),
		health.NATSProber(nc),
		health.OpenSearchProber(osURL),
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", health.LiveHandler())
	mux.Handle("/health/ready", health.ReadyHandler(checker))
	httpSrv := &http.Server{Addr: fmt.Sprintf(":%s", httpPort), Handler: mux}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP error")
		}
	}()

	// ── Graceful Shutdown ─────────────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	log.Info().Msg("shutting down search service...")
	cancel()
	grpcSrv.GracefulStop()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	httpSrv.Shutdown(shutCtx) //nolint:errcheck
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
