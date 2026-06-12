package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	httpHandler "github.com/osv/agent-service/adapter/handler/http"
	pgRepo "github.com/osv/agent-service/adapter/repository/postgres"
	submitreport "github.com/osv/agent-service/internal/usecase/submit_report"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	logger := log.With().Str("service", "agent-service").Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Postgres ────────────────────────────────────────────────────────────
	dbPool, err := pgxpool.New(ctx, getEnv("DATABASE_URL",
		"postgres://postgres:postgres@localhost:5432/openvulnscan?search_path=agent&sslmode=disable"))
	if err != nil {
		logger.Fatal().Err(err).Msg("postgres connect failed")
	}
	defer dbPool.Close()

	// ── Repositories ─────────────────────────────────────────────────────
	agentRepo  := pgRepo.NewAgentRepo(dbPool)
	reportRepo := pgRepo.NewReportRepo(dbPool)
	pkgRepo    := pgRepo.NewPackageRepo(dbPool)

	// ── Use Cases ─────────────────────────────────────────────────────────
	submitUC := submitreport.NewUseCase(agentRepo, reportRepo, pkgRepo, nil) // publisher via NATS (future)

	// ── HTTP Server ────────────────────────────────────────────────────────
	h      := httpHandler.NewAgentHandler(submitUC, agentRepo, reportRepo, logger)
	router := httpHandler.NewRouter(h, logger)

	port := getEnv("HTTP_PORT", "9105")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info().Str("port", port).Msg("agent-service HTTP starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("HTTP server error")
		}
	}()

	logger.Info().Str("http", ":"+port).Msg("agent-service ready")
	<-ctx.Done()

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	srv.Shutdown(shutCtx) //nolint:errcheck
	logger.Info().Msg("agent-service stopped")
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}
