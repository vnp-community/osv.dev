// Package http provides HTTP handlers for risk acceptance endpoints.
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	ra "github.com/osv/finding-service/internal/domain/riskacceptance"
	rauc "github.com/osv/finding-service/internal/usecase/riskacceptance"
)

// RiskAcceptanceHandler handles /api/v1/risk-acceptances and /api/v2/risk-acceptances endpoints.
type RiskAcceptanceHandler struct {
	createUC *rauc.CreateRiskAcceptanceUseCase
	removeUC *rauc.RemoveFindingFromRAUseCase
	repo     ra.Repository // for List, Get, Delete
}

// NewRiskAcceptanceHandler creates a new handler with optional repo for read operations.
func NewRiskAcceptanceHandler(c *rauc.CreateRiskAcceptanceUseCase, r *rauc.RemoveFindingFromRAUseCase) *RiskAcceptanceHandler {
	return &RiskAcceptanceHandler{createUC: c, removeUC: r}
}

// WithRepo attaches a Repository for List/Get/Delete operations.
func (h *RiskAcceptanceHandler) WithRepo(repo ra.Repository) *RiskAcceptanceHandler {
	h.repo = repo
	return h
}

// createRequest is the POST /api/v2/risk-acceptances body.
type createRequest struct {
	Name                     string   `json:"name"`
	ProductID                string   `json:"product_id"`
	FindingIDs               []string `json:"findings"`
	ExpirationDate           string   `json:"expiration_date"` // "2026-12-31" or ""
	Notes                    string   `json:"notes"`
	ProofFileKey             string   `json:"proof_file_key"`
	ReactivateExpired        bool     `json:"reactivate_expired"`
	ReactivateNoteText       string   `json:"reactivate_note"`
	RestartSLAOnReactivation bool     `json:"restart_sla_on_reactivation"`
}

// Create handles POST /api/v2/risk-acceptances.
func (h *RiskAcceptanceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product_id")
		return
	}

	// Extract requester from X-User-ID header (injected by gateway)
	requesterID, err := uuid.Parse(r.Header.Get("X-User-ID"))
	if err != nil {
		respondError(w, http.StatusUnauthorized, "missing user identity")
		return
	}

	// Parse finding IDs
	findingIDs := make([]uuid.UUID, 0, len(req.FindingIDs))
	for _, fid := range req.FindingIDs {
		id, err := uuid.Parse(fid)
		if err == nil {
			findingIDs = append(findingIDs, id)
		}
	}

	// Parse expiration date
	var expDate *time.Time
	if req.ExpirationDate != "" {
		t, err := time.Parse("2006-01-02", req.ExpirationDate)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid expiration_date format (use YYYY-MM-DD)")
			return
		}
		expDate = &t
	}

	ra, err := h.createUC.Execute(r.Context(), rauc.CreateRiskAcceptanceInput{
		Name:                     req.Name,
		ProductID:                productID,
		RequesterUserID:          requesterID,
		FindingIDs:               findingIDs,
		ExpirationDate:           expDate,
		Notes:                    req.Notes,
		ProofFileKey:             req.ProofFileKey,
		ReactivateExpired:        req.ReactivateExpired,
		ReactivateNoteText:       req.ReactivateNoteText,
		RestartSLAOnReactivation: req.RestartSLAOnReactivation,
	})
	if err != nil {
		switch err {
		case rauc.ErrNotOwner:
			respondError(w, http.StatusForbidden, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	respondJSON(w, http.StatusCreated, ra)
}

// RemoveFinding handles POST /api/v2/risk-acceptances/{id}/findings/{fid}/remove.
func (h *RiskAcceptanceHandler) RemoveFinding(w http.ResponseWriter, r *http.Request) {
	raID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid risk_acceptance id")
		return
	}
	findingID, err := uuid.Parse(chi.URLParam(r, "fid"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding id")
		return
	}
	requesterID, _ := uuid.Parse(r.Header.Get("X-User-ID"))

	if err := h.removeUC.Execute(r.Context(), rauc.RemoveFindingInput{
		RAID:            raID,
		FindingID:       findingID,
		RequesterUserID: requesterID,
	}); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// List handles GET /api/v1/risk-acceptances and GET /api/v2/risk-acceptances
// TASK-009: Implement list for frontend Risk Acceptance page.
func (h *RiskAcceptanceHandler) List(w http.ResponseWriter, r *http.Request) {
	// Filter by product_id (optional)
	var productID *uuid.UUID
	if pidStr := r.URL.Query().Get("product_id"); pidStr != "" {
		if pid, err := uuid.Parse(pidStr); err == nil {
			productID = &pid
		}
	}

	// When repo not wired, return empty list gracefully
	if h.repo == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"risk_acceptances": []*ra.RiskAcceptance{},
			"total":            0,
		})
		return
	}

	var (acceptances []*ra.RiskAcceptance; err error)
	if productID != nil {
		acceptances, err = h.repo.ListByProduct(r.Context(), *productID)
	} else {
		// No product filter: return empty (table scan without product_id not supported yet)
		acceptances = make([]*ra.RiskAcceptance, 0)
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list risk acceptances")
		return
	}
	if acceptances == nil {
		acceptances = make([]*ra.RiskAcceptance, 0)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"risk_acceptances": acceptances,
		"total":            len(acceptances),
	})
}

// Get handles GET /api/v2/risk-acceptances/{id}
func (h *RiskAcceptanceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid risk_acceptance id")
		return
	}
	if h.repo == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	ra_, err := h.repo.FindByID(r.Context(), id)
	if err != nil || ra_ == nil {
		respondError(w, http.StatusNotFound, "risk acceptance not found")
		return
	}
	respondJSON(w, http.StatusOK, ra_)
}

// Delete handles DELETE /api/v1/risk-acceptances/{id} and DELETE /api/v2/risk-acceptances/{id}
func (h *RiskAcceptanceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid risk_acceptance id")
		return
	}
	if h.repo == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete risk acceptance")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
