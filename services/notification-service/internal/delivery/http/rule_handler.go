// Package http — rule_handler.go
// RuleHandler provides CRUD endpoints for notification rules backed by PostgreSQL.
//
// Routes (add to notification-service router):
//   GET    /api/v1/rules          → List()   — all rules for the authenticated user
//   POST   /api/v1/rules          → Create() — create a new rule
//   PUT    /api/v1/rules/{id}     → Update() — update channel lists
//   DELETE /api/v1/rules/{id}     → Delete() — soft-delete (is_active = FALSE)
//
// Auth: user identity from X-User-ID header (set by gateway JWT middleware).
package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/notification-service/internal/domain/rule"
	"github.com/osv/notification-service/internal/infra/persistence/postgres"
)

// RuleHandler provides HTTP handlers for notification rule management.
type RuleHandler struct {
	ruleRepo *postgres.PostgresRuleRepo
	log      zerolog.Logger
}

// NewRuleHandler creates a new RuleHandler.
func NewRuleHandler(ruleRepo *postgres.PostgresRuleRepo, log zerolog.Logger) *RuleHandler {
	return &RuleHandler{ruleRepo: ruleRepo, log: log}
}

// ── Request/Response types ────────────────────────────────────────────────────

// CreateRuleRequest is the JSON body for POST /api/v1/rules.
type CreateRuleRequest struct {
	ProductID               *string  `json:"product_id,omitempty"`
	ScanAdded               []string `json:"scan_added"`
	FindingAdded            []string `json:"finding_added"`
	FindingStatusChanged    []string `json:"finding_status_changed"`
	JIRAUpdate              []string `json:"jira_update"`
	EngagementAdded         []string `json:"engagement_added"`
	EngagementClosed        []string `json:"engagement_closed"`
	RiskAcceptanceExpiration []string `json:"risk_acceptance_expiration"`
	SLABreach               []string `json:"sla_breach"`
	SLAExpiringSoon         []string `json:"sla_expiring_soon"`
}

// ToDomainRule converts CreateRuleRequest to a NotificationRule domain object.
func (req *CreateRuleRequest) ToDomainRule(userID uuid.UUID) *rule.NotificationRule {
	nr := &rule.NotificationRule{
		UserID:                  &userID,
		ScanAdded:               toChannels(req.ScanAdded),
		FindingAdded:            toChannels(req.FindingAdded),
		FindingStatusChanged:    toChannels(req.FindingStatusChanged),
		JIRAUpdate:              toChannels(req.JIRAUpdate),
		EngagementAdded:         toChannels(req.EngagementAdded),
		EngagementClosed:        toChannels(req.EngagementClosed),
		RiskAcceptanceExpiration: toChannels(req.RiskAcceptanceExpiration),
		SLABreach:               toChannels(req.SLABreach),
		SLAExpiringSoon:         toChannels(req.SLAExpiringSoon),
	}
	if req.ProductID != nil {
		pid, err := uuid.Parse(*req.ProductID)
		if err == nil {
			nr.ProductID = &pid
		}
	}
	return nr
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// List handles GET /api/v1/rules — returns all active rules for the current user.
func (h *RuleHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := extractUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rules, err := h.ruleRepo.ListForUser(r.Context(), userID)
	if err != nil {
		h.log.Error().Err(err).Msg("rule.List")
		writeJSONError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rules": rules,
		"total": len(rules),
	})
}

// Create handles POST /api/v1/rules — creates a new notification rule.
func (h *RuleHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := extractUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	nr := req.ToDomainRule(userID)
	if err := h.ruleRepo.Create(r.Context(), nr); err != nil {
		h.log.Error().Err(err).Msg("rule.Create")
		writeJSONError(w, http.StatusInternalServerError, "failed to create rule")
		return
	}

	writeJSON(w, http.StatusCreated, nr)
}

// Update handles PUT /api/v1/rules/{id} — updates channel lists for an existing rule.
func (h *RuleHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := extractUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ruleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid rule id")
		return
	}

	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	nr := req.ToDomainRule(userID)
	nr.ID = ruleID
	if err := h.ruleRepo.Update(r.Context(), nr); err != nil {
		h.log.Error().Err(err).Str("rule_id", ruleID.String()).Msg("rule.Update")
		writeJSONError(w, http.StatusInternalServerError, "failed to update rule")
		return
	}

	writeJSON(w, http.StatusOK, nr)
}

// Delete handles DELETE /api/v1/rules/{id} — soft-deletes (sets is_active = FALSE).
func (h *RuleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	_, ok := extractUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ruleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid rule id")
		return
	}

	if err := h.ruleRepo.Delete(r.Context(), ruleID); err != nil {
		h.log.Error().Err(err).Str("rule_id", ruleID.String()).Msg("rule.Delete")
		writeJSONError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// BulkCreateNotificationRules handles POST /api/v1/rules/bulk
func (h *RuleHandler) BulkCreateNotificationRules(w http.ResponseWriter, r *http.Request) {
	userID, ok := extractUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Rules []CreateRuleRequest `json:"rules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	results := make([]map[string]any, 0, len(req.Rules))
	created := 0
	for _, ruleReq := range req.Rules {
		nr := ruleReq.ToDomainRule(userID)
		err := h.ruleRepo.Create(r.Context(), nr)
		key := "global"
		if ruleReq.ProductID != nil {
			key = *ruleReq.ProductID
		}
		if err != nil {
			results = append(results, map[string]any{"product_id": key, "status": "error", "message": err.Error()})
		} else {
			results = append(results, map[string]any{"product_id": key, "status": "created", "id": nr.ID})
			created++
		}
	}
	writeJSON(w, http.StatusMultiStatus, map[string]any{"created_count": created, "results": results})
}

// ── helpers ──────────────────────────────────────────────────────────────────

// extractUserID reads X-User-ID header (set by gateway JWT middleware).
func extractUserID(r *http.Request) (uuid.UUID, bool) {
	raw := r.Header.Get("X-User-ID")
	if raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	return id, err == nil
}

// toChannels converts a string slice to []rule.Channel.
func toChannels(strs []string) []rule.Channel {
	if len(strs) == 0 {
		return nil
	}
	ch := make([]rule.Channel, 0, len(strs))
	for _, s := range strs {
		ch = append(ch, rule.Channel(s))
	}
	return ch
}
