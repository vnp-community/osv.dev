// domain/policy/routing_policy.go — Route definitions and DefaultRoutes
package policy

import "time"

// Route defines a single API Gateway routing rule.
type Route struct {
	Path          string
	Method        string    // GET | POST | PUT | DELETE | * (any)
	Upstream      ServiceRef
	AuthRequired  bool
	RequiredRole  Role
	RateLimit     RateLimitTier
	BodySizeLimit int64     // bytes; 0 = no limit
	Timeout       time.Duration
	InternalOnly  bool
}

// ServiceRef identifies a backend service.
type ServiceRef struct {
	Name    string // "vulnerability-query-service"
	Address string // "vuln-query-svc:50051"
	TLS     bool
}

// Role represents an access role.
type Role string

const (
	RoleReader   Role = "reader"
	RoleImporter Role = "importer"
	RoleAdmin    Role = "admin"
)

// RateLimitTier controls per-route rate limits.
type RateLimitTier string

const (
	TierPublic   RateLimitTier = "public"   // 100 req/min
	TierPremium  RateLimitTier = "premium"  // 1000 req/min
	TierInternal RateLimitTier = "internal" // unlimited
)

// DefaultRoutes returns the standard route table derived from config/routes.yaml.
func DefaultRoutes() []Route {
	return []Route{
		// ── Vulnerability Query Service ──────────────────────────────────────
		{
			Path:     "/v1/vulns/{id}",
			Method:   "GET",
			Upstream: ServiceRef{Name: "vulnerability-query", Address: "vuln-query-svc:50051"},
			RateLimit: TierPublic,
			Timeout:  30 * time.Second,
		},
		{
			Path:     "/v1/query",
			Method:   "POST",
			Upstream: ServiceRef{Name: "vulnerability-query", Address: "vuln-query-svc:50051"},
			RateLimit: TierPublic,
			Timeout:  30 * time.Second,
		},
		{
			Path:          "/v1/querybatch",
			Method:        "POST",
			Upstream:      ServiceRef{Name: "vulnerability-query", Address: "vuln-query-svc:50051"},
			RateLimit:     TierPublic,
			BodySizeLimit: 1 * 1024 * 1024, // 1MB
			Timeout:       35 * time.Second,
		},
		// ── Version Index Service ─────────────────────────────────────────────
		{
			Path:     "/v1experimental/determineversion",
			Method:   "POST",
			Upstream: ServiceRef{Name: "version-index", Address: "version-index-svc:50051"},
			RateLimit: TierPublic,
			Timeout:  60 * time.Second,
		},
		// ── Ingestion Service (admin) ─────────────────────────────────────────
		{
			Path:         "/v1experimental/importfindings/{source}",
			Method:       "GET",
			Upstream:     ServiceRef{Name: "ingestion", Address: "ingestion-svc:50051"},
			AuthRequired: false,
			RateLimit:    TierPublic,
			Timeout:      15 * time.Second,
		},
		// ── Source Sync Admin ─────────────────────────────────────────────────
		{
			Path:         "/v1/admin/sources",
			Method:       "POST",
			Upstream:     ServiceRef{Name: "source-sync", Address: "source-sync-svc:50051"},
			AuthRequired: true,
			RequiredRole: RoleAdmin,
			RateLimit:    TierInternal,
			Timeout:      30 * time.Second,
		},
		// ── Search ────────────────────────────────────────────────────────────
		{
			Path:     "/v1/query/search",
			Method:   "GET",
			Upstream: ServiceRef{Name: "search", Address: "search-svc:50051"},
			RateLimit: TierPublic,
			Timeout:  10 * time.Second,
		},
		// ── Web BFF ───────────────────────────────────────────────────────────
		{
			Path:     "/{id}",
			Method:   "GET",
			Upstream: ServiceRef{Name: "web-bff", Address: "web-bff-svc:8080"},
			RateLimit: TierPublic,
			Timeout:  10 * time.Second,
		},
	}
}
