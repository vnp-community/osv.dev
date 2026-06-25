package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Handler holds product-service HTTP handlers with DB access.
type Handler struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

// NewHandler creates the HTTP handler.
func NewHandler(db *pgxpool.Pool, logger zerolog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// Router defines product-service routes.
func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	// Product CRUD
	r.Post("/products", h.CreateProduct)
	r.Get("/products", h.ListProducts)
	r.Get("/products/{id}", h.GetProduct)
	r.Put("/products/{id}", h.UpdateProduct)
	r.Patch("/products/{id}", h.UpdateProduct)
	r.Delete("/products/{id}", h.DeleteProduct)

	// Engagements
	r.Post("/products/{id}/engagements", h.CreateEngagement)
	r.Get("/products/{id}/engagements", h.ListEngagements)
	r.Get("/engagements/{id}", h.GetEngagement)

	// Tests
	r.Post("/engagements/{id}/tests", h.CreateTest)
	r.Get("/engagements/{id}/tests", h.ListTests)

	return r
}

// ── Products ─────────────────────────────────────────────────────────────────

type productRow struct {
	ID                         string   `json:"id"`
	Name                       string   `json:"name"`
	Description                string   `json:"description"`
	BusinessCriticality        string   `json:"business_criticality"`
	Platform                   string   `json:"platform"`
	Lifecycle                  string   `json:"lifecycle"`
	Origin                     string   `json:"origin"`
	ExternalAudience           bool     `json:"external_audience"`
	InternetAccessible         bool     `json:"internet_accessible"`
	EnableFullRiskAcceptance   bool     `json:"enable_full_risk_acceptance"`
	EnableSimpleRiskAcceptance bool     `json:"enable_simple_risk_acceptance"`
	Tags                       []string `json:"tags"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name                       string   `json:"name"`
		Description                string   `json:"description"`
		ProductTypeID              string   `json:"product_type_id"` // ignored — no product_types table
		BusinessCriticality        string   `json:"business_criticality"`
		Platform                   string   `json:"platform"`
		Lifecycle                  string   `json:"lifecycle"`
		Origin                     string   `json:"origin"`
		ExternalAudience           bool     `json:"external_audience"`
		InternetAccessible         bool     `json:"internet_accessible"`
		EnableFullRiskAcceptance   bool     `json:"enable_full_risk_acceptance"`
		EnableSimpleRiskAcceptance bool     `json:"enable_simple_risk_acceptance"`
		Tags                       []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	if req.BusinessCriticality == "" { req.BusinessCriticality = "medium" }
	if req.Platform == "" { req.Platform = "web" }
	if req.Lifecycle == "" { req.Lifecycle = "production" }
	if req.Origin == "" { req.Origin = "internal" }
	if req.Tags == nil { req.Tags = []string{} }

	var row productRow
	err := h.db.QueryRow(context.Background(), `
		INSERT INTO products (name, description, business_criticality, platform, lifecycle, origin,
		  external_audience, internet_accessible, enable_full_risk_acceptance, enable_simple_risk_acceptance, tags)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, name, description, business_criticality, platform, lifecycle, origin,
		  external_audience, internet_accessible, enable_full_risk_acceptance, enable_simple_risk_acceptance, tags, created_at, updated_at`,
		req.Name, req.Description, req.BusinessCriticality, req.Platform, req.Lifecycle, req.Origin,
		req.ExternalAudience, req.InternetAccessible, req.EnableFullRiskAcceptance, req.EnableSimpleRiskAcceptance, req.Tags,
	).Scan(&row.ID, &row.Name, &row.Description, &row.BusinessCriticality, &row.Platform,
		&row.Lifecycle, &row.Origin, &row.ExternalAudience, &row.InternetAccessible,
		&row.EnableFullRiskAcceptance, &row.EnableSimpleRiskAcceptance, &row.Tags, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		h.logger.Error().Err(err).Msg("create product")
		jsonError(w, "failed to create product", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, http.StatusCreated, row)
}

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(context.Background(), `
		SELECT id, name, description, business_criticality, platform, lifecycle, origin,
		  external_audience, internet_accessible, enable_full_risk_acceptance, enable_simple_risk_acceptance, tags, created_at, updated_at
		FROM products ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		jsonError(w, "failed to list products", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var products []productRow
	for rows.Next() {
		var p productRow
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.BusinessCriticality, &p.Platform,
			&p.Lifecycle, &p.Origin, &p.ExternalAudience, &p.InternetAccessible,
			&p.EnableFullRiskAcceptance, &p.EnableSimpleRiskAcceptance, &p.Tags, &p.CreatedAt, &p.UpdatedAt); err == nil {
			products = append(products, p)
		}
	}
	if products == nil { products = []productRow{} }
	jsonResponse(w, http.StatusOK, map[string]interface{}{"items": products, "count": len(products)})
}

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var row productRow
	err := h.db.QueryRow(context.Background(), `
		SELECT id, name, description, business_criticality, platform, lifecycle, origin,
		  external_audience, internet_accessible, enable_full_risk_acceptance, enable_simple_risk_acceptance, tags, created_at, updated_at
		FROM products WHERE id = $1`, id,
	).Scan(&row.ID, &row.Name, &row.Description, &row.BusinessCriticality, &row.Platform,
		&row.Lifecycle, &row.Origin, &row.ExternalAudience, &row.InternetAccessible,
		&row.EnableFullRiskAcceptance, &row.EnableSimpleRiskAcceptance, &row.Tags, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		jsonError(w, "product not found", http.StatusNotFound)
		return
	}
	jsonResponse(w, http.StatusOK, row)
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req map[string]interface{}
	json.NewDecoder(r.Body).Decode(&req)
	// Simple: return current state
	h.GetProductByID(w, r, id)
}

