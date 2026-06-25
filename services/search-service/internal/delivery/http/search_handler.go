// Package http provides the HTTP delivery layer for the CVE search service.
package http

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	domainerrors "github.com/osv/search-service/internal/domain/errors"
	"github.com/osv/search-service/internal/domain/repository"
	"github.com/osv/search-service/internal/infra/opensearch"
	"github.com/osv/search-service/internal/infra/pgvector"
	"github.com/osv/search-service/internal/infra/postgres"
	search "github.com/osv/search-service/internal/usecase/cvesearch"
	"github.com/osv/search-service/internal/usecase/getbyid"
)

// Handler holds HTTP handlers for the CVE search service.
type Handler struct {
	searchUC    *search.UseCase
	getByIDUC   *getbyid.UseCase
	semanticUC  *pgvector.UseCase
	osClient    *opensearch.Client
	cveRepo     repository.CVERepository
	// [FIX TASK-HC-007] historyRepo persists search queries; nil = skip (backward-compat)
	historyRepo *postgres.SearchHistoryRepo
	log         zerolog.Logger
}

// NewHandler creates an HTTP Handler.
func NewHandler(searchUC *search.UseCase, getByIDUC *getbyid.UseCase, semanticUC *pgvector.UseCase, osClient *opensearch.Client, cveRepo repository.CVERepository, log zerolog.Logger) *Handler {
	return &Handler{
		searchUC:   searchUC,
		getByIDUC:  getByIDUC,
		semanticUC: semanticUC,
		osClient:   osClient,
		cveRepo:    cveRepo,
		log:        log,
	}
}

// WithHistoryRepo enables search history persistence.
// Call from embedded.go after creating the handler.
func (h *Handler) WithHistoryRepo(repo *postgres.SearchHistoryRepo) *Handler {
	h.historyRepo = repo
	return h
}

// Health handles GET /health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "cve-search-service"})
}

// SearchCVEs handles GET /api/v2/cves.
func (h *Handler) SearchCVEs(w http.ResponseWriter, r *http.Request) {
	req := &search.Request{
		Query:    r.URL.Query().Get("query"),
		Severity: strings.ToUpper(r.URL.Query().Get("severity")),
		Source:   strings.ToUpper(r.URL.Query().Get("source")),
		Sort:     r.URL.Query().Get("sort"),
		Page:     parseInt(r.URL.Query().Get("page"), 0),
		Limit:    parseInt(r.URL.Query().Get("limit"), 50),
		// CR-GCV-002: EPSS filters
		CWE:     r.URL.Query().Get("cwe"),
		Vendor:  strings.ToLower(r.URL.Query().Get("vendor")),
		Product: strings.ToLower(r.URL.Query().Get("product")),
	}

	// Parse min_epss
	if v := r.URL.Query().Get("min_epss"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			req.MinEPSS = &f
		}
	}
	// Parse max_epss
	if v := r.URL.Query().Get("max_epss"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			req.MaxEPSS = &f
		}
	}
	// Parse is_kev (CR-GCV-003)
	if v := r.URL.Query().Get("is_kev"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			req.IsKEV = &b
		}
	}
	// Parse is_exploit
	if v := r.URL.Query().Get("is_exploit"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			req.IsExploit = &b
		}
	}

	resp, err := h.searchUC.Execute(r.Context(), req)
	if err != nil {
		h.log.Error().Err(err).Msg("search failed")
		respondError(w, http.StatusInternalServerError, "search failed")
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

const maxExportRows = 10000

// ExportCVEs handles GET /api/v2/cves/export.
func (h *Handler) ExportCVEs(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	if format != "csv" && format != "json" {
		respondError(w, http.StatusBadRequest, "format must be 'csv' or 'json'")
		return
	}

	req := &search.Request{
		Query:    r.URL.Query().Get("query"),
		Severity: strings.ToUpper(r.URL.Query().Get("severity")),
		Source:   strings.ToUpper(r.URL.Query().Get("source")),
		Sort:     r.URL.Query().Get("sort"),
		Page:     0,
		Limit:    maxExportRows,
		CWE:      r.URL.Query().Get("cwe"),
		Vendor:   strings.ToLower(r.URL.Query().Get("vendor")),
		Product:  strings.ToLower(r.URL.Query().Get("product")),
	}

	if v := r.URL.Query().Get("min_epss"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			req.MinEPSS = &f
		}
	}
	if v := r.URL.Query().Get("is_kev"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			req.IsKEV = &b
		}
	}
	if v := r.URL.Query().Get("is_exploit"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			req.IsExploit = &b
		}
	}

	resp, err := h.searchUC.Execute(r.Context(), req)
	if err != nil {
		h.log.Error().Err(err).Msg("export search failed")
		respondError(w, http.StatusInternalServerError, "export failed")
		return
	}
	cves := resp.CVEs

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="cves-export.csv"`)
		enc := csv.NewWriter(w)
		enc.Write([]string{"CVE ID", "Severity", "CVSS3", "EPSS", "Published", "Is KEV", "Is Exploit", "Source", "Description"})
		for _, cve := range cves {
			cvss3 := 0.0
			if cve.CVSS3Score != nil {
				cvss3 = *cve.CVSS3Score
			}
			epss := 0.0
			if cve.EPSS != nil {
				epss = *cve.EPSS
			}

			enc.Write([]string{
				cve.ID,
				string(cve.Severity),
				fmt.Sprintf("%.1f", cvss3),
				fmt.Sprintf("%.6f", epss),
				cve.Published.Format("2006-01-02"),
				strconv.FormatBool(cve.IsKEV),
				strconv.FormatBool(cve.IsExploit),
				string(cve.Source),
				cve.Description,
			})
		}
		enc.Flush()
		if err := enc.Error(); err != nil {
			h.log.Error().Err(err).Msg("CSV flush error")
		}
	case "json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="cves-export.json"`)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data":        cves,
			"count":       len(cves),
			"exported_at": time.Now().UTC().Format(time.RFC3339),
		}) //nolint:errcheck
	}
}

