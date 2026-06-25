// Package http — bulk_handler.go
// BulkHandler handles POST /api/v2/findings/bulk for bulk state transitions and tag operations.
//
// Routes:
//   POST /api/v2/findings/bulk        → Bulk operation (close/reopen/delete/tag/severity)
//   GET  /api/v2/findings/severity_count → Aggregate severity counts per product
package http

import (
	"context"
	"encoding/json"
	"net/http"

	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/finding-service/internal/domain/finding"
	findinguc "github.com/osv/finding-service/internal/usecase/finding"
)

type EventBus interface {
	Publish(ctx context.Context, subject string, data interface{}) error
}

// BulkHandler handles bulk finding operations.
type BulkHandler struct {
	bulkUC      *findinguc.BulkUpdateFindingsUseCase
	findingRepo finding.Repository
	eventBus    EventBus
	log         zerolog.Logger
}

// NewBulkHandler creates a new BulkHandler.
func NewBulkHandler(bulkUC *findinguc.BulkUpdateFindingsUseCase, repo finding.Repository, eb EventBus, log zerolog.Logger) *BulkHandler {
	return &BulkHandler{bulkUC: bulkUC, findingRepo: repo, eventBus: eb, log: log}
}

// BulkUpdateRequest is the body for POST /api/v2/findings/bulk.
type BulkUpdateRequest struct {
	FindingIDs []string `json:"finding_ids"`
	Operation  string   `json:"operation"` // close|reopen|false_positive|out_of_scope|delete|add_tags|remove_tags|set_severity
	Tags       []string `json:"tags,omitempty"`
	Severity   string   `json:"severity,omitempty"`
}

// BulkUpdateResponse is the response for POST /api/v2/findings/bulk.
type BulkUpdateResponse struct {
	Updated int      `json:"updated"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}

// RegisterRoutes adds bulk routes to the chi router.
func (h *BulkHandler) RegisterRoutes(r chi.Router) {
	r.Post("/api/v2/findings/bulk", h.BulkUpdate)
}

// BulkUpdate handles POST /api/v2/findings/bulk.
// Applies a single operation to up to 1000 findings at once.
func (h *BulkHandler) BulkUpdate(w http.ResponseWriter, r *http.Request) {
	var req BulkUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.FindingIDs) == 0 {
		respondJSON(w, http.StatusOK, &BulkUpdateResponse{})
		return
	}

	if len(req.FindingIDs) > 1000 {
		respondError(w, http.StatusBadRequest, "bulk operation limited to 1000 findings")
		return
	}

	// Parse finding IDs to UUIDs
	findingUUIDs := make([]uuid.UUID, 0, len(req.FindingIDs))
	for _, idStr := range req.FindingIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid finding id: "+idStr)
			return
		}
		findingUUIDs = append(findingUUIDs, id)
	}

	requesterIDStr := r.Header.Get("X-User-ID")
	requesterID, _ := uuid.Parse(requesterIDStr)

	result, err := h.bulkUC.Execute(r.Context(), findinguc.BulkUpdateInput{
		FindingIDs:  findingUUIDs,
		Operation:   findinguc.BulkOperation(req.Operation),
		Tags:        req.Tags,
		Severity:    req.Severity,
		RequesterID: requesterID,
	})
	if err != nil {
		h.log.Error().Err(err).Str("operation", req.Operation).Msg("BulkHandler.BulkUpdate")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, &BulkUpdateResponse{
		Updated: result.Updated,
		Failed:  result.Failed,
		Errors:  result.Errors,
	})
}

// POST /v2/findings/bulk_reopen
func (h *BulkHandler) BulkReopen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FindingIDs []string `json:"finding_ids"`
		Comment    string   `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "invalid request body")
		return
	}
	if len(req.FindingIDs) == 0 {
		respondError(w, 400, "finding_ids is required")
		return
	}
	if len(req.FindingIDs) > 100 {
		respondError(w, 400, "maximum 100 findings per bulk operation")
		return
	}

	userID := r.Header.Get("X-User-ID")
	success, failed := 0, []string{}

	for _, idStr := range req.FindingIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			failed = append(failed, idStr)
			continue
		}

		f, err := h.findingRepo.FindByID(r.Context(), id)
		if err != nil {
			failed = append(failed, idStr)
			continue
		}

		if !f.IsMitigated && !f.FalsePositive && !f.RiskAccepted {
			failed = append(failed, idStr)
			continue
		}

		f.IsMitigated = false
		f.FalsePositive = false
		f.RiskAccepted = false
		f.OutOfScope = false
		f.UpdatedAt = time.Now()

		if err := h.findingRepo.Save(r.Context(), f); err != nil {
			failed = append(failed, idStr)
			continue
		}

		if h.eventBus != nil {
			h.eventBus.Publish(r.Context(), "finding.status.changed", map[string]interface{}{
				"finding_id": idStr,
				"new_status": "Active",
				"changed_by": userID,
			})
		}

		success++
	}

	respondJSON(w, 200, map[string]interface{}{
		"success_count": success,
		"failed_ids":    failed,
	})
}

// POST /v2/findings/bulk_assign
func (h *BulkHandler) BulkAssign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FindingIDs []string `json:"finding_ids"`
		AssignedTo string   `json:"assigned_to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "invalid request body")
		return
	}
	if len(req.FindingIDs) == 0 || req.AssignedTo == "" {
		respondError(w, 400, "finding_ids and assigned_to are required")
		return
	}

	count, err := h.findingRepo.BulkUpdateAssignee(r.Context(), req.FindingIDs, req.AssignedTo)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}

	respondJSON(w, 200, map[string]interface{}{
		"success_count": count,
		"failed_ids":    []string{},
	})
}

// GET /findings/stats
func (h *BulkHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	productID := r.URL.Query().Get("product_id")

	stats, err := h.findingRepo.GetStats(r.Context(), productID)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}
	respondJSON(w, 200, stats)
}

// BulkClose handles POST /api/v1/findings/bulk/close
// Delegates to the existing BulkUpdate use case with operation "close".
func (h *BulkHandler) BulkClose(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FindingIDs []string `json:"finding_ids"`
		Reason     string   `json:"reason,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.FindingIDs) == 0 {
		respondError(w, http.StatusBadRequest, "finding_ids is required")
		return
	}
	if len(req.FindingIDs) > 100 {
		respondError(w, http.StatusBadRequest, "maximum 100 findings per bulk operation")
		return
	}

	requesterID, _ := uuid.Parse(r.Header.Get("X-User-ID"))
	findingUUIDs := make([]uuid.UUID, 0, len(req.FindingIDs))
	for _, idStr := range req.FindingIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid finding id: "+idStr)
			return
		}
		findingUUIDs = append(findingUUIDs, id)
	}

	result, err := h.bulkUC.Execute(r.Context(), findinguc.BulkUpdateInput{
		FindingIDs:  findingUUIDs,
		Operation:   findinguc.BulkOperation("close"),
		RequesterID: requesterID,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("BulkHandler.BulkClose")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, &BulkUpdateResponse{
		Updated: result.Updated,
		Failed:  result.Failed,
		Errors:  result.Errors,
	})
}
