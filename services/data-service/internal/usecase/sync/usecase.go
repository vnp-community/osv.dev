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
	Total    int           `json:"total"`
	Inserted int           `json:"inserted"`
	Updated  int           `json:"updated"`
	Duration time.Duration `json:"duration_ms"`
	SyncedAt time.Time     `json:"synced_at"`
}

// UseCase orchestrates fetching from CISA and persisting to the repository.
type UseCase struct {
	cisaClient CISAClient
	kevRepo    repository.KEVRepository
	log        zerolog.Logger
}

// New creates a sync UseCase.
func New(cisaClient CISAClient, kevRepo repository.KEVRepository, log zerolog.Logger) *UseCase {
	return &UseCase{
		cisaClient: cisaClient,
		kevRepo:    kevRepo,
		log:        log.With().Str("usecase", "kev.sync").Logger(),
	}
}

// Sync downloads the CISA KEV catalog and upserts all entries into the database.
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

	// 2. Upsert into PostgreSQL.
	inserted, updated, err := uc.kevRepo.UpsertBatch(ctx, entries)
	if err != nil {
		uc.log.Error().Err(err).Msg("KEV upsert failed")
		return nil, fmt.Errorf("kev sync: upsert: %w", err)
	}

	result := &SyncResult{
		Total:    len(entries),
		Inserted: inserted,
		Updated:  updated,
		Duration: time.Since(start),
		SyncedAt: time.Now().UTC(),
	}
	uc.log.Info().
		Int("inserted", inserted).
		Int("updated", updated).
		Dur("duration", result.Duration).
		Msg("KEV sync completed")

	return result, nil
}
