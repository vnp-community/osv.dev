// Package handler provides the HTTP REST API handlers for product-management.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	product_uc "github.com/defectdojo/product-service/internal/usecase/product"
	"github.com/defectdojo/product-service/internal/domain/repository"
)

// ProductHandler provides CRUD HTTP handlers for Products.
type ProductHandler struct {
	createUC  *product_uc.CreateProductUseCase
	listRepo  repository.ProductRepository
	log       zerolog.Logger
}

func NewProductHandler(
	create *product_uc.CreateProductUseCase,
	repo repository.ProductRepository,
	log zerolog.Logger,
) *ProductHandler {
	return &ProductHandler{createUC: create, listRepo: repo, log: log}
}

// RegisterRoutes mounts all product routes on the provided chi.Router.
func (h *ProductHandler) RegisterRoutes(r chi.Router) {
	r.Post("/api/v2/products", h.Create)
	r.Get("/api/v2/products", h.List)
	r.Get("/api/v2/products/{id}", h.Get)
}

// Create handles POST /api/v2/products.
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		ProductTypeID string `json:"prod_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ptID, err := uuid.Parse(body.ProductTypeID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid prod_type UUID")
		return
	}

	userID, _ := uuid.Parse(r.Header.Get("X-User-ID"))

	p, err := h.createUC.Execute(r.Context(), product_uc.CreateProductInput{
		Name:          body.Name,
		Description:   body.Description,
		ProductTypeID: ptID,
		RequestorID:   userID,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("create product")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

// List handles GET /api/v2/products with optional ?product_type_id=&limit=&offset=.
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := repository.ProductFilter{
		Limit:  25,
		Offset: 0,
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			filter.Limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			filter.Offset = v
		}
	}
	if ptid := r.URL.Query().Get("product_type_id"); ptid != "" {
		id, err := uuid.Parse(ptid)
		if err == nil {
			filter.ProductTypeID = &id
		}
	}
	filter.Search = r.URL.Query().Get("search")

	products, total, err := h.listRepo.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list products failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":   total,
		"results": products,
	})
}

// Get handles GET /api/v2/products/{id}.
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	p, err := h.listRepo.FindByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
