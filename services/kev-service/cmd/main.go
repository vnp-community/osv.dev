// kev-service: CISA Known Exploited Vulnerabilities service.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/globalcve/kev-service/internal/adapter/external/cisa"
	keprepo "github.com/globalcve/kev-service/internal/adapter/repository/postgres"
	deliveryhttp "github.com/globalcve/kev-service/internal/delivery/http"
	"github.com/globalcve/kev-service/internal/delivery/scheduler"
	"github.com/globalcve/kev-service/internal/usecase/check"
	"github.com/globalcve/kev-service/internal/usecase/query"
	"github.com/globalcve/kev-service/internal/usecase/sync"
	pgpool "github.com/osv/pkg/database/postgres"
)

// appConfig holds all configuration for the service.
type appConfig struct {
	DatabaseURL     string `env:"DATABASE_URL,required"`
	Port            string `env:"PORT"`
	CISAURL         string `env:"CISA_KEV_URL"`
	LogLevel        string `env:"LOG_LEVEL"`
	RunOnStartup    bool   `env:"SYNC_ON_STARTUP"`
	SchedulerEnabled bool  `env:"SCHEDULER_ENABLED"`
	DBMaxConns      int32  `env:"DB_MAX_CONNS"`
	DBMinConns      int32  `env:"DB_MIN_CONNS"`
}

func main() {
	// ── Config ────────────────────────────────────────────────────────────────
	cfg := loadConfig()

	// ── Logger ────────────────────────────────────────────────────────────────
	log := buildLogger(cfg.LogLevel)
	log.Info().Msg("Starting kev-service")

	// ── PostgreSQL pool ───────────────────────────────────────────────────────
	ctx := context.Background()
	poolCfg := pgpool.DefaultConfig()
	poolCfg.URL = cfg.DatabaseURL
	if cfg.DBMaxConns > 0 {
		poolCfg.MaxConns = cfg.DBMaxConns
	}
	if cfg.DBMinConns > 0 {
		poolCfg.MinConns = cfg.DBMinConns
	}

	pool, err := pgpool.NewPool(ctx, poolCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer pool.Close()
	log.Info().Msg("PostgreSQL pool connected")

	// ── Dependencies ──────────────────────────────────────────────────────────
	cisaClient := cisa.NewClient(cfg.CISAURL)
	kevRepo := keprepo.NewKEVRepository(pool)

	// ── Use cases ─────────────────────────────────────────────────────────────
	syncUC := sync.New(cisaClient, kevRepo, log)
	queryUC := query.New(kevRepo)
	checkUC := check.New(kevRepo)

	// ── HTTP server ───────────────────────────────────────────────────────────
	handler := deliveryhttp.NewHandler(queryUC, checkUC, syncUC, kevRepo, log)
	router := deliveryhttp.NewRouter(handler, log)

	port := cfg.Port
	if port == "" {
		port = "8083"
	}
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Scheduler ─────────────────────────────────────────────────────────────
	sched := scheduler.New(syncUC, log)
	if cfg.SchedulerEnabled {
		sched.Start()
		if cfg.RunOnStartup {
			log.Info().Msg("Running initial KEV sync on startup")
			sched.RunNow()
		}
	}

	// ── Start HTTP ────────────────────────────────────────────────────────────
	go func() {
		log.Info().Str("addr", srv.Addr).Msg("HTTP server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	log.Info().Msg("Shutting down kev-service...")

	sched.Stop()

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("HTTP shutdown error")
	}
	log.Info().Msg("kev-service stopped")
}

func loadConfig() appConfig {
	cfg := appConfig{
		Port:             "8083",
		LogLevel:         "info",
		SchedulerEnabled: true,
	}
	// Simple env overlay (DATABASE_URL etc. are required via env tag).
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = v
	}
	if v := os.Getenv("CISA_KEV_URL"); v != "" {
		cfg.CISAURL = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if os.Getenv("SYNC_ON_STARTUP") == "true" {
		cfg.RunOnStartup = true
	}
	if os.Getenv("SCHEDULER_ENABLED") == "false" {
		cfg.SchedulerEnabled = false
	}
	return cfg
}

func buildLogger(level string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	return zerolog.New(os.Stdout).Level(lvl).With().Timestamp().Str("service", "kev-service").Logger()
}
