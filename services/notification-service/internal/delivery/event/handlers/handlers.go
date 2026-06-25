// Package handlers contains NATS event handlers for notification-service.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/osv/notification-service/internal/domain/rule"
	"github.com/osv/notification-service/internal/usecase/dispatch"
)

// ─── SLABreachedHandler ───────────────────────────────────────────────────────

// SLABreachedHandler handles "sla.breached" NATS events.
type SLABreachedHandler struct {
	dispatchUC *dispatch.DispatchUseCase
}

// NewSLABreached creates a new SLABreachedHandler.
func NewSLABreached(uc *dispatch.DispatchUseCase) *SLABreachedHandler {
	return &SLABreachedHandler{dispatchUC: uc}
}

// Subject returns the NATS subject this handler subscribes to.
func (h *SLABreachedHandler) Subject() string { return "defectdojo.sla.breached" }

// Handle processes the NATS message.
func (h *SLABreachedHandler) Handle(msg *nats.Msg) {
	var event struct {
		FindingID         string `json:"finding_id"`
		ProductID         string `json:"product_id"`
		Severity          string `json:"severity"`
		SLAExpirationDate string `json:"sla_expiration_date"`
		DaysOverdue       int    `json:"days_overdue"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		slog.Error("sla.breached: unmarshal failed", "error", err)
		return
	}

	_ = h.dispatchUC.Execute(context.Background(), &dispatch.NotificationEvent{
		Type:      rule.EventSLABreach,
		ProductID: &event.ProductID,
		FindingID: &event.FindingID,
		Severity:  &event.Severity,
		Title:     fmt.Sprintf("SLA Breach: %s finding is %d day(s) overdue", event.Severity, event.DaysOverdue),
		Description: fmt.Sprintf("Finding %s breached its SLA deadline (%s) by %d days",
			event.FindingID, event.SLAExpirationDate, event.DaysOverdue),
		Metadata: map[string]interface{}{
			"days_overdue":        event.DaysOverdue,
			"sla_expiration_date": event.SLAExpirationDate,
		},
	})
}

// ─── FindingCreatedHandler ────────────────────────────────────────────────────

// FindingCreatedHandler handles "defectdojo.finding.batch_created" events.
type FindingCreatedHandler struct {
	dispatchUC *dispatch.DispatchUseCase
}

// NewFindingCreated creates a new FindingCreatedHandler.
func NewFindingCreated(uc *dispatch.DispatchUseCase) *FindingCreatedHandler {
	return &FindingCreatedHandler{dispatchUC: uc}
}

// Subject returns the NATS subject.
func (h *FindingCreatedHandler) Subject() string { return "defectdojo.finding.batch_created" }

// Handle processes the NATS message.
func (h *FindingCreatedHandler) Handle(msg *nats.Msg) {
	var event struct {
		ProductID   string `json:"product_id"`
		NewFindings int    `json:"new_findings"`
		Severity    string `json:"max_severity"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return
	}
	_ = h.dispatchUC.Execute(context.Background(), &dispatch.NotificationEvent{
		Type:        rule.EventFindingAdded,
		ProductID:   &event.ProductID,
		Severity:    &event.Severity,
		Title:       fmt.Sprintf("%d new finding(s) imported", event.NewFindings),
		Description: fmt.Sprintf("A scan produced %d new findings (max severity: %s)", event.NewFindings, event.Severity),
	})
}

// ─── FindingStatusChangedHandler ─────────────────────────────────────────────

// FindingStatusChangedHandler handles "defectdojo.finding.status_changed" events.
type FindingStatusChangedHandler struct {
	dispatchUC *dispatch.DispatchUseCase
}

// NewFindingStatusChanged creates a new FindingStatusChangedHandler.
func NewFindingStatusChanged(uc *dispatch.DispatchUseCase) *FindingStatusChangedHandler {
	return &FindingStatusChangedHandler{dispatchUC: uc}
}

// Subject returns the NATS subject.
func (h *FindingStatusChangedHandler) Subject() string {
	return "defectdojo.finding.status_changed"
}

