// Package main is the entrypoint for the gateway-service.
// Combines api-gateway (OSV), dd-api-gateway (DefectDojo), web-bff and info-service
// into a single deployment unit.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/osv/shared/pkg/observability"

	"github.com/osv/gateway-service/internal/health"
)

func main() {
	// [FIX BUG-006] SERVICE_VERSION env var — set by CI/CD at build time or via k8s downward API
	// Falls back to "dev" instead of hardcoded "1.0.0" to make fallback obvious in logs
	version := envOrDefault("SERVICE_VERSION", "dev")

	log := observability.InitLogger("gateway-service", version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metrics := observability.NewCommonMetrics("gateway-service")
	// [FIX BUG-005] METRICS_PORT env var — default per port map (9090 for gateway)
	metricsPort := parseInt(envOrDefault("METRICS_PORT", "9090"), 9090)
	observability.StartMetricsServer(metricsPort)

	shutdown, err := observability.InitTracer(ctx, "gateway-service", version) // [FIX BUG-006]
	if err != nil {
		log.Warn().Err(err).Msg("tracing init failed, continuing without tracing")
	}
	defer shutdown()

	httpPort := envOrDefault("HTTP_PORT", "8080")
	grpcPort := envOrDefault("GRPC_PORT", "9090")

	r := chi.NewRouter()
	r.Use(observability.LoggingMiddleware(log))
	r.Use(observability.MetricsMiddleware(metrics))
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(cors.AllowAll().Handler)
	r.Use(chiMiddleware.Timeout(30 * time.Second))

	// Setup upstreams for health check
	upstreams := []health.UpstreamConfig{
		{Name: "data-service", URL: envOrDefault("DATA_SERVICE_HTTP", "http://data-service:8082")},
		{Name: "search-service", URL: envOrDefault("SEARCH_SERVICE_HTTP", "http://search-service:8081")},
		{Name: "notification-service", URL: envOrDefault("NOTIFICATION_SERVICE_HTTP", "http://notification-service:8084")},
	}
	healthUseCase := health.NewAggregateUseCase(upstreams, version) // [FIX BUG-006]
	aggHealthHandler := health.NewAggregateHandler(healthUseCase)

	// Health & info endpoints
	r.Get("/health", aggHealthHandler.ServeHTTP)
	r.Get("/ready",  aggHealthHandler.ServeHTTP)
	r.Get("/info",   health.HandleInfo)

	// OSV routes
	r.Mount("/v1", osvRouter())

	// DefectDojo routes
	r.Mount("/api/v2", ddRouter())

	httpSrv := &http.Server{
		Addr:         ":" + httpPort,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("port", httpPort).Msg("gateway-service HTTP listening")
		log.Info().Str("port", grpcPort).Msg("gateway-service gRPC proxy listening")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down gateway-service...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	httpSrv.Shutdown(shutCtx)
}

// osvRouter wires OSV (OpenVulnScan) routes.
func osvRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/vulns/{id}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "proxied to data-service:50051", http.StatusServiceUnavailable)
	})
	r.Get("/search", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "proxied to search-service:50056", http.StatusServiceUnavailable)
	})
	return r
}

// ddRouter wires DefectDojo routes.
func ddRouter() http.Handler {
	r := chi.NewRouter()
	r.Route("/findings", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "proxied to finding-service:50060", http.StatusServiceUnavailable)
		})
	})
	r.Route("/products", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "proxied to product-service:50061", http.StatusServiceUnavailable)
		})
	})
	return r
}

func envOrDefault(key, def string) string {
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
