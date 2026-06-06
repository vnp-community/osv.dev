// Package handlers provides the OVS authentication HTTP middleware for the API gateway.
// This middleware sits in front of the HTTPProxy and handles:
//   1. Checking skip paths (auth/health endpoints)
//   2. Selecting auth mode (JWT Bearer vs API Key)
//   3. Calling GRPCAuthValidator
//   4. RBAC permission check
//   5. Injecting X-User-* headers before forwarding
package handlers

import (
	"net/http"
	"strings"

	"github.com/defectdojo/api-gateway/internal/domain/auth"
	"github.com/defectdojo/api-gateway/internal/domain/policy"
	gatewayauth "github.com/defectdojo/api-gateway/internal/infra/auth"
	"github.com/defectdojo/api-gateway/internal/infra/proxy"
	"github.com/rs/zerolog"
)

// OVSAuthMiddleware wraps HTTPProxy with authentication and RBAC enforcement.
type OVSAuthMiddleware struct {
	validator  *gatewayauth.GRPCAuthValidator
	httpProxy  *proxy.HTTPProxy
	skipPaths  []string
	log        zerolog.Logger
}

// NewOVSAuthMiddleware creates the auth middleware.
func NewOVSAuthMiddleware(
	validator *gatewayauth.GRPCAuthValidator,
	httpProxy *proxy.HTTPProxy,
	skipPaths []string,
	log zerolog.Logger,
) *OVSAuthMiddleware {
	return &OVSAuthMiddleware{
		validator: validator,
		httpProxy: httpProxy,
		skipPaths: skipPaths,
		log:       log,
	}
}

// ServeHTTP implements http.Handler. It is the main entry point for all OVS API requests.
func (m *OVSAuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Step 1: Find matching route
	route := proxy.FindRoute(path)
	if route == nil {
		// No OVS route — pass through to underlying mux (health, metrics, etc.)
		http.NotFound(w, r)
		return
	}

	// Step 2: Skip auth for public endpoints
	if route.SkipAuth || m.isSkipPath(path) {
		// Strip internal headers to prevent spoofing
		r.Header.Del("X-User-ID")
		r.Header.Del("X-User-Role")
		r.Header.Del("X-User-Permissions")
		m.httpProxy.ServeHTTP(w, r)
		return
	}

	// Step 3: Authenticate
	var principal *auth.Principal
	var err error

	if route.UseAPIKey {
		// API key path (e.g., /agents/report)
		apiKey := extractBearerToken(r)
		if apiKey == "" {
			writeError(w, http.StatusUnauthorized, "api_key_required", "API key required in Authorization header")
			return
		}
		principal, err = m.validator.ValidateAPIKey(r.Context(), apiKey)
	} else {
		// JWT Bearer token path
		token := extractBearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "token_required", "Bearer token required")
			return
		}
		principal, err = m.validator.ValidateToken(r.Context(), token)
	}

	if err != nil {
		m.log.Debug().Err(err).Str("path", path).Msg("auth failed")
		writeError(w, http.StatusUnauthorized, "auth_failed", err.Error())
		return
	}

	// Step 4: RBAC permission check
	requiredPerm := route.RequiredPerm
	if requiredPerm == "" {
		// Use dynamic lookup from MethodPermissions table
		prefix := policy.FindMatchingPrefix(path)
		requiredPerm = policy.RequiredPermission(prefix, r.Method)
	}
	if !policy.CheckPermission(principal, requiredPerm) {
		writeError(w, http.StatusForbidden, "permission_denied",
			"missing required permission: "+requiredPerm)
		return
	}

	// Step 5: Inject principal headers and forward
	role := ""
	if len(principal.Roles) > 0 {
		role = string(principal.Roles[0])
	}
	r = proxy.InjectPrincipalHeaders(r, principal.ID, role, principal.Permissions)
	m.httpProxy.ServeHTTP(w, r)
}

// isSkipPath returns true if the request path matches any configured skip path.
func (m *OVSAuthMiddleware) isSkipPath(path string) bool {
	for _, skip := range m.skipPaths {
		if strings.HasPrefix(path, skip) {
			return true
		}
	}
	return false
}

// extractBearerToken extracts the token from "Authorization: Bearer <token>" header.
func extractBearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":"` + code + `","message":"` + message + `"}`))
}
