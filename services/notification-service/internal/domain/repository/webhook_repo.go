package repository

import (
	"context"

	"github.com/osv/notification-service/internal/domain/aggregate/webhook"
)

// WebhookRepository defines the persistence interface for webhook aggregates.
type WebhookRepository interface {
	Save(ctx context.Context, wh *webhook.Webhook) error
	FindByID(ctx context.Context, id, ownerID string) (*webhook.Webhook, error)
	FindByOwner(ctx context.Context, ownerID string) ([]*webhook.Webhook, error)
	FindByEvent(ctx context.Context, event webhook.EventType) ([]*webhook.Webhook, error)
	FindActiveByEvent(ctx context.Context, event string) ([]*webhook.Webhook, error)
	Update(ctx context.Context, wh *webhook.Webhook) error
	Delete(ctx context.Context, id, ownerID string) error
	SaveDelivery(ctx context.Context, d *webhook.WebhookDelivery) error
	UpdateDelivery(ctx context.Context, d *webhook.WebhookDelivery) error
	ListDeliveries(ctx context.Context, webhookID string, limit int) ([]*webhook.WebhookDelivery, error)
	GetPendingRetries(ctx context.Context) ([]*webhook.WebhookDelivery, error)
}
