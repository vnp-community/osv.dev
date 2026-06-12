// Package repository defines the webhook persistence interface.
package repository

import (
	"context"

	"github.com/osv/notification-service/internal/domain/aggregate/webhook"
)

// WebhookRepository defines the persistence interface for webhooks.
type WebhookRepository interface {
	Save(ctx context.Context, w *webhook.Webhook) error
	FindByID(ctx context.Context, id string) (*webhook.Webhook, error)
	FindByOwner(ctx context.Context, ownerID string) ([]*webhook.Webhook, error)
	Delete(ctx context.Context, id, ownerID string) error
	FindActiveByEvent(ctx context.Context, event string) ([]*webhook.Webhook, error)
	Count(ctx context.Context) (int64, error)
}
