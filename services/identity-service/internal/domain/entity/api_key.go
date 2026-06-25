package entity

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ── API Key Scope Constants ──────────────────────────────────────────────────

// Predefined API key scopes — mirrors cve-search access control model.
// Format: "{resource}:{action}" or "{resource}:*" for all actions.
const (
	ScopeCVERead      = "cve:read"      // Read CVE data (search, lookup, feed)
	ScopeCVEWrite     = "cve:write"     // Create/modify CVE records
	ScopeSearchRead   = "search:read"   // Perform searches (browse, query)
	ScopeIngestAdmin  = "ingest:admin"  // Trigger data ingestion (sync)
	ScopeFindingRead  = "finding:read"  // Read security findings
	ScopeFindingWrite = "finding:write" // Create/update findings
	ScopeScanExecute  = "scan:execute"  // Execute vulnerability scans
	ScopeRankingRead  = "ranking:read"  // Read CPE rankings
	ScopeRankingWrite = "ranking:write" // Manage CPE rankings
	ScopeAdminAll     = "admin:*"       // Full admin access (all permissions)
)

// ValidScopes is the exhaustive list of recognized scopes.
var ValidScopes = []string{
	ScopeCVERead, ScopeCVEWrite,
	ScopeSearchRead,
	ScopeIngestAdmin,
	ScopeFindingRead, ScopeFindingWrite,
	ScopeScanExecute,
	ScopeRankingRead, ScopeRankingWrite,
	ScopeAdminAll,
}


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

	// Scopes defines granular permissions for this API key.
	// Examples: ["cve:read"], ["cve:*"], ["admin:*"]
	// Use HasScope() to check access; ValidateScopes() to validate on creation.
	Scopes []string

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

// HasScope checks if the API key has a specific scope.
// Matching rules:
//  1. Exact match: scope == requested
//  2. Admin wildcard: "admin:*" grants everything
//  3. Resource wildcard: "cve:*" grants "cve:read" and "cve:write"
func (k *APIKey) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		// Admin wildcard — grants everything
		if s == ScopeAdminAll {
			return true
		}
		// Exact match
		if s == scope {
			return true
		}
		// Resource-level wildcard: "cve:*" matches "cve:read", "cve:write"
		if strings.HasSuffix(s, ":*") {
			prefix := strings.TrimSuffix(s, ":*")
			if strings.HasPrefix(scope, prefix+":") {
				return true
			}
		}
	}
	return false
}

// ValidateScopes checks that all requested scopes are recognized.
// Returns error listing all invalid scopes.
func ValidateScopes(scopes []string) error {
	valid := make(map[string]bool, len(ValidScopes))
	for _, s := range ValidScopes {
		valid[s] = true
	}
	var invalid []string
	for _, s := range scopes {
		if !valid[s] {
			invalid = append(invalid, s)
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf("invalid scopes: %s (valid: %s)",
			strings.Join(invalid, ", "),
			strings.Join(ValidScopes, ", "),
		)
	}
	return nil
}
