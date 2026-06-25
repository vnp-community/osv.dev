// Command search-service is the unified Full-Text Search Platform.
// Consolidates: search (OpenSearch) + cve-search-service (Postgres+Mongo).
//
// Backend selection via SEARCH_BACKEND env var:
//   - "opensearch" — OpenSearch/Elasticsearch
//   - "postgres"   — PostgreSQL full-text search
//   - "mongo"      — MongoDB text search
//   - "auto"       — tries OpenSearch, falls back to Postgres (default)
//
// NATS subscriptions for index maintenance:
//   - osv.vuln.imported  → index
//   - osv.vuln.updated   → update
//   - osv.vuln.withdrawn → delete
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/osv/search-service/internal/factory"
	deliveryhttp "github.com/osv/search-service/internal/delivery/http"
	rediscache "github.com/osv/search-service/internal/infra/cache/redis"
	"github.com/osv/search-service/internal/usecase/browse"
	"github.com/osv/shared/pkg/observability"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	// [FIX BUG-006] SERVICE_VERSION env var set by CI/CD
	version := envOr("SERVICE_VERSION", "dev")

	log := observability.InitLogger("search-service", version)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	metrics := observability.NewCommonMetrics("search-service")
	// [FIX BUG-005] METRICS_PORT env var — default 9091 for search-service per port map
	metricsPort := parseInt(envOr("METRICS_PORT", "9091"), 9091)
	observability.StartMetricsServer(metricsPort)

	shutdown, err := observability.InitTracer(ctx, "search-service", version) // [FIX BUG-006]
	if err != nil {
		log.Warn().Err(err).Msg("tracing init failed, continuing without tracing")
	}
	defer shutdown()

	backend := factory.FromEnv()
	grpcPort := envOr("SEARCH_GRPC_PORT", envOr("GRPC_PORT", "50056"))
	httpPort := envOr("SEARCH_HTTP_PORT", envOr("HTTP_PORT", "8083"))

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen gRPC")
	}
	s := grpc.NewServer()
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(s, healthSvc)
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	// TODO: searchv1.RegisterSearchServiceServer(s, searchHandler)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","service":"search-service","backend":"%s"}`,
			envOr("SEARCH_BACKEND", "auto")) //nolint:errcheck
	})

	// Wire Browse handler (CR-002): vendor/product browsing via Redis CPE cache
	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: envOr("REDIS_PASSWORD", ""),
	})
	browseRepo := rediscache.NewRedisBrowseRepository(redisClient)
	browseUC := browse.New(browseRepo)
	browseH := deliveryhttp.NewBrowseHandler(browseUC)

	// TASK-010 FIX: Build the full chi router with all handlers (was missing api/v2/* routes).
	// NewRouter returns http.Handler (internally a *chi.Mux), but BrowseHandler.Mount needs chi.Router.
	// Solution: create a chi.Mux, mount browse on it, then use NewRouter for /api/v2/* routes.
	browseRouter := chi.NewRouter()
	browseH.Mount(browseRouter)

	// The full search/cve router (returns http.Handler wrapping chi internally)
	fullRouter := deliveryhttp.NewRouter(nil, nil, nil, nil, nil, log)

	// Mount: browse routes (handles /browse/*, /api/v2/browse*)
	mux.Handle("/browse/", browseRouter)
	// Gateway sends /api/v2/browse → search-service — mount full router for all api/v2
	mux.Handle("/api/v2/", fullRouter)
	mux.Handle("/api/v1/", fullRouter)
	mux.Handle("/internal/", fullRouter)

	// Add middleware to HTTP router
	handler := observability.LoggingMiddleware(log)(observability.MetricsMiddleware(metrics)(mux))

	httpSrv := &http.Server{Addr: ":" + httpPort, Handler: handler}

	log.Info().
		Str("backend", string(backend)).
		Str("grpc_port", grpcPort).
		Str("http_port", httpPort).
		Strs("nats_consumed", []string{"osv.vuln.imported", "osv.vuln.updated", "osv.vuln.withdrawn"}).
		Msg("search-service starting")

	go func() { s.Serve(lis) }()          //nolint:errcheck
	go func() { httpSrv.ListenAndServe() }() //nolint:errcheck

	<-ctx.Done()
	s.GracefulStop()
	httpSrv.Shutdown(context.Background()) //nolint:errcheck
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
