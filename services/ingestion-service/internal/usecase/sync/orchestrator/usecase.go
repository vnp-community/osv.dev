// Package orchestrator coordinates sync from all CVE sources.
package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/ingestion-service/internal/domain/entity"
	"github.com/osv/ingestion-service/internal/domain/repository"
)

// SourceSyncer is implemented by each per-source use case.
type SourceSyncer interface {
	Source() entity.SourceName
	Sync(ctx context.Context) (synced, skipped, errCount int, err error)
}

// Result holds the result of a single source sync.
type Result struct {
	Source  entity.SourceName
	Synced  int
	Skipped int
	Errors  int
	Error   error
	Elapsed time.Duration
}

// OrchestratorResult holds results from all sources.
type OrchestratorResult struct {
	Results []Result
	Total   int
	Elapsed time.Duration
}

// UseCase runs all source syncers in parallel and records jobs.
type UseCase struct {
	syncers  []SourceSyncer
	syncRepo repository.SyncRepository
	log      zerolog.Logger
}

// New creates an orchestrator UseCase.
func New(syncers []SourceSyncer, syncRepo repository.SyncRepository, log zerolog.Logger) *UseCase {
	return &UseCase{
		syncers:  syncers,
		syncRepo: syncRepo,
		log:      log.With().Str("usecase", "sync.orchestrator").Logger(),
	}
}

// SyncAll runs all syncers concurrently and returns aggregated results.
func (uc *UseCase) SyncAll(ctx context.Context) (*OrchestratorResult, error) {
	start := time.Now()
	results := make([]Result, len(uc.syncers))
	var wg sync.WaitGroup

	for i, syncer := range uc.syncers {
		wg.Add(1)
		go func(idx int, s SourceSyncer) {
			defer wg.Done()
			r := uc.runSource(ctx, s)
			results[idx] = r
		}(i, syncer)
	}
	wg.Wait()

	total := 0
	for _, r := range results {
		total += r.Synced
	}

	return &OrchestratorResult{
		Results: results,
		Total:   total,
		Elapsed: time.Since(start),
	}, nil
}

// SyncSource runs a single source by name.
func (uc *UseCase) SyncSource(ctx context.Context, source entity.SourceName) (*Result, error) {
	for _, s := range uc.syncers {
		if s.Source() == source {
			r := uc.runSource(ctx, s)
			return &r, r.Error
		}
	}
	return nil, fmt.Errorf("unknown source: %s", source)
}

func (uc *UseCase) runSource(ctx context.Context, s SourceSyncer) Result {
	start := time.Now()
	uc.log.Info().Str("source", string(s.Source())).Msg("sync start")

	// Create job record
	job, err := uc.syncRepo.CreateJob(ctx, s.Source())
	if err != nil {
		uc.log.Error().Err(err).Str("source", string(s.Source())).Msg("failed to create sync job")
	}

	if job != nil {
		uc.syncRepo.UpdateJobStatus(ctx, job.ID, entity.SyncStatusRunning, &entity.SyncStats{}) //nolint:errcheck
	}

	synced, skipped, errCount, syncErr := s.Sync(ctx)

	status := entity.SyncStatusCompleted
	if syncErr != nil {
		status = entity.SyncStatusFailed
	}

	if job != nil {
		errMsg := ""
		if syncErr != nil {
			errMsg = syncErr.Error()
		}
		uc.syncRepo.UpdateJobStatus(ctx, job.ID, status, &entity.SyncStats{ //nolint:errcheck
			Synced: synced, Skipped: skipped, Errors: errCount, ErrorMsg: errMsg,
		})
	}

	elapsed := time.Since(start)
	uc.log.Info().
		Str("source", string(s.Source())).
		Int("synced", synced).
		Int("skipped", skipped).
		Int("errors", errCount).
		Dur("elapsed", elapsed).
		Err(syncErr).
		Msg("sync done")

	return Result{
		Source:  s.Source(),
		Synced:  synced,
		Skipped: skipped,
		Errors:  errCount,
		Error:   syncErr,
		Elapsed: elapsed,
	}
}
