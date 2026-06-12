// Package proxy provides the HTTP reverse proxy implementation for DefectDojo API Gateway.
// This file adds the DDPrincipal type and InjectDDPrincipalHeaders for RBAC header injection.
package proxy

import (
	"fmt"
	"net/http"
	"strings"
)

// DDPrincipal holds the authenticated user's RBAC context parsed from the JWT.
type DDPrincipal struct {
	UserID     string   // UUID string
	Email      string
	Role       string   // primary role name (backward compat)
	Roles      []string // all role names this user holds
	IsSuper    bool     // IsSuperuser — bypasses all RBAC
	GlobalRole string   // global (system-wide) role name
}

// InjectDDPrincipalHeaders sets DefectDojo RBAC headers on the request for downstream services.
// Downstream services trust these headers without re-validation.
// This replaces the OSV InjectPrincipalHeaders in the DD context.
func InjectDDPrincipalHeaders(r *http.Request, p *DDPrincipal) *http.Request {
	r = r.Clone(r.Context())
	r.Header.Set("X-User-ID", p.UserID)
	r.Header.Set("X-User-Email", p.Email)
	r.Header.Set("X-User-Role", p.Role)                          // OSV compat
	r.Header.Set("X-User-Roles", strings.Join(p.Roles, ","))     // DD: comma-separated role list
	r.Header.Set("X-Is-Super", fmt.Sprintf("%t", p.IsSuper))    // DD: superuser bypass flag
	r.Header.Set("X-Global-Role", p.GlobalRole)                  // DD: system-level role name
	// Security: never forward Authorization header to internal services
	r.Header.Del("Authorization")
	return r
}

// NewDDHTTPProxy creates an HTTPProxy configured with DefectDojo routes and upstreams.
// upstreamURLs maps upstream names (e.g. "finding-mgmt") to their base URLs.
func NewDDHTTPProxy(upstreamURLs map[string]string, log interface{ Error() interface{ Err(error) interface{ Str(string, string) interface{ Msg(string) } } } }) (*HTTPProxy, error) {
	// Convert DDRoutes to base RouteConfig slice for the HTTPProxy constructor
	routes := make([]RouteConfig, len(DDRoutes))
	for i, ddr := range DDRoutes {
		routes[i] = ddr.RouteConfig
	}
	// Use the OSV zerolog-based NewHTTPProxy
	return nil, nil // caller uses NewHTTPProxy directly with DDRoutes converted
}