// Handle processes the NATS message.
func (h *FindingStatusChangedHandler) Handle(msg *nats.Msg) {
	var event struct {
		FindingID string `json:"finding_id"`
		ProductID string `json:"product_id"`
		OldState  string `json:"old_state"`
		NewState  string `json:"new_state"`
		Title     string `json:"title"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return
	}
	_ = h.dispatchUC.Execute(context.Background(), &dispatch.NotificationEvent{
		Type:        rule.EventFindingStatusChanged,
		ProductID:   &event.ProductID,
		FindingID:   &event.FindingID,
		Title:       fmt.Sprintf("Finding status changed: %s → %s", event.OldState, event.NewState),
		Description: fmt.Sprintf("'%s' transitioned from %s to %s", event.Title, event.OldState, event.NewState),
	})
}

// ─── EngagementClosedHandler ──────────────────────────────────────────────────

// EngagementClosedHandler handles "defectdojo.engagement.closed" events.
type EngagementClosedHandler struct {
	dispatchUC *dispatch.DispatchUseCase
}

// NewEngagementClosed creates a new EngagementClosedHandler.
func NewEngagementClosed(uc *dispatch.DispatchUseCase) *EngagementClosedHandler {
	return &EngagementClosedHandler{dispatchUC: uc}
}

// Subject returns the NATS subject.
func (h *EngagementClosedHandler) Subject() string { return "defectdojo.engagement.closed" }

// Handle processes the NATS message.
func (h *EngagementClosedHandler) Handle(msg *nats.Msg) {
	var event struct {
		EngagementID string `json:"engagement_id"`
		ProductID    string `json:"product_id"`
		Name         string `json:"name"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return
	}
	_ = h.dispatchUC.Execute(context.Background(), &dispatch.NotificationEvent{
		Type:         rule.EventEngagementClosed,
		ProductID:    &event.ProductID,
		EngagementID: &event.EngagementID,
		Title:        fmt.Sprintf("Engagement closed: %s", event.Name),
	})
}

// ─── RiskAcceptanceExpiredHandler ────────────────────────────────────────────

// RiskAcceptanceExpiredHandler handles "defectdojo.risk_acceptance.expired" events.
type RiskAcceptanceExpiredHandler struct {
	dispatchUC *dispatch.DispatchUseCase
}

// NewRiskAcceptanceExpired creates a new RiskAcceptanceExpiredHandler.
func NewRiskAcceptanceExpired(uc *dispatch.DispatchUseCase) *RiskAcceptanceExpiredHandler {
	return &RiskAcceptanceExpiredHandler{dispatchUC: uc}
}

// Subject returns the NATS subject.
func (h *RiskAcceptanceExpiredHandler) Subject() string {
	return "defectdojo.risk_acceptance.expired"
}

// Handle processes the NATS message.
func (h *RiskAcceptanceExpiredHandler) Handle(msg *nats.Msg) {
	var event struct {
		ProductID string `json:"product_id"`
		Name      string `json:"name"`
		Count     int    `json:"reactivated_count"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return
	}
	_ = h.dispatchUC.Execute(context.Background(), &dispatch.NotificationEvent{
		Type:        rule.EventRiskAcceptanceExpiration,
		ProductID:   &event.ProductID,
		Title:       fmt.Sprintf("Risk Acceptance expired: %s", event.Name),
		Description: fmt.Sprintf("%d findings reactivated after risk acceptance expired", event.Count),
	})
}

// ─── JIRAUpdateHandler ────────────────────────────────────────────────────────

// JIRAUpdateHandler handles "defectdojo.jira.issue.created" / "defectdojo.jira.synced" events.
type JIRAUpdateHandler struct {
	dispatchUC *dispatch.DispatchUseCase
}

// NewJIRAUpdate creates a new JIRAUpdateHandler.
func NewJIRAUpdate(uc *dispatch.DispatchUseCase) *JIRAUpdateHandler {
	return &JIRAUpdateHandler{dispatchUC: uc}
}

// Subject returns the NATS subject.
func (h *JIRAUpdateHandler) Subject() string { return "defectdojo.jira.issue.created" }

// Handle processes the NATS message.
func (h *JIRAUpdateHandler) Handle(msg *nats.Msg) {
	var event struct {
		ProductID string `json:"product_id"`
		JIRAKey   string `json:"jira_key"`
		JIRAURL   string `json:"jira_url"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return
	}
	_ = h.dispatchUC.Execute(context.Background(), &dispatch.NotificationEvent{
		Type:      rule.EventJIRAUpdate,
		ProductID: &event.ProductID,
		Title:     fmt.Sprintf("JIRA issue created: %s", event.JIRAKey),
		URL:       event.JIRAURL,
	})
}
