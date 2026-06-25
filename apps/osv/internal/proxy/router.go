// Package proxy provides a configurable HTTP reverse proxy for the OSV gateway.
// Routes are defined as prefix-based matching (longest prefix wins).
// Each RouteConfig maps a URL prefix to an upstream service name.
package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// RouteConfig maps a URL path prefix to an upstream backend.
type RouteConfig struct {
	// PathPrefix is the URL prefix to match (e.g. "/api/v1/cve/search").
	// Longest prefix match wins.
	PathPrefix string

	// Upstream is the logical service name (e.g. "data-service", "search-service").
	// Must correspond to a key in the upstream map passed to NewRouter.
	Upstream string

	// SkipAuth: if true, the route is public (no auth check).
	// Auth middleware is not yet implemented — this is a placeholder for future enforcement.
	SkipAuth bool

	// RequiredPerm: if non-empty, the caller must have this permission.
	// Auth middleware is not yet implemented — placeholder for future enforcement.
	RequiredPerm string
}

// OVSRoutes contains the base OSV gateway routes (existing functionality).
// These cover the OSV v1 API proxy to data-service and search-service.
var OVSRoutes = []RouteConfig{
	// Health — public
	{PathPrefix: "/health", Upstream: "data-service", SkipAuth: true},
	// OSV v1 API — proxied to data-service
	{PathPrefix: "/v1/", Upstream: "data-service"},
}

