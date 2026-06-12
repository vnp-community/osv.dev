// Package circl provides the CIRCL sync use case.
package circl

import (
	"context"

	circlclient "github.com/osv/ingestion-service/internal/adapter/external/circl"
	"github.com/osv/ingestion-service/internal/domain/entity"
	"github.com/osv/ingestion-service/internal/domain/repository"
)

// UseCase syncs CVEs from the CIRCL API.
type UseCase struct {
	client  *circlclient.Client
	cveRepo repository.CVERepository
}

// New creates a CIRCL sync use case.
func New(cveRepo repository.CVERepository) *UseCase {
	return &UseCase{client: circlclient.NewClient(), cveRepo: cveRepo}
}

// Source implements SourceSyncer.
func (uc *UseCase) Source() entity.SourceName { return entity.SourceNameCIRCL }

// Sync fetches CIRCL latest CVEs and upserts.
func (uc *UseCase) Sync(ctx context.Context) (synced, skipped, errCount int, err error) {
	cves, err := uc.client.FetchLatest(ctx)
	if err != nil {
		return 0, 0, 1, err
	}
	synced, skipped, err = uc.cveRepo.UpsertBatch(ctx, cves)
	return synced, skipped, 0, err
}
