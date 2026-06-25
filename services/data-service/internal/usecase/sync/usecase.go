// Package sync provides the CISA KEV catalog synchronization use case.
package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	entity "github.com/osv/data-service/internal/domain/kev"
	"github.com/osv/data-service/internal/domain/repository"
)

// CISAClient is the port interface for fetching the KEV catalog.
type CISAClient interface {
	FetchKEVCatalog(ctx context.Context) ([]*entity.KEVEntry, error)
}

// SyncResult holds statistics from a catalog sync run.
type SyncResult struct {
	Total      int               `json:"total"`
	Inserted   int               `json:"inserted"`
	Updated    int               `json:"updated"`
	Removed    int               `json:"removed"` // CR-GCV-007: entries removed from CISA catalog
	Duration   time.Duration     `json:"duration_ms"`
	SyncedAt   time.Time         `json:"synced_at"`
	NewEntries []*entity.KEVEntry `json:"-"` // Track new entries for NATS publishing
}

// KEVPublisher interface for NATS JetStream event publishing.
type KEVPublisher interface {
	PublishNewKEVBatch(ctx context.Context, entries []*entity.KEVEntry) error
}

// UseCase orchestrates fetching from CISA and persisting to the repository.
type UseCase struct {
	cisaClient  CISAClient
	kevRepo     repository.KEVRepository
	publisher   KEVPublisher // optional NATS publisher
	enableDiff  bool         // CR-GCV-007: enable diff-based removal of stale entries
	log         zerolog.Logger
}

// New creates a sync UseCase.
func New(cisaClient CISAClient, kevRepo repository.KEVRepository, publisher KEVPublisher, log zerolog.Logger) *UseCase {
	return &UseCase{
		cisaClient: cisaClient,
		kevRepo:    kevRepo,
		publisher:  publisher,
		enableDiff: true, // enabled by default
		log:        log.With().Str("usecase", "kev.sync").Logger(),
	}
}

// WithDiff enables or disables diff-based removal of stale KEV entries.
func (uc *UseCase) WithDiff(enabled bool) *UseCase {
	uc.enableDiff = enabled
	return uc
}

// Sync downloads the CISA KEV catalog and upserts all entries into the database.
// CR-GCV-007: Also removes entries that CISA removed from the catalog (diff-based).
func (uc *UseCase) Sync(ctx context.Context) (*SyncResult, error) {
	start := time.Now()
	uc.log.Info().Msg("Starting KEV catalog sync")

	// 1. Fetch from CISA.
	entries, err := uc.cisaClient.FetchKEVCatalog(ctx)
	if err != nil {
		uc.log.Error().Err(err).Msg("CISA fetch failed")
		return nil, fmt.Errorf("kev sync: fetch: %w", err)
	}
	uc.log.Info().Int("count", len(entries)).Msg("Fetched KEV entries from CISA")

	// 2. Diff detection (Find existing entries)
	existingIDs, err := uc.kevRepo.GetAllIDs(ctx)
	if err != nil {
		uc.log.Error().Err(err).Msg("failed to get existing KEV IDs")
		return nil, fmt.Errorf("kev sync: get existing ids: %w", err)
	}
	existingSet := make(map[string]bool, len(existingIDs))
	for _, id := range existingIDs {
		existingSet[id] = true
	}

	var newEntries []*entity.KEVEntry
	for _, e := range entries {
		if !existingSet[e.CVEID] {
			newEntries = append(newEntries, e)
		}
	}

	// 3. Upsert into PostgreSQL.
	inserted, updated, err := uc.kevRepo.UpsertBatch(ctx, entries)
	if err != nil {
		uc.log.Error().Err(err).Msg("KEV upsert failed")
		return nil, fmt.Errorf("kev sync: upsert: %w", err)
	}

	result := &SyncResult{
		Total:      len(entries),
		Inserted:   inserted,
		Updated:    updated,
		Duration:   time.Since(start),
		SyncedAt:   time.Now().UTC(),
		NewEntries: newEntries,
	}

	// 4. Publish NATS events for new entries
	if len(newEntries) > 0 && uc.publisher != nil {
		go func() {
			pubCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := uc.publisher.PublishNewKEVBatch(pubCtx, newEntries); err != nil {
				uc.log.Warn().Err(err).Int("count", len(newEntries)).Msg("kev: NATS publish failed (non-fatal)")
			}
		}()
	}

	// 5. CR-GCV-007: Diff-based removal — delete entries not in current CISA catalog.
	if uc.enableDiff && len(entries) > 0 {
		currentIDs := make([]string, 0, len(entries))
		for _, e := range entries {
			currentIDs = append(currentIDs, e.CVEID)
		}

		removed, err := uc.kevRepo.DeleteByIDs(ctx, currentIDs)
		if err != nil {
			// Non-fatal: log and continue. Stale entries won't cause harm.
			uc.log.Warn().Err(err).Msg("KEV diff removal failed (non-fatal)")
		} else {
			result.Removed = removed
			if removed > 0 {
				uc.log.Info().Int("removed", removed).Msg("KEV diff: removed stale entries")
			}
		}
	}

	uc.log.Info().
		Int("inserted", inserted).
		Int("updated", updated).
		Int("removed", result.Removed).
		Int("new_kev_entries", len(newEntries)).
		Dur("duration", result.Duration).
		Msg("KEV sync completed")

	return result, nil
}

