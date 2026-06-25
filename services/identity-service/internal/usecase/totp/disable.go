// Package totp — disable.go
// DisableUseCase disables TOTP/MFA after verifying current password.
package totp

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domainerr "github.com/osv/identity-service/internal/domain/error"
	"github.com/osv/identity-service/internal/domain/repository"
	"github.com/osv/identity-service/internal/infrastructure/crypto"
)

// DisableRequest carries user ID and current password for confirmation.
type DisableRequest struct {
	UserID          uuid.UUID
	CurrentPassword string // Required for security confirmation
}

// DisableUseCase disables TOTP/MFA on an account.
type DisableUseCase struct {
	userRepo repository.UserRepository
}

// NewDisableUseCase creates a new DisableUseCase.
func NewDisableUseCase(userRepo repository.UserRepository) *DisableUseCase {
	return &DisableUseCase{userRepo: userRepo}
}

// Execute disables MFA after verifying the current password using Argon2id.
func (uc *DisableUseCase) Execute(ctx context.Context, req DisableRequest) error {
	user, err := uc.userRepo.FindByID(ctx, req.UserID)
	if err != nil {
		return fmt.Errorf("disable totp: find user: %w", err)
	}

	if !user.MFAEnabled {
		return domainerr.ErrMFANotEnabled
	}

	// Require password confirmation using existing Argon2id verifier
	ok, err := crypto.VerifyPassword(req.CurrentPassword, user.HashedPassword)
	if err != nil || !ok {
		return domainerr.ErrInvalidCredentials
	}

	// Disable MFA: clear mfa_enabled and secret
	if err := uc.userRepo.DisableTOTP(ctx, req.UserID); err != nil {
		return fmt.Errorf("disable totp: clear mfa: %w", err)
	}

	return nil
}

