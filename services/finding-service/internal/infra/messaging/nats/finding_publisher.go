// Package nats — finding_publisher.go
// FindingEventPublisher publishes finding lifecycle events to NATS JetStream.
//
// Uses the same natsutil.Publisher pattern as the existing use cases
// (BatchCreateFindingsUseCase, StatusTransitionUseCase, etc.) — wraps
// shared/pkg/nats.Publisher to provide typed, subject-constant event methods.
//
// Subjects published (extend existing defectdojo namespace):
//   - defectdojo.finding.created        (new — per-finding, not batch)
//   - defectdojo.finding.status_changed  (already used by StatusTransitionUseCase)
//   - defectdojo.finding.risk_accepted   (new)
//   - defectdojo.sla.breached           (already used by CheckBreachesUseCase)
//   - defectdojo.sla.expiring_soon      (already used by CheckBreachesUseCase)
//
// These match SubjectFindingCreated / SubjectFindingStatusChanged already defined
// in bootstrap.go — no duplicate constants needed.
package nats

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	natsutil "github.com/osv/shared/pkg/nats"
)

// ── New typed subjects (complement bootstrap.go) ─────────────────────────────

const (
	// SubjectFindingRiskAccepted is published when risk is accepted for a finding.
	SubjectFindingRiskAccepted = "defectdojo.finding.risk_accepted"
)

// ── Typed event payloads ─────────────────────────────────────────────────────

// FindingCreatedEvent is published when a single finding is created.
// (BatchCreateFindingsUseCase already publishes defectdojo.finding.batch_created)
type FindingCreatedEvent struct {
	FindingID    uuid.UUID `json:"finding_id"`
	ProductID    uuid.UUID `json:"product_id"`
	EngagementID uuid.UUID `json:"engagement_id"`
	TestID       uuid.UUID `json:"test_id"`
	Severity     string    `json:"severity"`
	Title        string    `json:"title"`
	CVE          string    `json:"cve,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// FindingStatusChangedEvent is published when a finding transitions states.
// Complements the existing map[string]interface{} payload already used in
// StatusTransitionUseCase.transition() — this provides a typed equivalent.
type FindingStatusChangedEvent struct {
	FindingID uuid.UUID `json:"finding_id"`
	ProductID uuid.UUID `json:"product_id"`
	OldState  string    `json:"old_state"`
	NewState  string    `json:"new_state"`
	ChangedBy string    `json:"changed_by,omitempty"`
	ChangedAt time.Time `json:"changed_at"`
}

// FindingRiskAcceptedEvent is published when risk is accepted for a finding.
type FindingRiskAcceptedEvent struct {
	FindingID     uuid.UUID  `json:"finding_id"`
	ProductID     uuid.UUID  `json:"product_id"`
	AcceptedBy    string     `json:"accepted_by,omitempty"`
	Justification string     `json:"justification,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	AcceptedAt    time.Time  `json:"accepted_at"`
}

// ── Publisher ─────────────────────────────────────────────────────────────────

// FindingEventPublisher wraps natsutil.Publisher to provide typed finding event methods.
// It is ADDITIVE — the existing *natsutil.Publisher in use cases is unchanged.
type FindingEventPublisher struct {
	p   *natsutil.Publisher
	log zerolog.Logger
}

// NewFindingEventPublisher creates a FindingEventPublisher.
// Use the same *natsutil.Publisher that is shared across use cases.
func NewFindingEventPublisher(p *natsutil.Publisher, log zerolog.Logger) *FindingEventPublisher {
	return &FindingEventPublisher{p: p, log: log}
}

// PublishCreated publishes a FindingCreatedEvent (non-blocking).
func (fp *FindingEventPublisher) PublishCreated(ctx context.Context, event FindingCreatedEvent) {
	go fp.publish(ctx, SubjectFindingCreated, event)
}

// PublishStatusChanged publishes a FindingStatusChangedEvent (non-blocking).
func (fp *FindingEventPublisher) PublishStatusChanged(ctx context.Context, event FindingStatusChangedEvent) {
	go fp.publish(ctx, SubjectFindingStatusChanged, event)
}

// PublishRiskAccepted publishes a FindingRiskAcceptedEvent (non-blocking).
func (fp *FindingEventPublisher) PublishRiskAccepted(ctx context.Context, event FindingRiskAcceptedEvent) {
	go fp.publish(ctx, SubjectFindingRiskAccepted, event)
}

func (fp *FindingEventPublisher) publish(ctx context.Context, subject string, payload interface{}) {
	if err := fp.p.Publish(ctx, subject, payload); err != nil {
		fp.log.Warn().Err(err).Str("subject", subject).Msg("finding_publisher: publish failed")
	}
}
