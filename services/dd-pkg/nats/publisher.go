package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

// Publisher publishes CloudEvents to NATS JetStream.
// Each service creates one Publisher with its own source identifier.
type Publisher struct {
	js     jetstream.JetStream
	source string // e.g. "finding-management/v1"
}

// NewPublisher creates a Publisher for the given service source.
func NewPublisher(js jetstream.JetStream, source string) *Publisher {
	return &Publisher{js: js, source: source}
}

// Publish serializes data as a CloudEvent and publishes it to the given NATS subject.
// subject must start with "defectdojo." (e.g. "defectdojo.finding.created").
func (p *Publisher) Publish(ctx context.Context, subject string, data interface{}) error {
	event, err := New(subject, p.source, data)
	if err != nil {
		return fmt.Errorf("nats: build cloud event: %w", err)
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("nats: marshal cloud event: %w", err)
	}

	_, err = p.js.Publish(ctx, subject, body)
	if err != nil {
		return fmt.Errorf("nats: publish to %q: %w", subject, err)
	}
	return nil
}
