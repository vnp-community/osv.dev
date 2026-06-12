// Package usecase — KEV service use cases.
package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/kevservice/domain/entity"
	"github.com/globalcve/mono/internal/kevservice/domain/repository"
)

// --- KEV Sync ---

// SyncResult holds the result of a KEV catalog sync.
type SyncResult struct {
	Total    int
	Inserted int
	Updated  int
	Duration time.Duration
	SyncedAt time.Time
}

// CISAClient defines the interface for fetching the KEV catalog.
type CISAClient interface {
	FetchKEVCatalog(ctx context.Context) ([]*entity.KEVEntry, error)
}

// SyncUseCase syncs the CISA KEV catalog to the database.
type SyncUseCase struct {
	cisaClient CISAClient
	kevRepo    repository.KEVRepository
}

// NewSyncUseCase creates a new KEV sync use case.
func NewSyncUseCase(cisaClient CISAClient, kevRepo repository.KEVRepository) *SyncUseCase {
	return &SyncUseCase{cisaClient: cisaClient, kevRepo: kevRepo}
}

// Sync fetches the KEV catalog from CISA and upserts all entries.
func (uc *SyncUseCase) Sync(ctx context.Context) (*SyncResult, error) {
	start := time.Now()

	entries, err := uc.cisaClient.FetchKEVCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch kev catalog: %w", err)
	}

	inserted, updated, err := uc.kevRepo.UpsertBatch(ctx, entries)
	if err != nil {
		return nil, fmt.Errorf("upsert kev entries: %w", err)
	}

	result := &SyncResult{
		Total:    len(entries),
		Inserted: inserted,
		Updated:  updated,
		Duration: time.Since(start),
		SyncedAt: time.Now(),
	}

	log.Ctx(ctx).Info().
		Int("total", result.Total).
		Int("inserted", result.Inserted).
		Int("updated", result.Updated).
		Dur("duration", result.Duration).
		Msg("kev: sync completed")

	return result, nil
}

// --- Bulk Check ---

// CheckUseCase performs bulk KEV status checks.
type CheckUseCase struct {
	kevRepo repository.KEVRepository
}

// NewCheckUseCase creates a new KEV check use case.
func NewCheckUseCase(kevRepo repository.KEVRepository) *CheckUseCase {
	return &CheckUseCase{kevRepo: kevRepo}
}

// CheckMany checks a list of CVE IDs and returns KEV status.
// Limits to 500 IDs per request.
func (uc *CheckUseCase) CheckMany(ctx context.Context, cveIDs []string) ([]*entity.BulkCheckResult, error) {
	if len(cveIDs) > 500 {
		cveIDs = cveIDs[:500]
	}
	kevMap, err := uc.kevRepo.CheckMany(ctx, cveIDs)
	if err != nil {
		return nil, fmt.Errorf("check many: %w", err)
	}
	results := make([]*entity.BulkCheckResult, 0, len(cveIDs))
	for _, id := range cveIDs {
		results = append(results, &entity.BulkCheckResult{
			CVEID: id,
			IsKEV: kevMap[id],
		})
	}
	return results, nil
}

// IsKEV checks whether a single CVE ID is in the KEV catalog.
func (uc *CheckUseCase) IsKEV(ctx context.Context, cveID string) (bool, error) {
	m, err := uc.kevRepo.CheckMany(ctx, []string{cveID})
	if err != nil {
		return false, err
	}
	return m[cveID], nil
}

// --- Query ---

// QueryUseCase handles KEV list and get-by-id queries.
type QueryUseCase struct {
	kevRepo repository.KEVRepository
}

// NewQueryUseCase creates a new KEV query use case.
func NewQueryUseCase(kevRepo repository.KEVRepository) *QueryUseCase {
	return &QueryUseCase{kevRepo: kevRepo}
}

// List retrieves KEV entries with filtering and pagination.
func (uc *QueryUseCase) List(ctx context.Context, filter *entity.KEVFilter) ([]*entity.KEVEntry, int64, error) {
	return uc.kevRepo.List(ctx, filter)
}

// GetByID retrieves a single KEV entry by CVE ID.
func (uc *QueryUseCase) GetByID(ctx context.Context, cveID string) (*entity.KEVEntry, error) {
	return uc.kevRepo.FindByCVEID(ctx, cveID)
}

// GetAllIDs returns all KEV CVE IDs (for sync service to mark is_kev).
func (uc *QueryUseCase) GetAllIDs(ctx context.Context) ([]string, error) {
	return uc.kevRepo.GetAllIDs(ctx)
}

// --- Stats ---

// StatsUseCase retrieves KEV catalog statistics.
type StatsUseCase struct {
	kevRepo repository.KEVRepository
}

// NewStatsUseCase creates a new KEV stats use case.
func NewStatsUseCase(kevRepo repository.KEVRepository) *StatsUseCase {
	return &StatsUseCase{kevRepo: kevRepo}
}

// GetStats returns KEV catalog statistics.
func (uc *StatsUseCase) GetStats(ctx context.Context) (*entity.KEVStats, error) {
	return uc.kevRepo.Stats(ctx)
}
