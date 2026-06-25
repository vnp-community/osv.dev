// Package nats — sla_publisher.go
// SLAEventPublisher publishes SLA breach/due-soon events to NATS JetStream.
// Called by the daily SLACheckerJob scheduler.
//
// Subjects published:
//   defectdojo.sla.breached       → notification-service consuming
//   defectdojo.sla.expiring_soon  → notification-service consuming
package nats

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	natsgo "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

const (
	// SubjectSLABreached is published when a finding's SLA has already passed.
	SubjectSLABreached = "defectdojo.sla.breached"
	// SubjectSLAExpiringSoon is published when a finding's SLA expires within SLAWarningDays.
	SubjectSLAExpiringSoon = "defectdojo.sla.expiring_soon"
)

// SLABreachedEvent is published when a finding's SLA date has passed.
type SLABreachedEvent struct {
	FindingID   uuid.UUID `json:"finding_id"`
	ProductID   uuid.UUID `json:"product_id"`
	Severity    string    `json:"severity"`
	ExpiresAt   time.Time `json:"expires_at"`
	DaysOverdue int       `json:"days_overdue"`
}

// SLADueSoonEvent is published when a finding's SLA is about to expire.
type SLADueSoonEvent struct {
	FindingID     uuid.UUID `json:"finding_id"`
	ProductID     uuid.UUID `json:"product_id"`
	Severity      string    `json:"severity"`
	ExpiresAt     time.Time `json:"expires_at"`
	DaysRemaining int       `json:"days_remaining"`
}

// SLAEventPublisher publishes SLA-related events to NATS JetStream.
// All publishes are fire-and-forget (non-blocking goroutine).
type SLAEventPublisher struct {
	js  natsgo.JetStreamContext
	log zerolog.Logger
}

// NewSLAEventPublisher creates a new SLAEventPublisher.
func NewSLAEventPublisher(js natsgo.JetStreamContext, log zerolog.Logger) *SLAEventPublisher {
	return &SLAEventPublisher{js: js, log: log}
}

// PublishSLABreached publishes a SLABreachedEvent (non-blocking).
func (p *SLAEventPublisher) PublishSLABreached(ctx context.Context, event SLABreachedEvent) {
	go p.publish(SubjectSLABreached, event)
}

// PublishSLADueSoon publishes a SLADueSoonEvent (non-blocking).
func (p *SLAEventPublisher) PublishSLADueSoon(ctx context.Context, event SLADueSoonEvent) {
	go p.publish(SubjectSLAExpiringSoon, event)
}

func (p *SLAEventPublisher) publish(subject string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		p.log.Error().Err(err).Str("subject", subject).Msg("sla_publisher: marshal error")
		return
	}
	if _, err := p.js.Publish(subject, data); err != nil {
		p.log.Warn().Err(err).Str("subject", subject).Msg("sla_publisher: publish failed")
	}
}
