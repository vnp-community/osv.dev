// Package http — CVE Search HTTP handler.
package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/globalcve/mono/internal/cvesearch/usecase"
)

// Handler holds all CVE search HTTP handlers.
type Handler struct {
	searchUC  *usecase.SearchUseCase
	getByIDUC *usecase.GetByIDUseCase
}

// NewHandler creates a new CVE search HTTP handler.
func NewHandler(searchUC *usecase.SearchUseCase, getByIDUC *usecase.GetByIDUseCase) *Handler {
	return &Handler{searchUC: searchUC, getByIDUC: getByIDUC}
}

// Health returns the service health status.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "cve-search"})
}

// SearchCVEs handles GET /api/v2/cves
func (h *Handler) SearchCVEs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	req := usecase.SearchRequest{
		Query:    q.Get("query"),
		Severity: q.Get("severity"),
		Source:   q.Get("source"),
		Sort:     q.Get("sort"),
	}

	// Page and limit
	req.Page, _ = strconv.Atoi(q.Get("page"))
	req.Limit, _ = strconv.Atoi(q.Get("limit"))

	// KEV filter
	if kevStr := q.Get("kev"); kevStr != "" {
		kev := kevStr == "true"
		req.IsKEV = &kev
	}

	// EPSS filter
	if epssStr := q.Get("min_epss"); epssStr != "" {
		if epss, err := strconv.ParseFloat(epssStr, 64); err == nil {
			req.MinEPSS = &epss
		}
	}

	resp, err := h.searchUC.Execute(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, buildSearchResponseDTO(resp))
}

// GetCVE handles GET /api/v2/cves/{id}
func (h *Handler) GetCVE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cve, err := h.getByIDUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cve)
}

// Count handles GET /internal/cves/count (internal endpoint)
func (h *Handler) Count(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- response DTOs ---

// SearchResponseDTO is the JSON envelope returned by the search endpoint.
type SearchResponseDTO struct {
	Query   string      `json:"query"`
	Total   int64       `json:"total"`
	Page    int         `json:"page"`
	Limit   int         `json:"limit"`
	HasMore bool        `json:"has_more"`
	Results interface{} `json:"results"`
	Cached  bool        `json:"cached"`
	TookMs  int64       `json:"took_ms"`
}

func buildSearchResponseDTO(resp *usecase.SearchResponse) SearchResponseDTO {
	return SearchResponseDTO{
		Query:   resp.Query,
		Total:   resp.Total,
		Page:    resp.Page,
		Limit:   resp.Limit,
		HasMore: resp.HasMore,
		Results: resp.CVEs,
		Cached:  resp.FromCache,
		TookMs:  resp.TookMs,
	}
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
