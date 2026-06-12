package entity

import (
	"time"

	"github.com/google/uuid"
)

// APIKey is a long-lived credential for agent and automation access.
// The full key is only returned once at creation; only the hash is stored.
type APIKey struct {
	ID     uuid.UUID
	UserID uuid.UUID

	// Name is a human-readable label for this key.
	Name string

	// KeyHash is hex(sha256(fullKey)). Never store the raw key.
	KeyHash string

	// Prefix is the first 12 characters of the full key, e.g. "ovs_Ab3xYz9q".
	// Used for display and for database lookup without revealing the full key.
	Prefix string

	// Permissions is the list of capabilities this key grants.
	// e.g. ["agent:report"] for agent keys, or full set for admin keys.
	Permissions []string

	CreatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time // nil means never expires
	RevokedAt  *time.Time
}

// IsActive returns true if the key has not been revoked and has not expired.
func (k *APIKey) IsActive() bool {
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
		return false
	}
	return true
}

// OAuthAccount links an external OAuth identity to a user.
type OAuthAccount struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Provider   string // "google" | "github"
	ProviderID string // external provider's user ID
	Email      string
	Name       string
	AvatarURL  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
