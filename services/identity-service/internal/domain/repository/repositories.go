// Package repository defines storage interfaces for the auth service domain.
// Implementations live in adapter/repository/postgres/.
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/osv/identity-service/internal/domain/entity"
)

// UserFilter defines criteria for listing users.
type UserFilter struct {
	Role     string
	IsActive string
	Query    string
	Page     int
	PageSize int
}

// UserRepository defines persistence operations for User entities.
type UserRepository interface {
	List(ctx context.Context, filter UserFilter) ([]*entity.User, int, error)
	Create(ctx context.Context, user *entity.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	FindByUsername(ctx context.Context, username string) (*entity.User, error)
	Update(ctx context.Context, user *entity.User) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error

	// MFA management
	UpdateMFA(ctx context.Context, userID uuid.UUID, enabled bool, encryptedSecret string) error

	// Pending TOTP secret (stored before user confirms the code)
	StorePendingTOTPSecret(ctx context.Context, userID uuid.UUID, secret string) error
	GetPendingTOTPSecret(ctx context.Context, userID uuid.UUID) (string, error)

	// Activate TOTP: sets mfa_enabled=true, stores confirmed secret, clears pending
	ActivateTOTP(ctx context.Context, userID uuid.UUID, secret string) error

	// Disable TOTP: sets mfa_enabled=false, clears secret
	DisableTOTP(ctx context.Context, userID uuid.UUID) error

	// ── SEED-001: Admin bulk-create & role assignment ─────────────────────────

	// CreateDirect inserts a user with a pre-hashed password.
	// Returns ErrEmailAlreadyExists on duplicate email.
	CreateDirect(ctx context.Context, user *entity.User, hashedPassword string) error

	// CreateBulk inserts multiple users in a single transaction.
	// Partial failures are captured per-item; the transaction is committed on success.
	CreateBulk(ctx context.Context, inputs []entity.UserCreateInput) ([]entity.UserCreateResult, error)

	// AssignRole upserts a global or product-scoped role for a user.
	AssignRole(ctx context.Context, assignment entity.RoleAssignment) error

	// Activate sets is_active=true and is_verified=true for the given user.
	// [FIX TASK-HC-014] Used by AcceptInvite to activate invited users.
	Activate(ctx context.Context, userID uuid.UUID) error
}



// SessionRepository defines persistence operations for Session entities.
type SessionRepository interface {
	Create(ctx context.Context, session *entity.Session) error
	FindByRefreshTokenHash(ctx context.Context, hash string) (*entity.Session, error)
	RevokeByID(ctx context.Context, id uuid.UUID) error
	// RevokeByFamily revokes all sessions in a token family.
	// Called on replay attack detection.
	RevokeByFamily(ctx context.Context, family uuid.UUID) error
	// RevokeByUserID revokes all sessions for a user (logout all devices).
	RevokeByUserID(ctx context.Context, userID uuid.UUID) error
	CleanExpired(ctx context.Context) error
}

// APIKeyRepository defines persistence operations for APIKey entities.
type APIKeyRepository interface {
	Create(ctx context.Context, key *entity.APIKey) error
	// FindByPrefix looks up an API key by its prefix ("ovs_" + 8 chars).
	FindByPrefix(ctx context.Context, prefix string) (*entity.APIKey, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.APIKey, error)
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
	Revoke(ctx context.Context, id uuid.UUID) error
}

// OAuthAccountRepository defines persistence for linked OAuth accounts.
type OAuthAccountRepository interface {
	Upsert(ctx context.Context, account *entity.OAuthAccount) error
	FindByProviderID(ctx context.Context, provider, providerID string) (*entity.OAuthAccount, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.OAuthAccount, error)
}
