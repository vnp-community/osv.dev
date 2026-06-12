// infra/messaging/nats/consumer.go — Subscribe domain events and dispatch DeliverNotification
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
)

// DomainEvent is a minimal deserialization of any OSV domain event.
type DomainEvent struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	OccurredAt time.Time `json:"occurred_at"`
	VulnID     string    `json:"vuln_id"`
	Source     string    `json:"source,omitempty"`
}

// DeliveryDispatcher dispatches a delivery command for each consumed event.
type DeliveryDispatcher interface {
	Dispatch(ctx context.Context, eventID, eventType, vulnID string, payload []byte) error
}

// VulnEventConsumer subscribes to all vuln domain events and triggers notification delivery.
type VulnEventConsumer struct {
	js         jetstream.JetStream
	dispatcher DeliveryDispatcher
	log        zerolog.Logger
}

// NewVulnEventConsumer creates a VulnEventConsumer.
func NewVulnEventConsumer(js jetstream.JetStream, dispatcher DeliveryDispatcher, log zerolog.Logger) *VulnEventConsumer {
	return &VulnEventConsumer{js: js, dispatcher: dispatcher, log: log}
}

// Start begins consuming osv.vuln.> events (blocks until ctx is cancelled).
func (c *VulnEventConsumer) Start(ctx context.Context) error {
	consumer, err := c.js.Consumer(ctx, "OSV-EVENTS", "notification")
	if err != nil {
		return fmt.Errorf("get notification consumer: %w", err)
	}

	cc, err := consumer.Consume(func(msg jetstream.Msg) {
		var evt DomainEvent
		if err := json.Unmarshal(msg.Data(), &evt); err != nil {
			c.log.Error().Err(err).Msg("failed to unmarshal domain event")
			msg.Nak() //nolint:errcheck
			return
		}

		cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := c.dispatcher.Dispatch(cmdCtx, evt.EventID, evt.EventType, evt.VulnID, msg.Data()); err != nil {
			c.log.Warn().Err(err).
				Str("event_id", evt.EventID).
				Str("vuln_id", evt.VulnID).
				Msg("notification dispatch failed")
			msg.Nak() //nolint:errcheck
			return
		}

		msg.Ack() //nolint:errcheck
	})
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}

	<-ctx.Done()
	cc.Stop()
	return nil
}
