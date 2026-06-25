// Package repository — API Key repository interface for gateway-service.
package repository

import (
	"context"
	"errors"

	"github.com/osv/gateway-service/internal/domain/entity"
)

// ErrAPIKeyNotFound is returned when a key lookup returns no result.
var ErrAPIKeyNotFound = errors.New("api key not found")

// APIKeyRepository defines persistence operations for API keys.
type APIKeyRepository interface {
	// FindByHash looks up an API key by its SHA-256 hash.
	// Returns ErrAPIKeyNotFound if no active key matches.
	FindByHash(ctx context.Context, hash string) (*entity.APIKey, error)

	// Save persists a new API key.
	Save(ctx context.Context, key *entity.APIKey) error

	// ListByOwner returns all active API keys for an owner (no plain key).
	ListByOwner(ctx context.Context, ownerID string) ([]*entity.APIKey, error)

	// Revoke soft-deletes (is_active=false) an API key by ID.
	// Returns ErrAPIKeyNotFound if the key does not belong to ownerID.
	Revoke(ctx context.Context, id, ownerID string) error

	// UpdateLastUsed sets last_used_at = NOW() for the given key ID.
	// Intended for async/fire-and-forget calls — errors are non-fatal.
	UpdateLastUsed(ctx context.Context, id string) error
}
