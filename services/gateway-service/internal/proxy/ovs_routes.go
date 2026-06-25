// Package proxy provides HTTP reverse proxy routing for OpenVulnScan services.
// This file defines the OVS route table used by the HTTP proxy middleware.
package proxy

import "time"

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

	// CacheTTL defines how long to cache GET requests. 0 = no cache.
	CacheTTL time.Duration
}

// OVSRoutes is the route table for OpenVulnScan.
// Routes are matched by longest PathPrefix; first match wins on equal length.
// Order: most specific routes FIRST (e.g. /agents/report before /agents).
var OVSRoutes = []RouteConfig{
	// ── Auth service — PUBLIC endpoints (no JWT required) ───────────────────
	// IMPORTANT: More specific paths must come BEFORE the catch-all /api/v1/auth
	{PathPrefix: "/api/v1/auth/login",    Upstream: "identity-service", SkipAuth: true},
	{PathPrefix: "/api/v1/auth/refresh",  Upstream: "identity-service", SkipAuth: true},
	{PathPrefix: "/api/v1/auth/register", Upstream: "identity-service", SkipAuth: true},
	{PathPrefix: "/api/v1/auth/oauth",    Upstream: "identity-service", SkipAuth: true},
	{PathPrefix: "/api/v1/auth/callback", Upstream: "identity-service", SkipAuth: true},
	{PathPrefix: "/api/v1/auth/providers",Upstream: "identity-service", SkipAuth: true},

	// ── Auth service — PROTECTED endpoints (JWT required) ────────────────────
	// /me, /logout, /mfa/*, /api-keys, /profile, /admin/* require auth
	{PathPrefix: "/api/v1/auth", Upstream: "identity-service"},

	// ── Admin user management (JWT + admin role required) ────────────────────
	{PathPrefix: "/api/v1/admin",  Upstream: "identity-service",  RequiredPerm: "user:manage"},
	{PathPrefix: "/api/v1/profile", Upstream: "identity-service"},
	{PathPrefix: "/api/v1/api-keys", Upstream: "identity-service"},
	{PathPrefix: "/api/v1/audit-log", Upstream: "identity-service"},

	// ── CVE v2 routes (specific before generic) ──────────────────────────────
	{PathPrefix: "/api/v2/cves/search/semantic", Upstream: "search-service", SkipAuth: true},
	{PathPrefix: "/api/v2/cves/search",          Upstream: "search-service", SkipAuth: true},
	{PathPrefix: "/api/v2/cves/aggregations",    Upstream: "search-service", SkipAuth: true, CacheTTL: 5 * time.Minute},
	{PathPrefix: "/api/v2/cves/export",          Upstream: "search-service", SkipAuth: true},
	{PathPrefix: "/api/v2/cves",                 Upstream: "search-service", SkipAuth: true},

	// ── EPSS stats ───────────────────────────────────────────────────────────
	{PathPrefix: "/api/v2/epss/stats",           Upstream: "search-service", SkipAuth: true, CacheTTL: 30 * time.Minute},

	// ── KEV routes ───────────────────────────────────────────────────────────
	{PathPrefix: "/api/v2/kev/ransomware",       Upstream: "data-service",   SkipAuth: true, CacheTTL: 10 * time.Minute},
	{PathPrefix: "/api/v2/kev/stats",            Upstream: "data-service",   SkipAuth: true, CacheTTL: 10 * time.Minute},
	{PathPrefix: "/api/v2/kev/sync/status",      Upstream: "data-service",   SkipAuth: true},
	{PathPrefix: "/api/v2/kev/check",            Upstream: "data-service",   SkipAuth: true},
	{PathPrefix: "/api/v2/kev",                  Upstream: "data-service",   SkipAuth: true},

	// ── Taxonomy & Catalog ───────────────────────────────────────────────────
	{PathPrefix: "/api/v2/cwe",                  Upstream: "search-service", SkipAuth: true, CacheTTL: 1 * time.Hour},
	{PathPrefix: "/api/v2/capec",                Upstream: "search-service", SkipAuth: true, CacheTTL: 1 * time.Hour},
	{PathPrefix: "/api/v2/vendors",              Upstream: "search-service", SkipAuth: true, CacheTTL: 1 * time.Hour},
	{PathPrefix: "/api/v2/products",             Upstream: "search-service", SkipAuth: true, CacheTTL: 1 * time.Hour},

	// ── Stats & Dashboard ────────────────────────────────────────────────────
	{PathPrefix: "/api/v2/stats/dashboard",      Upstream: "search-service", SkipAuth: true, CacheTTL: 5 * time.Minute},

	// ── Admin / Sync (authenticated, admin scope) ────────────────────────────
	{PathPrefix: "/api/v2/sync",                 Upstream: "data-service",   RequiredPerm: "sync:admin"},

	// ── Webhooks + Subscriptions (authenticated) ─────────────────────────────
	{PathPrefix: "/api/v1/webhooks",             Upstream: "notification-service"},
	{PathPrefix: "/api/v2/webhooks",             Upstream: "notification-service"},
	{PathPrefix: "/api/v2/subscriptions",        Upstream: "notification-service"},

	// ── Notification rules + alerts (authenticated) ───────────────────────────
	{PathPrefix: "/api/v2/notification-rules",   Upstream: "notification-service"},
	{PathPrefix: "/api/v2/alerts",               Upstream: "notification-service"},

	// ── SLA configurations (authenticated) ───────────────────────────────────
	{PathPrefix: "/api/v2/sla-configurations",   Upstream: "sla-service"},
	{PathPrefix: "/api/v2/sla-violations",       Upstream: "sla-service"},
	{PathPrefix: "/api/v2/sla-dashboard",        Upstream: "sla-service"},

	// ── Finding service v2 routes ─────────────────────────────────────────────
	{PathPrefix: "/api/v2/findings",    Upstream: "finding-service"},
	{PathPrefix: "/api/v2/finding-groups", Upstream: "finding-service"},

	// ── Finding service v1 routes ─────────────────────────────────────────────
	{PathPrefix: "/api/v1/findings",        Upstream: "finding-service"},
	{PathPrefix: "/api/v1/products/grades", Upstream: "finding-service"}, // more specific
	{PathPrefix: "/api/v1/products",        Upstream: "product-service"}, // default for products CRUD
	{PathPrefix: "/api/v1/engagements",     Upstream: "finding-service"},
	{PathPrefix: "/api/v1/tests",       Upstream: "finding-service"},
	{PathPrefix: "/api/v1/reports",     Upstream: "finding-service"},

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

	// ── Legacy CVE endpoints (data-service) ──────────────────────────────────
	{PathPrefix: "/cve/", Upstream: "data-service", SkipAuth: true},
	{PathPrefix: "/info", Upstream: "data-service", SkipAuth: true},

	// ── Schedule service ──────────────────────────────────────────────────────
	{PathPrefix: "/api/v1/schedules", Upstream: "schedule-service"},

	// ── Report service ────────────────────────────────────────────────────────
	{PathPrefix: "/api/v1/reports", Upstream: "report-service", RequiredPerm: "report:download"},

	// ── Notification service (admin-level) ───────────────────────────────────
	{PathPrefix: "/api/v1/notifications", Upstream: "notification-service", RequiredPerm: "system:configure"},

	// ── Search & Audit ───────────────────────────────────────────────────────
	{PathPrefix: "/api/v1/search", Upstream: "search-service"},

	// ── Jira Integrations ────────────────────────────────────────────────────
	{PathPrefix: "/api/v1/jira",                 Upstream: "jira-service", RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v1/integrations/jira",    Upstream: "jira-service", RequiredPerm: "system:configure"}, // note: jira-service might need to handle this alias
	{PathPrefix: "/api/v2/jira-configurations",  Upstream: "jira-service"},
	{PathPrefix: "/api/v2/jira-issues",          Upstream: "jira-service"},
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
