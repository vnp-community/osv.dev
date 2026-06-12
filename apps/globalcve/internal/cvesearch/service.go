// Package cvesearch is the CVE Search service goroutine.
package cvesearch

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/config"
	searchpostgres "github.com/globalcve/mono/internal/cvesearch/adapter/postgres"
	searchredis "github.com/globalcve/mono/internal/cvesearch/adapter/redis"
	cvesearchhttp "github.com/globalcve/mono/internal/cvesearch/http"
	"github.com/globalcve/mono/internal/cvesearch/usecase"
)

// Service is the CVE Search goroutine service.
type Service struct {
	cfg     config.Config
	handler *cvesearchhttp.Handler
	server  *http.Server
}

// New creates a new CVE Search service.
func New(cfg config.Config, pool *pgxpool.Pool, redis *goredis.Client) *Service {
	// Repositories
	cveRepo := searchpostgres.NewCVEReadRepo(pool)
	cacheRepo := searchredis.NewCVECacheRepo(redis)

	// Use cases
	searchUC := usecase.NewSearchUseCase(cveRepo, cacheRepo, cfg.Cache.SearchTTL)
	getByIDUC := usecase.NewGetByIDUseCase(cveRepo, cacheRepo, cfg.Cache.SingleTTL)

	handler := cvesearchhttp.NewHandler(searchUC, getByIDUC)

	return &Service{
		cfg:     cfg,
		handler: handler,
	}
}

// Handler returns the HTTP handler (used by API Gateway for direct routing).
func (s *Service) Handler() *cvesearchhttp.Handler {
	return s.handler
}

// Start launches the CVE Search service HTTP server.
func (s *Service) Start(ctx context.Context) error {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/health", s.handler.Health)
	r.Route("/api/v2", func(r chi.Router) {
		r.Get("/cves", s.handler.SearchCVEs)
		r.Get("/cves/{id}", s.handler.GetCVE)
	})
	r.Get("/internal/cves/count", s.handler.Count)

	addr := fmt.Sprintf(":%d", s.cfg.Server.CVESearchPort)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
	}

	log.Ctx(ctx).Info().Str("addr", addr).Msg("cvesearch: starting server")

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("cvesearch server: %w", err)
	}
	return nil
}
