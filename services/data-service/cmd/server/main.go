// Command data-service is the unified Vulnerability Data Platform.
// Consolidates: cve-service + kev-service + taxonomy-service + alias-relations.
//
// Exposed gRPC services (backward-compatible names retained):
//   - CVEService       (from cve-service)
//   - VulnerabilityService (new unified name)
//
// NATS subscriptions:
//   - osv.vuln.imported           → alias group detection
//   - osv.ai.enrichment.completed → alias embedding update
//
// NATS publications:
//   - osv.vuln.imported, osv.vuln.updated, osv.vuln.withdrawn
//   - osv.kev.updated
//
// FIX BUG-001: Wire MongoDB + Fetcher Registry + Scheduler (Sprint C implementation)
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/osv/shared/pkg/observability"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/jackc/pgx/v5/pgxpool"

	// [FIX TASK-HC-015] Wire CVEDBHandler use cases
	grpchandler "github.com/osv/data-service/internal/adapter/grpc/handler"
	pb "github.com/osv/shared/proto/gen/go/cvedb/v1"
	"github.com/osv/data-service/internal/usecase/backupdb"
	"github.com/osv/data-service/internal/usecase/exportdb"
	"github.com/osv/data-service/internal/usecase/importdb"
	"github.com/osv/data-service/internal/usecase/initdb"
	"github.com/osv/data-service/internal/usecase/lookupcves"
	"github.com/osv/data-service/internal/usecase/populatedb"

	"github.com/osv/data-service/internal/delivery/scheduler"
	"github.com/osv/data-service/internal/fetcher"
	cisaclient "github.com/osv/data-service/internal/infra/external/cisa"
	natspkg "github.com/osv/data-service/internal/infra/messaging/nats"
	"github.com/osv/data-service/internal/infra/persistence/postgres"
	"github.com/osv/data-service/internal/usecase/sync"
)

