// Package engagement_usecase implements CRUD and auto-create use cases for Engagement.
package engagement_usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	natsutil "github.com/osv/shared/pkg/nats"
	"github.com/osv/finding-service/internal/domain/engagement"
	"github.com/osv/finding-service/internal/domain/repository"
)

// ─── GetOrCreateEngagement ────────────────────────────────────────────────────

// GetOrCreateInput is used by Scan Orchestrator's auto-create context pipeline.
type GetOrCreateInput struct {
	ProductID                 uuid.UUID
	Name                      string
	EngagementType            engagement.Type
	Version                   string
	BuildID                   string
	CommitHash                string
	BranchTag                 string
	DeduplicationOnEngagement bool
}

type GetOrCreateEngagementUseCase struct {
	engRepo  repository.EngagementRepository
	eventPub *natsutil.Publisher
}

func NewGetOrCreate(repo repository.EngagementRepository, pub *natsutil.Publisher) *GetOrCreateEngagementUseCase {
	return &GetOrCreateEngagementUseCase{engRepo: repo, eventPub: pub}
}

// Execute finds an engagement by name+product or creates a new one.
// Returns (engagement, created, error).
func (uc *GetOrCreateEngagementUseCase) Execute(ctx context.Context, in GetOrCreateInput) (*engagement.Engagement, bool, error) {
	existing, err := uc.engRepo.FindByNameAndProduct(ctx, in.Name, in.ProductID)
	if err == nil && existing != nil {
		return existing, false, nil
	}

	eng, err := engagement.New(in.ProductID, in.Name, in.EngagementType)
	if err != nil {
		return nil, false, err
	}
	eng.Version = in.Version
	eng.BuildID = in.BuildID
	eng.CommitHash = in.CommitHash
	eng.BranchTag = in.BranchTag
	eng.DeduplicationOnEngagement = in.DeduplicationOnEngagement

	if err := uc.engRepo.Create(ctx, eng); err != nil {
		return nil, false, fmt.Errorf("create engagement: %w", err)
	}

	_ = uc.eventPub.Publish(ctx, "defectdojo.engagement.created", map[string]interface{}{
		"engagement_id": eng.ID,
		"product_id":    eng.ProductID,
		"name":          eng.Name,
		"type":          eng.EngagementType,
	})

	return eng, true, nil
}

// ─── CloseEngagement ──────────────────────────────────────────────────────────

type CloseEngagementUseCase struct {
	engRepo  repository.EngagementRepository
	eventPub *natsutil.Publisher
}

func NewClose(repo repository.EngagementRepository, pub *natsutil.Publisher) *CloseEngagementUseCase {
	return &CloseEngagementUseCase{engRepo: repo, eventPub: pub}
}

func (uc *CloseEngagementUseCase) Execute(ctx context.Context, id uuid.UUID) error {
	eng, err := uc.engRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	eng.Close()
	if err := uc.engRepo.Update(ctx, eng); err != nil {
		return err
	}
	_ = uc.eventPub.Publish(ctx, "defectdojo.engagement.closed", map[string]interface{}{
		"engagement_id": eng.ID,
		"product_id":    eng.ProductID,
	})
	return nil
}
