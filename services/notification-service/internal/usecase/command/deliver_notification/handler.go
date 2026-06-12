// Package deliver_notification is the application command handler for webhook fan-out.
package deliver_notification

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/osv/notification-service/internal/domain/aggregate/webhook"
	"github.com/rs/zerolog"
)

// DeliveryEvent is the inbound vulnerability event to be fan-out to webhooks.
type DeliveryEvent struct {
	EventID   string
	EventType string // e.g. "osv.vuln.published"
	VulnID    string
	Payload   []byte
}

// WebhookDeliverer sends an HTTP POST to a single webhook.
type WebhookDeliverer interface {
	Deliver(ctx context.Context, wh *webhook.Webhook, payload []byte) error
}

// WebhookRepository provides read access to registered webhooks.
type WebhookRepository interface {
	ListActive(ctx context.Context) ([]*webhook.Webhook, error)
}

// IdempotencyStore prevents duplicate deliveries.
type IdempotencyStore interface {
	IsAlreadyDelivered(ctx context.Context, deliveryKey string) (bool, error)
	MarkDelivered(ctx context.Context, deliveryKey string, ttl time.Duration) error
}

// Handler fan-outs a delivery event to all matching webhooks.
type Handler struct {
	webhooks    WebhookRepository
	deliverer   WebhookDeliverer
	idempotency IdempotencyStore
	maxWorkers  int
	log         zerolog.Logger
}

// NewHandler creates a DeliverNotification handler.
func NewHandler(
	webhooks WebhookRepository,
	deliverer WebhookDeliverer,
	idempotency IdempotencyStore,
	maxWorkers int,
	log zerolog.Logger,
) *Handler {
	if maxWorkers <= 0 {
		maxWorkers = 10
	}
	return &Handler{
		webhooks:    webhooks,
		deliverer:   deliverer,
		idempotency: idempotency,
		maxWorkers:  maxWorkers,
		log:         log,
	}
}

// Handle fans out the event to all active webhooks that match the event type.
func (h *Handler) Handle(ctx context.Context, evt DeliveryEvent) error {
	webhooks, err := h.webhooks.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("list webhooks: %w", err)
	}

	// Filter to webhooks that should receive this event
	matching := make([]*webhook.Webhook, 0, len(webhooks))
	for _, wh := range webhooks {
		if wh.ShouldDeliver(evt.EventType) {
			matching = append(matching, wh)
		}
	}

	if len(matching) == 0 {
		h.log.Debug().Str("event", evt.EventType).Msg("no matching webhooks")
		return nil
	}

	h.log.Info().
		Str("event_id", evt.EventID).
		Str("vuln_id", evt.VulnID).
		Int("webhooks", len(matching)).
		Msg("delivering event")

	// Fan-out with bounded concurrency
	sem := make(chan struct{}, h.maxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, wh := range matching {
		sem <- struct{}{}
		wg.Add(1)

		go func(wh *webhook.Webhook) {
			defer func() {
				<-sem
				wg.Done()
			}()

			deliveryKey := fmt.Sprintf("%s:%s", evt.EventID, wh.ID())

			// Idempotency check
			delivered, err := h.idempotency.IsAlreadyDelivered(ctx, deliveryKey)
			if err != nil {
				h.log.Warn().Err(err).Str("webhook_id", wh.ID()).Msg("idempotency check failed")
			}
			if delivered {
				h.log.Debug().Str("key", deliveryKey).Msg("skip: already delivered")
				return
			}

			// Sign payload for this specific webhook
			signedPayload := wh.Sign(evt.Payload)

			// Deliver with retries (handled inside WebhookDeliverer)
			if err := h.deliverer.Deliver(ctx, wh, []byte(signedPayload)); err != nil {
				h.log.Warn().Err(err).Str("webhook_id", wh.ID()).Str("vuln_id", evt.VulnID).Msg("delivery failed")
				mu.Lock()
				errs = append(errs, fmt.Errorf("webhook %s: %w", wh.ID(), err))
				mu.Unlock()
				return
			}

			// Mark idempotent (24h TTL)
			if err := h.idempotency.MarkDelivered(ctx, deliveryKey, 24*time.Hour); err != nil {
				h.log.Warn().Err(err).Msg("idempotency mark failed")
			}
		}(wh)
	}

	wg.Wait()

	// Return partial error summary — caller decides whether to nack
	if len(errs) > 0 {
		return fmt.Errorf("%d/%d deliveries failed for event %s", len(errs), len(matching), evt.EventID)
	}

	return nil
}
