// Package handler provides the search HTTP API for the DefectDojo search service.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/defectdojo/search/internal/infrastructure/elasticsearch"
)

// SearchHandler handles GET /api/v2/search.
type SearchHandler struct {
	indexer *elasticsearch.Indexer
}

func New(indexer *elasticsearch.Indexer) *SearchHandler {
	return &SearchHandler{indexer: indexer}
}

// HandleSearch processes full-text search queries across findings, products, and engagements.
// Query params: q=<query>&type=finding|product|engagement&severity=Critical|High|Medium|Low|Info&limit=25&offset=0
func (h *SearchHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "q parameter is required"})
		return
	}

	limit := 25
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o >= 0 {
		offset = o
	}

	results, err := h.indexer.Search(r.Context(), elasticsearch.SearchQuery{
		Query:    q,
		Type:     r.URL.Query().Get("type"),
		Severity: r.URL.Query().Get("severity"),
		UserID:   r.Header.Get("X-User-ID"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":   results.Total,
		"results": results.Hits,
	})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
