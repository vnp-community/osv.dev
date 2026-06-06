// Package http provides the HTTP delivery layer for the KEV service.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/globalcve/kev-service/internal/domain/entity"
	domainerrors "github.com/globalcve/kev-service/internal/domain/errors"
	"github.com/globalcve/kev-service/internal/domain/repository"
	"github.com/globalcve/kev-service/internal/usecase/check"
	"github.com/globalcve/kev-service/internal/usecase/query"
	"github.com/globalcve/kev-service/internal/usecase/sync"
)

// Handler holds all HTTP handlers for the KEV service.
type Handler struct {
	queryUC *query.UseCase
	checkUC *check.UseCase
	syncUC  *sync.UseCase
	kevRepo repository.KEVRepository
	log     zerolog.Logger
}

// NewHandler creates an HTTP Handler.
func NewHandler(
	queryUC *query.UseCase,
	checkUC *check.UseCase,
	syncUC *sync.UseCase,
	kevRepo repository.KEVRepository,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		queryUC: queryUC,
		checkUC: checkUC,
		syncUC:  syncUC,
		kevRepo: kevRepo,
		log:     log,
	}
}

// Health handles GET /health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "kev-service"})
}

// ListKEV handles GET /api/v2/kev.
func (h *Handler) ListKEV(w http.ResponseWriter, r *http.Request) {
	req := &query.Request{
		Query:         r.URL.Query().Get("query"),
		VendorProject: r.URL.Query().Get("vendor"),
		Page:          parseInt(r.URL.Query().Get("page"), 0),
		Limit:         parseInt(r.URL.Query().Get("limit"), 50),
	}
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			req.Since = &t
		}
	}

	resp, err := h.queryUC.Execute(r.Context(), req)
	if err != nil {
		h.log.Error().Err(err).Msg("list kev failed")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// GetKEVEntry handles GET /api/v2/kev/{cveId}.
func (h *Handler) GetKEVEntry(w http.ResponseWriter, r *http.Request) {
	cveID := chi.URLParam(r, "cveId")
	entry, err := h.kevRepo.FindByCVEID(r.Context(), cveID)
	if err == domainerrors.ErrKEVNotFound {
		respondError(w, http.StatusNotFound, "KEV entry not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, entry)
}

// BulkCheck handles GET /api/v2/kev/check?ids=CVE-1,CVE-2,...
func (h *Handler) BulkCheck(w http.ResponseWriter, r *http.Request) {
	idsParam := r.URL.Query().Get("ids")
	if idsParam == "" {
		respondError(w, http.StatusBadRequest, "ids parameter is required")
		return
	}
	ids := strings.Split(idsParam, ",")
	for i, id := range ids {
		ids[i] = strings.TrimSpace(id)
	}

	results, err := h.checkUC.CheckMany(r.Context(), ids)
	if err == domainerrors.ErrEmptyCVEIDs {
		respondError(w, http.StatusBadRequest, "no CVE IDs provided")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"results": results})
}

// GetStats handles GET /api/v2/kev/stats.
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.kevRepo.Stats(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

// TriggerSync handles POST /internal/kev/sync.
func (h *Handler) TriggerSync(w http.ResponseWriter, r *http.Request) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if result, err := h.syncUC.Sync(ctx); err != nil {
			h.log.Error().Err(err).Msg("manual sync failed")
		} else {
			h.log.Info().Int("inserted", result.Inserted).Int("updated", result.Updated).Msg("manual sync done")
		}
	}()
	respondJSON(w, http.StatusAccepted, map[string]string{"status": "sync started"})
}

// GetAllIDs handles GET /internal/kev/ids.
func (h *Handler) GetAllIDs(w http.ResponseWriter, r *http.Request) {
	ids, err := h.kevRepo.GetAllIDs(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ids": ids, "count": len(ids)})
}

// GetSyncStatus handles GET /api/v2/kev/sync/status.
func (h *Handler) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	count, err := h.kevRepo.Count(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"count":  count,
	})
}

// --- helpers ---

func respondJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func respondError(w http.ResponseWriter, code int, msg string) {
	respondJSON(w, code, map[string]string{"error": msg})
}

func parseInt(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

// ensure entity import is used (for unused import prevention in minimal builds)
var _ = (*entity.KEVEntry)(nil)
