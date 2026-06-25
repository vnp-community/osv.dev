// embed.go — EmbeddedServer allows data-service to be run inside apps/osv orchestrator.
//
// FIX BUG-002: Implement Sprint C — full data-service wiring for embedded (unified binary) mode.
// Replaces the previous placeholder that only served /health.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/osv/data-service/internal/delivery/scheduler"
	"github.com/osv/data-service/internal/fetcher"
	cisaclient "github.com/osv/data-service/internal/infra/external/cisa"
	natspkg "github.com/osv/data-service/internal/infra/messaging/nats"
	"github.com/osv/data-service/internal/infra/persistence/postgres"
	"github.com/osv/data-service/internal/usecase/sync"
)

// DataServiceEmbeddedConfig holds configuration for running data-service
// inside another process (apps/osv orchestrator). When this mode is used,
// the service respects ctx for shutdown instead of os.Signal.
type DataServiceEmbeddedConfig struct {
	HTTPPort    int    // REST API port (default: 8082)
	GRPCPort    int    // gRPC port (default: 50053)
	NATSURL     string // NATS connection URL
	MongoURI    string // MongoDB URI
	MongoDB     string // MongoDB database name (default: cvedb)
	PostgresDSN string // PostgreSQL DSN for KEV repo + alias groups
	NVDAPIKey   string // NVD API key (optional — increases rate limit)
}

// DataServiceEmbeddedServer wraps data-service for embedding in apps/osv.
// Implements orchestrator.Service interface (Name() string, Start(ctx) error).
type DataServiceEmbeddedServer struct {
	cfg DataServiceEmbeddedConfig
}

// NewDataServiceEmbeddedServer creates a new embeddable server instance.
// Does not start serving until Start() is called.
func NewDataServiceEmbeddedServer(cfg DataServiceEmbeddedConfig) *DataServiceEmbeddedServer {
	return &DataServiceEmbeddedServer{cfg: cfg}
}

// Name satisfies the orchestrator.Service interface.
func (s *DataServiceEmbeddedServer) Name() string { return "data-service" }

