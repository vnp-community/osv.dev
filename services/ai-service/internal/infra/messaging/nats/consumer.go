// Package nats implements NATS JetStream consumer and publisher for AI Enrichment Service.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/osv/ai-service/internal/usecase/enrich_cve"
	"github.com/rs/zerolog"
)

const (
	streamName    = "OSV_EVENTS"
	consumerName  = "ai-enrichment-vuln-imported"
	filterSubject = "osv.vuln.imported"
	publishTopic  = "osv.ai.enrichment.completed"
)

// vulnImportedEvent is the minimal shape of the VulnImported event.
type vulnImportedEvent struct {
	EventID    string   `json:"event_id"`
	VulnID     string   `json:"vuln_id"`
	Summary    string   `json:"summary"`
	Details    string   `json:"details"`
	References []string `json:"references"`
	Ecosystems []string `json:"ecosystems"`
	CVSSScore  float64  `json:"cvss_score,omitempty"`
	CVSSVector string   `json:"cvss_vector,omitempty"`
}

// VulnImportedConsumer consumes VulnImported events and triggers AI enrichment.
type VulnImportedConsumer struct {
	js      jetstream.JetStream
	handler *enrich_vulnerability.Handler
	log     zerolog.Logger
}

// NewVulnImportedConsumer creates a new consumer for VulnImported events.
func NewVulnImportedConsumer(
	js jetstream.JetStream,
	handler *enrich_vulnerability.Handler,
	log zerolog.Logger,
) *VulnImportedConsumer {
	return &VulnImportedConsumer{js: js, handler: handler, log: log}
}

// Start subscribes and processes VulnImported events.
// Blocks until ctx is cancelled.
func (c *VulnImportedConsumer) Start(ctx context.Context) error {
	consumer, err := c.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    3,
		AckWait:       5 * time.Minute, // enrichment can take time (LLM calls)
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	msgs, err := consumer.Messages()
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	c.log.Info().Str("filter", filterSubject).Msg("AI enrichment consumer started")

	go func() {
		<-ctx.Done()
		msgs.Stop()
	}()

	for {
		msg, err := msgs.Next()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("next message: %w", err)
		}

		if err := c.handleMsg(ctx, msg); err != nil {
			c.log.Error().Err(err).Str("subject", msg.Subject()).Msg("enrichment failed, nacking")
			msg.Nak() //nolint:errcheck
		} else {
			msg.Ack() //nolint:errcheck
		}
	}
}

func (c *VulnImportedConsumer) handleMsg(ctx context.Context, msg jetstream.Msg) error {
	var evt vulnImportedEvent
	if err := json.Unmarshal(msg.Data(), &evt); err != nil {
		c.log.Warn().Err(err).Msg("invalid event format, skipping")
		return nil // ack to avoid redelivery of unparseable messages
	}

	cmd := enrich_vulnerability.Command{
		VulnID:     evt.VulnID,
		Summary:    evt.Summary,
		Details:    evt.Details,
		References: evt.References,
		Ecosystems: evt.Ecosystems,
		CVSSScore:  evt.CVSSScore,
		CVSSVector: evt.CVSSVector,
	}

	if _, err := c.handler.Handle(ctx, cmd); err != nil {
		return fmt.Errorf("enrich %s: %w", evt.VulnID, err)
	}

	return nil
}

// ── Publisher ─────────────────────────────────────────────────────────────────

// AIEnrichmentPublisher publishes AIEnrichmentCompleted events.
type AIEnrichmentPublisher struct {
	js  jetstream.JetStream
	log zerolog.Logger
}

// NewAIEnrichmentPublisher creates a publisher for AI enrichment events.
func NewAIEnrichmentPublisher(js jetstream.JetStream, log zerolog.Logger) *AIEnrichmentPublisher {
	return &AIEnrichmentPublisher{js: js, log: log}
}

// Publish sends an AIEnrichmentCompleted domain event to NATS.
func (p *AIEnrichmentPublisher) Publish(ctx context.Context, evt interface{}) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	type topicProvider interface{ Topic() string }
	topic := publishTopic
	if tp, ok := evt.(topicProvider); ok {
		topic = tp.Topic()
	}

	if _, err := p.js.Publish(ctx, topic, data); err != nil {
		return fmt.Errorf("publish to %s: %w", topic, err)
	}

	p.log.Debug().Str("topic", topic).Msg("event published")
	return nil
}
