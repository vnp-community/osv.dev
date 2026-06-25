// Package totp — verify.go
// VerifyUseCase validates the TOTP code against the pending secret and activates MFA.
package totp

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domainerr "github.com/osv/identity-service/internal/domain/error"
	"github.com/osv/identity-service/internal/domain/repository"
	"github.com/osv/identity-service/internal/infrastructure/crypto"
)

// VerifyRequest carries the user ID and TOTP code to verify.
type VerifyRequest struct {
	UserID uuid.UUID
	Code   string // 6-digit TOTP code from authenticator app
}

// VerifyUseCase validates a TOTP code and activates MFA on the account.
type VerifyUseCase struct {
	userRepo repository.UserRepository
}

// NewVerifyUseCase creates a new VerifyUseCase.
func NewVerifyUseCase(userRepo repository.UserRepository) *VerifyUseCase {
	return &VerifyUseCase{userRepo: userRepo}
}

// Execute verifies the TOTP code and activates MFA if valid.
func (uc *VerifyUseCase) Execute(ctx context.Context, req VerifyRequest) error {
	// Get pending TOTP secret
	secret, err := uc.userRepo.GetPendingTOTPSecret(ctx, req.UserID)
	if err != nil || secret == "" {
		return domainerr.ErrTOTPNotSetup
	}

	// Validate TOTP code using existing crypto.ValidateTOTP
	if !crypto.ValidateTOTP(secret, req.Code) {
		return domainerr.ErrInvalidTOTPCode
	}

	// Activate MFA: set mfa_enabled = TRUE, store secret, clear pending
	if err := uc.userRepo.ActivateTOTP(ctx, req.UserID, secret); err != nil {
		return fmt.Errorf("verify totp: activate: %w", err)
	}

	return nil
}
