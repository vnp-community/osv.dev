// Package scheduler provides the schedule service's leader-elected cron worker and scheduler.
package scheduler

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/osv/scan-service/internal/domain/schedule/repository"
	"github.com/rs/zerolog"
)

// envDuration parses a duration string from an env var (e.g. "5m", "30s").
// Returns def on missing or invalid input, logging a warning on invalid values.
func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		slog.Warn("invalid duration env var, using default",
			"env_key", key, "value", v, "fallback", def.String())
		return def
	}
	return d
}


// Publisher publishes schedule trigger events.
type Publisher interface {
	PublishTriggerFired(ctx context.Context, scheduleID, userID string, targets []string, scanType string) error
}

// RedisLock implements distributed leader election via Redis SET NX.
type RedisLock interface {
	TryAcquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// CronWorker polls for due schedules using leader election.
// Tick interval and leader TTL are configurable via env vars or fields.
type CronWorker struct {
	lock      RedisLock
	schedRepo repository.ScheduleRepository
	publisher Publisher
	log       zerolog.Logger

	// tickInterval is how often the worker polls for due schedules.
	// Override via SCHEDULER_TICK_INTERVAL env var (e.g. "30s"). Default: 1m.
	tickInterval time.Duration

	// leaderTTL is the Redis lock TTL for leader election.
	// Override via SCHEDULER_LEADER_TTL env var (e.g. "60s"). Default: 90s.
	leaderTTL time.Duration
}

// NewCronWorker creates a CronWorker.
// [FIX BUG-012] tickInterval and leaderTTL are now configurable via env vars:
//   - SCHEDULER_TICK_INTERVAL (default: 1m)
//   - SCHEDULER_LEADER_TTL    (default: 90s)
func NewCronWorker(lock RedisLock, schedRepo repository.ScheduleRepository, publisher Publisher, log zerolog.Logger) *CronWorker {
	return &CronWorker{
		lock:         lock,
		schedRepo:    schedRepo,
		publisher:    publisher,
		log:          log,
		tickInterval: envDuration("SCHEDULER_TICK_INTERVAL", 1*time.Minute),
		leaderTTL:    envDuration("SCHEDULER_LEADER_TTL", 90*time.Second),
	}
}

// Start launches the ticker loop. Blocks until ctx is cancelled.
func (w *CronWorker) Start(ctx context.Context) {
	// [FIX BUG-012] Use w.tickInterval instead of hardcoded 1 * time.Minute
	ticker := time.NewTicker(w.tickInterval)
	defer ticker.Stop()
	w.log.Info().
		Dur("tick_interval", w.tickInterval).
		Dur("leader_ttl", w.leaderTTL).
		Msg("cron worker started")

	for {
		select {
		case <-ctx.Done():
			w.log.Info().Msg("cron worker stopping")
			return
		case now := <-ticker.C:
			// Leader election: only one pod processes at a time
			// [FIX BUG-012] Use w.leaderTTL instead of hardcoded 90*time.Second
			acquired, err := w.lock.TryAcquire(ctx, "schedule:leader", w.leaderTTL)
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
