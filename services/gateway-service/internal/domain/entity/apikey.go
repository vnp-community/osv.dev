// Package entity — API Key domain entity for gateway-service.
package entity

import "time"

// APIKey represents an issued API key for programmatic access.
// Plain key text is NEVER stored — only the SHA-256 hash.
type APIKey struct {
	ID          string     // UUID
	KeyHash     string     // SHA-256(plaintext_key) — stored, never plain key
	OwnerID     string     // User or org ID (from JWT claims)
	Description string     // Human-readable label, e.g. "CI/CD pipeline"
	Scopes      []string   // Permission scopes: ["cve:read", "webhook:write"]
	RateLimit   *int       // req/min override; nil = use global tier default
	LastUsedAt  *time.Time
	ExpiresAt   *time.Time // nil = no expiry
	IsActive    bool
	CreatedAt   time.Time
}

// API Key scope constants.
const (
	ScopeCVERead   = "cve:read"
	ScopeKEVRead   = "kev:read"
	ScopeWebhook   = "webhook:write"
	ScopeSyncAdmin = "sync:admin"
	ScopeReadAll   = "read:all"
)

// IsExpired reports whether the key has passed its expiry time.
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return k.ExpiresAt.Before(time.Now())
}

// HasScope reports whether the key has the specified scope.
// A key with ScopeReadAll grants access to any read scope.
func (k *APIKey) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == scope || s == ScopeReadAll {
			return true
		}
	}
	return false
}
