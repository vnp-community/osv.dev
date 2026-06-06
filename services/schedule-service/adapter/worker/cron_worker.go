// Package cronworker provides the schedule service's leader-elected cron worker.
package cronworker

import (
	"context"
	"time"

	"github.com/osv/schedule-service/internal/domain/repository"
	"github.com/rs/zerolog"
)

// Publisher publishes schedule trigger events.
type Publisher interface {
	PublishTriggerFired(ctx context.Context, scheduleID, userID string, targets []string, scanType string) error
}

// RedisLock implements distributed leader election via Redis SET NX.
type RedisLock interface {
	TryAcquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// CronWorker polls for due schedules every minute using leader election.
type CronWorker struct {
	lock      RedisLock
	schedRepo repository.ScheduleRepository
	publisher Publisher
	log       zerolog.Logger
}

// NewCronWorker creates a CronWorker.
func NewCronWorker(lock RedisLock, schedRepo repository.ScheduleRepository, publisher Publisher, log zerolog.Logger) *CronWorker {
	return &CronWorker{lock: lock, schedRepo: schedRepo, publisher: publisher, log: log}
}

// Start launches the 1-minute ticker loop. Blocks until ctx is cancelled.
func (w *CronWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	w.log.Info().Msg("cron worker started")

	for {
		select {
		case <-ctx.Done():
			w.log.Info().Msg("cron worker stopping")
			return
		case now := <-ticker.C:
			// Leader election: only one pod processes at a time
			acquired, err := w.lock.TryAcquire(ctx, "schedule:leader", 90*time.Second)
			if err != nil || !acquired {
				continue
			}
			w.processDue(ctx, now)
		}
	}
}

func (w *CronWorker) processDue(ctx context.Context, now time.Time) {
	due, err := w.schedRepo.FindDue(ctx, now)
	if err != nil {
		w.log.Error().Err(err).Msg("find due schedules failed")
		return
	}

	for _, s := range due {
		if err := w.publisher.PublishTriggerFired(ctx, s.ID.String(), s.UserID.String(), s.Targets, s.ScanType); err != nil {
			w.log.Error().Err(err).Str("schedule_id", s.ID.String()).Msg("publish trigger failed")
			continue
		}

		// Update next run time
		nextRun := s.CalculateNextRun(now)
		if err := w.schedRepo.UpdateNextRun(ctx, s.ID, nextRun); err != nil {
			w.log.Error().Err(err).Str("schedule_id", s.ID.String()).Msg("update next run failed")
		}
	}

	if len(due) > 0 {
		w.log.Info().Int("count", len(due)).Time("tick", now).Msg("processed due schedules")
	}
}
