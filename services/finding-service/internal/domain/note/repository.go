package note

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines persistence operations for finding notes.
type Repository interface {
	Save(ctx context.Context, n *FindingNote) error
	FindByID(ctx context.Context, id uuid.UUID) (*FindingNote, error)
	ListByFinding(ctx context.Context, findingID uuid.UUID) ([]*FindingNote, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