// GetCVE handles GET /api/v2/cves/{id}.
func (h *Handler) GetCVE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cve, err := h.getByIDUC.Execute(r.Context(), id)
	switch err {
	case nil:
		respondJSON(w, http.StatusOK, mapToCVEDetailResponse(cve))
	case domainerrors.ErrInvalidCVEID:
		respondError(w, http.StatusBadRequest, "invalid CVE ID format")
	case domainerrors.ErrCVENotFound:
		respondError(w, http.StatusNotFound, "CVE not found")
	default:
		h.log.Error().Err(err).Str("id", id).Msg("get cve failed")
		respondError(w, http.StatusInternalServerError, "internal error")
	}
}

type FullTextSearchRequest struct {
	Query    string   `json:"query"`
	Severity []string `json:"severity"`
	CWE      []string `json:"cwe"`
	Vendor   string   `json:"vendor"`
	KEVOnly  *bool    `json:"kev_only"`
	MinEPSS  *float64 `json:"min_epss"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
	SortBy   string   `json:"sort_by"`
}

func (h *Handler) FullTextSearch(w http.ResponseWriter, r *http.Request) {
	var req FullTextSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	limit := req.PageSize
	if limit <= 0 { limit = 50 }
	if limit > 500 { limit = 500 }

	searchReq := &search.Request{
		Query:   req.Query,
		MinEPSS: req.MinEPSS,
		Vendor:  req.Vendor,
		IsKEV:   req.KEVOnly,
		Page:    req.Page,
		Limit:   limit,
		Sort:    req.SortBy,
	}
	if len(req.Severity) > 0 {
		searchReq.Severity = req.Severity[0]
	}
	if len(req.CWE) > 0 {
		searchReq.CWE = req.CWE[0]
	}

	resp, err := h.searchUC.Execute(r.Context(), searchReq)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "search failed")
		return
	}
	dtoResp := mapToSearchResponse(resp)
	respondJSON(w, http.StatusOK, dtoResp)
}

func (h *Handler) SemanticSearch(w http.ResponseWriter, r *http.Request) {
	// [FIX TASK-HC-005] Return 503 when AI embedder not configured — not a silent zero-vector result.
	if h.semanticUC == nil {
		respondError(w, http.StatusServiceUnavailable, "semantic search unavailable: AI embedding service not configured")
		return
	}

	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Query == "" {
		respondError(w, http.StatusBadRequest, "query is required")
		return
	}

	limit := req.Limit
	if limit <= 0 { limit = 20 }

	start := time.Now()
	cves, err := h.semanticUC.Execute(r.Context(), &pgvector.SemanticSearchRequest{
		Query: req.Query,
		Limit: limit,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "semantic search failed")
		return
	}
	elapsed := float64(time.Since(start).Milliseconds())
	
	data := make([]CVEResponseItem, 0, len(cves))
	for _, c := range cves {
		item := mapToCVEResponseItem(&c.CVE)
		sim := c.SimilarityScore
		item.SimilarityScore = &sim
		data = append(data, item)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":               data,
		"count":              len(data),
		"total":              len(data),
		"query_embedding_ms": elapsed,
	})
}

func (h *Handler) GetAggregations(w http.ResponseWriter, r *http.Request) {
	if h.osClient != nil {
		aggs, err := h.osClient.GetAggregations(r.Context())
		if err == nil {
			respondJSON(w, http.StatusOK, aggs)
			return
		}
		h.log.Warn().Err(err).Msg("OpenSearch aggregations failed, falling back to PostgreSQL")
	}

	aggs, err := h.cveRepo.GetAggregations(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "aggregation failed")
		return
	}
	respondJSON(w, http.StatusOK, aggs)
}

// GetCount handles GET /internal/cves/count.
func (h *Handler) GetCount(w http.ResponseWriter, r *http.Request) {
	n, err := h.cveRepo.Count(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "count failed")
		return
	}
	respondJSON(w, http.StatusOK, map[string]int64{"count": n})
}

// NewRouter creates the chi router for the CVE search service.
func NewRouter(h *Handler, taxH *TaxonomyHandler, vendorH *VendorHandler, internalH *InternalHandler, statsH *StatsHandler, log zerolog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(requestLogger(log))

	r.Get("/health", h.Health)

	// CR-GCV-002: EPSS endpoints — MUST be registered before /api/v2/cves to avoid shadowing
	r.Get("/api/v2/epss/stats", h.GetEPSSStats)
	r.Get("/api/v2/epss/top", h.GetTopEPSS)
	r.Get("/api/v2/epss/distribution", h.GetEPSSDistribution)

	// SPECIFIC routes before generic:
	r.Post("/api/v2/cves/search/semantic", h.SemanticSearch) // must be first
	r.Post("/api/v2/cves/search", h.FullTextSearch)
	r.Get("/api/v2/cves/aggregations", h.GetAggregations)
	r.Get("/api/v2/cves/export", h.ExportCVEs)

	r.Get("/api/v2/cves", h.SearchCVEs)
	r.Get("/api/v2/cves/{id}", h.GetCVE)
	r.Get("/internal/cves/count", h.GetCount)

	// CR-GCV-010: Dashboard Stats
	if statsH != nil {
		r.Get("/api/v2/stats/dashboard", statsH.Dashboard)
	}

	// CR-GCV-003: Taxonomy Endpoints
	if taxH != nil {
		r.Get("/api/v2/cwe", taxH.ListCWE)
		r.Get("/api/v2/cwe/{id}", taxH.GetCWE)
		r.Get("/api/v2/capec", taxH.ListCAPEC)
		r.Get("/api/v2/capec/{id}", taxH.GetCAPEC)
	}

	// CR-GCV-005: Vendor/Product Endpoints
	if vendorH != nil {
		r.Get("/api/v2/vendors", vendorH.ListVendors)
		r.Get("/api/v2/vendors/{vendor}/products", vendorH.GetVendorProducts)
		r.Get("/api/v2/products", vendorH.ListProducts)
	}

	// Internal routes (only accessible within cluster)
	if internalH != nil {
		r.Post("/internal/opensearch/index", internalH.IndexCVE)
		r.Post("/internal/opensearch/bulk", internalH.BulkIndex)
	}

	// SOL-009/BUG-017 FIX: Search recent queries and suggestions
	r.Get("/api/v1/search/recent", h.GetSearchRecent)
	r.Get("/api/v1/search/suggested", h.GetSearchSuggested)
	// SOL-009/BUG-003 FIX: Semantic search suggestions (must be before /semantic POST)
	r.Get("/api/v2/cves/search/semantic/suggestions", h.GetSemanticSuggestions)

	return r
}

func requestLogger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.Status()).
				Dur("latency", time.Since(start)).
				Msg("request")
		})
	}
}

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

// GetSearchRecent handles GET /api/v1/search/recent
// [FIX TASK-HC-007] Reads real search history from PostgreSQL instead of returning empty list.
func (h *Handler) GetSearchRecent(w http.ResponseWriter, r *http.Request) {
	limit := parseInt(r.URL.Query().Get("limit"), 10)
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	if h.historyRepo == nil {
		// Graceful degradation: repo not wired (e.g., no DB connection)
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"items": []interface{}{},
			"total": 0,
			"limit": limit,
		})
		return
	}

	userIDStr := r.Header.Get("X-User-ID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "X-User-ID header missing or invalid")
		return
	}

	entries, err := h.historyRepo.ListByUser(r.Context(), userID, limit)
	if err != nil {
		h.log.Warn().Err(err).Msg("GetSearchRecent: DB error")
		entries = nil // graceful degradation
	}

	items := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		items = append(items, map[string]interface{}{
			"query":       e.Query,
			"result_count": e.ResultCount,
			"search_type": e.SearchType,
			"created_at":  e.CreatedAt.Format(time.RFC3339),
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
		"total": len(items),
		"limit": limit,
	})
}

// GetSearchSuggested handles GET /api/v1/search/suggested — BUG-017 (SOL-009 fix)
// Returns popular/suggested search queries for the discovery UI.
func (h *Handler) GetSearchSuggested(w http.ResponseWriter, r *http.Request) {
	// Default curated suggestions — can be replaced with Redis-cached popular queries
	suggestions := []string{
		"log4shell", "spring4shell", "CVE-2021-44228",
		"remote code execution", "sql injection", "buffer overflow",
		"privilege escalation", "zero day", "authentication bypass",
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"items": suggestions,
		"total": len(suggestions),
	})
}

// GetSemanticSuggestions handles GET /api/v2/cves/search/semantic/suggestions — BUG-003 (SOL-009 fix)
// Returns autocomplete suggestions for the semantic search bar.
func (h *Handler) GetSemanticSuggestions(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := parseInt(r.URL.Query().Get("limit"), 10)
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	if q == "" {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"suggestions": []interface{}{},
			"query":       q,
		})
		return
	}

	// Simple prefix-based suggestions — in production would use OpenSearch suggest API
	suggestions := []string{}
	prefixes := []string{
		q + " vulnerability", q + " exploit", q + " CVE",
		q + " remote code execution", q + " authentication",
	}
	for i, s := range prefixes {
		if i >= limit {
			break
		}
		suggestions = append(suggestions, s)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"suggestions": suggestions,
		"query":       q,
	})
}
