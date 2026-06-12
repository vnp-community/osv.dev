// Package cvesync is the CVE Sync service goroutine.
// It manages background data synchronization from all CVE sources.
package cvesync

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/config"
	syncpostgres "github.com/globalcve/mono/internal/cvesync/adapter/postgres"
	"github.com/globalcve/mono/internal/cvesync/domain/entity"
	"github.com/globalcve/mono/internal/cvesync/fetcher"
	"github.com/globalcve/mono/internal/cvesync/scheduler"
	"github.com/globalcve/mono/internal/cvesync/usecase"
)

// Service is the CVE Sync goroutine service.
type Service struct {
	cfg          config.Config
	pool         *pgxpool.Pool
	orchestrator *usecase.Orchestrator
	scheduler    *scheduler.Scheduler
	server       *http.Server
}

// New creates a new CVE Sync service with all dependencies wired.
func New(cfg config.Config, pool *pgxpool.Pool) *Service {
	// Repositories
	cveRepo := syncpostgres.NewCVERepo(pool)
	syncRepo := syncpostgres.NewSyncRepo(pool)

	// Build fetchers
	fetchers := buildFetchers(cfg, cveRepo)

	// Orchestrator
	orch := usecase.NewOrchestrator(fetchers, syncRepo)

	// Scheduler
	sched := scheduler.New(orch, cfg.Scheduler)

	return &Service{
		cfg:          cfg,
		pool:         pool,
		orchestrator: orch,
		scheduler:    sched,
	}
}

// Start launches the CVE Sync service goroutine.
// It starts the scheduler and an internal admin HTTP server.
func (s *Service) Start(ctx context.Context) error {
	// Start cron scheduler
	s.scheduler.Start(ctx)
	defer s.scheduler.Stop()

	// Internal admin HTTP server for status/trigger endpoints
	r := chi.NewRouter()
	r.Get("/health", s.handleHealth)
	r.Get("/api/v2/sync/status", s.handleStatus)
	r.Post("/api/v2/sync/trigger", s.handleTrigger)
	r.Post("/api/v2/sync/trigger/{source}", s.handleTriggerSource)

	addr := fmt.Sprintf(":%d", s.cfg.Server.CVESyncPort)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
	}

	log.Ctx(ctx).Info().Str("addr", addr).Msg("cvesync: starting admin server")

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("cvesync server: %w", err)
	}
	return nil
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","service":"cve-sync"}`))
}

func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	// TODO: query sync job status from DB
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"running"}`))
}

func (s *Service) handleTrigger(w http.ResponseWriter, r *http.Request) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		results, err := s.orchestrator.SyncAll(ctx)
		if err != nil {
			log.Error().Err(err).Msg("manual sync all failed")
			return
		}
		for _, res := range results {
			log.Info().Str("source", string(res.Source)).Int("synced", res.Synced).Msg("manual sync result")
		}
	}()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"triggered"}`))
}

func (s *Service) handleTriggerSource(w http.ResponseWriter, r *http.Request) {
	sourceStr := chi.URLParam(r, "source")
	source := entity.SourceName(sourceStr)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		result, err := s.orchestrator.SyncSource(ctx, source)
		if err != nil {
			log.Error().Err(err).Str("source", sourceStr).Msg("manual sync failed")
			return
		}
		log.Info().Str("source", string(result.Source)).Int("synced", result.Synced).Msg("manual sync result")
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"triggered","source":"` + sourceStr + `"}`))
}

// buildFetchers constructs all enabled fetchers.
func buildFetchers(cfg config.Config, cveRepo *syncpostgres.CVERepo) []fetcher.Fetcher {
	var fetchers []fetcher.Fetcher

	fetchers = append(fetchers,
		fetcher.NewNVDFetcher(cfg.Sources.NVD.APIKEY, cfg.Sources.NVD.Timeout, cveRepo),
		fetcher.NewCIRCLFetcher(cfg.Sources.CIRCL.BaseURL, cfg.Sources.CIRCL.Timeout, cveRepo),
		fetcher.NewJVNFetcher(cfg.Sources.JVN.FeedURL, cfg.Sources.JVN.Timeout, cveRepo),
		fetcher.NewExploitDBFetcher(cfg.Sources.ExploitDB.CSVURL, cfg.Sources.ExploitDB.Timeout, cveRepo),
		fetcher.NewCVEOrgFetcher(cfg.Sources.CVEOrg.ReleaseURL, cfg.Sources.CVEOrg.Timeout, cveRepo),
		fetcher.NewEPSSFetcher(cfg.Sources.EPSS.URL, cfg.Sources.EPSS.Timeout, cveRepo),
	)

	return fetchers
}
