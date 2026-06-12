// Package proxy provides HTTP reverse proxy routing for DefectDojo services.
// This file defines the DD route table — do not modify ovs_routes.go.
package proxy

import "time"

// PerRouteRateLimit overrides the global rate limit for a specific route.
type PerRouteRateLimit struct {
	RPM   int    // requests per minute
	Burst int    // burst allowance
	KeyBy string // "user_id" | "ip" | "endpoint"
}

// DDRouteConfig extends RouteConfig with DefectDojo-specific fields.
type DDRouteConfig struct {
	RouteConfig
	RateLimit    *PerRouteRateLimit // nil = use global default
	UseWebSocket bool               // upgrade connection to WebSocket
	Timeout      time.Duration      // 0 = use global default (30s)
}

// DDRoutes is the DefectDojo route table.
// Routes are matched by longest PathPrefix — order most-specific FIRST.
var DDRoutes = []DDRouteConfig{
	// ── Identity & Auth (public — no JWT required) ──────────────────────────
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/auth", Upstream: "identity", SkipAuth: true}},
	{RouteConfig: RouteConfig{PathPrefix: "/.well-known", Upstream: "identity", SkipAuth: true}},

	// ── Login rate limit (brute-force protection) ────────────────────────────
	{
		RouteConfig: RouteConfig{PathPrefix: "/api/v2/auth/login", Upstream: "identity", SkipAuth: true},
		RateLimit:   &PerRouteRateLimit{RPM: 5, Burst: 2, KeyBy: "ip"},
	},

	// ── Users & API Key management ───────────────────────────────────────────
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/users", Upstream: "identity"}},
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/api-keys", Upstream: "identity"}},

	// ── Product management ───────────────────────────────────────────────────
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/product-types", Upstream: "product-mgmt"}},
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/products", Upstream: "product-mgmt"}},
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/engagements", Upstream: "product-mgmt"}},
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/tests", Upstream: "product-mgmt"}},
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/risk-acceptances", Upstream: "product-mgmt"}},

	// ── Scan Orchestrator ─────────────────────────────────────────────────────
	// NOTE: /reimport-scan MUST come before /import-scan (longer prefix takes priority)
	{
		RouteConfig: RouteConfig{PathPrefix: "/api/v2/reimport-scan", Upstream: "scan-orchestrator"},
		RateLimit:   &PerRouteRateLimit{RPM: 10, Burst: 3, KeyBy: "user_id"},
		Timeout:     120 * time.Second,
	},
	{
		RouteConfig: RouteConfig{PathPrefix: "/api/v2/import-scan", Upstream: "scan-orchestrator"},
		RateLimit:   &PerRouteRateLimit{RPM: 10, Burst: 3, KeyBy: "user_id"},
		Timeout:     120 * time.Second,
	},
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/parsers", Upstream: "scan-orchestrator"}},
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/test-imports", Upstream: "scan-orchestrator"}},

	// ── Finding management ───────────────────────────────────────────────────
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/finding-groups", Upstream: "finding-mgmt"}},
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/findings", Upstream: "finding-mgmt"}},
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/endpoints", Upstream: "finding-mgmt"}},

	// ── SLA ──────────────────────────────────────────────────────────────────
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/sla-configurations", Upstream: "sla"}},

	// ── Notifications & In-app Alerts ─────────────────────────────────────────
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/notification-rules", Upstream: "notification"}},
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/notifications", Upstream: "notification"}},
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/alerts", Upstream: "notification"}},

	// ── JIRA integration ─────────────────────────────────────────────────────
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/jira-configurations", Upstream: "jira"}},
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/jira", Upstream: "jira"}},

	// ── Reports ──────────────────────────────────────────────────────────────
	{
		RouteConfig: RouteConfig{PathPrefix: "/api/v2/reports", Upstream: "report"},
		RateLimit:   &PerRouteRateLimit{RPM: 5, Burst: 2, KeyBy: "user_id"},
		Timeout:     120 * time.Second,
	},

	// ── Full-text search ─────────────────────────────────────────────────────
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/search", Upstream: "search"}},

	// ── AI / Triage ──────────────────────────────────────────────────────────
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/ai", Upstream: "ai"}},

	// ── Audit log ────────────────────────────────────────────────────────────
	{RouteConfig: RouteConfig{PathPrefix: "/api/v2/audit", Upstream: "audit"}},
}

// FindDDRoute returns the best matching DDRouteConfig for a given request path.
// Uses longest-prefix matching. Returns nil if no route matches.
func FindDDRoute(path string) *DDRouteConfig {
	var best *DDRouteConfig
	bestLen := 0

	for i := range DDRoutes {
		r := &DDRoutes[i]
		if len(r.PathPrefix) > bestLen &&
			len(path) >= len(r.PathPrefix) &&
			path[:len(r.PathPrefix)] == r.PathPrefix {
			best = r
			bestLen = len(r.PathPrefix)
		}
	}
	return best
}
