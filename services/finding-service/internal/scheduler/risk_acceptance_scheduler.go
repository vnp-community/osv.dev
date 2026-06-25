// Package scheduler provides cron-based scheduled tasks for finding-service.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	rauc "github.com/osv/finding-service/internal/usecase/riskacceptance"
)

// RiskAcceptanceScheduler runs the daily expiry check for risk acceptances.
type RiskAcceptanceScheduler struct {
	expireUC *rauc.ExpireRiskAcceptancesUseCase
	// runAt is the time-of-day (UTC) to run the job (default 06:00)
	runAt time.Duration
}

// NewRiskAcceptanceScheduler creates a scheduler that runs at 06:00 UTC daily.
func NewRiskAcceptanceScheduler(uc *rauc.ExpireRiskAcceptancesUseCase) *RiskAcceptanceScheduler {
	return &RiskAcceptanceScheduler{
		expireUC: uc,
		runAt:    6 * time.Hour, // 06:00 UTC
	}
}

// Run starts the daily scheduler loop. Blocks until ctx is cancelled.
// Use go scheduler.Run(ctx) in main.
func (s *RiskAcceptanceScheduler) Run(ctx context.Context) {
	slog.Info("risk acceptance scheduler started", "runs_at", "06:00 UTC daily")
	for {
		next := nextRunTime(s.runAt)
		slog.Info("next risk acceptance expiry check", "at", next.UTC().Format(time.RFC3339))

		select {
		case <-ctx.Done():
			slog.Info("risk acceptance scheduler stopped")
			return
		case <-time.After(time.Until(next)):
			s.runExpiry(ctx)
		}
	}
}

func (s *RiskAcceptanceScheduler) runExpiry(ctx context.Context) {
	slog.Info("running risk acceptance expiry check")
	expireCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := s.expireUC.Execute(expireCtx); err != nil {
		slog.Error("risk acceptance expiry check failed", "error", err)
		return
	}
	slog.Info("risk acceptance expiry check completed")
}

// nextRunTime calculates the next occurrence of runAt (duration since midnight UTC).
func nextRunTime(runAt time.Duration) time.Time {
	now := time.Now().UTC()
	// Midnight UTC today
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	next := midnight.Add(runAt)
	if next.Before(now) {
		// If we've already passed today's run time, schedule for tomorrow
		next = next.Add(24 * time.Hour)
	}
	return next
}