func (h *Handler) GetProductByID(w http.ResponseWriter, r *http.Request, id string) {
	var row productRow
	err := h.db.QueryRow(context.Background(), `
		SELECT id, name, description, business_criticality, platform, lifecycle, origin,
		  external_audience, internet_accessible, enable_full_risk_acceptance, enable_simple_risk_acceptance, tags, created_at, updated_at
		FROM products WHERE id = $1`, id,
	).Scan(&row.ID, &row.Name, &row.Description, &row.BusinessCriticality, &row.Platform,
		&row.Lifecycle, &row.Origin, &row.ExternalAudience, &row.InternetAccessible,
		&row.EnableFullRiskAcceptance, &row.EnableSimpleRiskAcceptance, &row.Tags, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		jsonError(w, "product not found", http.StatusNotFound)
		return
	}
	jsonResponse(w, http.StatusOK, row)
}

func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.db.Exec(context.Background(), "DELETE FROM products WHERE id = $1", id)
	w.WriteHeader(http.StatusNoContent)
}

// ── Engagements ──────────────────────────────────────────────────────────────

type engagementRow struct {
	ID                       string    `json:"id"`
	ProductID                string    `json:"product_id"`
	Name                     string    `json:"name"`
	Description              string    `json:"description"`
	EngagementType           string    `json:"engagement_type"`
	Status                   string    `json:"status"`
	StartDate                *string   `json:"start_date"`
	EndDate                  *string   `json:"end_date"`
	Version                  string    `json:"version"`
	Tags                     []string  `json:"tags"`
	DeduplicationOnEngagement bool     `json:"deduplication_on_engagement"`
	CreatedAt                time.Time `json:"created_at"`
}

func (h *Handler) CreateEngagement(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	// Validate product exists
	var exists bool
	h.db.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM products WHERE id=$1)", productID).Scan(&exists)
	if !exists {
		jsonError(w, "product not found", http.StatusNotFound)
		return
	}

	var req struct {
		Name                      string   `json:"name"`
		Description               string   `json:"description"`
		EngagementType            string   `json:"engagement_type"`
		Status                    string   `json:"status"`
		StartDate                 *string  `json:"start_date"`
		EndDate                   *string  `json:"end_date"`
		Version                   string   `json:"version"`
		Tags                      []string `json:"tags"`
		DeduplicationOnEngagement bool     `json:"deduplication_on_engagement"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" { req.Name = "Engagement" }
	if req.EngagementType == "" { req.EngagementType = "Interactive" }
	if req.Status == "" { req.Status = "In Progress" }
	if req.Version == "" { req.Version = "1.0.0" }
	if req.Tags == nil { req.Tags = []string{} }

	// Parse dates
	var startDate, endDate interface{}
	if req.StartDate != nil && *req.StartDate != "" { startDate = *req.StartDate } else { startDate = nil }
	if req.EndDate != nil && *req.EndDate != "" { endDate = *req.EndDate } else { endDate = nil }

	var row engagementRow
	err := h.db.QueryRow(context.Background(), `
		INSERT INTO engagements (product_id, name, description, engagement_type, status, start_date, end_date, version, tags, deduplication_on_engagement)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, product_id, name, description, engagement_type, status,
		  to_char(start_date,'YYYY-MM-DD'), to_char(end_date,'YYYY-MM-DD'),
		  version, tags, deduplication_on_engagement, created_at`,
		productID, req.Name, req.Description, req.EngagementType, req.Status,
		startDate, endDate, req.Version, req.Tags, req.DeduplicationOnEngagement,
	).Scan(&row.ID, &row.ProductID, &row.Name, &row.Description, &row.EngagementType, &row.Status,
		&row.StartDate, &row.EndDate, &row.Version, &row.Tags, &row.DeduplicationOnEngagement, &row.CreatedAt)
	if err != nil {
		h.logger.Error().Err(err).Str("product_id", productID).Msg("create engagement")
		jsonError(w, "failed to create engagement", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, http.StatusCreated, row)
}

func (h *Handler) ListEngagements(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	rows, err := h.db.Query(context.Background(), `
		SELECT id, product_id, name, description, engagement_type, status,
		  to_char(start_date,'YYYY-MM-DD'), to_char(end_date,'YYYY-MM-DD'),
		  version, tags, deduplication_on_engagement, created_at
		FROM engagements WHERE product_id=$1 ORDER BY created_at DESC`, productID)
	if err != nil {
		jsonError(w, "failed to list engagements", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var engagements []engagementRow
	for rows.Next() {
		var e engagementRow
		if err := rows.Scan(&e.ID, &e.ProductID, &e.Name, &e.Description, &e.EngagementType, &e.Status,
			&e.StartDate, &e.EndDate, &e.Version, &e.Tags, &e.DeduplicationOnEngagement, &e.CreatedAt); err == nil {
			engagements = append(engagements, e)
		}
	}
	if engagements == nil { engagements = []engagementRow{} }
	jsonResponse(w, http.StatusOK, map[string]interface{}{"items": engagements, "count": len(engagements)})
}

func (h *Handler) GetEngagement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var row engagementRow
	err := h.db.QueryRow(context.Background(), `
		SELECT id, product_id, name, description, engagement_type, status,
		  to_char(start_date,'YYYY-MM-DD'), to_char(end_date,'YYYY-MM-DD'),
		  version, tags, deduplication_on_engagement, created_at
		FROM engagements WHERE id=$1`, id,
	).Scan(&row.ID, &row.ProductID, &row.Name, &row.Description, &row.EngagementType, &row.Status,
		&row.StartDate, &row.EndDate, &row.Version, &row.Tags, &row.DeduplicationOnEngagement, &row.CreatedAt)
	if err != nil {
		jsonError(w, "engagement not found", http.StatusNotFound)
		return
	}
	jsonResponse(w, http.StatusOK, row)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

type testRow struct {
	ID           string    `json:"id"`
	EngagementID string    `json:"engagement_id"`
	ScanType     string    `json:"scan_type"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	TargetStart  *string   `json:"target_start"`
	TargetEnd    *string   `json:"target_end"`
	Tags         []string  `json:"tags"`
	CreatedAt    time.Time `json:"created_at"`
}

