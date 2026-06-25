// Package main — sla-service entry point.
// Manages SLA configurations, computes expiration dates, and monitors breaches.
// Ports: 8086 (HTTP REST), 9006 (gRPC)
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	httpdelivery "github.com/osv/sla-service/internal/delivery/http"
	pgrepo "github.com/osv/sla-service/internal/infra/postgres"
	ucconfig "github.com/osv/sla-service/internal/usecase/config"
)

type config struct {
	HTTPPort    int
	DatabaseURL string
	NATSAddress string
	FindingGRPC string // address of finding-service gRPC
}

func loadConfig() *config {
	return &config{
		HTTPPort:    envInt("SLA_HTTP_PORT", 8086),
		DatabaseURL: os.Getenv("SLA_DATABASE_URL"),
		NATSAddress: os.Getenv("NATS_ADDRESS"),
		FindingGRPC: os.Getenv("FINDING_GRPC_ADDR"),
	}
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	cfg := loadConfig()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ── Database ────────────────────────────────────────────────────────────
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("database ping failed")
	}
	log.Info().Msg("database connected")

	// ── HTTP Server ─────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"sla-service"}`)
	})

	// P1-09 + BUG-H2-004 FIX: initialize SLAConfigHandler before mounting routes
	slaCfgRepo := pgrepo.NewSLAConfigRepo(pool)
	slaCfgRepoImpl := slaCfgRepo.(interface {
		ucconfig.Repository
	})
	slaCfgHandler := httpdelivery.NewSLAConfigHandler(
		ucconfig.NewCreate(slaCfgRepoImpl, nil),
		ucconfig.NewUpdate(slaCfgRepoImpl, nil),
		ucconfig.NewDelete(slaCfgRepoImpl, nil),
		ucconfig.NewAssignProduct(slaCfgRepoImpl, nil, nil),
		slaCfgRepoImpl,
	)

	// SLA config routes — wired to SLAConfigHandler (BUG-H2-004 FIX)
	r.Route("/api/v2/sla-configurations", func(r chi.Router) {
		r.Post("/bulk", slaCfgHandler.BulkCreateConfigs)
		r.Post("/assign-bulk", slaCfgHandler.BulkAssign)
		r.Get("/", slaCfgHandler.List)
		r.Post("/", slaCfgHandler.Create)
		r.Get("/{id}", slaCfgHandler.Get)
		r.Put("/{id}", slaCfgHandler.Update)
		r.Delete("/{id}", slaCfgHandler.Delete)
	})

	// P1-09: /api/v1/sla/config — GET returns {global, product_overrides}, PUT updates default config
	r.Get("/api/v1/sla/config", slaCfgHandler.GetConfig)
	r.Put("/api/v1/sla/config", slaCfgHandler.UpdateConfig)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP in background
	go func() {
		log.Info().Int("port", cfg.HTTPPort).Msg("sla-service HTTP started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP server error")
		}
	}()

	// Wait for signal
	<-ctx.Done()
	log.Info().Msg("shutting down sla-service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Info().Msg("sla-service stopped")
}

// ── helpers ───────────────────────────────────────────────────────────────────

func envInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
