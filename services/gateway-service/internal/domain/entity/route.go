// Package entity defines the route entity for the GlobalCVE API gateway.
package entity

import "strings"

// UpstreamRoute defines a proxy routing rule.
type UpstreamRoute struct {
	PathPrefix   string
	Upstream     string
	SkipAuth     bool
	RequiredPerm string // "" = authenticated; specific perm enforced
	UseAPIKey    bool
}

// GlobalCVERoutes is the v2 route table for GlobalCVE services.
// Order: most specific first (longest prefix wins on equal length).
var GlobalCVERoutes = []UpstreamRoute{
	// ── CVE search service (public) ───────────────────────────────────────────
	{PathPrefix: "/api/v2/cves", Upstream: "cve-search-service", SkipAuth: true},

	// ── KEV service (public) ──────────────────────────────────────────────────
	{PathPrefix: "/api/v2/kev/check", Upstream: "kev-service", SkipAuth: true},
	{PathPrefix: "/api/v2/kev/stats", Upstream: "kev-service", SkipAuth: true},
	{PathPrefix: "/api/v2/kev", Upstream: "kev-service", SkipAuth: true},

	// ── Notification / webhooks (authenticated) ───────────────────────────────
	{PathPrefix: "/api/v2/webhooks", Upstream: "notification-service"},

	// ── Sync status (read = authenticated) ───────────────────────────────────
	{PathPrefix: "/api/v2/sync/status", Upstream: "cve-sync-service"},

	// ── Sync trigger (admin only) ─────────────────────────────────────────────
	{PathPrefix: "/api/v2/sync/trigger", Upstream: "cve-sync-service", RequiredPerm: "system:configure"},

	// ── NEW: cve-search v1 routes ─────────────────────────────────────────────

	// Auth (public — no authentication required)
	{PathPrefix: "/api/v1/auth", Upstream: "identity-service", SkipAuth: true},

	// CVE detail service
	{PathPrefix: "/api/v1/cve", Upstream: "cve-service"},

	// Browse service — vendor/product browsing and version search
	{PathPrefix: "/api/v1/browse", Upstream: "browse-service"},
	{PathPrefix: "/api/v1/search", Upstream: "browse-service"},
	{PathPrefix: "/api/v1/vendors", Upstream: "browse-service"},
	{PathPrefix: "/api/v1/products", Upstream: "browse-service"},
	{PathPrefix: "/api/v1/versions", Upstream: "browse-service"},

	// Query service — structured CVE queries
	{PathPrefix: "/api/v1/query", Upstream: "query-service"},

	// Full-text search service
	{PathPrefix: "/api/v1/fulltext", Upstream: "search-service"},

	// Taxonomy service — CWE and CAPEC
	{PathPrefix: "/api/v1/cwe", Upstream: "taxonomy-service"},
	{PathPrefix: "/api/v1/capec", Upstream: "taxonomy-service"},

	// Ranking service — EPSS-based CVE rankings
	{PathPrefix: "/api/v1/ranking", Upstream: "ranking-service"},

	// Info service — database stats and metadata
	{PathPrefix: "/api/v1/dbinfo", Upstream: "info-service"},

	// Ingest service — admin-only data ingestion triggers
	{PathPrefix: "/api/v1/ingest", Upstream: "ingest-service", RequiredPerm: "admin"},
}

// FindGlobalCVERoute returns the best matching UpstreamRoute for a path.
// Uses longest-prefix matching. Returns (nil, false) if no route matches.
func FindGlobalCVERoute(path string) (*UpstreamRoute, bool) {
	var best *UpstreamRoute
	bestLen := 0
	for i := range GlobalCVERoutes {
		r := &GlobalCVERoutes[i]
		if len(r.PathPrefix) > bestLen &&
			len(path) >= len(r.PathPrefix) &&
			strings.HasPrefix(path, r.PathPrefix) {
			best = r
			bestLen = len(r.PathPrefix)
		}
	}
	if best == nil {
		return nil, false
	}
	return best, true
}
