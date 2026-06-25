package apikey

import (
    "strings"
    "time"

    "github.com/google/uuid"
)

const (
    // KeyPrefix is the required prefix for all API keys
    // Allows instant identification (similar to GitHub's ghp_, Stripe's sk_)
    KeyPrefix = "ovs_"

    // PrefixDisplayLength is how many chars of the key are stored/displayed
    // "ovs_" (4) + 8 random chars = 12 chars total
    PrefixDisplayLength = 12
)

// APIKey represents a long-lived API key for programmatic access
type APIKey struct {
    ID          uuid.UUID
    UserID      uuid.UUID
    Name        string      // Human-readable label (e.g., "CI Pipeline")
    KeyHash     string      // SHA-256 of the full plain key; never store plain key
    Prefix      string      // First 12 chars of the key (for lookup)
    Permissions []string    // Subset of the user's permissions
    LastUsedAt  *time.Time
    ExpiresAt   *time.Time  // nil = no expiry
    RevokedAt   *time.Time  // nil = active
    CreatedAt   time.Time
}

// NewAPIKey creates an APIKey entity from a pre-hashed key
// plainKey and keyHash are provided by the crypto layer
func NewAPIKey(userID uuid.UUID, name, plainKey, keyHash string, permissions []string, expiresAt *time.Time) (*APIKey, error) {
    name = strings.TrimSpace(name)
    if name == "" {
        return nil, ErrNameRequired
    }
    if len(name) > 100 {
        return nil, ErrNameTooLong
    }
    if !strings.HasPrefix(plainKey, KeyPrefix) {
        return nil, ErrInvalidKeyFormat
    }
    if len(plainKey) < PrefixDisplayLength {
        return nil, ErrInvalidKeyFormat
    }

    return &APIKey{
        ID:          uuid.New(),
        UserID:      userID,
        Name:        name,
        KeyHash:     keyHash,
        Prefix:      plainKey[:PrefixDisplayLength],
        Permissions: permissions,
        ExpiresAt:   expiresAt,
        CreatedAt:   time.Now().UTC(),
    }, nil
}

// IsActive returns true if the key can be used for authentication
func (k *APIKey) IsActive() bool {
    if k.RevokedAt != nil {
        return false
    }
    if k.ExpiresAt != nil && time.Now().UTC().After(*k.ExpiresAt) {
        return false
    }
    return true
}

// Revoke invalidates the API key
func (k *APIKey) Revoke() {
    now := time.Now().UTC()
    k.RevokedAt = &now
}

// RecordUse updates the last used timestamp (called asynchronously)
func (k *APIKey) RecordUse() {
    now := time.Now().UTC()
    k.LastUsedAt = &now
}

// HasPermission checks if this key grants a specific permission
func (k *APIKey) HasPermission(perm string) bool {
    for _, p := range k.Permissions {
        if p == perm {
            return true
        }
    }
    return false
}
