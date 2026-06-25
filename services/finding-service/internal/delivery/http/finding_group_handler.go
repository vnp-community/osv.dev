// Package http provides HTTP handlers for finding groups.
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/finding-service/internal/domain/group"
)

// FindingGroupHandler provides HTTP endpoints for finding groups.
type FindingGroupHandler struct {
	repo group.Repository
	log  zerolog.Logger
}

// NewFindingGroupHandler creates a new FindingGroupHandler.
func NewFindingGroupHandler(repo group.Repository, log zerolog.Logger) *FindingGroupHandler {
	return &FindingGroupHandler{repo: repo, log: log}
}

// createGroupRequest is the request body for creating a finding group.
type createGroupRequest struct {
	Name       string      `json:"name"`
	ProductID  uuid.UUID   `json:"product_id"`
	FindingIDs []uuid.UUID `json:"finding_ids"`
}

// Create handles POST /api/v2/finding-groups
// Creates a finding group and persists it to the database.
func (h *FindingGroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}

	if req.Name == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if req.ProductID == uuid.Nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "product_id is required"})
		return
	}

	g := group.New(req.ProductID, req.Name)
	g.FindingCount = len(req.FindingIDs)

	if err := h.repo.Save(r.Context(), g); err != nil {
		h.log.Error().Err(err).Msg("failed to create finding group")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create finding group"})
		return
	}

	h.log.Info().Str("group_id", g.ID.String()).Str("name", g.Name).Msg("finding group created")
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":           g.ID,
		"name":         g.Name,
		"product_id":   g.ProductID,
		"finding_count": g.FindingCount,
		"created_at":   g.CreatedAt.Format(time.RFC3339),
	})
}

// Get handles GET /api/v2/finding-groups/{id}
func (h *FindingGroupHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid group ID"})
		return
	}

	g, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to get finding group")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not get finding group"})
		return
	}
	if g == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "finding group not found"})
		return
	}

	respondJSON(w, http.StatusOK, g)
}

// ListByProduct handles GET /api/v2/finding-groups?product_id={id}
func (h *FindingGroupHandler) ListByProduct(w http.ResponseWriter, r *http.Request) {
	pidStr := r.URL.Query().Get("product_id")
	if pidStr == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "product_id is required"})
		return
	}
	pid, err := uuid.Parse(pidStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product_id"})
		return
	}

	groups, err := h.repo.ListByProduct(r.Context(), pid)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to list finding groups")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list finding groups"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"count":  len(groups),
		"groups": groups,
	})
}

// Delete handles DELETE /api/v2/finding-groups/{id}
func (h *FindingGroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid group ID"})
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		h.log.Error().Err(err).Msg("failed to delete finding group")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not delete finding group"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
