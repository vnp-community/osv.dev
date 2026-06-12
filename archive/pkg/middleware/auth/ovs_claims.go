// Package auth provides OpenVulnScan JWT claims types and context helpers.
// This file extends the existing osv pkg/middleware/auth with OVS-specific
// JWT claim structures used by auth-service and api-gateway.
package auth

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OVSClaims represents the JWT claims issued by auth-service.
// All fields match the JSON keys embedded in the JWT payload.
type OVSClaims struct {
	jwt.RegisteredClaims

	// UserID is the authenticated user's UUID.
	UserID string `json:"uid"`

	// Role is the user's primary role: admin|user|readonly|agent.
	Role string `json:"role"`

	// Permissions is the flat list of permissions derived from Role.
	// e.g. ["scan:create", "scan:read", "asset:read", "asset:write"]
	Permissions []string `json:"perms"`
}

// ExpiryTime returns the token expiry as a time.Time (nil-safe).
func (c *OVSClaims) ExpiryTime() time.Time {
	if c.RegisteredClaims.ExpiresAt == nil {
		return time.Time{}
	}
	return c.RegisteredClaims.ExpiresAt.Time
}

// HasPermission returns true if the claims contain the given permission string.
func (c *OVSClaims) HasPermission(perm string) bool {
	for _, p := range c.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// OVSClaimsKey is the context key for OVSClaims injection.
// Use a typed struct to avoid collisions with other context values.
type OVSClaimsKey struct{}

// OVSClaimsFromContext retrieves OVSClaims from ctx.
// Returns nil, false if no claims are present.
func OVSClaimsFromContext(ctx context.Context) (*OVSClaims, bool) {
	claims, ok := ctx.Value(OVSClaimsKey{}).(*OVSClaims)
	return claims, ok
}

// InjectOVSClaims returns a new context with the given claims embedded.
// Used by api-gateway middleware after ValidateToken gRPC call.
func InjectOVSClaims(ctx context.Context, claims *OVSClaims) context.Context {
	return context.WithValue(ctx, OVSClaimsKey{}, claims)
}

// APIKeyContextKey is the context key for API key principal injection.
type APIKeyContextKey struct{}

// OVSAPIKeyPrincipal holds resolved API key information injected into context
// by api-gateway after a successful ValidateAPIKey gRPC call.
type OVSAPIKeyPrincipal struct {
	UserID      string
	KeyID       string
	Permissions []string
}

// APIKeyPrincipalFromContext retrieves the OVSAPIKeyPrincipal from ctx.
func APIKeyPrincipalFromContext(ctx context.Context) (*OVSAPIKeyPrincipal, bool) {
	p, ok := ctx.Value(APIKeyContextKey{}).(*OVSAPIKeyPrincipal)
	return p, ok
}

// InjectAPIKeyPrincipal returns a new context with the given API key principal.
func InjectAPIKeyPrincipal(ctx context.Context, p *OVSAPIKeyPrincipal) context.Context {
	return context.WithValue(ctx, APIKeyContextKey{}, p)
}
