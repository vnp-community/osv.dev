// Package http — Browse HTTP handlers for vendor/product browsing.
// Implements CR-002: vendor/product catalog endpoints.
//
// Routes (mounted by BrowseHandler.Mount):
//
//	GET /browse/vendors                      — list all vendors
//	GET /browse/vendors/{vendor}/products    — list products for vendor (new API)
//	GET /browse/                             — alias for /browse/vendors (backward compat)
//	GET /browse/{vendor}                     — alias for vendor products (backward compat)
package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/osv/search-service/internal/usecase/browse"
)

// BrowseHandler handles vendor/product browse endpoints.
type BrowseHandler struct {
	uc *browse.UseCase
}

// NewBrowseHandler creates a BrowseHandler.
func NewBrowseHandler(uc *browse.UseCase) *BrowseHandler {
	return &BrowseHandler{uc: uc}
}

// Mount registers all browse routes to a chi router.
//
// Routes registered:
//
//	GET /browse/vendors                      — list all vendors
//	GET /browse/vendors/{vendor}/products    — list products for vendor (new canonical API)
//	GET /browse/                             — alias for /browse/vendors (backward compat)
//	GET /browse/{vendor}                     — alias for vendor products (backward compat)
//
// NOTE: More specific routes MUST be registered before less specific ones
// to avoid chi router ambiguity.
func (h *BrowseHandler) Mount(r chi.Router) {
	// Canonical new API
	r.Get("/browse/vendors", h.ListVendors)
	r.Get("/browse/vendors/{vendor}/products", h.ListProductsByVendor)

	// Backward compat with cve-search Python API
	r.Get("/browse/", h.ListVendors)        // GET /browse/ → vendor list
	r.Get("/browse/{vendor}", h.ListProductsByVendorCompat)
}

// ListVendors handles: GET /browse/vendors and GET /browse/
//
// @Summary List all vendors in CPE database
// @Description Returns sorted list of all known vendors from Redis CPE cache
// @Success 200 {object} entity.VendorCatalog
// @Router /browse/vendors [get]
func (h *BrowseHandler) ListVendors(w http.ResponseWriter, r *http.Request) {
	catalog, err := h.uc.ListVendors(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list vendors: "+err.Error())
		return
	}
	respondJSON(w, http.StatusOK, catalog)
}

// ListProductsByVendor handles: GET /browse/vendors/{vendor}/products
//
// @Summary List products for a specific vendor
// @Param vendor path string true "Vendor name (case-insensitive)"
// @Success 200 {object} entity.ProductCatalog
// @Failure 404 {object} map[string]string
// @Router /browse/vendors/{vendor}/products [get]
func (h *BrowseHandler) ListProductsByVendor(w http.ResponseWriter, r *http.Request) {
	vendor := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "vendor")))
	if vendor == "" {
		respondError(w, http.StatusBadRequest, "vendor name is required")
		return
	}

	catalog, err := h.uc.ListProductsByVendor(r.Context(), vendor)
	if err != nil {
		// Distinguish "not found" from internal error using error message heuristic
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	respondJSON(w, http.StatusOK, catalog)
}

// ListProductsByVendorCompat handles: GET /browse/{vendor}
// Backward compatibility with cve-search Python API pattern.
func (h *BrowseHandler) ListProductsByVendorCompat(w http.ResponseWriter, r *http.Request) {
	// Chi URL param name is "vendor" for both routes
	h.ListProductsByVendor(w, r)
}
