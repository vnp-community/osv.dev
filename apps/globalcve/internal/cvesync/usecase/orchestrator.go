// Package usecase — CVE Sync Orchestrator.
// Orchestrates parallel sync across all configured data sources.
package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/globalcve/mono/internal/cvesync/domain/entity"
	"github.com/globalcve/mono/internal/cvesync/domain/repository"
	"github.com/globalcve/mono/internal/cvesync/fetcher"
)

// SyncResult holds the result of syncing one source.
type SyncResult struct {
	Source  entity.SourceName
	Synced  int
	Elapsed time.Duration
	Err     error
}

// Orchestrator manages all CVE data source syncs.
type Orchestrator struct {
	fetchers []fetcher.Fetcher
	syncRepo repository.SyncJobRepository
}

// NewOrchestrator creates a new sync orchestrator.
func NewOrchestrator(fetchers []fetcher.Fetcher, syncRepo repository.SyncJobRepository) *Orchestrator {
	return &Orchestrator{
		fetchers: fetchers,
		syncRepo: syncRepo,
	}
}

// SyncAll runs all fetchers in parallel (one goroutine per fetcher).
// Each sync job is tracked in the database.
func (o *Orchestrator) SyncAll(ctx context.Context) ([]*SyncResult, error) {
	results := make([]*SyncResult, len(o.fetchers))
	resultCh := make(chan *SyncResult, len(o.fetchers))

	g, gctx := errgroup.WithContext(ctx)
	for _, f := range o.fetchers {
		f := f // capture
		g.Go(func() error {
			result := o.syncOne(gctx, f, fetcher.FetchOptions{})
			resultCh <- result
			return nil // errors are captured in SyncResult.Err
		})
	}

	// Wait for all goroutines
	if err := g.Wait(); err != nil {
		return nil, err
	}
	close(resultCh)

	i := 0
	for r := range resultCh {
		results[i] = r
		i++
	}

	return results, nil
}

// SyncSource runs a single fetcher by source name.
func (o *Orchestrator) SyncSource(ctx context.Context, source entity.SourceName) (*SyncResult, error) {
	for _, f := range o.fetchers {
		if f.Source() == source {
			return o.syncOne(ctx, f, fetcher.FetchOptions{}), nil
		}
	}
	return nil, fmt.Errorf("unknown source: %s", source)
}

// SyncRecent runs all fetchers with a "since" time filter.
func (o *Orchestrator) SyncRecent(ctx context.Context, since time.Time) ([]*SyncResult, error) {
	results := make([]*SyncResult, 0, len(o.fetchers))
	resultCh := make(chan *SyncResult, len(o.fetchers))

	g, gctx := errgroup.WithContext(ctx)
	for _, f := range o.fetchers {
		f := f
		g.Go(func() error {
			opts := fetcher.FetchOptions{Since: &since}
			resultCh <- o.syncOne(gctx, f, opts)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	close(resultCh)

	for r := range resultCh {
		results = append(results, r)
	}
	return results, nil
}

// syncOne runs a single fetcher and records the sync job.
func (o *Orchestrator) syncOne(ctx context.Context, f fetcher.Fetcher, opts fetcher.FetchOptions) *SyncResult {
	start := time.Now()
	source := f.Source()

	// Create sync job record
	job := &entity.SyncJob{
		Source:    source,
		Status:    entity.SyncStatusRunning,
		StartedAt: start,
	}
	if err := o.syncRepo.Create(ctx, job); err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("source", string(source)).Msg("sync: failed to create job record")
	}

	// Run the fetcher
	n, fetchErr := f.Fetch(ctx, opts)
	elapsed := time.Since(start)

	// Update job status
	status := entity.SyncStatusCompleted
	stats := entity.SyncStats{Synced: n}
	if fetchErr != nil {
		status = entity.SyncStatusFailed
		stats.Errors = 1
		stats.ErrorMsg = fetchErr.Error()
		log.Ctx(ctx).Error().Err(fetchErr).Str("source", string(source)).Dur("elapsed", elapsed).Msg("sync: FAILED")
	} else {
		log.Ctx(ctx).Info().
			Str("source", string(source)).
			Int("synced", n).
			Dur("elapsed", elapsed).
			Msg("sync: COMPLETED")
	}

	if job.ID > 0 {
		if err := o.syncRepo.UpdateStatus(ctx, job.ID, status, stats); err != nil {
			log.Ctx(ctx).Warn().Err(err).Str("source", string(source)).Msg("sync: failed to update job status")
		}
	}

	return &SyncResult{
		Source:  source,
		Synced:  n,
		Elapsed: elapsed,
		Err:     fetchErr,
	}
}
