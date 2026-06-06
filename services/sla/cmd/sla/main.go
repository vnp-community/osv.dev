// Command sla starts the DefectDojo SLA microservice.
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

	pgdb "github.com/defectdojo/pkg/database/postgres"
	natsutil "github.com/defectdojo/pkg/nats"
	"github.com/defectdojo/sla/internal/usecase"
	"github.com/defectdojo/sla/internal/infrastructure/postgres"
	findingv1 "github.com/defectdojo/proto/finding/v1"
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
	pub := natsutil.NewPublisher(js, "sla/v1")
	sub := natsutil.NewSubscriber(js, "sla", log.Logger)

	// ── Finding gRPC client ───────────────────────────────────────────────────
	findingConn, err := grpc.NewClient(mustEnv("FINDING_GRPC_ADDR"),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("finding grpc dial")
	}
	defer findingConn.Close()
	findingClient := findingv1.NewFindingServiceClient(findingConn)

	// ── Repositories & Use Cases ──────────────────────────────────────────────
	slaRepo := postgres.NewSLAConfigRepo(pool)
	computeSLA := usecase.NewComputeSLA(slaRepo, findingClient, pub)
	checkBreaches := usecase.NewCheckBreaches(findingClient, pub)

	// ── NATS Subscription ─────────────────────────────────────────────────────
	if err := sub.Subscribe(ctx, "defectdojo.finding.batch_created", func(ctx context.Context, event *natsutil.CloudEvent) error {
		var payload usecase.FindingBatchCreatedPayload
		if err := event.UnmarshalData(&payload); err != nil {
			return err
		}
		return computeSLA.Execute(ctx, &payload)
	}); err != nil {
		log.Fatal().Err(err).Msg("nats subscribe")
	}

	// ── Hourly Breach Checker ─────────────────────────────────────────────────
	go func() {
		// Run once at startup after a short delay
		time.Sleep(10 * time.Second)
		if err := checkBreaches.Execute(ctx); err != nil {
			log.Error().Err(err).Msg("initial breach check failed")
		}
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := checkBreaches.Execute(ctx); err != nil {
					log.Error().Err(err).Msg("breach check failed")
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// ── HTTP Server ───────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/health/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	// SLA config CRUD routes registered here in handler package

	httpAddr := envOr("HTTP_ADDR", ":8085")
	srv := &http.Server{Addr: httpAddr, Handler: r}
	go func() {
		log.Info().Str("addr", httpAddr).Msg("SLA service listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	<-ctx.Done()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
	log.Info().Msg("SLA service shutdown complete")
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
