// Package scheduler — CVE Sync cron-based scheduler.
package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/config"
	"github.com/globalcve/mono/internal/cvesync/domain/entity"
	"github.com/globalcve/mono/internal/cvesync/usecase"
)

// Scheduler manages cron-based CVE sync jobs.
type Scheduler struct {
	cron         *cron.Cron
	orchestrator *usecase.Orchestrator
	cfg          config.SchedulerConfig
}

// New creates a new CVE sync scheduler.
func New(orchestrator *usecase.Orchestrator, cfg config.SchedulerConfig) *Scheduler {
	c := cron.New(cron.WithSeconds())
	return &Scheduler{
		cron:         c,
		orchestrator: orchestrator,
		cfg:          cfg,
	}
}

// Start registers all cron jobs and starts the scheduler.
func (s *Scheduler) Start(ctx context.Context) {
	if !s.cfg.Enabled {
		log.Ctx(ctx).Info().Msg("scheduler: disabled, skipping")
		return
	}

	// Helper to wrap source sync in a timeout context
	syncSource := func(source entity.SourceName) func() {
		return func() {
			syncCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
			defer cancel()
			result, err := s.orchestrator.SyncSource(syncCtx, source)
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Str("source", string(source)).Msg("scheduler: sync error")
				return
			}
			log.Ctx(ctx).Info().
				Str("source", string(source)).
				Int("synced", result.Synced).
				Dur("elapsed", result.Elapsed).
				Msg("scheduler: sync complete")
		}
	}

	schedules := []struct {
		expr   string
		source entity.SourceName
	}{
		{s.cfg.NVDCVECron, entity.SourceNameNVD},       // every 2h
		{s.cfg.JVNCron, entity.SourceNameJVN},           // every 1h
		{s.cfg.CIRCLCron, entity.SourceNameCIRCL},       // every 6h
		{s.cfg.ExploitDBCron, entity.SourceNameExploitDB}, // daily 2am
		{s.cfg.CVEOrgCron, entity.SourceNameCVEOrg},     // every 12h
		{s.cfg.EPSSCron, entity.SourceNameEPSS},         // daily 3am
		{s.cfg.CPECron, entity.SourceNameNVDCPE},        // weekly
		{s.cfg.CAPECCWECron, entity.SourceNameCAPEC},    // weekly
	}

	for _, sc := range schedules {
		if sc.expr == "" {
			continue
		}
		if _, err := s.cron.AddFunc(sc.expr, syncSource(sc.source)); err != nil {
			log.Ctx(ctx).Error().Err(err).
				Str("cron", sc.expr).
				Str("source", string(sc.source)).
				Msg("scheduler: failed to register cron")
		} else {
			log.Ctx(ctx).Info().
				Str("cron", sc.expr).
				Str("source", string(sc.source)).
				Msg("scheduler: registered")
		}
	}

	s.cron.Start()
	log.Ctx(ctx).Info().Msg("scheduler: started")
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	if s.cron != nil {
		<-s.cron.Stop().Done()
	}
}
