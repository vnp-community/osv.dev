// Package http provides the HTTP delivery layer for cve-service v1 API.
// Routes: GET /cve/{id}, GET /cve/last/{n}, GET /cve/recent/{timeframe}, GET /cve/search
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/osv/data-service/internal/domain/repository"
	taxonomy "github.com/osv/data-service/internal/delivery/http/taxonomy"
	"github.com/osv/data-service/internal/usecase/getlast"
	"github.com/osv/data-service/internal/usecase/getrecent"
	"github.com/osv/data-service/internal/usecase/searchbycpe"
)

// Handler holds the HTTP handlers for the cve-service v1 API.
type Handler struct {
	lastUC     *getlast.UseCase
	recentUC   *getrecent.UseCase
	searchUC   *searchbycpe.UseCase
	cveRepo    repository.MongoDBCVERepository
	queryH     *QueryHandler
	taxonomyH  *taxonomy.Handler // CR-003: CWE/CAPEC taxonomy endpoints
}

// NewHandler creates a Handler wired to the given use cases.
func NewHandler(
	last *getlast.UseCase,
	recent *getrecent.UseCase,
	search *searchbycpe.UseCase,
	repo repository.MongoDBCVERepository,
	queryH *QueryHandler,
	taxonomyH *taxonomy.Handler,
) *Handler {
	return &Handler{
		lastUC:    last,
		recentUC:  recent,
		searchUC:  search,
		cveRepo:   repo,
		queryH:    queryH,
		taxonomyH: taxonomyH,
	}
}

// RegisterCVERoutes registers all CVE routes onto the provided chi router.
func RegisterCVERoutes(r chi.Router, h *Handler, infoH *InfoHandler) {
	r.Use(middleware.RealIP, middleware.Recoverer, middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "cve-service"})
	})

	// DB info endpoint (CR-005): GET /info
	// Performance: uses EstimatedDocumentCount (O(1)), no auth required
	if infoH != nil {
		r.Get("/info", infoH.GetDBInfo)
		// TASK-010 FIX: gateway forwards /api/v2/dbinfo → data-service
		// data-service only had /info; add the gateway-facing path as alias
		r.Get("/api/v2/dbinfo", infoH.GetDBInfo)
		r.Get("/api/v1/dbinfo", infoH.GetDBInfo)
	}

	// NOTE: order matters — more specific patterns first
	r.Get("/cve/last/{n}", h.GetLast)
	r.Get("/cve/recent/{timeframe}", h.GetRecent)
	r.Get("/cve/search", h.SearchByCPE)
	// Flexible query endpoints (CR-008)
	if h.queryH != nil {
		r.Get("/cve/query", h.queryH.GetQuery)
		r.Post("/query", h.queryH.PostQuery)
	}
	r.Get("/cve/{id}", h.GetByID)

	// Taxonomy endpoints (CR-003): CWE and CAPEC lookups
	// Routes: GET /cwe/{id}, GET /cwe/{id}/capec, GET /capec/{id}, GET /capec/{id}/cwe
	if h.taxonomyH != nil {
		h.taxonomyH.Mount(r)
	}

}

// GetByID handles GET /cve/{id}
// Example: GET /cve/CVE-2021-44228
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cve, err := h.cveRepo.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "CVE not found", "id": id})
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return
	}
	writeJSON(w, http.StatusOK, cve)
}

// GetLast handles GET /cve/last/{n}
// Query params:
//   - format: "json" | "atom" | "rss" (default: json)
//
// Example: GET /cve/last/10?format=atom
func (h *Handler) GetLast(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(chi.URLParam(r, "n"))
	format := r.URL.Query().Get("format")
	result, err := h.lastUC.Execute(r.Context(), n)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	switch strings.ToLower(format) {
	case "atom":
		renderAtomFeed(w, result)
	case "rss":
		renderRSSFeed(w, result)
	default:
		writeJSON(w, http.StatusOK, result)
	}
}

// GetRecent handles GET /cve/recent/{timeframe}
// Timeframe: today | week | month
func (h *Handler) GetRecent(w http.ResponseWriter, r *http.Request) {
	frame := chi.URLParam(r, "timeframe")
	result, err := h.recentUC.Execute(r.Context(), frame)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchByCPE handles GET /cve/search
// Query parameters:
//   - cpe:    CPE string (required)
//   - mode:   "standard" | "lax" | "strict" (default: standard)
//             backward compat: lax=true, strict=true also accepted
//   - limit:  max results (default 100, max 1000)
//   - skip:   pagination offset (default 0)
//   - enrich: comma-separated: "ranking,capec,via4" (Phase 1: parsed but not applied)
func (h *Handler) SearchByCPE(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit, _ := strconv.Atoi(q.Get("limit"))
	skip, _ := strconv.Atoi(q.Get("skip"))

	// Parse enrichment options (comma-separated, e.g. "ranking,via4")
	enrichStr := q.Get("enrich")
	var enrichRanking, enrichCAPEC, enrichVIA4 bool
	for _, e := range strings.Split(enrichStr, ",") {
		switch strings.TrimSpace(e) {
		case "ranking":
			enrichRanking = true
		case "capec":
			enrichCAPEC = true
		case "via4":
			enrichVIA4 = true
		}
	}

	result, err := h.searchUC.Execute(r.Context(), searchbycpe.Input{
		CPE:                 q.Get("cpe"),
		Mode:                q.Get("mode"),
		Lax:                 q.Get("lax") == "true",
		StrictVendorProduct: q.Get("strict") == "true",
		Limit:               limit,
		Skip:                skip,
		EnrichRanking:       enrichRanking,
		EnrichCAPEC:         enrichCAPEC,
		EnrichVIA4:          enrichVIA4,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
