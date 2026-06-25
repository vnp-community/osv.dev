// Package totp — setup.go
// SetupUseCase generates a new TOTP secret and QR URL for the user.
// The secret is stored as "pending" until the user verifies it.
// ADDITIVE: existing login/register use cases untouched.
package totp

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/osv/identity-service/internal/domain/repository"
	"github.com/osv/identity-service/internal/infrastructure/crypto"
)

// SetupRequest carries the user ID for TOTP setup.
type SetupRequest struct {
	UserID uuid.UUID
}

// SetupResponse carries the QR URL and secret to display to the user.
type SetupResponse struct {
	QRCodeURL string `json:"qr_code_url"` // otpauth:// URI for QR display
	Secret    string `json:"secret"`      // base32 secret (for manual entry)
}

// SetupUseCase generates a pending TOTP secret for a user.
type SetupUseCase struct {
	userRepo repository.UserRepository
	issuer   string // e.g. "OSV Platform"
}

// NewSetupUseCase creates a new SetupUseCase.
func NewSetupUseCase(userRepo repository.UserRepository, issuer string) *SetupUseCase {
	return &SetupUseCase{userRepo: userRepo, issuer: issuer}
}

// Execute generates and stores a pending TOTP secret.
// Returns the QR URL for display; the secret is NOT activated until Verify succeeds.
func (uc *SetupUseCase) Execute(ctx context.Context, req SetupRequest) (*SetupResponse, error) {
	user, err := uc.userRepo.FindByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("setup totp: find user: %w", err)
	}

	// Generate new TOTP secret
	secret, qrURL, err := crypto.GenerateTOTPSecret(uc.issuer, user.Email)
	if err != nil {
		return nil, fmt.Errorf("setup totp: generate secret: %w", err)
	}

	// Store as pending (not yet active — requires verify step)
	if err := uc.userRepo.StorePendingTOTPSecret(ctx, req.UserID, secret); err != nil {
		return nil, fmt.Errorf("setup totp: store pending: %w", err)
	}

	return &SetupResponse{QRCodeURL: qrURL, Secret: secret}, nil
}
