// Package kevservice is the KEV service goroutine.
package kevservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/config"
	"github.com/globalcve/mono/internal/kevservice/adapter/cisa"
	kevpostgres "github.com/globalcve/mono/internal/kevservice/adapter/postgres"
	"github.com/globalcve/mono/internal/kevservice/domain/entity"
	"github.com/globalcve/mono/internal/kevservice/usecase"
)

// Service is the KEV goroutine service.
type Service struct {
	cfg      config.Config
	syncUC   *usecase.SyncUseCase
	checkUC  *usecase.CheckUseCase
	queryUC  *usecase.QueryUseCase
	statsUC  *usecase.StatsUseCase
	cronSched *cron.Cron
	server   *http.Server
}

// New creates a new KEV service with all dependencies wired.
func New(cfg config.Config, pool *pgxpool.Pool) *Service {
	cisaClient := cisa.New(cfg.CISA.KEVURL, cfg.CISA.Timeout)
	kevRepo := kevpostgres.NewKEVRepo(pool)

	return &Service{
		cfg:     cfg,
		syncUC:  usecase.NewSyncUseCase(cisaClient, kevRepo),
		checkUC: usecase.NewCheckUseCase(kevRepo),
		queryUC: usecase.NewQueryUseCase(kevRepo),
		statsUC: usecase.NewStatsUseCase(kevRepo),
	}
}

// Start launches the KEV service goroutine.
func (s *Service) Start(ctx context.Context) error {
	// Cron scheduler for periodic KEV sync
	s.cronSched = cron.New(cron.WithSeconds())
	if s.cfg.Scheduler.KEVCron != "" {
		if _, err := s.cronSched.AddFunc(s.cfg.Scheduler.KEVCron, func() {
			syncCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
			defer cancel()
			if _, err := s.syncUC.Sync(syncCtx); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("kev: scheduled sync failed")
			}
		}); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("kev: failed to register cron")
		}
	}
	s.cronSched.Start()
	defer func() { <-s.cronSched.Stop().Done() }()

	// HTTP server
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/health", s.handleHealth)
	r.Route("/api/v2/kev", func(r chi.Router) {
		r.Get("/", s.handleList)
		r.Get("/check", s.handleCheck)
		r.Get("/stats", s.handleStats)
		r.Get("/{cveId}", s.handleGetByID)
	})
	r.Post("/internal/kev/sync", s.handleSync)
	r.Get("/internal/kev/ids", s.handleGetAllIDs)

	addr := fmt.Sprintf(":%d", s.cfg.Server.KEVServicePort)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
	}

	log.Ctx(ctx).Info().Str("addr", addr).Msg("kevservice: starting server")

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("kev server: %w", err)
	}
	return nil
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "kev"})
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := &entity.KEVFilter{
		VendorProject: q.Get("vendor"),
	}
	if p := q.Get("page"); p != "" {
		fmt.Sscan(p, &filter.Page)
	}
	if l := q.Get("limit"); l != "" {
		fmt.Sscan(l, &filter.Limit)
	}

	entries, total, err := s.queryUC.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"entries":  entries,
		"total":    total,
		"page":     filter.Page,
		"limit":    filter.Limit,
		"has_more": int64(filter.Page*filter.Limit+len(entries)) < total,
	})
}

func (s *Service) handleGetByID(w http.ResponseWriter, r *http.Request) {
	cveID := chi.URLParam(r, "cveId")
	entry, err := s.queryUC.GetByID(r.Context(), cveID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (s *Service) handleCheck(w http.ResponseWriter, r *http.Request) {
	idsStr := r.URL.Query().Get("ids")
	if idsStr == "" {
		writeError(w, http.StatusBadRequest, "ids parameter required")
		return
	}
	ids := strings.Split(idsStr, ",")
	for i, id := range ids {
		ids[i] = strings.TrimSpace(id)
	}

	results, err := s.checkUC.CheckMany(r.Context(), ids)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Service) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.statsUC.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Service) handleSync(w http.ResponseWriter, r *http.Request) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if _, err := s.syncUC.Sync(ctx); err != nil {
			log.Error().Err(err).Msg("kev: manual sync failed")
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "triggered"})
}

func (s *Service) handleGetAllIDs(w http.ResponseWriter, r *http.Request) {
	ids, err := s.queryUC.GetAllIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ids": ids, "total": len(ids)})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
