// Package postgres — invitation_repo.go
// TASK-HC-014: PostgreSQL implementation of InvitationRepository.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/identity-service/internal/domain/repository"
)

// InvitationRepo implements repository.InvitationRepository using pgx/v5.
type InvitationRepo struct{ pool *pgxpool.Pool }

// NewInvitationRepo creates a new InvitationRepo.
func NewInvitationRepo(pool *pgxpool.Pool) *InvitationRepo {
	return &InvitationRepo{pool: pool}
}

// Create inserts a new invitation.
func (r *InvitationRepo) Create(ctx context.Context, inv *repository.Invitation) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_invitations (id, user_id, email, token, expires_at, invited_by)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, inv.ID, inv.UserID, inv.Email, inv.Token, inv.ExpiresAt, inv.InvitedBy)
	if err != nil {
		return fmt.Errorf("invitation_repo.Create: %w", err)
	}
	return nil
}

// FindByToken retrieves a valid (not accepted, not expired) invitation by token.
func (r *InvitationRepo) FindByToken(ctx context.Context, token string) (*repository.Invitation, error) {
	inv := &repository.Invitation{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, email, token, expires_at, accepted_at, invited_by, created_at
		FROM user_invitations
		WHERE token = $1 AND accepted_at IS NULL AND expires_at > NOW()
	`, token).Scan(
		&inv.ID, &inv.UserID, &inv.Email, &inv.Token,
		&inv.ExpiresAt, &inv.AcceptedAt, &inv.InvitedBy, &inv.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("invitation_repo.FindByToken: %w", err)
	}
	return inv, nil
}

// MarkAccepted sets accepted_at = NOW() for the given token.
func (r *InvitationRepo) MarkAccepted(ctx context.Context, token string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE user_invitations SET accepted_at = NOW() WHERE token = $1`, token)
	return err
}
