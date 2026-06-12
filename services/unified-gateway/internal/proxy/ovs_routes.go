// Package proxy provides HTTP reverse proxy routing for OpenVulnScan services.
// This file defines the OVS route table used by the HTTP proxy middleware.
package proxy

// RouteConfig defines a single proxy route from the gateway to an upstream service.
type RouteConfig struct {
	// PathPrefix is the URL path prefix to match (longest match wins).
	PathPrefix string

	// Upstream is the logical name of the upstream service.
	// Must match a key in the upstreams configuration map.
	Upstream string

	// SkipAuth skips all authentication checks for this route.
	// Used for public endpoints like /api/v1/auth/login and /api/v1/auth/register.
	SkipAuth bool

	// UseAPIKey indicates this route accepts API key authentication (Bearer ovs_xxx)
	// instead of JWT. Used for the agent report endpoint.
	UseAPIKey bool

	// RequiredPerm overrides the MethodPermissions lookup from the policy package.
	// If empty, the policy.RequiredPermission(path, method) lookup is used.
	// Set to "" to use dynamic lookup; set to "-" to allow all authenticated users.
	RequiredPerm string
}

// OVSRoutes is the route table for OpenVulnScan.
// Routes are matched by longest PathPrefix; first match wins on equal length.
// Order: most specific routes FIRST (e.g. /agents/report before /agents).
var OVSRoutes = []RouteConfig{
	// ── Auth service (public endpoints — no auth required) ───────────────────
	{PathPrefix: "/api/v1/auth", Upstream: "auth-service", SkipAuth: true},

	// ── Agent report ingestion (API key authentication, not JWT) ─────────────
	// Note: /agents/report MUST come before /agents to avoid prefix shadowing.
	{PathPrefix: "/api/v1/agents/report", Upstream: "agent-service", UseAPIKey: true},

	// ── Agent management (JWT required) ──────────────────────────────────────
	{PathPrefix: "/api/v1/agents", Upstream: "agent-service"},

	// ── Scan service ─────────────────────────────────────────────────────────
	{PathPrefix: "/api/v1/scans", Upstream: "scan-service"},

	// ── Asset service ─────────────────────────────────────────────────────────
	{PathPrefix: "/api/v1/assets", Upstream: "asset-service"},

	// ── CVE service ───────────────────────────────────────────────────────────
	{PathPrefix: "/api/v1/cves", Upstream: "cve-service", RequiredPerm: "scan:read"},

	// ── Schedule service ──────────────────────────────────────────────────────
	{PathPrefix: "/api/v1/schedules", Upstream: "schedule-service"},

	// ── Report service ────────────────────────────────────────────────────────
	{PathPrefix: "/api/v1/reports", Upstream: "report-service", RequiredPerm: "report:download"},

	// ── Notification service (admin-level) ───────────────────────────────────
	{PathPrefix: "/api/v1/notifications", Upstream: "notification-service", RequiredPerm: "system:configure"},
}

// FindRoute returns the best matching RouteConfig for a given request path.
// Uses longest-prefix matching. Returns nil if no route matches.
func FindRoute(path string) *RouteConfig {
	var best *RouteConfig
	bestLen := 0

	for i := range OVSRoutes {
		r := &OVSRoutes[i]
		if len(r.PathPrefix) > bestLen && len(path) >= len(r.PathPrefix) && path[:len(r.PathPrefix)] == r.PathPrefix {
			best = r
			bestLen = len(r.PathPrefix)
		}
	}
	return best
}
