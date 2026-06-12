// Command audit starts the DefectDojo Audit microservice.
// Subscribes to ALL DefectDojo events via wildcard "defectdojo.>" and records them.
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

	pgdb "github.com/defectdojo/pkg/database/postgres"
	natsutil "github.com/defectdojo/pkg/nats"
	"github.com/defectdojo/audit/internal/infrastructure/postgres"
	"github.com/defectdojo/audit/internal/usecase"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ── Postgres ──────────────────────────────────────────────────────────────
	pool := pgdb.MustNewPool(ctx, &pgdb.Config{URL: mustEnv("DATABASE_URL")})
	defer pool.Close()

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
	sub := natsutil.NewSubscriber(js, "audit", log.Logger)

	// ── Use Cases ─────────────────────────────────────────────────────────────
	auditRepo := postgres.NewAuditRepo(pool)
	recordEventUC := usecase.NewRecordEvent(auditRepo)

	// ── Subscribe to ALL events using wildcard ─────────────────────────────────
	if err := sub.Subscribe(ctx, "defectdojo.>", func(ctx context.Context, event *natsutil.CloudEvent) error {
		return recordEventUC.RecordEvent(ctx, event)
	}); err != nil {
		log.Fatal().Err(err).Msg("nats subscribe defectdojo.>")
	}
	log.Info().Msg("Subscribed to defectdojo.> (all events)")

	// ── HTTP Server ───────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/health/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	// Audit query API registered here

	httpAddr := envOr("HTTP_ADDR", ":8090")
	srv := &http.Server{Addr: httpAddr, Handler: r}
	go func() {
		log.Info().Str("addr", httpAddr).Msg("Audit service listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	<-ctx.Done()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
	log.Info().Msg("Audit service shutdown complete")
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
