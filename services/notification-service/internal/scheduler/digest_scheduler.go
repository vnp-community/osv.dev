// Package scheduler — digest_scheduler.go
// DigestScheduler runs daily (08:00 UTC) and weekly (Monday 08:00 UTC) digest jobs.
// S3-NOTIF-02: additive, existing scheduler/finding_consumer.go is unchanged.
package scheduler

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/notification-service/internal/usecase/send_digest"
)

// DigestScheduler triggers daily and weekly digest dispatches.
type DigestScheduler struct {
	uc  *send_digest.SendDigestUseCase
	log zerolog.Logger
}

// NewDigestScheduler creates a DigestScheduler.
func NewDigestScheduler(uc *send_digest.SendDigestUseCase, log zerolog.Logger) *DigestScheduler {
	return &DigestScheduler{uc: uc, log: log}
}

// Start launches the scheduler in the background.
// Call this from main.go after wiring.
func (s *DigestScheduler) Start(ctx context.Context) {
	go s.run(ctx)
}

func (s *DigestScheduler) run(ctx context.Context) {
	for {
		now := time.Now().UTC()
		nextDaily := nextOccurrence(now, 8, 0)   // next 08:00 UTC
		nextWeekly := nextMonday(now, 8, 0)       // next Monday 08:00 UTC

		// Use the nearest trigger
		wait := nextDaily
		if nextWeekly.Before(wait) {
			wait = nextWeekly
		}

		delay := time.Until(wait)
		s.log.Info().Str("next_trigger", wait.Format(time.RFC3339)).Msg("digest: next scheduled at")

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		now = time.Now().UTC()

		// Run daily digest
		s.log.Info().Msg("digest: running daily digest")
		if err := s.uc.Execute(ctx, send_digest.DigestTypeDaily); err != nil {
			s.log.Error().Err(err).Msg("digest: daily digest failed")
		}

		// Run weekly digest on Monday
		if now.Weekday() == time.Monday {
			s.log.Info().Msg("digest: running weekly digest")
			if err := s.uc.Execute(ctx, send_digest.DigestTypeWeekly); err != nil {
				s.log.Error().Err(err).Msg("digest: weekly digest failed")
			}
		}
	}
}

// nextOccurrence returns the next wall-clock time for hour:minute UTC.
func nextOccurrence(from time.Time, hour, minute int) time.Time {
	t := time.Date(from.Year(), from.Month(), from.Day(), hour, minute, 0, 0, time.UTC)
	if !t.After(from) {
		t = t.Add(24 * time.Hour)
	}
	return t
}

// nextMonday returns the next Monday at hour:minute UTC.
func nextMonday(from time.Time, hour, minute int) time.Time {
	t := time.Date(from.Year(), from.Month(), from.Day(), hour, minute, 0, 0, time.UTC)
	daysUntilMonday := (int(time.Monday) - int(t.Weekday()) + 7) % 7
	if daysUntilMonday == 0 && !t.After(from) {
		daysUntilMonday = 7
	}
	return t.AddDate(0, 0, daysUntilMonday)
}
