// Package scheduler — sla_checker.go
// SLACheckerJob runs daily to find SLA breaches and upcoming expirations,
// then publishes events to NATS for notification-service to consume.
//
// Usage:
//   job := scheduler.NewSLACheckerJob(findingRepo, slaPublisher, logger)
//   go job.Start(ctx)  // starts daily ticker, runs once immediately
package scheduler

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/finding-service/internal/domain/finding"
	natsinfra "github.com/osv/finding-service/internal/infra/messaging/nats"
)

// SLAWarningDays defines how many days before breach to send a warning.
const SLAWarningDays = 3

// FindingRepo is the minimal interface needed by SLACheckerJob.
// FindingRepo (postgres) implements both methods via sla_queries.go.
type FindingRepo interface {
	FindSLABreached(ctx context.Context) ([]finding.Finding, error)
	FindSLADueSoon(ctx context.Context, deadline time.Time) ([]finding.Finding, error)
}

// SLACheckerJob checks for SLA breaches and upcoming expirations.
// Results are published to NATS JetStream.
type SLACheckerJob struct {
	repo      FindingRepo
	publisher *natsinfra.SLAEventPublisher
	log       zerolog.Logger
}

// NewSLACheckerJob creates a new SLACheckerJob.
func NewSLACheckerJob(repo FindingRepo, publisher *natsinfra.SLAEventPublisher, log zerolog.Logger) *SLACheckerJob {
	return &SLACheckerJob{repo: repo, publisher: publisher, log: log}
}

// Start runs the SLA checker once immediately, then every 24 hours.
// Blocks until ctx is cancelled.
func (j *SLACheckerJob) Start(ctx context.Context) {
	j.log.Info().Msg("sla_checker: starting (runs immediately then every 24h)")
	j.Run(ctx)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			j.Run(ctx)
		case <-ctx.Done():
			j.log.Info().Msg("sla_checker: stopped")
			return
		}
	}
}

// Run executes one SLA check pass. Safe to call directly for testing.
func (j *SLACheckerJob) Run(ctx context.Context) {
	j.log.Info().Msg("sla_checker: starting daily SLA check")
	now := time.Now().UTC()
	checked := 0

	// ── Check 1: Already breached ────────────────────────────────────────────
	breached, err := j.repo.FindSLABreached(ctx)
	if err != nil {
		j.log.Error().Err(err).Msg("sla_checker: FindSLABreached failed")
	} else {
		for _, f := range breached {
			if f.SLAExpirationDate == nil {
				continue
			}
			daysOverdue := int(now.Sub(*f.SLAExpirationDate).Hours() / 24)

			j.publisher.PublishSLABreached(ctx, natsinfra.SLABreachedEvent{
				FindingID:   f.ID,
				ProductID:   f.ProductID,
				Severity:    string(f.Severity),
				ExpiresAt:   *f.SLAExpirationDate,
				DaysOverdue: daysOverdue,
			})

			j.log.Warn().
				Str("finding_id", f.ID.String()).
				Int("days_overdue", daysOverdue).
				Msg("sla_checker: breach published")
			checked++
		}
	}

	// ── Check 2: Due soon (within SLAWarningDays) ────────────────────────────
	warnDeadline := now.Add(time.Duration(SLAWarningDays) * 24 * time.Hour)
	dueSoon, err := j.repo.FindSLADueSoon(ctx, warnDeadline)
	if err != nil {
		j.log.Error().Err(err).Msg("sla_checker: FindSLADueSoon failed")
	} else {
		for _, f := range dueSoon {
			if f.SLAExpirationDate == nil {
				continue
			}
			daysRemaining := int(time.Until(*f.SLAExpirationDate).Hours() / 24)

			j.publisher.PublishSLADueSoon(ctx, natsinfra.SLADueSoonEvent{
				FindingID:     f.ID,
				ProductID:     f.ProductID,
				Severity:      string(f.Severity),
				ExpiresAt:     *f.SLAExpirationDate,
				DaysRemaining: daysRemaining,
			})

			j.log.Info().
				Str("finding_id", f.ID.String()).
				Int("days_remaining", daysRemaining).
				Msg("sla_checker: due-soon published")
			checked++
		}
	}

	j.log.Info().Int("published", checked).Msg("sla_checker: daily check complete")
}
