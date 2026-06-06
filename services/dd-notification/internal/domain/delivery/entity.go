// Package delivery defines the DeliveryRecord entity for notification tracking.
package delivery

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Status represents the delivery state of a notification.
type Status string

const (
	StatusPending  Status = "pending"
	StatusSent     Status = "sent"
	StatusFailed   Status = "failed"
	StatusRetrying Status = "retrying"
)

// Record tracks one notification delivery attempt.
type Record struct {
	ID            uuid.UUID
	RuleID        *uuid.UUID
	EventType     string
	Channel       string
	Recipient     string
	Status        Status
	Attempts      int
	LastAttemptAt *time.Time
	NextRetryAt   *time.Time
	ErrorMessage  string
	Payload       json.RawMessage
	CreatedAt     time.Time
}

// Repository persists delivery records.
type Repository interface {
	Save(ctx context.Context, r *Record) error
	ListFailed(ctx context.Context, limit int) ([]*Record, error)
}
