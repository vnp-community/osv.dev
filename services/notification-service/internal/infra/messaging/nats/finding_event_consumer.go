// Package nats — finding_event_consumer.go
// FindingEventConsumer subscribes to cross-service NATS events and routes them
// to the DispatchUseCase for notification delivery.
//
// ADDITIVE: VulnEventConsumer (consumer.go) is unchanged.
// Uses JetStream push consumers (same pattern as VulnEventConsumer).
//
// Subscribed subjects:
//   - defectdojo.sla.breached         (from finding-service SLA checker)
//   - defectdojo.sla.expiring_soon    (from finding-service SLA checker)
//   - defectdojo.finding.batch_created (from finding-service)
//   - defectdojo.finding.status_changed (from finding-service)
//   - ai.epss.updated                  (from ai-service)
package nats

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"

	dispatch "github.com/osv/notification-service/internal/usecase/dispatch_alert"
)

// ── Consumed subjects ─────────────────────────────────────────────────────────

const (
	// SubjectSLABreached matches events from finding-service CheckBreachesUseCase.
	SubjectSLABreached = "defectdojo.sla.breached"
	// SubjectSLAExpiringSoon matches events from finding-service CheckBreachesUseCase.
	SubjectSLAExpiringSoon = "defectdojo.sla.expiring_soon"
	// SubjectFindingBatchCreated matches events from finding-service BatchCreateFindingsUseCase.
	SubjectFindingBatchCreated = "defectdojo.finding.batch_created"
	// SubjectFindingStatusChanged matches events from finding-service StatusTransitionUseCase.
	SubjectFindingStatusChanged = "defectdojo.finding.status_changed"
	// SubjectAIEPSSUpdated matches EPSS score update events from ai-service.
	SubjectAIEPSSUpdated = "ai.epss.updated"
)

// epssNotifyThreshold is the EPSS score above which a spike notification is sent.
const epssNotifyThreshold = 0.7

// ── Event payload types ───────────────────────────────────────────────────────

// slaBreachedPayload is the NATS event from finding-service CheckBreachesUseCase.
type slaBreachedPayload struct {
	FindingID      string    `json:"finding_id"`
	ProductID      string    `json:"product_id"`
	Severity       string    `json:"severity"`
	ExpirationDate time.Time `json:"expiration_date"`
	DaysOverdue    int       `json:"days_overdue"`
}

// slaExpiringSoonPayload is the NATS event from finding-service CheckBreachesUseCase.
type slaExpiringSoonPayload struct {
	FindingID     string `json:"finding_id"`
	ProductID     string `json:"product_id"`
	DaysRemaining int    `json:"days_remaining"`
	Severity      string `json:"severity"`
}

// batchCreatedPayload is the NATS event from BatchCreateFindingsUseCase.
type batchCreatedPayload struct {
	FindingIDs   []string `json:"finding_ids"`
	TestID       string   `json:"test_id"`
	EngagementID string   `json:"engagement_id"`
	ProductID    string   `json:"product_id"`
	Count        int      `json:"count"`
}

// epssUpdatedPayload is the NATS event from ai-service.
type epssUpdatedPayload struct {
	CVEID      string  `json:"cve_id"`
	OldScore   float64 `json:"old_score"`
	NewScore   float64 `json:"new_score"`
	Percentile float64 `json:"percentile"`
}

// ── Consumer ──────────────────────────────────────────────────────────────────

// FindingEventConsumer subscribes to cross-service events and triggers notifications.
// Implements the same structural pattern as VulnEventConsumer.
type FindingEventConsumer struct {
	js         jetstream.JetStream
	dispatchUC *dispatch.DispatchUseCase
	log        zerolog.Logger
}

// NewFindingEventConsumer creates a new FindingEventConsumer.
func NewFindingEventConsumer(
	js jetstream.JetStream,
	dispatchUC *dispatch.DispatchUseCase,
	log zerolog.Logger,
) *FindingEventConsumer {
	return &FindingEventConsumer{
		js:         js,
		dispatchUC: dispatchUC,
		log:        log,
	}
}

// Start subscribes to all relevant NATS subjects and blocks until ctx is cancelled.
// Returns nil on clean shutdown (ctx.Err() == context.Canceled).
func (c *FindingEventConsumer) Start(ctx context.Context) error {
	subscriptions := []struct {
		subject  string
		consumer string
		handler  func(jetstream.Msg)
	}{
		{SubjectSLABreached, "notif-sla-breached", c.handleSLABreached},
		{SubjectSLAExpiringSoon, "notif-sla-expiring", c.handleSLAExpiringSoon},
		{SubjectFindingBatchCreated, "notif-finding-batch", c.handleFindingBatchCreated},
		{SubjectFindingStatusChanged, "notif-finding-status", c.handleFindingStatusChanged},
		{SubjectAIEPSSUpdated, "notif-epss-updated", c.handleEPSSUpdated},
	}

	var stops []func()
	for _, s := range subscriptions {
		consumer, err := c.js.CreateOrUpdateConsumer(ctx, "DEFECTDOJO-EVENTS", jetstream.ConsumerConfig{
			Name:           s.consumer,
			FilterSubject:  s.subject,
			DeliverPolicy:  jetstream.DeliverNewPolicy,
			AckPolicy:      jetstream.AckExplicitPolicy,
			MaxDeliver:     5,
			AckWait:        30 * time.Second,
		})
		if err != nil {
			c.log.Warn().Err(err).Str("subject", s.subject).Msg("finding_consumer: create consumer failed (non-fatal)")
			continue
		}

		cc, err := consumer.Consume(s.handler)
		if err != nil {
			c.log.Warn().Err(err).Str("subject", s.subject).Msg("finding_consumer: consume failed (non-fatal)")
			continue
		}
		c.log.Info().Str("subject", s.subject).Msg("finding_consumer: subscribed")
		stops = append(stops, cc.Stop)
	}

	// Block until shutdown
	<-ctx.Done()

	for _, stop := range stops {
		stop()
	}
	return nil
}

