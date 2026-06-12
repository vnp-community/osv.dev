// Package app manages the GlobalCVE monolithic application lifecycle.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/globalcve/mono/internal/config"
	"github.com/globalcve/mono/internal/cvesearch"
	"github.com/globalcve/mono/internal/cvesync"
	"github.com/globalcve/mono/internal/gateway"
	infraNATS "github.com/globalcve/mono/internal/infra/nats"
	infraOpenSearch "github.com/globalcve/mono/internal/infra/opensearch"
	infraPostgres "github.com/globalcve/mono/internal/infra/postgres"
	infraRedis "github.com/globalcve/mono/internal/infra/redis"
	"github.com/globalcve/mono/internal/kevservice"
	"github.com/globalcve/mono/internal/notification"
)

// App is the root application struct that manages all service goroutines.
type App struct {
	cfg *config.Config
}

// New creates a new App from the given configuration.
func New(cfg *config.Config) *App {
	return &App{cfg: cfg}
}

// Run initializes all dependencies, runs migrations, and starts all service goroutines.
// It blocks until all services stop or a signal is received.
func (a *App) Run() error {
	// Configure logger
	level, err := zerolog.ParseLevel(a.cfg.Observability.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().Timestamp().Logger().Level(level)
	log.Logger = logger

	log.Info().Msg("GlobalCVE Monolithic App starting…")

	// Root context with signal cancellation
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// --- Shared Infrastructure ---

	log.Info().Msg("connecting to PostgreSQL…")
	pool, err := infraPostgres.NewPool(ctx, a.cfg.Database)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pool.Close()
	log.Info().Msg("PostgreSQL connected")

	log.Info().Msg("connecting to Redis…")
	redisClient, err := infraRedis.NewClient(ctx, a.cfg.Redis)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	defer redisClient.Close()
	log.Info().Msg("Redis connected")

	// OpenSearch (optional)
	osClient, osErr := infraOpenSearch.NewClient(a.cfg.OpenSearch)
	if osErr != nil {
		log.Warn().Err(osErr).Msg("OpenSearch unavailable, will use PostgreSQL GIN only")
		osClient = nil
	} else {
		log.Info().Msg("OpenSearch connected")
	}
	_ = osClient // used by future opensearch adapter

	// NATS (optional)
	natsClient, natsErr := infraNATS.NewClient(ctx, a.cfg.NATS)
	if natsErr != nil {
		log.Warn().Err(natsErr).Msg("NATS unavailable, inter-service events disabled")
		natsClient = nil
	} else {
		log.Info().Msg("NATS connected")
		defer natsClient.Close()
	}

	// --- Database Migrations ---
	log.Info().Msg("running database migrations…")
	if err := runMigrations(pool, a.cfg.Migrations.Dir); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	log.Info().Msg("migrations complete")

	// --- Build Services ---

	cveSearchSvc := cvesearch.New(*a.cfg, pool, redisClient)
	cveSyncSvc := cvesync.New(*a.cfg, pool)
	kevSvc := kevservice.New(*a.cfg, pool)
	notifSvc := notification.New(*a.cfg, pool, natsClient)
	gatewaySvc := gateway.New(*a.cfg, redisClient, cveSearchSvc.Handler())

	// --- Launch Service Goroutines ---

	log.Info().Msg("starting service goroutines…")
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Info().Int("port", a.cfg.Server.CVESyncPort).Msg("cvesync: goroutine started")
		return cveSyncSvc.Start(gctx)
	})

	g.Go(func() error {
		log.Info().Int("port", a.cfg.Server.CVESearchPort).Msg("cvesearch: goroutine started")
		return cveSearchSvc.Start(gctx)
	})

	g.Go(func() error {
		log.Info().Int("port", a.cfg.Server.KEVServicePort).Msg("kevservice: goroutine started")
		return kevSvc.Start(gctx)
	})

	g.Go(func() error {
		log.Info().Int("port", a.cfg.Server.NotificationPort).Msg("notification: goroutine started")
		return notifSvc.Start(gctx)
	})

	g.Go(func() error {
		log.Info().Int("port", a.cfg.Server.GatewayPort).Msg("gateway: goroutine started")
		return gatewaySvc.Start(gctx)
	})

	log.Info().
		Int("gateway", a.cfg.Server.GatewayPort).
		Int("cvesearch", a.cfg.Server.CVESearchPort).
		Int("cvesync", a.cfg.Server.CVESyncPort).
		Int("kev", a.cfg.Server.KEVServicePort).
		Int("notification", a.cfg.Server.NotificationPort).
		Msg("all goroutines started — GlobalCVE is READY")

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		log.Error().Err(err).Msg("service goroutine error")
		return err
	}

	log.Info().Msg("GlobalCVE shutdown complete")
	return nil
}

// runMigrations applies goose migrations using pgx stdlib (database/sql wrapper).
func runMigrations(pool *pgxpool.Pool, dir string) error {
	if dir == "" {
		dir = "migrations"
	}

	// pgx/v5/stdlib registers the "pgx" driver for database/sql
	// Use the pool's config to get the DSN
	connStr := pool.Config().ConnConfig.ConnString()
	sqlDB, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("open stdlib db for migrations: %w", err)
	}
	defer sqlDB.Close()

	if err := goose.SetDialect("pgx"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}

	if err := goose.Up(sqlDB, dir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
