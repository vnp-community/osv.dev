// Command scan-orchestrator starts the DefectDojo Scan Orchestrator microservice.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	natsutil "github.com/defectdojo/pkg/nats"
	"github.com/defectdojo/scan-orchestrator/adapter/http/handler"
	infparser "github.com/defectdojo/scan-orchestrator/internal/infrastructure/parser"
	import_uc "github.com/defectdojo/scan-orchestrator/internal/usecase/import"
	findingv1 "github.com/defectdojo/proto/finding/v1"
	productv1 "github.com/defectdojo/proto/product/v1"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ── NATS ──────────────────────────────────────────────────────────────────
	nc, err := natsutil.Connect(mustEnv("NATS_URL"))
	if err != nil {
		log.Fatal().Err(err).Msg("nats connect")
	}
	defer nc.Drain()
	js, err := natsutil.SetupStream(ctx, nc)
	if err != nil {
		log.Fatal().Err(err).Msg("nats setup stream")
	}
	pub := natsutil.NewPublisher(js, "scan-orchestrator/v1")

	// ── gRPC Clients ──────────────────────────────────────────────────────────
	productConn, err := grpc.NewClient(mustEnv("PRODUCT_GRPC_ADDR"),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("product grpc dial")
	}
	defer productConn.Close()

	findingConn, err := grpc.NewClient(mustEnv("FINDING_GRPC_ADDR"),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("finding grpc dial")
	}
	defer findingConn.Close()

	productClient := productv1.NewProductServiceClient(productConn)
	findingClient := findingv1.NewFindingServiceClient(findingConn)

	// ── Use Cases & Handlers ─────────────────────────────────────────────────
	factory := infparser.NewFactory()
	importUC := import_uc.NewImportScan(factory, productClient, findingClient, pub, log.Logger)
	importHandler := handler.NewImportHandler(importUC, factory, log.Logger)

	// ── HTTP Server ───────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/health/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Post("/api/v2/import-scan", importHandler.HandleImportScan)
	r.Get("/api/v2/parsers", importHandler.HandleListParsers)

	httpAddr := envOr("HTTP_ADDR", ":8084")
	srv := &http.Server{Addr: httpAddr, Handler: r}

	go func() {
		log.Info().Str("addr", httpAddr).Msg("HTTP listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
	log.Info().Msg("shutdown complete")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatal().Str("key", key).Msg("required env var not set")
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
