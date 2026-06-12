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

	httpHandler "github.com/osv/asset-service/adapter/handler/http"
	pgRepo "github.com/osv/asset-service/adapter/repository/postgres"
	upsertasset "github.com/osv/asset-service/internal/usecase/upsert_asset"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	logger := log.With().Str("service", "asset-service").Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Postgres ────────────────────────────────────────────────────────────
	dbPool, err := pgxpool.New(ctx, getEnv("DATABASE_URL",
		"postgres://postgres:postgres@localhost:5432/openvulnscan?search_path=asset&sslmode=disable"))
	if err != nil {
		logger.Fatal().Err(err).Msg("postgres connect failed")
	}
	defer dbPool.Close()

	// ── Repositories ─────────────────────────────────────────────────────
	assetRepo := pgRepo.NewAssetRepo(dbPool)
	vulnRepo  := pgRepo.NewVulnerabilityRepo(dbPool)

	// ── Use Cases ─────────────────────────────────────────────────────────
	upsertUC := upsertasset.NewUseCase(assetRepo, nil) // publisher via NATS (future)

	// ── HTTP Server ────────────────────────────────────────────────────────
	h      := httpHandler.NewAssetHandler(upsertUC, assetRepo, vulnRepo, logger)
	router := httpHandler.NewRouter(h, logger)

	port := getEnv("HTTP_PORT", "9103")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info().Str("port", port).Msg("asset-service HTTP starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("HTTP server error")
		}
	}()

	logger.Info().Str("http", ":"+port).Msg("asset-service ready")
	<-ctx.Done()

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	srv.Shutdown(shutCtx) //nolint:errcheck
	logger.Info().Msg("asset-service stopped")
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
