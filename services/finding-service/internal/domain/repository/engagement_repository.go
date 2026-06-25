package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/engagement"
)

type EngagementRepository interface {
	FindByNameAndProduct(ctx context.Context, name string, productID uuid.UUID) (*engagement.Engagement, error)
	Create(ctx context.Context, eng *engagement.Engagement) error
	FindByID(ctx context.Context, id uuid.UUID) (*engagement.Engagement, error)
	Update(ctx context.Context, eng *engagement.Engagement) error
	ListByProduct(ctx context.Context, productID uuid.UUID) ([]*engagement.Engagement, error)
	ListExpiredOpen(ctx context.Context, today time.Time) ([]*engagement.Engagement, error)
}
