// Package repository defines the sync repository interfaces.
package repository

import (
	"context"

	syncentity "github.com/globalcve/mono/internal/cvesync/domain/entity"
	cveentity "github.com/globalcve/mono/internal/cvesearch/domain/entity"
)

// CVEWriteRepository defines write operations for the sync service.
type CVEWriteRepository interface {
	// UpsertBatch inserts or updates a batch of CVEs.
	// Returns counts of inserted and updated records.
	UpsertBatch(ctx context.Context, cves []*cveentity.CVE) (inserted, updated int, err error)

	// MarkKEV updates the is_kev flag for a list of CVE IDs.
	MarkKEV(ctx context.Context, ids []string, isKEV bool) error

	// UpdateEPSS updates EPSS scores for CVEs.
	UpdateEPSS(ctx context.Context, updates []EPSSUpdate) error
}

// EPSSUpdate holds EPSS data for a single CVE.
type EPSSUpdate struct {
	CVEID      string
	Score      float64
	Percentile float64
}

// SyncJobRepository defines persistence operations for sync jobs.
type SyncJobRepository interface {
	// Create records a new sync job.
	Create(ctx context.Context, job *syncentity.SyncJob) error

	// UpdateStatus updates the status and stats of an existing sync job.
	UpdateStatus(ctx context.Context, id int64, status syncentity.SyncStatus, stats syncentity.SyncStats) error

	// GetLatest retrieves the most recent sync job for a given source.
	GetLatest(ctx context.Context, source syncentity.SourceName) (*syncentity.SyncJob, error)

	// List retrieves the most recent sync jobs, ordered by started_at DESC.
	List(ctx context.Context, limit int) ([]*syncentity.SyncJob, error)
}
