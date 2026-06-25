package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/osv/notification-service/internal/domain/repository"
	entity "github.com/osv/notification-service/internal/domain/subscription"
	"github.com/osv/shared/pkg/middleware/auth"
)

type SubscriptionHandler struct {
	subscriptRepo repository.SubscriptionRepository
}

func NewSubscriptionHandler(subscriptRepo repository.SubscriptionRepository) *SubscriptionHandler {
	return &SubscriptionHandler{
		subscriptRepo: subscriptRepo,
	}
}

// POST /api/v2/subscriptions
// Body: {"type":"vendor","value":"apache","min_severity":"CRITICAL","min_epss":0.8}
func (h *SubscriptionHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.OVSClaimsFromContext(r.Context())
	if !ok || claims == nil {
		respondError(w, 401, "authentication required")
		return
	}

	var req struct {
		Type        string   `json:"type"`
		Value       string   `json:"value"`
		MinSeverity string   `json:"min_severity"`
		MinEPSS     *float64 `json:"min_epss"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "invalid request body")
		return
	}

	// kev type is a global feed — value is optional (empty = subscribe to all KEV)
	// vendor/product types require a non-empty value to filter
	if req.Type == "" {
		respondError(w, 400, "type is required")
		return
	}
	if req.Value == "" && req.Type != "kev" {
		respondError(w, 400, "value is required for subscription type: "+req.Type)
		return
	}
	if req.MinSeverity == "" {
		req.MinSeverity = "HIGH"
	}

	sub := &entity.AlertSubscription{
		ID:          uuid.New().String(),
		OwnerID:     claims.UserID,
		Type:        entity.SubscriptionType(req.Type),
		Value:       req.Value,
		MinSeverity: req.MinSeverity,
		MinEPSS:     req.MinEPSS,
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
	}

	if err := h.subscriptRepo.Save(r.Context(), sub); err != nil {
		respondError(w, 500, "failed to save subscription")
		return
	}
	respondJSON(w, 201, sub)
}

// GET /api/v2/subscriptions
func (h *SubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.OVSClaimsFromContext(r.Context())
	if !ok || claims == nil {
		respondError(w, 401, "authentication required")
		return
	}

	subs, err := h.subscriptRepo.FindByOwner(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, 500, "failed to list subscriptions")
		return
	}
	respondJSON(w, 200, map[string]interface{}{"subscriptions": subs})
}

// DELETE /api/v2/subscriptions/{id}
func (h *SubscriptionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.OVSClaimsFromContext(r.Context())
	if !ok || claims == nil {
		respondError(w, 401, "authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.subscriptRepo.Delete(r.Context(), id, claims.UserID); err != nil {
		respondError(w, 404, "subscription not found")
		return
	}
	w.WriteHeader(204)
}

// POST /api/v2/subscriptions/bulk
func (h *SubscriptionHandler) BulkCreateSubscriptions(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.OVSClaimsFromContext(r.Context())
	if !ok || claims == nil {
		respondError(w, 401, "authentication required")
		return
	}

	var req struct {
		Subscriptions []struct {
			Type        string   `json:"type"`
			Value       string   `json:"value"`
			MinSeverity string   `json:"min_severity"`
			MinEPSS     *float64 `json:"min_epss"`
		} `json:"subscriptions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "invalid request body")
		return
	}

	results := make([]map[string]any, 0, len(req.Subscriptions))
	created := 0
	for _, subInput := range req.Subscriptions {
		if subInput.Type == "" || (subInput.Value == "" && subInput.Type != "kev") {
			results = append(results, map[string]any{"key": "unknown", "status": "error", "message": "type and value are required"})
			continue
		}
		if subInput.MinSeverity == "" {
			subInput.MinSeverity = "HIGH"
		}

		sub := &entity.AlertSubscription{
			ID:          uuid.New().String(),
			OwnerID:     claims.UserID,
			Type:        entity.SubscriptionType(subInput.Type),
			Value:       subInput.Value,
			MinSeverity: subInput.MinSeverity,
			MinEPSS:     subInput.MinEPSS,
			IsActive:    true,
			CreatedAt:   time.Now().UTC(),
		}

		key := subInput.Type + ":" + subInput.Value
		if err := h.subscriptRepo.Save(r.Context(), sub); err != nil {
			results = append(results, map[string]any{"key": key, "status": "error", "message": err.Error()})
		} else {
			results = append(results, map[string]any{"key": key, "status": "created", "id": sub.ID})
			created++
		}
	}
	respondJSON(w, http.StatusMultiStatus, map[string]any{"created_count": created, "results": results})
}