// CVESearchRoutes contains the new routes from CVE Search CRs (CR-001 → CR-009).
// IMPORTANT: More specific routes (longer PathPrefix) MUST appear before
// less specific routes for longest-prefix matching to work correctly.
var CVESearchRoutes = []RouteConfig{
	// ── DB Info — Public (no auth) ─────────────────────────────────────────
	// CR-005: database statistics
	{PathPrefix: "/api/v1/dbinfo", Upstream: "data-service", SkipAuth: true},

	// ── CVE Query — Authenticated ──────────────────────────────────────────
	// NOTE: /api/v1/cve/query and /api/v1/cve/search MUST come before /api/v1/cve
	{PathPrefix: "/api/v1/cve/query",  Upstream: "data-service"}, // CR-008
	{PathPrefix: "/api/v1/cve/search", Upstream: "data-service"}, // CR-001
	{PathPrefix: "/api/v1/cve/last",   Upstream: "data-service"}, // CR-008
	{PathPrefix: "/api/v1/cve/recent", Upstream: "data-service"}, // CR-008
	// CR-001/008: CVE by ID — /cve/ (singular, data-service REST)
	{PathPrefix: "/api/v1/cve", Upstream: "data-service"},
	// CR-008: flexible POST query
	{PathPrefix: "/api/v1/query", Upstream: "data-service"},

	// ── Browse Vendor/Product — Authenticated ──────────────────────────────
	// NOTE: /api/v1/browse/vendors MUST come before /api/v1/browse
	{PathPrefix: "/api/v1/browse/vendors", Upstream: "search-service"}, // CR-002
	{PathPrefix: "/api/v1/browse",         Upstream: "search-service"}, // CR-002 (backward compat)
	{PathPrefix: "/api/v1/search",         Upstream: "data-service"},   // CR-002 vendor/product CVE

	// ── Taxonomy — Authenticated ───────────────────────────────────────────
	// CR-003: CWE and CAPEC lookups (served from data-service)
	{PathPrefix: "/api/v1/cwe",   Upstream: "data-service"},
	{PathPrefix: "/api/v1/capec", Upstream: "data-service"},

	// ── SEED-005: Asset Service ────────────────────────────────────────────
	{PathPrefix: "/api/v1/assets/tags",          Upstream: "asset-service"},
	{PathPrefix: "/api/v1/assets/bulk",          Upstream: "asset-service", RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v1/assets/import",        Upstream: "asset-service", RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v1/assets",               Upstream: "asset-service"},

	// ── SEED-005-C: Agent and Scheduled Scans (scan-service) ───────────────
	{PathPrefix: "/api/v1/agents",               Upstream: "scan-service"},
	{PathPrefix: "/api/v1/scans/scheduled",      Upstream: "scan-service"},

	// ── Ranking — Authenticated (read) / Admin (write) ─────────────────────────
	// NOTE: /api/v1/ranking/lookup MUST come before /api/v1/ranking
	{PathPrefix: "/api/v1/ranking/lookup", Upstream: "ranking-service"},
	// SEED-004: /api/v1/ranking/bulk MUST come before /api/v1/ranking (literal before wildcard)
	{PathPrefix: "/api/v1/ranking/bulk",   Upstream: "ranking-service",  RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v1/ranking",        Upstream: "ranking-service",  RequiredPerm: "system:configure"}, // CR-004

	// ── SEED-004: CVE write endpoints ─────────────────────────────────────────
	// Literal paths MUST come before /api/v1/cve (longer prefix wins)
	{PathPrefix: "/api/v1/cve/custom",      Upstream: "data-service",    RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v1/cve/bulk-triage", Upstream: "data-service"},   // any authenticated user
	{PathPrefix: "/api/v1/cve/import",      Upstream: "data-service",    RequiredPerm: "system:configure"},


	// ── KEV (Known Exploited Vulnerabilities) — Public ─────────────────────────
	// IMPORTANT: /api/v1/kev/sync/status, /api/v1/kev/check, /api/v1/kev/stats MUST come before /api/v1/kev
	{PathPrefix: "/api/v1/kev/sync/status", Upstream: "data-service", SkipAuth: true},
	{PathPrefix: "/api/v1/kev/check",       Upstream: "data-service", SkipAuth: true},
	{PathPrefix: "/api/v1/kev/stats",       Upstream: "data-service", SkipAuth: true},
	{PathPrefix: "/api/v1/kev/ransomware",  Upstream: "data-service", SkipAuth: true},
	{PathPrefix: "/api/v1/kev/",            Upstream: "data-service", SkipAuth: true}, // /api/v1/kev/{cveId}
	{PathPrefix: "/api/v1/kev",             Upstream: "data-service", SkipAuth: true}, // /api/v1/kev list

	// ── EPSS scores — Public ────────────────────────────────────────────────────
	{PathPrefix: "/api/v1/epss", Upstream: "data-service", SkipAuth: true},

	// ── Auth & Identity ────────────────────────────────────────────────────────
	{PathPrefix: "/api/v1/auth", Upstream: "identity-service", SkipAuth: true},

	// ── Data Sync — Admin only ─────────────────────────────────────────────────
	// CR-006
	{PathPrefix: "/api/v1/admin/sync",  Upstream: "data-service",     RequiredPerm: "system:configure"},
	// CR-007: admin user management
	// SEED-001: /api/v1/admin/users/bulk MUST come before /api/v1/admin/users (longest prefix wins)
	{PathPrefix: "/api/v1/admin/users/bulk",   Upstream: "identity-service", RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v1/admin/users/invite", Upstream: "identity-service", RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v1/admin/users/",       Upstream: "identity-service", RequiredPerm: "system:configure"}, // /{id}/roles, /{id}/unlock
	{PathPrefix: "/api/v1/admin/users",        Upstream: "identity-service", RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v1/admin/roles",        Upstream: "identity-service", RequiredPerm: "system:configure"},

	// ── SEED-006: Config Seed Bulk Routes ──────────────────────────────────────
	{PathPrefix: "/api/v2/sla-configurations/bulk",        Upstream: "sla-service",          RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v2/sla-configurations/assign-bulk", Upstream: "sla-service",          RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v2/notification-rules/bulk",        Upstream: "notification-service", RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v2/subscriptions/bulk",             Upstream: "notification-service", RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v2/webhooks/bulk",                  Upstream: "notification-service", RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v2/jira-configurations/bulk",       Upstream: "jira-service",         RequiredPerm: "system:configure"},
}


// AllRoutes returns the complete merged route table: CVESearchRoutes (more specific)
// followed by OVSRoutes (general). Order matters for longest-prefix matching.
func AllRoutes() []RouteConfig {
	combined := make([]RouteConfig, 0, len(DefectDojoRoutes)+len(CVESearchRoutes)+len(OVSRoutes))
	combined = append(combined, DefectDojoRoutes...)
	combined = append(combined, CVESearchRoutes...)
	combined = append(combined, OVSRoutes...)
	return combined
}

// DefectDojoRoutes contains routes for the DefectDojo-compatible API (CR-DD-001 → CR-DD-011).
// All routes proxy to finding-service or scan-service.
// IMPORTANT: More specific routes MUST come before less specific ones.
var DefectDojoRoutes = []RouteConfig{
	// ── CR-DD-001: Product Hierarchy (finding-service) ─────────────────────────
	// Product types
	{PathPrefix: "/api/v2/product-types", Upstream: "finding-service"},
	// Product members — more specific than /api/v2/products
	{PathPrefix: "/api/v2/products/", Upstream: "finding-service"},
	// Products
	{PathPrefix: "/api/v2/products", Upstream: "finding-service"},
	// Engagements
	{PathPrefix: "/api/v2/engagements", Upstream: "finding-service"},
	// Tests
	{PathPrefix: "/api/v2/tests", Upstream: "finding-service"},
	// Tool configurations
	{PathPrefix: "/api/v2/tool-configurations", Upstream: "finding-service"},
	// Tool types
	{PathPrefix: "/api/v2/tool-types", Upstream: "finding-service"},

	// ── CR-DD-004: Finding State Machine ──────────────────────────────────────
	// Finding state transitions — more specific than /api/v2/findings
	{PathPrefix: "/api/v2/findings/bulk",           Upstream: "finding-service"},
	{PathPrefix: "/api/v2/findings/severity_count", Upstream: "finding-service", SkipAuth: false},
	// Finding-level actions — /api/v2/findings/{id}/close etc.
	{PathPrefix: "/api/v2/findings/", Upstream: "finding-service"},
	// Finding CRUD
	{PathPrefix: "/api/v2/findings", Upstream: "finding-service"},

	// ── CR-DD-005: Finding Groups + Notes ─────────────────────────────────────
	{PathPrefix: "/api/v2/finding-groups", Upstream: "finding-service"},

	// ── CR-DD-008: Scan Import ────────────────────────────────────────────────
	{PathPrefix: "/api/v2/import-scan",   Upstream: "scan-service"},
	{PathPrefix: "/api/v2/reimport-scan", Upstream: "scan-service"},
	// Scan history
	{PathPrefix: "/api/v2/import-history", Upstream: "scan-service"},

	// ── CR-DD-009: Parser/Scan Type registry ──────────────────────────────────
	{PathPrefix: "/api/v2/scan-types", Upstream: "scan-service", SkipAuth: true},

	// ── CR-DD-010: SLA ────────────────────────────────────────────────────────
	{PathPrefix: "/api/v2/sla-configurations", Upstream: "sla-service"},

	// ── CR-DD-011: Risk Acceptance ────────────────────────────────────────────
	{PathPrefix: "/api/v2/risk-acceptances", Upstream: "finding-service"},

	// ── Notifications ─────────────────────────────────────────────────────────
	{PathPrefix: "/api/v2/notifications", Upstream: "notification-service"},

	// ── SEED-002: Product Hierarchy Seed ──────────────────────────────────────
	// Literal paths MUST come before wildcard /api/v2/products/
	{PathPrefix: "/api/v2/product-types/bulk",  Upstream: "finding-service", RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v2/products/bulk",        Upstream: "finding-service", RequiredPerm: "system:configure"},
	{PathPrefix: "/api/v2/products/import",      Upstream: "finding-service", RequiredPerm: "system:configure"},

	// ── SEED-003: Finding Bulk Create & Import ────────────────────────────────
	// Literal paths MUST come before wildcard /api/v2/findings/{id}
	{PathPrefix: "/api/v2/findings/bulk-create", Upstream: "finding-service"},
	{PathPrefix: "/api/v2/findings/import",      Upstream: "finding-service"},
}


// FindRoute returns the best (longest prefix) matching RouteConfig for a request path.
// Returns nil if no route matches.
func FindRoute(path string) *RouteConfig {
	var best *RouteConfig
	bestLen := 0

	for _, rc := range AllRoutes() {
		if len(rc.PathPrefix) > bestLen &&
			len(path) >= len(rc.PathPrefix) &&
			path[:len(rc.PathPrefix)] == rc.PathPrefix {
			rc := rc // capture loop variable
			best = &rc
			bestLen = len(rc.PathPrefix)
		}
	}
	return best
}

// Router is a reverse proxy that routes requests based on the route table.
// It holds a map of upstream service name → reverse proxy.
type Router struct {
	upstreams map[string]*httputil.ReverseProxy
	mu        sync.RWMutex
}

// NewRouter creates a gateway Router with the given upstream URL map.
// upstreamURLs: map of service name → HTTP base URL (e.g. "data-service" → "http://localhost:8082")
func NewRouter(upstreamURLs map[string]string) *Router {
	proxies := make(map[string]*httputil.ReverseProxy, len(upstreamURLs))
	for name, rawURL := range upstreamURLs {
		u, err := url.Parse(rawURL)
		if err != nil {
			log.Warn().Str("service", name).Str("url", rawURL).Err(err).Msg("invalid upstream URL")
			continue
		}
		proxies[name] = newReverseProxy(u)
	}
	return &Router{upstreams: proxies}
}

// ServeHTTP handles incoming requests using the route table.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	route := FindRoute(req.URL.Path)
	if route == nil {
		http.Error(w, `{"error":"no route found for path"}`, http.StatusNotFound)
		return
	}

	r.mu.RLock()
	proxy, ok := r.upstreams[route.Upstream]
	r.mu.RUnlock()

	if !ok {
		log.Warn().Str("upstream", route.Upstream).Str("path", req.URL.Path).Msg("upstream not configured")
		http.Error(w, `{"error":"upstream not available"}`, http.StatusBadGateway)
		return
	}

	log.Debug().
		Str("path", req.URL.Path).
		Str("upstream", route.Upstream).
		Msg("proxy request")

	proxy.ServeHTTP(w, req)
}

// newReverseProxy creates a single-host reverse proxy targeting the given URL.
func newReverseProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	// Custom error handler to return JSON instead of plain text
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error().Err(err).Str("path", r.URL.Path).Msg("proxy error")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"upstream unavailable"}`)) //nolint:errcheck
	}
	// Preserve the original request path (do not strip prefix)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Remove X-Forwarded-For manipulation if already set
		if prior, ok := req.Header["X-Forwarded-For"]; ok {
			req.Header.Set("X-Forwarded-For", strings.Join(prior, ", "))
		}
	}
	return proxy
}
