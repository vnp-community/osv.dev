package repository

import (
	"context"
	"github.com/google/uuid"
	"github.com/osv/scan-service/internal/domain/asset/entity"
)

// AssetFilter defines options for listing assets.
type AssetFilter struct {
	Search   string
	TagIDs   []uuid.UUID
	OS       string
	HasCVE   *bool
	Severity *entity.Severity
	Page     int
	PageSize int
	SortBy   string // ip_address|hostname|updated_at
	SortOrder string // asc|desc
}

// AssetRepository defines persistence for assets.
type AssetRepository interface {
	Create(ctx context.Context, a *entity.Asset) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Asset, error)
	FindByIPAddress(ctx context.Context, ip string) (*entity.Asset, error)
	Update(ctx context.Context, a *entity.Asset) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter AssetFilter) ([]*entity.Asset, int64, error)
	UpdateLastScanned(ctx context.Context, id uuid.UUID) error
	AttachTag(ctx context.Context, assetID, tagID uuid.UUID) error
	DetachTag(ctx context.Context, assetID, tagID uuid.UUID) error
}

// TagRepository defines persistence for tags.
type TagRepository interface {
	Create(ctx context.Context, t *entity.Tag) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Tag, error)
	FindByName(ctx context.Context, name string) (*entity.Tag, error)
	List(ctx context.Context) ([]*entity.Tag, error)
	Update(ctx context.Context, t *entity.Tag) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// VulnerabilityRepository defines persistence for vulnerabilities.
type VulnerabilityRepository interface {
	Upsert(ctx context.Context, v *entity.Vulnerability) error
	FindByAssetID(ctx context.Context, assetID uuid.UUID, page, pageSize int) ([]*entity.Vulnerability, int64, error)
	MarkRemediated(ctx context.Context, assetID uuid.UUID, cveID string) error
	GetSummaryByAsset(ctx context.Context, assetID uuid.UUID) (*entity.VulnSummary, error)
}
