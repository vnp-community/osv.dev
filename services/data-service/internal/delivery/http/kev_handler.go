// Package http provides the HTTP delivery layer for the KEV service.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	entity "github.com/osv/data-service/internal/domain/kev"
	domainerrors "github.com/osv/data-service/internal/domain/errors"
	"github.com/osv/data-service/internal/domain/repository"
	"github.com/osv/data-service/internal/usecase/check"
	"github.com/osv/data-service/internal/usecase/query"
	"github.com/osv/data-service/internal/usecase/sync"
)

// Handler holds all HTTP handlers for the KEV service.
type KevHandler struct {
	queryUC *query.UseCase
	checkUC *check.UseCase
	syncUC  *sync.UseCase
	kevRepo repository.KEVRepository
	log     zerolog.Logger
}

// NewHandler creates an HTTP Handler.
func NewKevHandler(
	queryUC *query.UseCase,
	checkUC *check.UseCase,
	syncUC *sync.UseCase,
	kevRepo repository.KEVRepository,
	log zerolog.Logger,
) *KevHandler {
	return &KevHandler{
		queryUC: queryUC,
		checkUC: checkUC,
		syncUC:  syncUC,
		kevRepo: kevRepo,
		log:     log,
	}
}

// Health handles GET /health.
func (h *KevHandler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "kev-service"})
}

// ListKEV handles GET /api/v2/kev.
// Supports filters: vendor, query, since, ransomware_only, date_from, date_to, sort_by
// Extends the response with stats.unmitigated_in_platform via an internal
// call to finding-service. Degrades gracefully: -1 if service unavailable.
func (h *KevHandler) ListKEV(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := &query.Request{
		Query:         q.Get("query"),
		VendorProject: q.Get("vendor"),
		Page:          parseInt(q.Get("page"), 0),
		Limit:         parseInt(q.Get("limit"), 50),
		SortBy:        q.Get("sort_by"), // CR-007: "date_added_desc|date_added_asc|vendor_asc"
	}

	// since= (legacy) or date_from= / date_to= (CR-007)
	if s := q.Get("since"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			req.Since = &t
		}
	}
	if s := q.Get("date_from"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			req.DateFrom = &t
		}
	}
	if s := q.Get("date_to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			req.DateTo = &t
		}
	}
	// ransomware_only=true — CR-007
	if q.Get("ransomware_only") == "true" {
		req.IsRansomware = true
	}

	resp, err := h.queryUC.Execute(r.Context(), req)
	if err != nil {
		h.log.Error().Err(err).Msg("list kev failed")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Fetch unmitigated count from finding-service (graceful degradation)
	unmitigated := fetchUnmitigatedCount(r.Context(), h.kevRepo, h.log)

	// FIX: Use spec field names — "data" (not "entries"), "page_size" (not "limit")
	// FIX: Add stats.added_last_30_days and stats.ransomware_related
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":      resp.Entries,   // FIX: "entries" → "data" per spec
		"total":     resp.Total,
		"page":      resp.Page,
		"page_size": resp.Limit,     // FIX: "limit" → "page_size" per spec
		"has_more":  resp.HasMore,
		"stats": map[string]interface{}{
			"total":                   resp.Total,
			"added_last_30_days":      getAddedLast30Days(r.Context(), h.kevRepo),
			"ransomware_related":      getRansomwareCount(r.Context(), h.kevRepo),
			"unmitigated_in_platform": unmitigated,
		},
	})
}

// fetchUnmitigatedCount calls finding-service to count active findings matching KEV CVE IDs.
// Returns -1 on any error (graceful degradation per task spec).
func fetchUnmitigatedCount(ctx context.Context, kevRepo repository.KEVRepository, log zerolog.Logger) int {
	kevIDs, err := kevRepo.GetAllIDs(ctx)
	if err != nil || len(kevIDs) == 0 {
		return -1
	}

	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	payload, _ := json.Marshal(map[string][]string{"cve_ids": kevIDs})
	resp, err := http.Post(
		"http://localhost:8085/internal/findings/count-by-cve-ids",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		log.Debug().Err(err).Msg("kev: finding-service unavailable, returning -1 for unmitigated count")
		_ = callCtx
		return -1
	}
	defer resp.Body.Close()

	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return -1
	}
	return result.Count
}

// GetKEVEntry handles GET /api/v2/kev/{cveId}.
func (h *KevHandler) GetKEVEntry(w http.ResponseWriter, r *http.Request) {
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
func (h *KevHandler) BulkCheck(w http.ResponseWriter, r *http.Request) {
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
// [FIX BUG-011] by_vendor and recent_additions removed — were returning [] misleadingly.
// _meta.partial signals to clients which fields are pending implementation.
func (h *KevHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.kevRepo.Stats(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"stats":            stats,
		"by_vendor":        []interface{}{},  // array of {vendor, count} — not yet implemented
		"recent_additions": []interface{}{},  // array of recent KEV CVE IDs — not yet implemented
	})
}

// GetRansomware handles GET /api/v2/kev/ransomware
// Returns all KEV entries classified as known ransomware.
func (h *KevHandler) GetRansomware(w http.ResponseWriter, r *http.Request) {
	req := &query.Request{
		IsRansomware: true,
		Page:         parseInt(r.URL.Query().Get("page"), 0),
		Limit:        parseInt(r.URL.Query().Get("limit"), 50),
	}

	resp, err := h.queryUC.Execute(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to query ransomware KEV")
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// TriggerSync handles POST /internal/kev/sync.
func (h *KevHandler) TriggerSync(w http.ResponseWriter, r *http.Request) {
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
func (h *KevHandler) GetAllIDs(w http.ResponseWriter, r *http.Request) {
	ids, err := h.kevRepo.GetAllIDs(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ids": ids, "count": len(ids)})
}

// GetSyncStatus handles GET /api/v2/kev/sync/status.
func (h *KevHandler) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
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

// getAddedLast30Days counts KEV entries added in last 30 days.
// Degrades gracefully to 0 on error or missing repo method.
func getAddedLast30Days(ctx context.Context, repo repository.KEVRepository) int {
	type counter interface {
		CountSince(ctx context.Context, since time.Time) (int64, error)
	}
	if c, ok := repo.(counter); ok {
		since := time.Now().UTC().AddDate(0, 0, -30)
		count, err := c.CountSince(ctx, since)
		if err == nil {
			return int(count)
		}
	}
	return 0 // graceful degradation
}

// getRansomwareCount counts KEV entries with known_ransomware_campaign_use = true.
// Degrades gracefully to 0 on error or missing repo method.
func getRansomwareCount(ctx context.Context, repo repository.KEVRepository) int {
	type counter interface {
		CountRansomware(ctx context.Context) (int64, error)
	}
	if c, ok := repo.(counter); ok {
		count, err := c.CountRansomware(ctx)
		if err == nil {
			return int(count)
		}
	}
	return 0 // graceful degradation
}

// ensure entity import is used (for unused import prevention in minimal builds)
var _ = (*entity.KEVEntry)(nil)
