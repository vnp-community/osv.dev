// Package http — cwe_handler.go (data-service)
// Provides CWE list endpoint with search: GET /api/v2/cwe?q=sql&page=1
// This is the searchable list endpoint; individual GET /api/v2/cwe/{id}
// is served by the existing taxonomy handler (MongoDB).
package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)


// CWERepository provides searchable CWE list queries from Postgres.
type CWERepository interface {
	List(ctx context.Context, query string, page, pageSize int) ([]CWEItem, int64, error)
	GetByID(ctx context.Context, cweID string) (*CWEItem, error) // P1-06: detail by CWE-ID
}


// CWEItem is a CWE entry with related CVE count.
type CWEItem struct {
	CWEID           string `json:"cwe_id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	RelatedCVECount int64  `json:"related_cve_count"`
}

// CWEHandler handles CWE list endpoints.
type CWEHandler struct {
	cweRepo CWERepository
}

// NewCWEHandler creates a new CWEHandler.
func NewCWEHandler(repo CWERepository) *CWEHandler {
	return &CWEHandler{cweRepo: repo}
}

// GET /api/v2/cwe?q=sql+injection&page=1&page_size=20
func (h *CWEHandler) ListCWE(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	page := 1
	pageSize := 20

	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && ps > 0 {
		if ps > 200 {
			ps = 200
		}
		pageSize = ps
	}

	list, total, err := h.cweRepo.List(r.Context(), q, page, pageSize)
	if err != nil {
		writeJSONData(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// FIX: never return null for list — JSON must be [] not null
	if list == nil {
		list = make([]CWEItem, 0)
	}

	writeJSONData(w, http.StatusOK, map[string]interface{}{
		"cwe_list":  list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GET /api/v2/cwe/{id} — P1-06: detail by CWE-ID (e.g. CWE-79)
func (h *CWEHandler) GetCWEByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONData(w, http.StatusBadRequest, map[string]string{"error": "cwe_id required"})
		return
	}

	item, err := h.cweRepo.GetByID(r.Context(), id)
	if err != nil || item == nil {
		writeJSONData(w, http.StatusNotFound, map[string]string{"error": "CWE not found", "cwe_id": id})
		return
	}
	writeJSONData(w, http.StatusOK, item)
}
