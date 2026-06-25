// Package asset — update_asset.go
// UpdateAssetUseCase implements PUT and PATCH for asset-service.
// TASK-008 FIX: added to support PATCH /api/v1/assets/{id}
package asset

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/google/osv.dev/services/asset-service/internal/domain/entity"
)

// UpdateRepository is the minimal interface needed for update/patch.
type UpdateRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Asset, error)
	Update(ctx context.Context, a *entity.Asset) error
}

// UpdateAssetUseCase handles full (PUT) and partial (PATCH) asset updates.
type UpdateAssetUseCase struct {
	repo UpdateRepository
}

// NewUpdateAssetUseCase creates an UpdateAssetUseCase.
func NewUpdateAssetUseCase(repo UpdateRepository) *UpdateAssetUseCase {
	return &UpdateAssetUseCase{repo: repo}
}

// UpdateAssetInput is the request body for PUT /assets/{id}.
type UpdateAssetInput struct {
	ID          uuid.UUID `json:"-"`
	Hostname    string    `json:"hostname"`
	IPAddress   string    `json:"ip_address"`
	Tags        []string  `json:"tags"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
}

// PatchAssetInput is the request body for PATCH /assets/{id}.
// Only non-nil pointer fields are applied.
type PatchAssetInput struct {
	ID          uuid.UUID `json:"-"`
	Hostname    *string   `json:"hostname,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Status      *string   `json:"status,omitempty"`
	Description *string   `json:"description,omitempty"`
}

// Execute performs a full asset update (PUT semantics).
func (uc *UpdateAssetUseCase) Execute(ctx context.Context, in UpdateAssetInput) (*entity.Asset, error) {
	a, err := uc.repo.FindByID(ctx, in.ID)
	if err != nil {
		return nil, fmt.Errorf("asset not found: %w", err)
	}

	// Replace all fields
	if in.Hostname != "" {
		a.Hostname = in.Hostname
	}
	if in.IPAddress != "" {
		a.IPAddress = in.IPAddress
	}
	if in.Tags != nil {
		a.Tags = in.Tags
	}
	if in.Status != "" {
		a.Status = entity.AssetStatus(in.Status)
	}
	a.UpdatedAt = time.Now().UTC()

	if err := uc.repo.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("failed to update asset: %w", err)
	}
	return a, nil
}

// Patch performs a partial asset update (PATCH semantics) — only non-nil fields applied.
func (uc *UpdateAssetUseCase) Patch(ctx context.Context, in PatchAssetInput) (*entity.Asset, error) {
	a, err := uc.repo.FindByID(ctx, in.ID)
	if err != nil {
		return nil, fmt.Errorf("asset not found: %w", err)
	}

	// Apply only provided fields
	if in.Hostname != nil {
		a.Hostname = *in.Hostname
	}
	if in.Tags != nil {
		a.Tags = in.Tags
	}
	if in.Status != nil {
		a.Status = entity.AssetStatus(*in.Status)
	}
	a.UpdatedAt = time.Now().UTC()

	if err := uc.repo.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("failed to patch asset: %w", err)
	}
	return a, nil
}
