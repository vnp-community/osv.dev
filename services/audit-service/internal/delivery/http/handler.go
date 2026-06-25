// Package http provides the HTTP delivery layer for audit-service.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// AuditFilter defines filterable fields for audit log queries.
type AuditFilter struct {
	UserID     string
	Action     string
	EntityType string
	EntityID   string
	Search     string     // CR-013: full-text across action, user_email, entity_type
	Severity   string     // CR-013: "Info"|"Warning"|"Critical"
	DateFrom   *time.Time
	DateTo     *time.Time
}

// AuditRepository is the persistence interface for reading audit events.
type AuditRepository interface {
	List(ctx context.Context, filter AuditFilter, page, pageSize int) ([]*AuditEvent, int64, error)
}

// AuditEvent is the domain struct for audit-service list queries.
type AuditEvent struct {
	ID         string
	UserID     string
	UserEmail  string
	Action     string
	EntityType string
	EntityID   string
	OldValue   interface{}
	NewValue   interface{}
	IPAddress  string
	UserAgent  string
	Severity   string // CR-013: "Info"|"Warning"|"Critical"
	CreatedAt  time.Time
}

// AuditEventDTO is the HTTP response shape.
type AuditEventDTO struct {
	ID         string      `json:"id"`
	UserID     string      `json:"user_id"`
	UserEmail  string      `json:"user_email"`
	Action     string      `json:"action"`
	EntityType string      `json:"entity_type"`
	EntityID   string      `json:"entity_id"`
	OldValue   interface{} `json:"old_value"`
	NewValue   interface{} `json:"new_value"`
	IPAddress  string      `json:"ip_address"`
	UserAgent  string      `json:"user_agent"`
	Severity   string      `json:"severity"`    // CR-013: Info|Warning|Critical
	CreatedAt  string      `json:"created_at"`
}

// AuditHandler handles /audit-log HTTP endpoints.
type AuditHandler struct {
	auditRepo AuditRepository
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(repo AuditRepository) *AuditHandler {
	return &AuditHandler{auditRepo: repo}
}

// GET /audit-log → /api/v1/audit-log
// CR-013: supports ?search=, ?severity=, ?date_from=, ?date_to= in addition to existing filters.
func (h *AuditHandler) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	filter := AuditFilter{
		UserID:     r.URL.Query().Get("user_id"),
		Action:     r.URL.Query().Get("action"),
		EntityType: r.URL.Query().Get("entity_type"),
		EntityID:   r.URL.Query().Get("entity_id"),
		Search:     r.URL.Query().Get("search"),   // CR-013
		Severity:   r.URL.Query().Get("severity"), // CR-013
		DateFrom:   parseTimeParam(r.URL.Query().Get("date_from")),
		DateTo:     parseTimeParam(r.URL.Query().Get("date_to")),
	}
	page, ps := parsePaginationAudit(r)

	events, total, err := h.auditRepo.List(r.Context(), filter, page, ps)
	if err != nil {
		respondAuditError(w, 500, "INTERNAL_ERROR", err.Error())
		return
	}

	respondAuditJSON(w, 200, map[string]interface{}{
		"events":    mapEvents(events),
		"total":     total,
		"page":      page,
		"page_size": ps,
	})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func mapEvents(events []*AuditEvent) []AuditEventDTO {
	dtos := make([]AuditEventDTO, len(events))
	for i, e := range events {
		// CR-013: derive severity from action if not explicitly set
		severity := e.Severity
		if severity == "" {
			severity = deriveSeverity(e.Action)
		}
		dtos[i] = AuditEventDTO{
			ID:         e.ID,
			UserID:     e.UserID,
			UserEmail:  e.UserEmail,
			Action:     e.Action,
			EntityType: e.EntityType,
			EntityID:   e.EntityID,
			OldValue:   e.OldValue,
			NewValue:   e.NewValue,
			IPAddress:  e.IPAddress,
			UserAgent:  e.UserAgent,
			Severity:   severity,
			CreatedAt:  e.CreatedAt.Format(time.RFC3339),
		}
	}
	return dtos
}

// deriveSeverity infers a severity level from action name.
// Critical: destructive ops; Warning: auth failures; Info: everything else.
func deriveSeverity(action string) string {
	switch {
	case containsAny(action, "delete", "purge", "locked", "revoke", "breach"):
		return "Critical"
	case containsAny(action, "failed", "denied", "blocked", "warn", "suspend"):
		return "Warning"
	default:
		return "Info"
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func parseTimeParam(s string) *time.Time {
	if s == "" {
		return nil
	}
	// Accept RFC3339 or date-only
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func parsePaginationAudit(r *http.Request) (page, pageSize int) {
	page = 1
	pageSize = 20

	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && ps > 0 {
		if ps > 200 {
			ps = 200 // max page_size
		}
		pageSize = ps
	}
	return
}

func respondAuditJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}

func respondAuditError(w http.ResponseWriter, status int, code, msg string) {
	respondAuditJSON(w, status, map[string]string{"error_code": code, "message": msg})
}
