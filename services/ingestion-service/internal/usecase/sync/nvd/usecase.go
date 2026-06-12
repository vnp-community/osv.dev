// Package nvd provides the NVD sync use case.
package nvd

import (
	"context"

	"github.com/osv/ingestion-service/internal/adapter/external/nvd"
	"github.com/osv/ingestion-service/internal/domain/entity"
	"github.com/osv/ingestion-service/internal/domain/repository"
)

// UseCase syncs CVEs from the NVD API.
type UseCase struct {
	client  *nvd.Client
	cveRepo repository.CVERepository
}

// New creates an NVD sync use case.
func New(apiKey string, cveRepo repository.CVERepository) *UseCase {
	return &UseCase{
		client:  nvd.NewClient(apiKey),
		cveRepo: cveRepo,
	}
}

// Source implements SourceSyncer.
func (uc *UseCase) Source() entity.SourceName { return entity.SourceNameNVD }

// Sync fetches all NVD CVEs and upserts them.
func (uc *UseCase) Sync(ctx context.Context) (synced, skipped, errCount int, err error) {
	fetchErr := uc.client.FetchAll(ctx, func(batch []*entity.CVE) {
		if err != nil {
			return
		}
		s, sk, e := uc.cveRepo.UpsertBatch(ctx, batch)
		if e != nil {
			err = e
			errCount++
			return
		}
		synced += s
		skipped += sk
	})
	if fetchErr != nil && err == nil {
		err = fetchErr
	}
	return synced, skipped, errCount, err
}
