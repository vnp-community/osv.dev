// Package alert defines the in-app Alert entity.
package alert

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Alert is an in-app notification visible in the DefectDojo UI.
type Alert struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	EventType   string
	Title       string
	Description string
	URL         string
	IsRead      bool
	CreatedAt   time.Time
}

// Repository defines persistence for in-app alerts.
type Repository interface {
	Create(ctx context.Context, a *Alert) error
	ListForUser(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit, offset int) ([]*Alert, int, error)
	CountUnread(ctx context.Context, userID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}
