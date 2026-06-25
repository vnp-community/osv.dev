// Package event defines the AuditEvent domain entity.
// AuditEvent is IMMUTABLE — records are never updated or deleted after creation.
package event

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AuditEvent is an immutable record of an action performed in the system.
// It captures WHO did WHAT to WHICH resource at WHEN, with a cryptographic signature.
type AuditEvent struct {
	ID        uuid.UUID
	EventID   string // NATS message ID or external correlation ID

	// Event type mirrors the NATS subject (e.g. "defectdojo.finding.status_changed")
	EventType string

	// WHO
	ActorID     *uuid.UUID
	ActorEmail  *string
	ActorType   string // "user" | "system" | "service"
	ServiceName string // originating microservice

	// WHAT
	ResourceType string // "finding" | "product" | "engagement" | etc.
	ResourceID   uuid.UUID
	Action       string // "created" | "updated" | "deleted" | "status_changed" | etc.

	// PAYLOAD
	Changes  map[string]interface{} // old/new values
	Metadata map[string]interface{} // additional context

	// WHEN
	OccurredAt time.Time
	RecordedAt time.Time

	// INTEGRITY
	// HMAC-SHA256 over: event_type|resource_id|occurred_at|actor_id
	Signature string
}

// New creates a new AuditEvent.
func New(eventType, resourceType, action string, resourceID uuid.UUID) *AuditEvent {
	now := time.Now().UTC()
	return &AuditEvent{
		ID:           uuid.New(),
		EventType:    eventType,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		ActorType:    "system",
		Changes:      make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
		OccurredAt:   now,
		RecordedAt:   now,
	}
}

// SignablePayload returns the canonical string used for HMAC signature.
func (e *AuditEvent) SignablePayload() string {
	actorID := ""
	if e.ActorID != nil {
		actorID = e.ActorID.String()
	}
	return strings.Join([]string{
		e.EventType,
		e.ResourceID.String(),
		e.OccurredAt.UTC().Format(time.RFC3339Nano),
		actorID,
	}, "|")
}

// ─── Repository ───────────────────────────────────────────────────────────────

// Query defines filtering parameters for audit event queries.
type Query struct {
	EventTypes   []string
	ResourceType *string
	ResourceID   *uuid.UUID
	ActorID      *uuid.UUID
	From         *time.Time
	To           *time.Time
	Limit        int
	Offset       int
	OrderBy      string // "occurred_at DESC" (default) | "occurred_at ASC"
}

// Repository is append-only — no Update() or Delete() methods by design.
// Enforced additionally at DB level via RLS policies.
type Repository interface {
	Create(ctx context.Context, event *AuditEvent) error
	FindByID(ctx context.Context, id uuid.UUID) (*AuditEvent, error)
	List(ctx context.Context, query Query) ([]*AuditEvent, int64, error)
	// ExportJSON streams audit events as NDJSON for compliance export.
	ExportJSON(ctx context.Context, query Query, w interface{ Write([]byte) (int, error) }) error
}

// ─── NATS Subjects ────────────────────────────────────────────────────────────

// AuditableSubjects is the list of all NATS subjects the audit service subscribes to.
// Every event in this list will be persisted to audit_events.
var AuditableSubjects = []string{
	// Findings
	"defectdojo.finding.created",
	"defectdojo.finding.updated",
	"defectdojo.finding.deleted",
	"defectdojo.finding.status_changed",
	"defectdojo.finding.bulk_updated",
	"defectdojo.finding.risk_accepted",
	"defectdojo.finding.false_positive_marked",
	"defectdojo.finding.duplicate_detected",
	// Products
	"defectdojo.product.created",
	"defectdojo.product.updated",
	"defectdojo.product.deleted",
	"defectdojo.product.member.added",
	"defectdojo.product.member.removed",
	"defectdojo.product.member.role_changed",
	// Engagements
	"defectdojo.engagement.created",
	"defectdojo.engagement.updated",
	"defectdojo.engagement.closed",
	"defectdojo.engagement.reopened",
	// Tests
	"defectdojo.test.created",
	"defectdojo.test.updated",
	// Scan imports
	"scan.import.started",
	"scan.import.completed",
	"scan.import.failed",
	// Risk acceptances
	"defectdojo.risk_acceptance.created",
	"defectdojo.risk_acceptance.updated",
	"defectdojo.risk_acceptance.expired",
	// SLA
	"defectdojo.sla.config.created",
	"defectdojo.sla.config.updated",
	"defectdojo.sla.config.deleted",
	"defectdojo.sla.breach",
	// JIRA
	"defectdojo.jira.issue.created",
	"defectdojo.jira.issue.updated",
	"defectdojo.jira.synced",
	// Users
	"identity.user.login",
	"identity.user.login_failed",
	"identity.user.logout",
	"identity.user.password_changed",
	"identity.user.role_changed",
	"identity.user.created",
	"identity.user.deleted",
	// Reports
	"defectdojo.report.generated",
	"defectdojo.report.deleted",
}

// SubjectToResourceType extracts the resource type from a NATS subject.
// e.g. "defectdojo.finding.status_changed" → "finding"
func SubjectToResourceType(subject string) string {
	parts := strings.Split(subject, ".")
	if len(parts) >= 2 {
		// Skip "defectdojo." or "scan." prefix
		if parts[0] == "defectdojo" || parts[0] == "scan" || parts[0] == "identity" {
			return parts[1]
		}
	}
	return subject
}

// SubjectToAction extracts the action from a NATS subject.
// e.g. "defectdojo.finding.status_changed" → "status_changed"
func SubjectToAction(subject string) string {
	parts := strings.Split(subject, ".")
	if len(parts) >= 3 {
		return fmt.Sprintf("%s", strings.Join(parts[2:], "."))
	}
	return subject
}
