// Package http provides the HTTP delivery layer for browse-service.
package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"github.com/osv/browse-service/internal/domain/entity"
	"github.com/osv/browse-service/internal/domain/repository"
)

// Handler holds HTTP handlers for browse-service.
type Handler struct {
	cache    repository.CacheRepository
	cpeRepo  repository.CPERepository
	log      zerolog.Logger
}

// NewHandler creates a Handler with Redis L1 cache and MongoDB L2 fallback.
func NewHandler(cache repository.CacheRepository, cpeRepo repository.CPERepository, log zerolog.Logger) *Handler {
	return &Handler{cache: cache, cpeRepo: cpeRepo, log: log}
}

// NewRouter builds the chi router for browse-service.
func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer, middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "browse-service"})
	})

	// Browse endpoints
	r.Get("/browse/", h.ListVendors)
	r.Get("/browse/{vendor}", h.ListProducts)

	// Search
	r.Get("/vendors/search", h.SearchVendors)

	// Version browsing (proxied to cve-service via gateway, stub here)
	r.Get("/search/{vendor}/{product}", h.ListVersions)
	r.Get("/search/{vendor}/{product}/{version}", h.ListVersions)

	return r
}

// ListVendors handles GET /browse/
// Returns application vendors (cpe type "a") with Redis L1 → MongoDB L2 fallback.
func (h *Handler) ListVendors(w http.ResponseWriter, r *http.Request) {
	cpeType := r.URL.Query().Get("type")
	if cpeType == "" {
		cpeType = "a"
	}

	// L1: Redis cache
	vendors, err := h.cache.GetVendors(r.Context(), cpeType)
	if err != nil {
		h.log.Warn().Err(err).Msg("Redis GetVendors failed, falling back to MongoDB")
	}

	// L2: MongoDB fallback
	if len(vendors) == 0 {
		vendors, err = h.cpeRepo.ListVendors(r.Context(), cpeType)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		// Populate cache async
		go h.cache.SetVendors(r.Context(), cpeType, vendors) //nolint:errcheck
	}

	writeJSON(w, http.StatusOK, entity.BrowseResult{
		Items:  vendors,
		Total:  len(vendors),
	})
}

// ListProducts handles GET /browse/{vendor}
// Returns products for the specified vendor with Redis L1 → MongoDB L2 fallback.
func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	vendor := chi.URLParam(r, "vendor")

	// L1: Redis cache
	products, err := h.cache.GetProducts(r.Context(), vendor)
	if err != nil {
		h.log.Warn().Err(err).Str("vendor", vendor).Msg("Redis GetProducts failed")
	}

	// L2: MongoDB fallback
	if len(products) == 0 {
		products, err = h.cpeRepo.ListProducts(r.Context(), vendor)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		go h.cache.SetProducts(r.Context(), vendor, products) //nolint:errcheck
	}

	writeJSON(w, http.StatusOK, entity.BrowseResult{
		Items:  products,
		Total:  len(products),
		Vendor: vendor,
	})
}

// SearchVendors handles GET /vendors/search?q=cisco
func (h *Handler) SearchVendors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "q parameter required"})
		return
	}

	vendors, err := h.cpeRepo.SearchVendors(r.Context(), q)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, entity.SearchResult{
		Items: vendors,
		Total: len(vendors),
		Query: q,
	})
}

// ListVersions handles GET /search/{vendor}/{product} and /search/{vendor}/{product}/{version}
// Returns CVEs for the vendor/product combination.
func (h *Handler) ListVersions(w http.ResponseWriter, r *http.Request) {
	vendor := chi.URLParam(r, "vendor")
	product := chi.URLParam(r, "product")
	version := chi.URLParam(r, "version")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"vendor":  vendor,
		"product": product,
		"version": version,
		"note":    "use cve-service /cve/search?cpe=cpe:2.3:a:{vendor}:{product}:{version} for CVE results",
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
