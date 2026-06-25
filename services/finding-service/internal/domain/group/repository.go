package group

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines persistence operations for finding groups.
type Repository interface {
	Save(ctx context.Context, g *FindingGroup) error
	FindByID(ctx context.Context, id uuid.UUID) (*FindingGroup, error)
	ListByProduct(ctx context.Context, productID uuid.UUID) ([]*FindingGroup, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// IncrementCount atomically increments finding_count by delta (can be negative).
	IncrementCount(ctx context.Context, groupID uuid.UUID, delta int) error
}
