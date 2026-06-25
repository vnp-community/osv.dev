package riskacceptance

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines persistence operations for RiskAcceptance.
type Repository interface {
	Save(ctx context.Context, ra *RiskAcceptance) error
	FindByID(ctx context.Context, id uuid.UUID) (*RiskAcceptance, error)
	ListByProduct(ctx context.Context, productID uuid.UUID) ([]*RiskAcceptance, error)
	// ListExpiring returns risk acceptances with expiration_date <= before that are not yet expired.
	ListExpiring(ctx context.Context, before time.Time) ([]*RiskAcceptance, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// MarkExpired sets is_expired=true and updated_at=now.
	MarkExpired(ctx context.Context, id uuid.UUID) error
	// AddFinding adds a finding to a risk acceptance.
	AddFinding(ctx context.Context, raID, findingID uuid.UUID) error
	// RemoveFinding removes a finding from a risk acceptance.
	RemoveFinding(ctx context.Context, raID, findingID uuid.UUID) error
}
