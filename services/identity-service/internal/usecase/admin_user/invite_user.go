// Package adminuser — invite_user.go
// TASK-HC-014: InviteUserUseCase creates user + invitation token + sends email.
package adminuser

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/osv/identity-service/internal/domain/entity"
	"github.com/osv/identity-service/internal/domain/repository"
)

// EmailSenderIface is a type alias for repository.EmailSender so that
// callers (embedded.go) can reference it without importing the domain package directly.
// [FIX TASK-HC-014]
type EmailSenderIface = repository.EmailSender

// InviteUserInput is the input for the invitation use-case.
type InviteUserInput struct {
	Email       string
	Name        string
	Role        string
	InvitedByID uuid.UUID
	InviterName string
	BaseURL     string // platform base URL for the accept-invite link
}

// InviteUserResult holds the invitation details returned to the handler.
type InviteUserResult struct {
	UserID    uuid.UUID
	Email     string
	Token     string
	ExpiresAt time.Time
}

// InviteUserUseCase orchestrates invitation: create user → create token → send email.
type InviteUserUseCase struct {
	userRepo       repository.UserRepository
	invitationRepo repository.InvitationRepository
	emailSender    repository.EmailSender // may be nil (email disabled)
	log            zerolog.Logger
}

// NewInviteUserUseCase creates the use case.
// emailSender may be nil — invitation still works, only email is skipped.
func NewInviteUserUseCase(
	userRepo repository.UserRepository,
	invitationRepo repository.InvitationRepository,
	emailSender repository.EmailSender,
	log zerolog.Logger,
) *InviteUserUseCase {
	return &InviteUserUseCase{
		userRepo:       userRepo,
		invitationRepo: invitationRepo,
		emailSender:    emailSender,
		log:            log,
	}
}

// Execute runs the invitation flow.
func (uc *InviteUserUseCase) Execute(ctx context.Context, in InviteUserInput) (*InviteUserResult, error) {
	// 1. Generate secure 32-byte token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("invite: generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	// 2. Generate temporary password (16 hex chars)
	passBytes := make([]byte, 8)
	if _, err := rand.Read(passBytes); err != nil {
		return nil, fmt.Errorf("invite: generate temp password: %w", err)
	}
	tempPass := hex.EncodeToString(passBytes)

	// 3. Create user in pending state (IsVerified = false, IsActive = false)
	user := &entity.User{
		Email:        in.Email,
		Username:     in.Name,
		Role:         in.Role,
		AuthProvider: entity.AuthProviderLocal,
		IsActive:     false,
		IsVerified:   false,
	}
	// Set a bcrypt-hashed temp password so account works after accept-invite
	if hash, err := bcrypt.GenerateFromPassword([]byte(tempPass), 12); err == nil {
		user.HashedPassword = string(hash)
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("invite: create user: %w", err)
	}

	// 4. Create invitation record
	invitedByID := in.InvitedByID
	inv := &repository.Invitation{
		ID:        uuid.New(),
		UserID:    user.ID,
		Email:     in.Email,
		Token:     token,
		ExpiresAt: time.Now().UTC().Add(48 * time.Hour),
		InvitedBy: &invitedByID,
	}
	if err := uc.invitationRepo.Create(ctx, inv); err != nil {
		return nil, fmt.Errorf("invite: create invitation: %w", err)
	}

	// 5. Send email (non-fatal — just warn if fails or not configured)
	inviteURL := fmt.Sprintf("%s/accept-invite?token=%s", in.BaseURL, token)
	if uc.emailSender != nil {
		if err := uc.emailSender.SendInvitation(ctx, in.Email, in.InviterName, inviteURL, tempPass); err != nil {
			uc.log.Warn().Err(err).Str("email", in.Email).Msg("invite: email send failed — invitation still created")
		} else {
			uc.log.Info().Str("email", in.Email).Msg("invite: email sent successfully")
		}
	} else {
		uc.log.Warn().Str("email", in.Email).
			Str("invite_url", inviteURL).
			Msg("invite: SMTP not configured — invitation created but email NOT sent")
	}

	return &InviteUserResult{
		UserID:    user.ID,
		Email:     in.Email,
		Token:     token,
		ExpiresAt: inv.ExpiresAt,
	}, nil
}
