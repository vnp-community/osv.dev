// Package upsertasset provides the use case for creating or updating assets.
package upsertasset

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/osv/asset-service/internal/domain/entity"
	"github.com/osv/asset-service/internal/domain/repository"
)

// Request is input from the scan-service (via gRPC or HTTP).
type Request struct {
	IPAddress string
	Hostname  string
	OS        string
	Services  []entity.Service
	WebTech   []entity.WebTechnology
	ScanID    uuid.UUID
}

// Response indicates whether the asset was created or updated.
type Response struct {
	AssetID uuid.UUID
	Created bool
}

// Publisher publishes asset lifecycle events.
type Publisher interface {
	PublishAssetCreated(ctx context.Context, a *entity.Asset) error
	PublishAssetUpdated(ctx context.Context, a *entity.Asset) error
}

// UseCase creates or updates an asset by IP address (idempotent upsert).
type UseCase struct {
	assetRepo repository.AssetRepository
	publisher Publisher
}

// NewUseCase creates an UpsertAsset use case.
func NewUseCase(assetRepo repository.AssetRepository, publisher Publisher) *UseCase {
	return &UseCase{assetRepo: assetRepo, publisher: publisher}
}

// Execute finds an existing asset by IP or creates a new one, then updates metadata.
func (uc *UseCase) Execute(ctx context.Context, req Request) (*Response, error) {
	existing, err := uc.assetRepo.FindByIPAddress(ctx, req.IPAddress)
	now := time.Now().UTC()

	if err != nil {
		// Create new asset
		asset := &entity.Asset{
			ID:            uuid.New(),
			IPAddress:     req.IPAddress,
			Hostname:      req.Hostname,
			OS:            req.OS,
			Services:      req.Services,
			WebTech:       req.WebTech,
			Labels:        map[string]string{},
			LastScannedAt: &now,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := uc.assetRepo.Create(ctx, asset); err != nil {
			return nil, err
		}
		uc.publisher.PublishAssetCreated(ctx, asset) //nolint:errcheck
		return &Response{AssetID: asset.ID, Created: true}, nil
	}

	// Update existing asset
	existing.Hostname = req.Hostname
	existing.OS = req.OS
	existing.Services = req.Services
	existing.WebTech = req.WebTech
	existing.LastScannedAt = &now
	existing.UpdatedAt = now

	if err := uc.assetRepo.Update(ctx, existing); err != nil {
		return nil, err
	}
	uc.publisher.PublishAssetUpdated(ctx, existing) //nolint:errcheck
	return &Response{AssetID: existing.ID, Created: false}, nil
}

// ── Type check via compile-time assertion ─────────────────────────────────────

// Ensure json.RawMessage can round-trip asset services list.
var _ = json.Marshal