func (h *Handler) CreateTest(w http.ResponseWriter, r *http.Request) {
	engagementID := chi.URLParam(r, "id")
	var exists bool
	h.db.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM engagements WHERE id=$1)", engagementID).Scan(&exists)
	if !exists {
		jsonError(w, "engagement not found", http.StatusNotFound)
		return
	}

	var req struct {
		ScanType    string   `json:"scan_type"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		TargetStart *string  `json:"target_start"`
		TargetEnd   *string  `json:"target_end"`
		Tags        []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Title == "" { req.Title = "Test" }
	if req.ScanType == "" { req.ScanType = "Manual Pentest" }
	if req.Tags == nil { req.Tags = []string{} }

	var startDate, endDate interface{}
	if req.TargetStart != nil && *req.TargetStart != "" { startDate = *req.TargetStart }
	if req.TargetEnd != nil && *req.TargetEnd != "" { endDate = *req.TargetEnd }

	var row testRow
	err := h.db.QueryRow(context.Background(), `
		INSERT INTO tests (engagement_id, scan_type, title, description, target_start, target_end, tags)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, engagement_id, scan_type, title, description,
		  to_char(target_start,'YYYY-MM-DD'), to_char(target_end,'YYYY-MM-DD'), tags, created_at`,
		engagementID, req.ScanType, req.Title, req.Description, startDate, endDate, req.Tags,
	).Scan(&row.ID, &row.EngagementID, &row.ScanType, &row.Title, &row.Description,
		&row.TargetStart, &row.TargetEnd, &row.Tags, &row.CreatedAt)
	if err != nil {
		h.logger.Error().Err(err).Str("engagement_id", engagementID).Msg("create test")
		jsonError(w, "failed to create test", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, http.StatusCreated, row)
}

func (h *Handler) ListTests(w http.ResponseWriter, r *http.Request) {
	engagementID := chi.URLParam(r, "id")
	rows, err := h.db.Query(context.Background(), `
		SELECT id, engagement_id, scan_type, title, description,
		  to_char(target_start,'YYYY-MM-DD'), to_char(target_end,'YYYY-MM-DD'), tags, created_at
		FROM tests WHERE engagement_id=$1 ORDER BY created_at DESC`, engagementID)
	if err != nil {
		jsonError(w, "failed to list tests", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tests []testRow
	for rows.Next() {
		var t testRow
		if err := rows.Scan(&t.ID, &t.EngagementID, &t.ScanType, &t.Title, &t.Description,
			&t.TargetStart, &t.TargetEnd, &t.Tags, &t.CreatedAt); err == nil {
			tests = append(tests, t)
		}
	}
	if tests == nil { tests = []testRow{} }
	jsonResponse(w, http.StatusOK, map[string]interface{}{"items": tests, "count": len(tests)})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func jsonResponse(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	jsonResponse(w, status, map[string]string{"error": msg})
}

// ensure uuid is used
var _ = uuid.New
