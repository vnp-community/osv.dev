package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
)

// HandlerFunc processes a received CloudEvent.
// Return nil to ACK the message; return error to NAK (triggers retry).
type HandlerFunc func(ctx context.Context, event *CloudEvent) error

// Subscriber creates durable JetStream consumers and dispatches messages.
type Subscriber struct {
	js          jetstream.JetStream
	serviceName string // used as consumer name prefix, e.g. "sla"
	log         zerolog.Logger
}

// NewSubscriber creates a Subscriber for the given service.
func NewSubscriber(js jetstream.JetStream, serviceName string, log zerolog.Logger) *Subscriber {
	return &Subscriber{js: js, serviceName: serviceName, log: log}
}

// Subscribe creates a durable consumer for subject and starts processing messages.
// subject can use NATS wildcards: "defectdojo.finding.*" or "defectdojo.>"
// This call is idempotent — re-subscribing to the same subject is safe.
func (s *Subscriber) Subscribe(ctx context.Context, subject string, handler HandlerFunc) error {
	consumerName := s.consumerName(subject)

	cons, err := s.js.CreateOrUpdateConsumer(ctx, StreamName, jetstream.ConsumerConfig{
		Name:          consumerName,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
		AckWait:       30 * time.Second,
		BackOff: []time.Duration{
			1 * time.Second,
			5 * time.Second,
			30 * time.Second,
			2 * time.Minute,
			5 * time.Minute,
		},
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		return fmt.Errorf("nats: create consumer %q for %q: %w", consumerName, subject, err)
	}

	go s.consume(ctx, cons, handler, subject)
	return nil
}

func (s *Subscriber) consume(ctx context.Context, cons jetstream.Consumer, handler HandlerFunc, subject string) {
	for {
		if ctx.Err() != nil {
			return
		}

		msgs, err := cons.Fetch(10, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(1 * time.Second)
			continue
		}

		for msg := range msgs.Messages() {
			var event CloudEvent
			if err := json.Unmarshal(msg.Data(), &event); err != nil {
				s.log.Error().Err(err).Str("subject", subject).Msg("unmarshal cloud event")
				_ = msg.Nak()
				continue
			}

			if err := handler(ctx, &event); err != nil {
				s.log.Error().Err(err).Str("type", event.Type).Msg("event handler failed")
				_ = msg.Nak()
				continue
			}

			_ = msg.Ack()
		}
	}
}

// consumerName derives a deterministic, NATS-safe consumer name from service + subject.
// NATS consumer names allow only alphanumeric, dash, and underscore.
func (s *Subscriber) consumerName(subject string) string {
	safe := strings.NewReplacer(".", "-", "*", "all", ">", "wildcard").Replace(subject)
	return s.serviceName + "-" + safe
}