// ── Message handlers ──────────────────────────────────────────────────────────

func (c *FindingEventConsumer) handleSLABreached(msg jetstream.Msg) {
	// Extract the inner payload from the CloudEvent wrapper
	var payload slaBreachedPayload
	if err := unmarshalCloudEvent(msg.Data(), &payload); err != nil {
		c.log.Warn().Err(err).Msg("finding_consumer: bad sla_breached payload")
		_ = msg.Nak()
		return
	}

	c.log.Info().
		Str("finding_id", payload.FindingID).
		Int("days_overdue", payload.DaysOverdue).
		Msg("SLA breach notification triggered")

	if err := c.dispatchUC.DispatchSLABreach(context.Background(), dispatch.SLABreachRequest{
		FindingID:   payload.FindingID,
		ProductID:   payload.ProductID,
		Severity:    payload.Severity,
		DaysOverdue: payload.DaysOverdue,
	}); err != nil {
		c.log.Error().Err(err).Msg("finding_consumer: dispatch sla_breach failed")
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

func (c *FindingEventConsumer) handleSLAExpiringSoon(msg jetstream.Msg) {
	var payload slaExpiringSoonPayload
	if err := unmarshalCloudEvent(msg.Data(), &payload); err != nil {
		c.log.Warn().Err(err).Msg("finding_consumer: bad sla_expiring_soon payload")
		_ = msg.Nak()
		return
	}

	c.log.Info().
		Str("finding_id", payload.FindingID).
		Int("days_remaining", payload.DaysRemaining).
		Msg("SLA expiring-soon notification triggered")

	if err := c.dispatchUC.DispatchSLADueSoon(context.Background(), dispatch.SLADueSoonRequest{
		FindingID:     payload.FindingID,
		DaysRemaining: payload.DaysRemaining,
	}); err != nil {
		c.log.Error().Err(err).Msg("finding_consumer: dispatch sla_due_soon failed")
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

func (c *FindingEventConsumer) handleFindingBatchCreated(msg jetstream.Msg) {
	var payload batchCreatedPayload
	if err := unmarshalCloudEvent(msg.Data(), &payload); err != nil {
		c.log.Warn().Err(err).Msg("finding_consumer: bad batch_created payload")
		_ = msg.Nak()
		return
	}

	c.log.Info().
		Str("product_id", payload.ProductID).
		Int("count", payload.Count).
		Msg("finding batch_created notification triggered")

	if err := c.dispatchUC.DispatchScanCompleted(context.Background(), dispatch.ScanCompletedRequest{
		ScanID:       payload.TestID,
		FindingCount: payload.Count,
		ProductID:    payload.ProductID,
	}); err != nil {
		c.log.Error().Err(err).Msg("finding_consumer: dispatch scan_completed failed")
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

func (c *FindingEventConsumer) handleFindingStatusChanged(msg jetstream.Msg) {
	// Status changes are informational — log and ack without dispatching
	// to avoid notification storms. Future: add per-rule opt-in.
	c.log.Debug().Msg("finding_consumer: finding.status_changed (ack only)")
	_ = msg.Ack()
}

func (c *FindingEventConsumer) handleEPSSUpdated(msg jetstream.Msg) {
	var payload epssUpdatedPayload
	if err := unmarshalCloudEvent(msg.Data(), &payload); err != nil {
		c.log.Warn().Err(err).Msg("finding_consumer: bad epss_updated payload")
		_ = msg.Nak()
		return
	}

	// Only notify on EPSS threshold crossing (old ≤ 0.7, new > 0.7)
	if payload.NewScore > epssNotifyThreshold && payload.OldScore <= epssNotifyThreshold {
		c.log.Info().
			Str("cve_id", payload.CVEID).
			Float64("score", payload.NewScore).
			Msg("EPSS spike notification triggered")

		if err := c.dispatchUC.DispatchEPSSSpike(context.Background(), dispatch.EPSSSpikeRequest{
			CVEID:      payload.CVEID,
			OldScore:   payload.OldScore,
			NewScore:   payload.NewScore,
			Percentile: payload.Percentile,
		}); err != nil {
			c.log.Error().Err(err).Msg("finding_consumer: dispatch epss_spike failed")
			_ = msg.Nak()
			return
		}
	}
	_ = msg.Ack()
}

// ── CloudEvent unwrapping ──────────────────────────────────────────────────────

// cloudEventWrapper is the minimal CloudEvent envelope from shared/pkg/nats.
// The actual event payload is in the `data` field.
type cloudEventWrapper struct {
	Data json.RawMessage `json:"data"`
}

// unmarshalCloudEvent tries to unwrap a CloudEvent envelope first,
// then falls back to direct JSON unmarshal (for legacy events).
func unmarshalCloudEvent(raw []byte, target interface{}) error {
	var wrapper cloudEventWrapper
	if err := json.Unmarshal(raw, &wrapper); err == nil && len(wrapper.Data) > 0 {
		return json.Unmarshal(wrapper.Data, target)
	}
	// Fallback: raw payload (non-CloudEvent format)
	return json.Unmarshal(raw, target)
}
