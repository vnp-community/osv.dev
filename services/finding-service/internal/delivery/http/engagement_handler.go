// Package http — engagement_handler.go
// EngagementHandler exposes Engagement CRUD and lifecycle endpoints.
//
// Routes (v2):
//   GET    /api/v2/engagements             → List
//   POST   /api/v2/engagements             → Create
//   GET    /api/v2/engagements/{id}        → Get
//   PUT    /api/v2/engagements/{id}        → Update
//   POST   /api/v2/engagements/{id}/close  → Close (status=Completed)
//   POST   /api/v2/engagements/{id}/reopen → Reopen (status=In Progress)
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/osv/finding-service/internal/domain/repository"
	"github.com/osv/finding-service/internal/domain/engagement"
	enguc "github.com/osv/finding-service/internal/usecase/engagement"
)

// EngagementHandler handles HTTP requests for engagements.
type EngagementHandler struct {
	repo       repository.EngagementRepository
	getOrCreate *enguc.GetOrCreateEngagementUseCase
	close       *enguc.CloseEngagementUseCase
	log         zerolog.Logger
}

// NewEngagementHandler creates a new EngagementHandler.
func NewEngagementHandler(
	repo repository.EngagementRepository,
	getOrCreate *enguc.GetOrCreateEngagementUseCase,
	closeUC *enguc.CloseEngagementUseCase,
	log zerolog.Logger,
) *EngagementHandler {
	return &EngagementHandler{
		repo:        repo,
		getOrCreate: getOrCreate,
		close:       closeUC,
		log:         log,
	}
}

// EngagementResponse is the JSON representation of an Engagement.
type EngagementResponse struct {
	ID                        string  `json:"id"`
	ProductID                 string  `json:"product_id"`
	Name                      string  `json:"name"`
	Description               string  `json:"description,omitempty"`
	Type                      string  `json:"engagement_type"`
	Status                    string  `json:"status"`
	StartDate                 string  `json:"start_date"`
	EndDate                   *string `json:"end_date,omitempty"`
	Version                   string  `json:"version,omitempty"`
	BuildID                   string  `json:"build_id,omitempty"`
	CommitHash                string  `json:"commit_hash,omitempty"`
	BranchTag                 string  `json:"branch_tag,omitempty"`
	SourceCodeManagementURI   string  `json:"source_code_management_uri,omitempty"`
	DeduplicationOnEngagement bool    `json:"deduplication_on_engagement"`
	Tags                      []string `json:"tags"`
}

// CreateEngagementRequest is the body for POST /engagements.
type CreateEngagementRequest struct {
	ProductID                 string   `json:"product_id"`
	Name                      string   `json:"name"`
	Description               string   `json:"description"`
	Type                      string   `json:"engagement_type"` // "Interactive"|"CI/CD"
	Version                   string   `json:"version"`
	BuildID                   string   `json:"build_id"`
	CommitHash                string   `json:"commit_hash"`
	BranchTag                 string   `json:"branch_tag"`
	SourceCodeManagementURI   string   `json:"source_code_management_uri"`
	DeduplicationOnEngagement bool     `json:"deduplication_on_engagement"`
	Tags                      []string `json:"tags"`
}

// List handles GET /api/v2/engagements.
func (h *EngagementHandler) List(w http.ResponseWriter, r *http.Request) {
	// Support product_id from path (chi param "id") or query param
	productID := chi.URLParam(r, "id")
	if productID == "" {
		productID = r.URL.Query().Get("product_id")
	}
	if productID == "" {
		respondError(w, http.StatusBadRequest, "product_id query param required")
		return
	}
	pid, err := uuid.Parse(productID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product_id")
		return
	}
	engs, err := h.repo.ListByProduct(r.Context(), pid)
	if err != nil {
		h.log.Error().Err(err).Msg("EngagementHandler.List")
		respondError(w, http.StatusInternalServerError, "failed to list engagements")
		return
	}
	resp := make([]*EngagementResponse, 0, len(engs))
	for _, e := range engs {
		resp = append(resp, toEngagementResponse(e))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"engagements": resp,
		"total":       len(resp),
	})
}

