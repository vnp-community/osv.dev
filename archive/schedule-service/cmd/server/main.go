package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	httpHandler "github.com/osv/schedule-service/adapter/handler/http"
	pgRepo "github.com/osv/schedule-service/adapter/repository/postgres"
	cronworker "github.com/osv/schedule-service/adapter/worker"
	"github.com/osv/schedule-service/internal/infrastructure/leader"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	logger := log.With().Str("service", "schedule-service").Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Postgres ────────────────────────────────────────────────────────────
	dbPool, err := pgxpool.New(ctx, getEnv("DATABASE_URL",
		"postgres://postgres:postgres@localhost:5432/openvulnscan?search_path=schedule&sslmode=disable"))
	if err != nil {
		logger.Fatal().Err(err).Msg("postgres connect failed")
	}
	defer dbPool.Close()

	// ── Redis ─────────────────────────────────────────────────────────────
	rdbOpts, _ := redis.ParseURL(getEnv("REDIS_URL", "redis://localhost:6379"))
	rdb := redis.NewClient(rdbOpts)
	defer rdb.Close()

	// ── Repositories ─────────────────────────────────────────────────────
	schedRepo := pgRepo.NewScheduleRepo(dbPool)

	// ── Leader Lock + Cron Worker ─────────────────────────────────────────
	lock   := leader.NewRedisLock(rdb)
	worker := cronworker.NewCronWorker(lock, schedRepo, nil, logger) // publisher wired via NATS (future)
	go worker.Start(ctx)

	// ── HTTP Server ────────────────────────────────────────────────────────
	h      := httpHandler.NewScheduleHandler(schedRepo, logger)
	router := httpHandler.NewRouter(h, logger)

	port := getEnv("HTTP_PORT", "9106")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info().Str("port", port).Msg("schedule-service HTTP starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("HTTP server error")
		}
	}()

	logger.Info().Str("http", ":"+port).Msg("schedule-service ready")
	<-ctx.Done()

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	srv.Shutdown(shutCtx) //nolint:errcheck
	logger.Info().Msg("schedule-service stopped")
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}
