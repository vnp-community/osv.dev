// Package repository defines storage interfaces for the auth service domain.
// Implementations live in adapter/repository/postgres/.
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/osv/auth-service/internal/domain/entity"
)

// UserRepository defines persistence operations for User entities.
type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	FindByUsername(ctx context.Context, username string) (*entity.User, error)
	Update(ctx context.Context, user *entity.User) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// SessionRepository defines persistence operations for Session entities.
type SessionRepository interface {
	Create(ctx context.Context, session *entity.Session) error
	FindByRefreshTokenHash(ctx context.Context, hash string) (*entity.Session, error)
	RevokeByID(ctx context.Context, id uuid.UUID) error
	// RevokeByFamily revokes all sessions in a token family.
	// Called on replay attack detection.
	RevokeByFamily(ctx context.Context, family string) error
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
