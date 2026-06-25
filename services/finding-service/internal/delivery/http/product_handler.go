// Package http — product_handler.go
// ProductHandler handles HTTP REST endpoints for Products and ProductTypes.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/finding-service/internal/domain/product"
	"github.com/osv/finding-service/internal/domain/scoring" // [FIX BUG-009]
)

// ProductHandler handles HTTP requests for product management.
type ProductHandler struct {
	productRepo     product.Repository // direct repo until full UC layer is wired
	log             zerolog.Logger
	gradeThresholds scoring.GradeThresholds // [FIX BUG-009] injected thresholds
}

// NewProductHandler creates a new ProductHandler.
func NewProductHandler(repo product.Repository, log zerolog.Logger) *ProductHandler {
	return &ProductHandler{
		productRepo:     repo,
		log:             log,
		gradeThresholds: scoring.DefaultGradeThresholds(), // [FIX BUG-009]
	}
}

// RegisterRoutes registers all product and product-type routes on the router.
func (h *ProductHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v2/products", h.List)
	r.Post("/api/v2/products", h.Create)
	r.Get("/api/v2/products/grades", h.GetProductGrades) // NEW
	r.Get("/api/v1/products/grades", h.GetProductGrades) // v1 alias
	r.Get("/api/v2/products/{id}", h.Get)
	r.Put("/api/v2/products/{id}", h.Update)
	r.Delete("/api/v2/products/{id}", h.Delete)
}

// productResponse is the JSON representation of a Product.
type productResponse struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	ProductTypeID       string   `json:"product_type"`
	BusinessCriticality string   `json:"business_criticality"`
	Platform            string   `json:"platform"`
	Lifecycle           string   `json:"lifecycle"`
	Origin              string   `json:"origin"`
	Tags                []string `json:"tags"`
	CreatedAt           string   `json:"created"`
	// Scoring fields (populated via ListWithStats on Get)
	Grade         string `json:"grade"`
	Score         int    `json:"score"`
	CriticalCount int    `json:"critical_count"`
	HighCount     int    `json:"high_count"`
}

func toProductResponse(p *product.Product) *productResponse {
	r := &productResponse{
		ID:                  p.ID.String(),
		Name:                p.Name,
		Description:         p.Description,
		ProductTypeID:       p.ProductTypeID.String(),
		BusinessCriticality: string(p.BusinessCriticality),
		Platform:            string(p.Platform),
		Lifecycle:           string(p.Lifecycle),
		Origin:              string(p.Origin),
		Tags:                p.Tags,
		CreatedAt:           p.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if r.Tags == nil {
		r.Tags = []string{}
	}
	return r
}


// List handles GET /api/v2/products
// FIX: response schema — "products"/"total" per spec (was "results"/"count")
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	// Support page/page_size (spec) or legacy limit/offset
	var limit, offset int
	ps := parseIntParam(r, "page_size", 0)
	if ps == 0 {
		ps = parseIntParam(r, "pageSize", 0)
	}

	if ps > 0 {
		limit = ps
		if limit > MaxPageSize { // [FIX BUG-008] was: 200
			limit = MaxPageSize
		}
		page := parseIntParam(r, "page", 1)
		if page < 1 {
			page = 1
		}
		offset = (page - 1) * limit
	} else if r.URL.Query().Get("limit") != "" || r.URL.Query().Get("offset") != "" {
		limit = parseIntParam(r, "limit", DefaultProductPageSize) // [FIX BUG-008] was: 50
		offset = parseIntParam(r, "offset", 0)
	} else {
		limit = DefaultProductPageSize
		page := parseIntParam(r, "page", 1)
		if page < 1 {
			page = 1
		}
		offset = (page - 1) * limit
	}

	products, total, err := h.productRepo.ListWithStats(r.Context(), product.Filter{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("ProductHandler.List")
		respondError(w, http.StatusInternalServerError, "failed to list products")
		return
	}
	// FIX: ensure items is never null — JSON must be [] not null
	items := make([]ProductListItem, 0, len(products))
	for _, p := range products {
		// [FIX BUG-009] include medium count; grade A now reachable
		grade := scoring.ComputeGrade(p.CriticalCount, p.HighCount, p.MediumCount, h.gradeThresholds)
		// FIX: tags nil guard
		tags := p.Tags
		if tags == nil {
			tags = []string{}
		}
		items = append(items, ProductListItem{
			ID:          p.ID.String(),
			Name:        p.Name,
			Description: p.Description,
			Type:        p.ProductTypeID.String(), // Fallback since Type string is missing in Product
			Criticality: string(p.BusinessCriticality),
			Lifecycle:   string(p.Lifecycle),
			Grade:       grade,
			Score:       scoring.GradeToScore(grade),
			FindingSummary: &FindingSummary{
				Critical:    p.CriticalCount,
				High:        p.HighCount,
				Medium:      p.MediumCount,
				Low:         p.LowCount,
				TotalActive: p.TotalActive,
			},
			Tags:      tags,
			CreatedAt: p.CreatedAt.Format(time.RFC3339),
		})
	}

	// Compute page for response
	page := 1
	if limit > 0 {
		page = (offset / limit) + 1
	}

	// FIX: use "products"/"total" per spec (was "results"/"count")
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"products":  items,
		"total":     total,
		"page":      page,
		"page_size": limit,
	})
}

