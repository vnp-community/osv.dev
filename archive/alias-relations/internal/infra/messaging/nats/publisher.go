// infra/messaging/nats/publisher.go — Publish AliasGroupUpdated events
package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
)

// Publisher publishes domain events to NATS JetStream.
type Publisher struct {
	js  jetstream.JetStream
	log zerolog.Logger
}

// NewPublisher creates a new NATS event publisher.
func NewPublisher(js jetstream.JetStream, log zerolog.Logger) *Publisher {
	return &Publisher{js: js, log: log}
}

// Publish serializes and publishes a domain event to the given NATS subject.
func (p *Publisher) Publish(ctx context.Context, subject string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	ack, err := p.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("publish to %s: %w", subject, err)
	}

	p.log.Debug().
		Str("subject", subject).
		Uint64("seq", ack.Sequence).
		Msg("event published")

	return nil
}
