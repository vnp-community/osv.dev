// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package scheduler provides a cron-style scheduler for per-source sync intervals.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	synccmd "github.com/osv/source-sync/internal/application/command/sync_source"
)

// SourceSchedule holds the scheduling config for a single source.
type SourceSchedule struct {
	SourceName string
	Interval   time.Duration
	NextRun    time.Time
}

// CronScheduler polls every 30 seconds and launches sync goroutines as needed.
type CronScheduler struct {
	syncHandler *synccmd.Handler
	sources     []*SourceSchedule
	pollInterval time.Duration
}

// NewCronScheduler creates a scheduler with the given source schedules.
func NewCronScheduler(syncHandler *synccmd.Handler, sources []*SourceSchedule) *CronScheduler {
	return &CronScheduler{
		syncHandler:  syncHandler,
		sources:      sources,
		pollInterval: 30 * time.Second,
	}
}

// Start begins the scheduling loop. Blocks until ctx is cancelled.
func (s *CronScheduler) Start(ctx context.Context) {
	log.Ctx(ctx).Info().Int("sources", len(s.sources)).Msg("scheduler starting")

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Ctx(ctx).Info().Msg("scheduler stopping")
			return
		case <-ticker.C:
			s.checkAndRun(ctx)
		}
	}
}

func (s *CronScheduler) checkAndRun(ctx context.Context) {
	now := time.Now()
	for _, sched := range s.sources {
		if now.Before(sched.NextRun) {
			continue
		}
		// Update next run time before launching goroutine
		sched.NextRun = now.Add(sched.Interval)

		go func(sourceName string) {
			syncCtx := log.Ctx(ctx).With().Str("source", sourceName).Logger().WithContext(ctx)
			result, err := s.syncHandler.Handle(syncCtx, synccmd.Command{SourceName: sourceName})
			if err != nil {
				log.Ctx(syncCtx).Error().Err(err).Msg("scheduled sync failed")
				return
			}
			log.Ctx(syncCtx).Info().
				Int("modified", result.Modified).
				Int("deleted", result.Deleted).
				Dur("duration", result.Duration).
				Msg("scheduled sync completed")
		}(sched.SourceName)
	}
}

// ParseInterval parses a duration string like "5m", "1h", "30s".
func ParseInterval(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid sync interval %q: %w", s, err)
	}
	if d < time.Minute {
		return 0, fmt.Errorf("sync interval %q is too short (minimum 1m)", s)
	}
	return d, nil
}
