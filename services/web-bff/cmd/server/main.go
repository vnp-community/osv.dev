// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command server is the main entry point for the Web BFF (Enterprise Edition).
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
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/osv/web-bff/interface/http/handler"
	"github.com/osv/web-bff/interface/http/middleware"
	client "github.com/osv/web-bff/internal/infra/client"
	"github.com/osv/pkg/health"
	"github.com/osv/pkg/observability"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("web-bff: fatal error")
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── OTel ────────────────────────────────────────────────────────────
	zlog := zerolog.New(os.Stdout).With().Timestamp().Str("service", "web-bff").Logger()
	otelShutdown := observability.MustSetup("web-bff", "1.0.0", zlog)
	defer otelShutdown()

	// ── Redis ────────────────────────────────────────────────────────────
	redisAddr := envOrDefault("REDIS_ADDR", "localhost:6379")
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()

	// ── Downstream gRPC Clients (circuit-breaker backed) ────────────────────────
	queryAddr := envOrDefault("QUERY_SERVICE_ADDR", "vulnerability-query:50051")
	searchAddr := envOrDefault("SEARCH_SERVICE_ADDR", "search:50052")
	aiAddr := envOrDefault("AI_ENRICHMENT_ADDR", "ai-enrichment:50054")

	queryClient, err := client.NewVulnerabilityQueryClient(queryAddr, zlog)
	if err != nil {
		zlog.Warn().Err(err).Str("addr", queryAddr).Msg("vuln-query client init failed — using stub")
		queryClient = nil
	}

	searchClient, err := client.NewSearchServiceClient(searchAddr, zlog)
	if err != nil {
		zlog.Warn().Err(err).Str("addr", searchAddr).Msg("search client init failed — using stub")
		searchClient = nil
	}

	aiClient, err := client.NewAIEnrichmentClient(aiAddr, zlog)
	if err != nil {
		zlog.Warn().Err(err).Str("addr", aiAddr).Msg("ai-enrichment client init failed — using stub")
		aiClient = nil
	}

	// ── Handlers ────────────────────────────────────────────────────────────
	homepageHandler := handler.NewHomepageHandler(queryClient, nil /* stats cache */)
	vulnHandler := handler.NewVulnerabilityHandler(queryClient, aiClient)
	searchHandler := handler.NewSearchHandler(searchClient)
	linterHandler := &handler.LinterHandler{}
	healthHandler := handler.NewHealthHandler(queryClient)

	// ── Router ────────────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Global middleware chain.
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logging)
	r.Use(middleware.CORS)
	r.Use(middleware.RateLimit(redisClient, 30, time.Minute))

	// Routes.
	r.Get("/api/v1/stats", homepageHandler.GetStats)
	r.Get("/api/v1/search", searchHandler.Search)
	r.Get("/api/v1/search/autocomplete", searchHandler.Autocomplete)
	r.Get("/api/v1/vulns/{id}", vulnHandler.GetDetail)
	r.Post("/api/v1/lint", linterHandler.Lint)

	// 301 redirect: /{CVE-xxx} → /vulnerability/{CVE-xxx}
	r.Get("/{id:[A-Z]+-[0-9].*}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		http.Redirect(w, r, "/vulnerability/"+id, http.StatusMovedPermanently)
	})

	// ── Health endpoints (pkg/health MultiChecker) ───────────────────────────
	checker := health.NewMultiChecker(5*time.Second,
		health.RedisProber(redisClient),
	)
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		health.LiveHandler()(w, r)
	})
	r.Get("/health/ready", health.ReadyHandler(checker).ServeHTTP)

	port := envOrDefault("PORT", "8080")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Info().Str("addr", srv.Addr).Msg("web-bff: starting")

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("web-bff: http server error")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("web-bff: shutting down")

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("web-bff: http shutdown: %w", err)
	}
	return nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