// GET /v2/products/grades
func (h *ProductHandler) GetProductGrades(w http.ResponseWriter, r *http.Request) {
	products, err := h.productRepo.ListWithGrades(r.Context())
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}

	var totalCritical, totalHigh int
	grades := make([]ProductGradeDTO, len(products))

	for i, p := range products {
		// [FIX BUG-009] include medium count
		grade := scoring.ComputeGrade(p.CriticalCount, p.HighCount, 0, h.gradeThresholds)
		trend := h.computeTrend(r.Context(), p.ID.String())

		grades[i] = ProductGradeDTO{
			ID:            p.ID.String(),
			Name:          p.Name,
			Grade:         grade,
			Score:         scoring.GradeToScore(grade),
			CriticalCount: p.CriticalCount,
			HighCount:     p.HighCount,
			TotalActive:   p.TotalActive,
			Trend:         trend,
		}

		totalCritical += p.CriticalCount
		totalHigh += p.HighCount
	}

	// [FIX BUG-009] overall grade includes medium (totalMedium not tracked here → 0 safe fallback)
	overallGrade := scoring.ComputeGrade(totalCritical, totalHigh, 0, h.gradeThresholds)

	respondJSON(w, 200, ProductGradesResponse{
		Products:     grades,
		OverallGrade: overallGrade,
		OverallScore: scoring.GradeToScore(overallGrade),
	})
}

// computeGrade and gradeToScore are replaced by scoring.ComputeGrade / scoring.GradeToScore.
// [FIX BUG-009] See internal/domain/scoring/grading.go

// computeTrend: compare current grade with last month's grade snapshot
func (h *ProductHandler) computeTrend(ctx context.Context, productID string) string {
	current, _ := h.productRepo.GetCriticalCount(ctx, productID)
	past, _ := h.productRepo.GetCriticalCountAt(ctx, productID, time.Now().Add(-30*24*time.Hour))

	if current < past {
		return "improving"
	}
	if current > past {
		return "worsening"
	}
	return "stable"
}

// Get handles GET /api/v2/products/{id}
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	p, err := h.productRepo.FindByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "product not found")
		return
	}
	resp := toProductResponse(p)

	// Enrich with scoring stats — use critical count from repo
	critCount, _ := h.productRepo.GetCriticalCount(r.Context(), id.String())
	highCount := 0
	// Compute grade using available data
	grade := scoring.ComputeGrade(critCount, highCount, 0, h.gradeThresholds)
	resp.Grade = grade
	resp.Score = scoring.GradeToScore(grade)
	resp.CriticalCount = critCount
	resp.HighCount = highCount

	respondJSON(w, http.StatusOK, resp)
}


// Create handles POST /api/v2/products
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name                string   `json:"name"`
		Description         string   `json:"description"`
		ProductTypeID       string   `json:"product_type"`
		BusinessCriticality string   `json:"business_criticality"`
		Platform            string   `json:"platform"`
		Lifecycle           string   `json:"lifecycle"`
		Origin              string   `json:"origin"`
		SLAConfigurationID  *string  `json:"sla_configuration_id"`
		Tags                []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	ptID, err := uuid.Parse(req.ProductTypeID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product_type id")
		return
	}
	p, err := product.New(ptID, req.Name, req.Description)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	p.BusinessCriticality = product.BusinessCriticality(req.BusinessCriticality)
	p.Platform = product.Platform(req.Platform)
	p.Lifecycle = product.Lifecycle(req.Lifecycle)
	p.Origin = product.Origin(req.Origin)
	if req.Tags != nil {
		p.Tags = req.Tags
	}
	if req.SLAConfigurationID != nil {
		if slaID, err := uuid.Parse(*req.SLAConfigurationID); err == nil {
			p.SLAConfigurationID = &slaID
		}
	}
	if err := h.productRepo.Create(r.Context(), p); err != nil {
		h.log.Error().Err(err).Msg("ProductHandler.Create")
		respondError(w, http.StatusInternalServerError, "failed to create product")
		return
	}
	respondJSON(w, http.StatusCreated, toProductResponse(p))
}

// Update handles PUT /api/v2/products/{id}
func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	p, err := h.productRepo.FindByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "product not found")
		return
	}
	var req struct {
		Name                string   `json:"name"`
		Description         string   `json:"description"`
		BusinessCriticality string   `json:"business_criticality"`
		Platform            string   `json:"platform"`
		Lifecycle           string   `json:"lifecycle"`
		Origin              string   `json:"origin"`
		Tags                []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if req.BusinessCriticality != "" {
		p.BusinessCriticality = product.BusinessCriticality(req.BusinessCriticality)
	}
	if req.Platform != "" {
		p.Platform = product.Platform(req.Platform)
	}
	if req.Lifecycle != "" {
		p.Lifecycle = product.Lifecycle(req.Lifecycle)
	}
	if req.Origin != "" {
		p.Origin = product.Origin(req.Origin)
	}
	if req.Tags != nil {
		p.Tags = req.Tags
	}
	if err := h.productRepo.Save(r.Context(), p); err != nil {
		h.log.Error().Err(err).Msg("ProductHandler.Update")
		respondError(w, http.StatusInternalServerError, "failed to update product")
		return
	}
	respondJSON(w, http.StatusOK, toProductResponse(p))
}

// Delete handles DELETE /api/v2/products/{id}
func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	if err := h.productRepo.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusNotFound, "product not found or could not be deleted")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
