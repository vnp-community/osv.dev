// Package postgres — totp_methods.go
// Implements 4 TOTP repository methods on userRepo (additive).
// Requires migration to add pending_totp_secret column — see migrations/002_totp_pending.sql.
package postgres

import (
	"context"

	"github.com/google/uuid"
)

// StorePendingTOTPSecret stores a TOTP secret that is not yet activated.
func (r *UserRepo) StorePendingTOTPSecret(ctx context.Context, userID uuid.UUID, secret string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET pending_totp_secret = $2, updated_at = NOW() WHERE id = $1`,
		userID, secret,
	)
	return err
}

// GetPendingTOTPSecret returns the pending (not-yet-activated) TOTP secret.
func (r *UserRepo) GetPendingTOTPSecret(ctx context.Context, userID uuid.UUID) (string, error) {
	var secret string
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(pending_totp_secret, '') FROM users WHERE id = $1`,
		userID,
	).Scan(&secret)
	return secret, err
}

// ActivateTOTP enables MFA: sets mfa_enabled=true, stores the secret, clears pending.
func (r *UserRepo) ActivateTOTP(ctx context.Context, userID uuid.UUID, secret string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users
		 SET mfa_enabled = TRUE,
		     mfa_totp_secret = $2,
		     pending_totp_secret = NULL,
		     updated_at = NOW()
		 WHERE id = $1`,
		userID, secret,
	)
	return err
}

// DisableTOTP disables MFA: clears mfa_enabled and secret.
func (r *UserRepo) DisableTOTP(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users
		 SET mfa_enabled = FALSE,
		     mfa_totp_secret = NULL,
		     pending_totp_secret = NULL,
		     updated_at = NOW()
		 WHERE id = $1`,
		userID,
	)
	return err
}