// Start implements Sprint C: connects MongoDB + PostgreSQL, builds fetcher registry,
// starts scheduler, and serves HTTP health + admin endpoints.
// Blocks until ctx is cancelled (graceful shutdown).
func (s *DataServiceEmbeddedServer) Start(ctx context.Context) error {
	log := zerolog.New(os.Stderr).With().
		Timestamp().
		Str("service", "data-service").
		Logger()

	cfg := s.cfg

	// ── Resolve config (struct > env > default) ──────────────────────────────
	mongoURI := firstNonEmpty(cfg.MongoURI, os.Getenv("MONGO_URI"), "mongodb://localhost:27017")
	mongoDB := firstNonEmpty(cfg.MongoDB, os.Getenv("MONGO_DB"), "cvedb")
	natsURL := firstNonEmpty(cfg.NATSURL, os.Getenv("NATS_URL"), "nats://localhost:4222")
	nvdKey := firstNonEmpty(cfg.NVDAPIKey, os.Getenv("NVD_API_KEY"), "")
	postgresDSN := firstNonEmpty(cfg.PostgresDSN, os.Getenv("POSTGRES_DSN"), buildDSNFromEnv())
	port := cfg.HTTPPort
	if port == 0 {
		port = 8082
	}

	// ── Step 1: Connect MongoDB ──────────────────────────────────────────────
	mongoClient, err := mongo.Connect(ctx,
		mongoopts.Client().
			ApplyURI(mongoURI).
			SetServerSelectionTimeout(10*time.Second),
	)
	if err != nil {
		return fmt.Errorf("data-service: MongoDB connect: %w", err)
	}
	defer mongoClient.Disconnect(context.Background()) //nolint:errcheck

	if err := mongoClient.Ping(ctx, nil); err != nil {
		return fmt.Errorf("data-service: MongoDB ping: %w", err)
	}
	db := mongoClient.Database(mongoDB)
	log.Info().Str("db", mongoDB).Msg("MongoDB connected")

	// ── Step 2: Connect PostgreSQL (for KEV repository) ───────────────────────
	pgPool, err := pgxpool.New(ctx, postgresDSN)
	if err != nil {
		return fmt.Errorf("data-service: PostgreSQL connect: %w", err)
	}
	defer pgPool.Close()

	if err := pgPool.Ping(ctx); err != nil {
		return fmt.Errorf("data-service: PostgreSQL ping: %w", err)
	}
	log.Info().Msg("PostgreSQL connected")

	// ── Step 3: Connect NATS (optional — graceful degradation) ────────────────
	var (
		cvePublisher *natspkg.CVEEventPublisher
		kevPublisher *fetcher.KEVPublisher
	)
	if os.Getenv("NATS_ENABLED") != "false" {
		nc, natsErr := natsgo.Connect(natsURL,
			natsgo.RetryOnFailedConnect(true),
			natsgo.MaxReconnects(3),
			natsgo.ReconnectWait(2*time.Second),
		)
		if natsErr != nil {
			log.Warn().Err(natsErr).Msg("NATS connect failed — running without event publishing")
		} else {
			js, jsErr := jetstream.New(nc)
			if jsErr != nil {
				log.Warn().Err(jsErr).Msg("JetStream init failed")
			} else {
				pub := natspkg.NewPublisher(js, log)
				cvePublisher = natspkg.NewCVEEventPublisher(pub)
			}

			kevPublisher, _ = fetcher.NewKEVPublisher(nc, log)
			log.Info().Str("url", natsURL).Msg("NATS connected")
		}
	}

	// ── Step 4: Build Fetcher Registry ───────────────────────────────────────
	reg := fetcher.NewRegistry()
	reg.Register(fetcher.WithCVEPublisher(
		fetcher.NewNVDCVEFetcher(db, nvdKey, 2002), cvePublisher, log))
	reg.Register(fetcher.NewCIRCLFetcher(db))
	reg.Register(fetcher.NewJVNFetcher(db))
	reg.Register(fetcher.NewExploitDBFetcher(db))
	reg.Register(fetcher.NewCVEOrgFetcher(db))
	reg.Register(fetcher.NewEPSSFetcher(db))
	reg.Register(fetcher.NewMITRECAPECFetcher(db))
	reg.Register(fetcher.NewMITRECWEFetcher(db))
	reg.Register(fetcher.NewNVDCPEFetcher(db, nvdKey))
	log.Info().Strs("fetchers", reg.Names()).Msg("Fetcher registry initialized")

	// ── Step 5: KEV Sync UseCase ─────────────────────────────────────────────
	cisaKEVClient := cisaclient.NewClient("") // official CISA endpoint
	kevRepo := postgres.NewKEVRepository(pgPool)
	syncUC := sync.New(cisaKEVClient, kevRepo, kevPublisher, log)

	// ── Step 6: Start Scheduler ──────────────────────────────────────────────
	sched := scheduler.NewWithRegistry(syncUC, reg, log)
	sched.Start()
	defer sched.Stop()
	log.Info().Msg("Scheduler started")

	// Startup sync: run immediately after boot
	if os.Getenv("STARTUP_SYNC_ENABLED") != "false" {
		sched.RunNow()                                        // KEV: immediate
		go sched.RunSourceNow(fetcher.SourceNVD.String(), 2) // NVD: last 2 days
		log.Info().Msg("Startup sync triggered (KEV + NVD incremental)")
	}

	// ── Step 7: HTTP Health + Admin Server ───────────────────────────────────
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"data-service","fetchers":%d}`, reg.Len()) //nolint:errcheck
	})
	mux.HandleFunc("/admin/sync/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		source := r.URL.Path[len("/admin/sync/"):]
		if source == "" {
			http.Error(w, `{"error":"source required"}`, http.StatusBadRequest)
			return
		}
		if source == "kev" || source == "KEV" {
			sched.RunNow()
		} else {
			sched.RunSourceNow(source, 7)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"triggered","source":%q}`, source) //nolint:errcheck
	})

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("data-service: listen :%d: %w", port, err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck

	log.Info().Int("http_port", port).Msg("data-service embedded ready")
	<-ctx.Done()
	return srv.Close()
}

// buildDSNFromEnv constructs a Postgres DSN from individual POSTGRES_* env vars.
func buildDSNFromEnv() string {
	host := envOr("POSTGRES_HOST", "localhost")
	port := envOr("POSTGRES_PORT", "5432")
	user := envOr("POSTGRES_USER", "osv")
	pass := envOr("POSTGRES_PASSWORD", "osv_dev")
	dbName := envOr("POSTGRES_DB", "osv")
	sslMode := envOr("POSTGRES_SSLMODE", "disable")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, pass, dbName, sslMode)
}

// firstNonEmpty returns the first non-empty string from the given values.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
