// Package audit defines the AuditEvent domain entity.
package domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AuditEvent is an immutable record of any change in the DefectDojo system.
type AuditEvent struct {
	ID         uuid.UUID
	EventType  string          // e.g., "defectdojo.finding.status_changed"
	EntityType string          // e.g., "finding", "product", "engagement"
	EntityID   string          // UUID of the affected entity
	ActorID    string          // user or service that caused the event
	OldState   json.RawMessage // state before the change (nullable)
	NewState   json.RawMessage // state after the change / event payload
	OccurredAt time.Time       // when the event occurred (from CloudEvent.Time)
	CreatedAt  time.Time       // when this audit record was created
}

// Repository defines persistence for audit events.
type Repository interface {
	Save(ctx context.Context, e *AuditEvent) error
	List(ctx context.Context, filter AuditFilter) ([]*AuditEvent, int, error)
	FindByID(ctx context.Context, id uuid.UUID) (*AuditEvent, error)
}

// AuditFilter defines query parameters for listing audit events.
type AuditFilter struct {
	EntityType  string
	EntityID    string
	ActorID     string
	EventType   string
	FromTime    *time.Time
	ToTime      *time.Time
	Limit       int
	Offset      int
}
