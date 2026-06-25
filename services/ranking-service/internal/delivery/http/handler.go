// Package http — HTTP handlers for ranking-service.
package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/osv/ranking-service/internal/domain"
)

// Handler provides HTTP handlers for ranking CRUD and lookup.
type Handler struct {
	repo domain.RankingRepository
}

// NewHandler creates a Handler.
func NewHandler(repo domain.RankingRepository) *Handler {
	return &Handler{repo: repo}
}

// LookupRanking handles: GET /ranking/lookup?cpe=...
//
// Returns ranking for a CPE string using loosy matching.
// Returns empty result (not 404) when no ranking found — to avoid breaking callers.
//
// @Summary Loosy CPE ranking lookup
// @Param cpe query string true "CPE string for lookup"
// @Success 200 {object} domain.LookupResult
// @Router /ranking/lookup [get]
func (h *Handler) LookupRanking(w http.ResponseWriter, r *http.Request) {
	cpe := r.URL.Query().Get("cpe")
	if cpe == "" {
		writeError(w, http.StatusBadRequest, "cpe query parameter is required")
		return
	}

	result, err := h.repo.FindByCPE(r.Context(), cpe)
	if err != nil {
		// Not found → return empty result (not error), to match cve-search behavior
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"cpe":          cpe,
			"ranks":        []interface{}{},
			"matched_part": "",
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ListRankings handles: GET /ranking?limit=&skip=
//
// @Summary List all ranking entries
// @Param limit query int false "Max results (default 100, max 1000)"
// @Param skip  query int false "Pagination offset (default 0)"
// @Success 200 {object} map[string]interface{}
// @Router /ranking [get]
func (h *Handler) ListRankings(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	skip, _ := strconv.Atoi(r.URL.Query().Get("skip"))

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	entries, total, err := h.repo.List(r.Context(), limit, skip)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rankings": entries,
		"total":    total,
		"limit":    limit,
		"skip":     skip,
	})
}

// CreateRanking handles: POST /ranking
//
// Body: {"cpe": "sap:netweaver", "rank": [{"group": "it", "rank": 3}]}
// Uses upsert — if CPE already exists, updates its ranks.
//
// @Summary Create or update a ranking entry
// @Accept  json
// @Success 201 {object} domain.RankingEntry
// @Router /ranking [post]
func (h *Handler) CreateRanking(w http.ResponseWriter, r *http.Request) {
	var entry domain.RankingEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	if err := entry.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	saved, err := h.repo.Save(r.Context(), &entry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "save failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

// DeleteRanking handles: DELETE /ranking/{id}
//
// @Summary Delete a ranking entry by ID
// @Param id path string true "MongoDB ObjectID hex"
// @Success 204
// @Failure 404 {object} map[string]string
// @Router /ranking/{id} [delete]
func (h *Handler) DeleteRanking(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(r.Context(), id); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			writeError(w, http.StatusNotFound, errMsg)
		} else if strings.Contains(errMsg, "invalid id") {
			writeError(w, http.StatusBadRequest, errMsg)
		} else {
			writeError(w, http.StatusInternalServerError, errMsg)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// BulkCreateRankings handles: POST /ranking/bulk
// SEED-004: Creates multiple ranking entries in one request.
// Returns 207 Multi-Status with per-entry results.
//
// @Summary Bulk create or update ranking entries
// @Accept  json
// @Success 207 {object} map[string]interface{}
// @Router /ranking/bulk [post]
func (h *Handler) BulkCreateRankings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entries []domain.RankingEntry `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if len(req.Entries) == 0 {
		writeError(w, http.StatusBadRequest, "entries list is required")
		return
	}
	if len(req.Entries) > 200 {
		writeError(w, http.StatusBadRequest, "bulk limit exceeded: max 200 entries per request")
		return
	}

	results := make([]map[string]interface{}, 0, len(req.Entries))
	createdCount := 0
	for _, entry := range req.Entries {
		if err := entry.Validate(); err != nil {
			results = append(results, map[string]interface{}{
				"cpe":     entry.CPE,
				"status":  "error",
				"message": err.Error(),
			})
			continue
		}

		saved, err := h.repo.Save(r.Context(), &entry)
		if err != nil {
			results = append(results, map[string]interface{}{
				"cpe":     entry.CPE,
				"status":  "error",
				"message": err.Error(),
			})
		} else {
			results = append(results, map[string]interface{}{
				"cpe":    saved.CPE,
				"status": "created",
				"id":     saved.ID,
			})
			createdCount++
		}
	}

	writeJSON(w, http.StatusMultiStatus, map[string]interface{}{
		"created_count": createdCount,
		"failed_count":  len(results) - createdCount,
		"results":       results,
	})
}


// ── Helpers ─────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
