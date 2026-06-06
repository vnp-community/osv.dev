// infra/messaging/nats/deliver_notification_command.go — Fan-out delivery dispatcher
package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"
)

// WebhookRepository is a minimal interface for fetching active webhooks.
type WebhookRepository interface {
	// ListByEventType returns all active webhooks that should receive this event.
	ListByEventType(ctx context.Context, eventType string) ([]WebhookSummary, error)
}

// WebhookSummary contains everything the dispatcher needs to deliver.
type WebhookSummary struct {
	ID       string
	URL      string
	Secret   []byte
	EventTypes []string
}

// HTTPDeliverer posts a JSON payload to a webhook URL.
type HTTPDeliverer interface {
	Deliver(ctx context.Context, url string, secret []byte, payload []byte, eventType string) error
}

// NotificationDispatcher implements DeliveryDispatcher for the NATS consumer.
type NotificationDispatcher struct {
	webhookRepo WebhookRepository
	deliverer   HTTPDeliverer
	idempotency IdempotencyChecker
	log         zerolog.Logger
}

// IdempotencyChecker prevents duplicate delivery.
type IdempotencyChecker interface {
	IsProcessed(ctx context.Context, eventID string) (bool, error)
	MarkProcessed(ctx context.Context, eventID string) (bool, error)
}

// NewNotificationDispatcher wires together a notification dispatcher.
func NewNotificationDispatcher(
	repo WebhookRepository,
	deliverer HTTPDeliverer,
	idem IdempotencyChecker,
	log zerolog.Logger,
) *NotificationDispatcher {
	return &NotificationDispatcher{
		webhookRepo: repo,
		deliverer:   deliverer,
		idempotency: idem,
		log:         log,
	}
}

// Dispatch fans out an event payload to all matching webhooks.
func (d *NotificationDispatcher) Dispatch(ctx context.Context, eventID, eventType, vulnID string, payload []byte) error {
	// Idempotency guard
	processed, err := d.idempotency.IsProcessed(ctx, eventID)
	if err != nil {
		d.log.Warn().Err(err).Msg("idempotency check failed, proceeding anyway")
	}
	if processed {
		d.log.Debug().Str("event_id", eventID).Msg("already processed, skipping")
		return nil
	}

	webhooks, err := d.webhookRepo.ListByEventType(ctx, eventType)
	if err != nil {
		return fmt.Errorf("list webhooks for %s: %w", eventType, err)
	}

	for _, wh := range webhooks {
		whCtx := ctx
		if err := d.deliverer.Deliver(whCtx, wh.URL, wh.Secret, payload, eventType); err != nil {
			d.log.Warn().
				Err(err).
				Str("webhook_id", wh.ID).
				Str("event_id", eventID).
				Msg("webhook delivery failed")
			// Continue to other webhooks (best-effort)
		}
	}

	d.idempotency.MarkProcessed(ctx, eventID) //nolint:errcheck
	return nil
}

// MarshalPayload serializes an arbitrary domain event to JSON.
func MarshalPayload(evt interface{}) ([]byte, error) {
	return json.Marshal(evt)
}
