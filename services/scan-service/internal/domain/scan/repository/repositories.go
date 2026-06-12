package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/osv/scan-service/internal/domain/entity"
)

// ScanFilter defines filtering options for listing scans.
type ScanFilter struct {
	UserID   *uuid.UUID
	Status   *entity.ScanStatus
	ScanType *entity.ScanType
	Page     int
	PageSize int
}

// ScanRepository defines persistence operations for scans.
type ScanRepository interface {
	Create(ctx context.Context, scan *entity.Scan) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Scan, error)
	List(ctx context.Context, filter ScanFilter) ([]*entity.Scan, int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status entity.ScanStatus) error
	UpdateProgress(ctx context.Context, id uuid.UUID, progress int) error
	IncrementFindingCount(ctx context.Context, id uuid.UUID, delta int) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// FindingRepository defines persistence for scan findings.
type FindingRepository interface {
	CreateBatch(ctx context.Context, findings []*entity.Finding) error
	FindByScanID(ctx context.Context, scanID uuid.UUID, page, pageSize int) ([]*entity.Finding, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Finding, error)
}

// WebAlertRepository defines persistence for ZAP web alerts.
type WebAlertRepository interface {
	CreateBatch(ctx context.Context, alerts []*entity.WebAlert) error
	FindByScanID(ctx context.Context, scanID uuid.UUID) ([]*entity.WebAlert, error)
}

// DiscoveryHostRepository defines persistence for discovery hosts.
type DiscoveryHostRepository interface {
	CreateBatch(ctx context.Context, hosts []*entity.DiscoveryHost) error
	FindByScanID(ctx context.Context, scanID uuid.UUID) ([]*entity.DiscoveryHost, error)
}
