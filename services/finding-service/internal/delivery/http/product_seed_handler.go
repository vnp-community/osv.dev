// Package http — product_seed_handler.go
// SEED-002: Handlers for bulk product/product-type creation and seeding.
// Routes (literal paths BEFORE wildcards):
//   POST /api/v2/product-types/bulk → BulkCreateProductTypes
//   POST /api/v2/products/bulk      → BulkCreateProducts  (BEFORE /{id})
//   POST /api/v2/products/import    → ImportProducts      (BEFORE /{id})
//   POST /api/v2/products/{id}/seed → SeedProduct
package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	product_usecase "github.com/osv/finding-service/internal/usecase/product"
	"github.com/osv/finding-service/internal/domain/repository"
)

// ProductSeedHandler handles SEED-002 bulk and import endpoints.
type ProductSeedHandler struct {
	seedUC *product_usecase.UseCase
	log    zerolog.Logger
}

// NewProductSeedHandler creates a ProductSeedHandler wired to the seed usecase.
func NewProductSeedHandler(
	productTypeRepo repository.ProductTypeRepository,
	productRepo     repository.ProductRepository,
	engagementRepo  repository.EngagementRepository,
	testRepo        repository.TestRepository,
	log             zerolog.Logger,
) *ProductSeedHandler {
	return &ProductSeedHandler{
		seedUC: product_usecase.New(productTypeRepo, productRepo, engagementRepo, testRepo),
		log:    log,
	}
}

// BulkCreateProductTypes handles POST /api/v2/product-types/bulk
// Body: {"names": ["Web Apps", "Mobile Apps", "APIs"]}
// Returns 207 Multi-Status with per-item results.
func (h *ProductSeedHandler) BulkCreateProductTypes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Names []string `json:"names"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Names) == 0 {
		respondError(w, http.StatusBadRequest, "names list is required")
		return
	}
	if len(req.Names) > 200 {
		respondError(w, http.StatusBadRequest, "bulk limit exceeded: max 200 items per request")
		return
	}

	summary := h.seedUC.BulkCreateProductTypes(r.Context(), req.Names)
	respondJSON(w, http.StatusMultiStatus, summary)
}

// BulkCreateProducts handles POST /api/v2/products/bulk
// Body: {"products": [{...ProductCreateInput...}]}
// Returns 207 Multi-Status with per-item results.
func (h *ProductSeedHandler) BulkCreateProducts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Products []product_usecase.ProductCreateInput `json:"products"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Products) == 0 {
		respondError(w, http.StatusBadRequest, "products list is required")
		return
	}
	if len(req.Products) > 200 {
		respondError(w, http.StatusBadRequest, "bulk limit exceeded: max 200 items per request")
		return
	}

	summary := h.seedUC.BulkCreateProducts(r.Context(), req.Products)
	respondJSON(w, http.StatusMultiStatus, summary)
}

// SeedProduct handles POST /api/v2/products/{id}/seed
// Creates an Engagement and optional Test for a product.
// Body: {"engagement": {...}, "test": {...}}
func (h *ProductSeedHandler) SeedProduct(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req product_usecase.ProductSeedInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.seedUC.SeedProductHierarchy(r.Context(), productID, req)
	if err != nil {
		h.log.Error().Err(err).Str("product_id", productID.String()).Msg("SeedProduct failed")
		respondError(w, http.StatusInternalServerError, "seed failed: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, result)
}

// ImportProducts handles POST /api/v2/products/import (multipart)
// Supports JSON array or CSV file. Field "format" = "json" | "csv".
// Max file size: 10MB.
func (h *ProductSeedHandler) ImportProducts(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, http.StatusRequestEntityTooLarge, "file exceeds 10MB limit")
		return
	}

	format := r.FormValue("format")
	if format == "" {
		format = "json"
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	var summary product_usecase.BulkSummary
	switch format {
	case "csv":
		summary, err = h.seedUC.ImportProductsFromCSV(r.Context(), file)
	default: // "json"
		summary, err = h.seedUC.ImportProductsFromJSON(r.Context(), file)
	}
	if err != nil {
		respondError(w, http.StatusBadRequest, "parse error: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"imported_count": summary.CreatedCount,
		"exists_count":   summary.ExistsCount,
		"failed_count":   summary.FailedCount,
		"results":        summary.Results,
	})
}
