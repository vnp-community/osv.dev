// Package repository defines CVE write-side persistence interfaces.
package repository

import (
	"context"

	"github.com/osv/ingestion-service/internal/domain/entity"
)

// CVERepository handles CVE upserts for the sync service.
type CVERepository interface {
	UpsertBatch(ctx context.Context, cves []*entity.CVE) (synced, skipped int, err error)
}

// SyncRepository tracks sync job state.
type SyncRepository interface {
	CreateJob(ctx context.Context, source entity.SourceName) (*entity.SyncJob, error)
	UpdateJobStatus(ctx context.Context, jobID int64, status entity.SyncStatus, stats *entity.SyncStats) error
	GetLastJob(ctx context.Context, source entity.SourceName) (*entity.SyncJob, error)
	ListRecentJobs(ctx context.Context, limit int) ([]*entity.SyncJob, error)
}
