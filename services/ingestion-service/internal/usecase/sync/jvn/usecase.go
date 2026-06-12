// Package jvn provides the JVN sync use case.
package jvn

import (
	"context"

	jvnclient "github.com/osv/ingestion-service/internal/adapter/external/jvn"
	"github.com/osv/ingestion-service/internal/domain/entity"
	"github.com/osv/ingestion-service/internal/domain/repository"
)

// UseCase syncs CVEs from the JVN API.
type UseCase struct {
	client  *jvnclient.Client
	cveRepo repository.CVERepository
}

// New creates a JVN sync use case.
func New(cveRepo repository.CVERepository) *UseCase {
	return &UseCase{client: jvnclient.NewClient(), cveRepo: cveRepo}
}

// Source implements SourceSyncer.
func (uc *UseCase) Source() entity.SourceName { return entity.SourceNameJVN }

// Sync fetches JVN latest CVEs and upserts.
func (uc *UseCase) Sync(ctx context.Context) (synced, skipped, errCount int, err error) {
	cves, err := uc.client.FetchLatest(ctx)
	if err != nil {
		return 0, 0, 1, err
	}
	synced, skipped, err = uc.cveRepo.UpsertBatch(ctx, cves)
	return synced, skipped, 0, err
}