func main() {
	// [FIX BUG-006] SERVICE_VERSION env var set by CI/CD
	version := envOr("SERVICE_VERSION", "dev")

	log := observability.InitLogger("data-service", version)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	metrics := observability.NewCommonMetrics("data-service")
	// [FIX BUG-005] METRICS_PORT env var — default 9092 for data-service per port map
	metricsPort := parseInt(envOr("METRICS_PORT", "9092"), 9092)
	observability.StartMetricsServer(metricsPort)

	shutdown, err := observability.InitTracer(ctx, "data-service", version) // [FIX BUG-006]
	if err != nil {
		log.Warn().Err(err).Msg("tracing init failed, continuing without tracing")
	}
	defer shutdown()

	grpcPort := envOr("DATA_GRPC_PORT", envOr("GRPC_PORT", "50053"))
	httpPort := envOr("DATA_HTTP_PORT", envOr("HTTP_PORT", "8082"))

	// ── Step 1: Connect MongoDB (required — fatal on failure) ─────────────────
	mongoURI := envOr("MONGO_URI", "mongodb://localhost:27017")
	mongoDB := envOr("MONGO_DB", "cvedb")

	mongoClient, err := mongo.Connect(ctx,
		mongoopts.Client().ApplyURI(mongoURI).SetServerSelectionTimeout(10*time.Second),
	)
	if err != nil {
		log.Fatal().Err(err).Str("uri", mongoURI).Msg("MongoDB connect failed")
	}
	defer mongoClient.Disconnect(context.Background()) //nolint:errcheck

	if err := mongoClient.Ping(ctx, nil); err != nil {
		log.Fatal().Err(err).Msg("MongoDB ping failed")
	}
	db := mongoClient.Database(mongoDB)
	log.Info().Str("db", mongoDB).Msg("MongoDB connected")

	// ── Step 2: Connect PostgreSQL (for KEV repo — required) ──────────────────
	postgresDSN := envOr("POSTGRES_DSN", buildPostgresDSN())
	pgPool, err := pgxpool.New(ctx, postgresDSN)
	if err != nil {
		log.Fatal().Err(err).Msg("PostgreSQL connect failed")
	}
	defer pgPool.Close()

	if err := pgPool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("PostgreSQL ping failed")
	}
	log.Info().Msg("PostgreSQL connected")

	// ── Step 3: Connect NATS (optional — graceful degradation) ────────────────
	natsURL := envOr("NATS_URL", "nats://localhost:4222")
	natsEnabled := envOr("NATS_ENABLED", "true") == "true"

	var (
		nc           *natsgo.Conn
		cvePublisher *natspkg.CVEEventPublisher
		kevPublisher *fetcher.KEVPublisher
	)
	if natsEnabled {
		nc, err = natsgo.Connect(natsURL,
			natsgo.RetryOnFailedConnect(true),
			natsgo.MaxReconnects(3),
			natsgo.ReconnectWait(2*time.Second),
		)
		if err != nil {
			log.Warn().Err(err).Str("url", natsURL).Msg("NATS connect failed — continuing without NATS")
		} else {
			js, jsErr := jetstream.New(nc)
			if jsErr != nil {
				log.Warn().Err(jsErr).Msg("JetStream init failed")
			} else {
				pub := natspkg.NewPublisher(js, log)
				cvePublisher = natspkg.NewCVEEventPublisher(pub)
			}

			// KEV publisher uses nats.go v1 JetStream context
			kevPublisher, err = fetcher.NewKEVPublisher(nc, log)
			if err != nil {
				log.Warn().Err(err).Msg("KEV NATS publisher init failed — KEV events disabled")
			}

			log.Info().Str("url", natsURL).Msg("NATS connected")
		}
	}

	// ── Step 4: Build Fetcher Registry ───────────────────────────────────────
	nvdAPIKey := envOr("NVD_API_KEY", "")

	reg := fetcher.NewRegistry()
	// Core CVE fetchers
	nvdFetcher := fetcher.NewNVDCVEFetcher(db, nvdAPIKey, 2002)
	reg.Register(fetcher.WithCVEPublisher(nvdFetcher, cvePublisher, log))
	reg.Register(fetcher.NewCIRCLFetcher(db))
	reg.Register(fetcher.NewJVNFetcher(db))
	reg.Register(fetcher.NewExploitDBFetcher(db))
	reg.Register(fetcher.NewCVEOrgFetcher(db))
	// Enrichment fetchers
	reg.Register(fetcher.NewEPSSFetcher(db))
	reg.Register(fetcher.NewMITRECAPECFetcher(db))
	reg.Register(fetcher.NewMITRECWEFetcher(db))
	reg.Register(fetcher.NewNVDCPEFetcher(db, nvdAPIKey))

	log.Info().Strs("fetchers", reg.Names()).Msg("Fetcher registry initialized")

	// ── Step 5: Init KEV Sync UseCase ────────────────────────────────────────
	cisaKEVClient := cisaclient.NewClient("") // uses official CISA endpoint
	kevRepo := postgres.NewKEVRepository(pgPool)
	syncUC := sync.New(cisaKEVClient, kevRepo, kevPublisher, log)

	// ── Step 6: Start Scheduler ──────────────────────────────────────────────
	sched := scheduler.NewWithRegistry(syncUC, reg, log)
	sched.Start()
	defer sched.Stop()
	log.Info().Msg("Scheduler started — all fetchers are scheduled")

	// ── Step 7: Startup sync — run immediately on boot ────────────────────────
	if envOr("STARTUP_SYNC_ENABLED", "true") == "true" {
		log.Info().Msg("Triggering startup sync (KEV + NVD incremental)")
		sched.RunNow()                                        // KEV: immediate
		go sched.RunSourceNow(fetcher.SourceNVD.String(), 2) // NVD: last 2 days
	}

	// ── gRPC Server ──────────────────────────────────────────────────────────
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen gRPC")
	}

	s := grpc.NewServer()
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(s, healthSvc)
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// [FIX TASK-HC-015] Register CVEDBHandler with real PostgreSQL-backed use cases
	cvedbRepo := postgres.NewCVEBinToolRepo(pgPool)
	exploitRepo := postgres.NewExploitRepo(pgPool)
	metricRepo := postgres.NewMetricRepo(pgPool)
	purl2cpeRepo := postgres.NewPURL2CPERepo(pgPool)
	dbAdminRepo := postgres.NewDBAdminRepo(pgPool)

	lookupUC := lookupcves.New(cvedbRepo, exploitRepo, metricRepo)
	populateUC := populatedb.New(cvedbRepo, metricRepo, purl2cpeRepo)
	initUC := initdb.New(dbAdminRepo)
	importUC := importdb.New(dbAdminRepo)
	exportUC := exportdb.New(dbAdminRepo)
	backupUC := backupdb.New(dbAdminRepo)

	cvedbHandler := grpchandler.New(lookupUC, populateUC, initUC, importUC, exportUC, backupUC, log)
	pb.RegisterCVEDBServiceServer(s, cvedbHandler)
	healthSvc.SetServingStatus("cvedb.v1.CVEDBService", healthpb.HealthCheckResponse_SERVING)
	log.Info().Msg("[FIX TASK-HC-015] CVEDBService registered on gRPC server")

	log.Info().
		Str("grpc_port", grpcPort).
		Str("http_port", httpPort).
		Strs("nats_consumed", []string{"osv.vuln.imported", "osv.ai.enrichment.completed"}).
		Msg("data-service starting")

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC serve failed")
		}
	}()

	// ── HTTP Server (health + admin endpoints) ─────────────────────────────────
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","service":"data-service","grpc_port":"%s","http_port":"%s","fetchers":%d}`, //nolint:errcheck
			grpcPort, httpPort, reg.Len())
	})

	// Admin: trigger manual sync — POST /admin/sync/{source}  e.g. POST /admin/sync/NVD
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
			sched.RunSourceNow(source, 7) // default: last 7 days
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"triggered","source":%q}`, source) //nolint:errcheck
	})

	handler := observability.LoggingMiddleware(log)(observability.MetricsMiddleware(metrics)(mux))
	httpSrv := &http.Server{Addr: ":" + httpPort, Handler: handler}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP serve failed")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down data-service")
	s.GracefulStop()
	httpSrv.Shutdown(context.Background()) //nolint:errcheck
}

// buildPostgresDSN constructs a DSN from individual POSTGRES_* env vars.
// Fallback if POSTGRES_DSN is not set directly.
func buildPostgresDSN() string {
	host := envOr("POSTGRES_HOST", "localhost")
	port := envOr("POSTGRES_PORT", "5432")
	user := envOr("POSTGRES_USER", "osv")
	pass := envOr("POSTGRES_PASSWORD", "osv_dev")
	dbName := envOr("POSTGRES_DB", "osv")
	sslMode := envOr("POSTGRES_SSLMODE", "disable")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, pass, dbName, sslMode)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseInt parses a string as int, returning def on error.
func parseInt(s string, def int) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n > 0 {
		return n
	}
	return def
}
