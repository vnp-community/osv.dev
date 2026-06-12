// Package main is the entrypoint for the gateway-service.
// Combines api-gateway (OSV), dd-api-gateway (DefectDojo), web-bff and info-service
// into a single deployment unit.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/osv/gateway-service/internal/health"
)

func main() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	httpPort := envOrDefault("HTTP_PORT", "8080")
	grpcPort := envOrDefault("GRPC_PORT", "9090")

	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(cors.AllowAll().Handler)
	r.Use(chiMiddleware.Timeout(30 * time.Second))

	// Health & info endpoints
	r.Get("/health", health.HandleHealth)
	r.Get("/ready",  health.HandleHealth)
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
