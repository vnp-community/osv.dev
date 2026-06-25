// Package scheduler provides the cron-based sync scheduler for all data sources.
// Supports both single-source (KEV) and multi-source (Registry) scheduling.
package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"

	"github.com/osv/data-service/internal/fetcher"
	"github.com/osv/data-service/internal/usecase/sync"
)

// Scheduler wraps robfig/cron to run KEV sync and multi-source fetch jobs.
type Scheduler struct {
	cron     *cron.Cron
	syncUC   *sync.UseCase
	registry *fetcher.Registry
	log      zerolog.Logger
}

// New creates a Scheduler. Use Start() to begin scheduling.
func New(syncUC *sync.UseCase, log zerolog.Logger) *Scheduler {
	return &Scheduler{
		cron:   cron.New(cron.WithSeconds()),
		syncUC: syncUC,
		log:    log.With().Str("component", "kev.scheduler").Logger(),
	}
}

// NewWithRegistry creates a Scheduler with a multi-source Registry.
// Enables scheduling all registered fetchers in addition to KEV sync.
func NewWithRegistry(syncUC *sync.UseCase, reg *fetcher.Registry, log zerolog.Logger) *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithSeconds()),
		syncUC:   syncUC,
		registry: reg,
		log:      log.With().Str("component", "scheduler").Logger(),
	}
}

// Start registers cron jobs and starts the scheduler.
//
// Schedule:
//   - KEV sync: every 6 hours
//   - CIRCL/JVN/ExploitDB (incremental): daily at 02:00 UTC
//   - CVE.org (incremental, last 7 days): daily at 03:00 UTC
//   - CNNVD (incremental): daily at 04:00 UTC
func (s *Scheduler) Start() {
	// KEV sync: every 6 hours
	s.cron.AddFunc("0 0 */6 * * *", func() { //nolint:errcheck
		s.runKEVSync()
	})

	// Multi-source incremental syncs (only if registry provided)
	if s.registry != nil {
		schedules := map[string]string{
			fetcher.SourceNVD.String():       "0 0 */2 * * *",  // every 2h
			fetcher.SourceEPSS.String():      "0 0 3 * * *",    // daily 3am
			fetcher.SourceNVDCPE.String():    "0 0 4 * * 0",    // Sunday 4am
			fetcher.SourceCAPEC.String():     "0 0 5 * * 0",    // Sunday 5am
			fetcher.SourceCWE.String():       "0 0 5 * * 0",    // Sunday 5am
			fetcher.SourceCIRCL.String():     "0 0 */6 * * *",  // every 6h
			fetcher.SourceJVN.String():       "0 0 * * * *",    // every 1h
			fetcher.SourceExploitDB.String(): "0 0 2 * * *",    // daily 2am
			fetcher.SourceCVEOrg.String():    "0 0 */12 * * *", // every 12h
			fetcher.SourceCNNVD.String():     "0 0 */12 * * *", // every 12h
		}

		for src, cronExpr := range schedules {
			sourceName := src // capture loop variable
			s.cron.AddFunc(cronExpr, func() { //nolint:errcheck
				s.runSourceSync(sourceName, 0)
			})
		}
	}

	s.cron.Start()
	s.log.Info().Msg("Scheduler started (KEV: 6h, sources: registered map)")
}

// Stop gracefully stops the scheduler and waits for running jobs to complete.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(30 * time.Second):
		s.log.Warn().Msg("Scheduler stop timed out")
	}
}

// RunNow triggers an immediate KEV sync (used for startup sync if configured).
func (s *Scheduler) RunNow() {
	go s.runKEVSync()
}

// RunSourceNow triggers an immediate sync for a specific source by name.
func (s *Scheduler) RunSourceNow(sourceName string, manualDays int) {
	go s.runSourceSync(sourceName, manualDays)
}

func (s *Scheduler) runKEVSync() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	s.log.Info().Msg("Scheduled KEV sync started")
	result, err := s.syncUC.Sync(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("Scheduled KEV sync failed")
		return
	}
	s.log.Info().
		Int("total", result.Total).
		Int("inserted", result.Inserted).
		Int("updated", result.Updated).
		Dur("duration", result.Duration).
		Msg("Scheduled KEV sync completed")
}

func (s *Scheduler) runSourceSync(sourceName string, manualDays int) {
	if s.registry == nil {
		return
	}

	f, ok := s.registry.Get(sourceName)
	if !ok {
		s.log.Warn().Str("source", sourceName).Msg("Scheduler: fetcher not registered")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	start := time.Now()
	s.log.Info().Str("source", sourceName).Int("manual_days", manualDays).Msg("Source sync started")

	count, err := f.FetchAndStore(ctx, fetcher.FetchOptions{ManualDays: manualDays})
	if err != nil {
		s.log.Error().Err(err).Str("source", sourceName).Msg("Source sync failed")
		return
	}

	s.log.Info().
		Str("source", sourceName).
		Int("count", count).
		Dur("duration", time.Since(start)).
		Msg("Source sync completed")
}
