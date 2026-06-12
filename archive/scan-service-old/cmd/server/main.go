package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	pgRepo "github.com/osv/scan-service/adapter/repository/postgres"
	httpHandler "github.com/osv/scan-service/adapter/handler/http"
	"github.com/osv/scan-service/adapter/scanner/nmap"
	"github.com/osv/scan-service/adapter/scanner/zap"
	"github.com/osv/scan-service/adapter/worker"
	createscan "github.com/osv/scan-service/internal/usecase/create_scan"
	executescan "github.com/osv/scan-service/internal/usecase/execute_scan"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	logger := log.With().Str("service", "scan-service").Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Postgres ──────────────────────────────────────────────────────────
	dbPool, err := pgxpool.New(ctx, getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/openvulnscan?search_path=scan&sslmode=disable"))
	if err != nil {
		logger.Fatal().Err(err).Msg("postgres connect failed")
	}
	defer dbPool.Close()

	// ── Redis ─────────────────────────────────────────────────────────────
	rdbOpts, _ := redis.ParseURL(getEnv("REDIS_URL", "redis://localhost:6379"))
	rdb := redis.NewClient(rdbOpts)
	defer rdb.Close()
	_ = rdb // used by status cache (future)

	// ── Repositories ─────────────────────────────────────────────────────
	scanRepo    := pgRepo.NewScanRepo(dbPool)
	findingRepo := pgRepo.NewFindingRepo(dbPool)

	// ── Scanners ─────────────────────────────────────────────────────────
	nmapScanner := nmap.NewNmapScanner(getEnv("NMAP_PATH", "nmap"), logger)
	zapClient   := zap.NewClient(getEnv("ZAP_BASE_URL", "http://localhost:8090"), "")
	zapScanner  := zap.NewZAPScanner(zapClient)

	// ── Use Cases ─────────────────────────────────────────────────────────
	createUC  := createscan.NewUseCase(scanRepo, nil)  // publisher wired via NATS (future)
	executeUC := executescan.NewUseCase(
		scanRepo, findingRepo, nil,
		nmapScanner, zapScanner,
		nil, nil, nil, // asset/cve clients + publisher (future gRPC wire)
		logger,
	)

	// ── Worker Pool ────────────────────────────────────────────────────────
	pool := worker.NewWorkerPool(
		getEnvInt("MAX_WORKERS", 5),
		func(ctx context.Context, job worker.ScanJob) error {
			return executeUC.Execute(ctx, job.ScanID)
		},
		logger,
	)
	go pool.Start(ctx)

	// ── HTTP Server ────────────────────────────────────────────────────────
	h := httpHandler.NewScanHandler(createUC, executeUC, scanRepo, findingRepo, pool, logger)
	router := httpHandler.NewRouter(h, logger)

	httpPort := getEnv("HTTP_PORT", "9102")
	srv := &http.Server{
		Addr:         ":" + httpPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info().Str("port", httpPort).Msg("scan-service HTTP starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("HTTP error")
		}
	}()

	logger.Info().Str("http", ":"+httpPort).Msg("scan-service ready")
	<-ctx.Done()

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	srv.Shutdown(shutCtx) //nolint:errcheck
	logger.Info().Msg("scan-service stopped")
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