// Create handles POST /api/v2/engagements.
func (h *EngagementHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateEngagementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	pid, err := uuid.Parse(req.ProductID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product_id")
		return
	}

	engType := engagement.TypeInteractive
	if req.Type == string(engagement.TypeCICD) {
		engType = engagement.TypeCICD
	}

	eng, _, err := h.getOrCreate.Execute(r.Context(), enguc.GetOrCreateInput{
		ProductID:                 pid,
		Name:                      req.Name,
		EngagementType:            engType,
		Version:                   req.Version,
		BuildID:                   req.BuildID,
		CommitHash:                req.CommitHash,
		BranchTag:                 req.BranchTag,
		DeduplicationOnEngagement: req.DeduplicationOnEngagement,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("EngagementHandler.Create")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, toEngagementResponse(eng))
}

// CreateForProduct handles POST /api/v1/products/{id}/engagements — CR-003
// Product ID is taken from URL path, not request body.
func (h *EngagementHandler) CreateForProduct(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "id")
	pid, err := uuid.Parse(productIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product_id in path")
		return
	}

	var req struct {
		Name           string   `json:"name"`
		Type           string   `json:"engagement_type"`
		Version        string   `json:"version"`
		BuildID        string   `json:"build_id"`
		CommitHash     string   `json:"commit_hash"`
		BranchTag      string   `json:"branch_tag"`
		Tags           []string `json:"tags"`
		Deduplication  bool     `json:"deduplication_on_engagement"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	engType := engagement.TypeInteractive
	if req.Type == string(engagement.TypeCICD) {
		engType = engagement.TypeCICD
	}

	eng, _, err := h.getOrCreate.Execute(r.Context(), enguc.GetOrCreateInput{
		ProductID:                 pid,
		Name:                      req.Name,
		EngagementType:            engType,
		Version:                   req.Version,
		BuildID:                   req.BuildID,
		CommitHash:                req.CommitHash,
		BranchTag:                 req.BranchTag,
		DeduplicationOnEngagement: req.Deduplication,
	})
	if err != nil {
		h.log.Error().Err(err).Str("product_id", productIDStr).Msg("EngagementHandler.CreateForProduct")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, toEngagementResponse(eng))
}

// Get handles GET /api/v2/engagements/{id}.
func (h *EngagementHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid engagement id")
		return
	}
	eng, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "engagement not found")
		return
	}
	respondJSON(w, http.StatusOK, toEngagementResponse(eng))
}

// Close handles POST /api/v2/engagements/{id}/close.
func (h *EngagementHandler) Close(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid engagement id")
		return
	}
	if err := h.close.Execute(r.Context(), id); err != nil {
		h.log.Error().Err(err).Str("engagement_id", id.String()).Msg("EngagementHandler.Close")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "Completed"})
}

// Reopen handles POST /api/v2/engagements/{id}/reopen.
func (h *EngagementHandler) Reopen(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid engagement id")
		return
	}
	eng, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "engagement not found")
		return
	}
	eng.Status = engagement.StatusInProgress
	now := time.Now().UTC()
	eng.EndDate = nil
	eng.UpdatedAt = now
	if err := h.repo.Update(r.Context(), eng); err != nil {
		h.log.Error().Err(err).Str("engagement_id", id.String()).Msg("EngagementHandler.Reopen")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "In Progress"})
}

// ── helpers ─────────────────────────────────────────────────────────────────

func toEngagementResponse(e *engagement.Engagement) *EngagementResponse {
	resp := &EngagementResponse{
		ID:                        e.ID.String(),
		ProductID:                 e.ProductID.String(),
		Name:                      e.Name,
		Description:               e.Description,
		Type:                      string(e.EngagementType),
		Status:                    string(e.Status),
		StartDate:                 e.StartDate.Format("2006-01-02"),
		Version:                   e.Version,
		BuildID:                   e.BuildID,
		CommitHash:                e.CommitHash,
		BranchTag:                 e.BranchTag,
		SourceCodeManagementURI:   e.SourceCodeManagementURI,
		DeduplicationOnEngagement: e.DeduplicationOnEngagement,
		Tags:                      e.Tags,
	}
	if e.EndDate != nil {
		s := e.EndDate.Format("2006-01-02")
		resp.EndDate = &s
	}
	return resp
}
