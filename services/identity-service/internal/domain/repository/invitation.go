// Package repository — invitation.go
// TASK-HC-014: Domain interfaces for user invitation flow.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Invitation represents a pending user invitation.
type Invitation struct {
	ID         uuid.UUID  `db:"id"`
	UserID     uuid.UUID  `db:"user_id"`
	Email      string     `db:"email"`
	Token      string     `db:"token"`
	ExpiresAt  time.Time  `db:"expires_at"`
	AcceptedAt *time.Time `db:"accepted_at"`
	InvitedBy  *uuid.UUID `db:"invited_by"`
	CreatedAt  time.Time  `db:"created_at"`
}

// InvitationRepository persists and retrieves invitations.
type InvitationRepository interface {
	Create(ctx context.Context, inv *Invitation) error
	FindByToken(ctx context.Context, token string) (*Invitation, error)
	MarkAccepted(ctx context.Context, token string) error
}

// EmailSender sends notification emails.
// When nil-injected, invitation flow degrades gracefully (log warning only).
type EmailSender interface {
	SendInvitation(ctx context.Context, to, inviterName, inviteURL, tempPassword string) error
}
