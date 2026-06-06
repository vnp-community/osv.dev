// domain/auth/principal.go — Authenticated principal value object
package auth

// PrincipalType distinguishes authentication mechanisms.
type PrincipalType string

const (
	PrincipalAPIKey         PrincipalType = "API_KEY"
	PrincipalOAuth2         PrincipalType = "OAUTH2"
	PrincipalServiceAccount PrincipalType = "SERVICE_ACCOUNT"
	PrincipalAnonymous      PrincipalType = "ANONYMOUS"
)

// Role represents an access role.
type Role string

const (
	RoleReader   Role = "reader"
	RoleImporter Role = "importer"
	RoleAdmin    Role = "admin"
)

// Principal represents an authenticated identity.
type Principal struct {
	ID            string
	Type          PrincipalType
	Email         string            // from JWT email claim
	OrgID         string            // from JWT org_id claim
	Roles         []Role
	RateLimitTier string            // "free" | "standard" | "premium" | "unlimited" | "internal"
	Metadata      map[string]string // arbitrary key-value (client name, org, etc.)

	// OpenVulnScan extensions:
	// Permissions is the flat list of permissions derived from the user's role.
	// Populated by GRPCAuthValidator from auth-service ValidateToken response.
	// e.g. ["scan:create", "scan:read", "asset:read", "asset:write"]
	Permissions []string

	// APIKeyID holds the UUID of the API key record when Type == PrincipalAPIKey.
	// Used for audit logging and key revocation checks.
	APIKeyID string
}

// IsAnonymous returns true when no credentials were provided.
func (p *Principal) IsAnonymous() bool {
	return p.Type == PrincipalAnonymous
}

// HasRole checks whether the principal has a specific role.
func (p *Principal) HasRole(role Role) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasPermission returns true if the principal holds the specified permission string.
// Used by api-gateway RBAC middleware for OpenVulnScan routes.
func (p *Principal) HasPermission(perm string) bool {
	for _, pp := range p.Permissions {
		if pp == perm {
			return true
		}
	}
	return false
}

// AnonymousPrincipal returns a minimal unauthenticated principal.
func AnonymousPrincipal(clientIP string) *Principal {
	return &Principal{
		ID:            "anon:" + clientIP,
		Type:          PrincipalAnonymous,
		RateLimitTier: "free",
		Metadata:      map[string]string{"ip": clientIP},
	}
}
