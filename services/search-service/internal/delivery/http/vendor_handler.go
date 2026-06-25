package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/osv/search-service/internal/domain/repository"
)

type VendorHandler struct {
	vendorRepo repository.VendorRepository
}

func NewVendorHandler(repo repository.VendorRepository) *VendorHandler {
	return &VendorHandler{vendorRepo: repo}
}

// GET /api/v2/vendors?q=apach&limit=100
func (h *VendorHandler) ListVendors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := parseInt(r.URL.Query().Get("limit"), 100)
	if limit > 500 {
		limit = 500
	}

	vendors, total, err := h.vendorRepo.ListVendors(r.Context(), q, limit)
	if err != nil {
		respondError(w, 500, "failed to list vendors")
		return
	}
	
	vendorNames := make([]string, 0, len(vendors))
	for _, v := range vendors {
		vendorNames = append(vendorNames, v.Vendor)
	}

	respondJSON(w, 200, map[string]interface{}{
		"vendors": vendorNames,
		"total":   total,
	})
}

// GET /api/v2/vendors/{vendor}/products
func (h *VendorHandler) GetVendorProducts(w http.ResponseWriter, r *http.Request) {
	vendor := chi.URLParam(r, "vendor")
	products, err := h.vendorRepo.GetProductsByVendor(r.Context(), vendor)
	if err != nil {
		respondError(w, 500, "failed to get products")
		return
	}
	
	if products == nil {
		products = []string{}
	}

	respondJSON(w, 200, map[string]interface{}{
		"vendor":   vendor,
		"products": products,
		"count":    len(products),
	})
}

// GET /api/v2/products?vendor=apache&q=log
func (h *VendorHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	vendor := r.URL.Query().Get("vendor")
	if vendor == "" {
		respondError(w, 400, "vendor parameter is required")
		return
	}
	q := r.URL.Query().Get("q")
	limit := parseInt(r.URL.Query().Get("limit"), 100)
	if limit > 500 {
		limit = 500
	}

	products, err := h.vendorRepo.ListProducts(r.Context(), vendor, q, limit)
	if err != nil {
		respondError(w, 500, "failed to list products")
		return
	}
	
	if products == nil {
		products = []string{}
	}

	respondJSON(w, 200, map[string]interface{}{
		"vendor":   vendor,
		"products": products,
		"count":    len(products),
	})
}
