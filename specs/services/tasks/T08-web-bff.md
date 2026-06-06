# Task T08 — Web BFF (Backend for Frontend)

> **Priority:** P1 | **Phase:** 3 | **Spec:** `specs/services/09-web-bff.md`  
> **Depends on:** T01-api-gateway, T02-vulnerability-query, T07-search, T11-ai-enrichment

## Mục Tiêu
Thay thế Flask website backend. Aggregate data từ backend services, tối ưu cho website consumers. Không có business logic.

## Trách Nhiệm
- Serve website data: homepage stats, search, vuln detail, list, OSV linter
- Aggregate: Query Service + Search Service + AI Enrichment (parallel)
- Ecosystem stats cache (Redis, TTL 24h)
- Rate limiting per IP: 30 req/min
- CORS handling
- Redirect `/{ID}` → `/vulnerability/{ID}`

## Không Làm
- Business logic, direct DB access, auth (API Gateway handles auth)

## Cấu Trúc File

```
services/web-bff/
├── cmd/server/main.go
├── internal/
│   ├── domain/entity/
│   │   ├── ecosystem_stats.go    # {Ecosystem, VulnCount}
│   │   ├── vuln_list_item.go     # Lightweight display model
│   │   └── linter_result.go
│   ├── application/query/
│   │   ├── get_homepage_stats/{query,handler}.go
│   │   ├── search_vulnerabilities/{query,handler}.go
│   │   ├── get_vulnerability_detail/{query,handler}.go
│   │   ├── list_vulnerabilities/{query,handler}.go
│   │   └── lint_osv/{query,handler}.go
│   └── infra/
│       ├── client/
│       │   ├── query_service_client.go    # gRPC client
│       │   ├── search_service_client.go   # gRPC client
│       │   └── ai_enrichment_client.go    # gRPC client
│       ├── cache/redis/
│       │   ├── ecosystem_stats_cache.go
│       │   └── popular_vuln_cache.go
│       └── ratelimit/redis_rate_limiter.go  # 30 req/min per IP
├── interface/http/
│   ├── handler/
│   │   ├── homepage_handler.go
│   │   ├── search_handler.go
│   │   ├── vulnerability_handler.go
│   │   ├── list_handler.go
│   │   ├── linter_handler.go
│   │   └── health_handler.go
│   └── middleware/
│       ├── cors_middleware.go
│       ├── rate_limit_middleware.go
│       ├── logging_middleware.go
│       └── tracing_middleware.go
└── config/config.go
```

## HTTP API Routes

```go
// interface/http/handler/router.go (dùng chi router)
mux.Use(middleware.Tracing(), middleware.Logging(), middleware.CORS())
mux.Use(middleware.RateLimit(30, time.Minute))  // per IP

mux.Get("/api/v1/stats", handlers.Homepage.GetStats)
mux.Get("/api/v1/search", handlers.Search.Search)
mux.Get("/api/v1/search/autocomplete", handlers.Search.Autocomplete)
mux.Get("/api/v1/list", handlers.List.List)
mux.Get("/api/v1/vulns/{id}", handlers.Vulnerability.GetDetail)
mux.Get("/api/v1/vulns/{id}/related", handlers.Vulnerability.GetRelated)
mux.Get("/{id:[A-Z]+-[0-9].*}", handlers.Redirect.RedirectToVuln)  // 301 redirect
mux.Post("/api/v1/lint", handlers.Linter.Lint)
mux.Get("/health/live", handlers.Health.Live)
mux.Get("/health/ready", handlers.Health.Ready)
mux.Get("/metrics", handlers.Metrics.Serve)
```

## Homepage Stats Handler

```go
// application/query/get_homepage_stats/handler.go
// Cache: hard TTL=24h, soft TTL=30min (stale-while-revalidate)
func Handle(ctx, q Query) (*Result, error):
  if cached, ok := statsCache.Get(ctx, "homepage_stats"); ok { return cached, nil }
  stats, err := queryClient.GetEcosystemStats(ctx)
  if err != nil {
    // Graceful degradation: return stale cache
    if stale, ok := statsCache.GetStale(ctx, "homepage_stats"); ok { return stale, nil }
    return nil, err
  }
  statsCache.Set(ctx, "homepage_stats", stats, 24*time.Hour)
  return stats, nil
```

## Vulnerability Detail (Aggregation Pattern)

```go
// application/query/get_vulnerability_detail/handler.go
// Parallel fetch: core data + AI metadata
func Handle(ctx, q Query) (*Result, error):
  var vuln *entity.Vulnerability
  var aiMeta *entity.AIMetadata

  var wg sync.WaitGroup
  wg.Add(2)
  go func() { defer wg.Done(); vuln, _ = queryClient.GetByID(ctx, q.VulnID) }()
  go func() { defer wg.Done(); aiMeta, _ = aiClient.GetEnrichment(ctx, q.VulnID) }()
  wg.Wait()

  result := &Result{Vulnerability: vuln}
  if aiMeta != nil { result.AIMetadata = aiMeta }  // AI is optional
  return result, nil
```

## OSV Linter

```go
// application/query/lint_osv/handler.go
// Pure in-process: JSON Schema validation, no external calls
func Handle(ctx, q Query) (*Result, error):
  errors, warnings := schemaValidator.Validate(q.RawJSON)
  return &Result{IsValid: len(errors) == 0, Errors: errors, Warnings: warnings}, nil
```

## Response Format (Website API)

```json
// GET /api/v1/vulns/CVE-2023-12345
{
  "id": "CVE-2023-12345",
  "summary": "Remote code execution in foo",
  "details": "...",
  "modified": "2024-01-15T00:00:00Z",
  "severity": "CRITICAL",
  "cvss_score": 9.8,
  "ecosystems": ["PyPI"],
  "packages": ["requests"],
  "aliases": ["GHSA-xxxx-xxxx-xxxx"],
  "affected": [...],
  "references": [...],
  "ai_metadata": {
    "technical_summary": "...",
    "remediation_advice": "...",
    "attack_vector_tags": ["network", "unauthenticated"],
    "exploitability_score": 0.87
  },
  "related_vulns": [{"id": "GHSA-xxxx", "summary": "...", "score": 0.92}]
}
```

## Config

```go
type Config struct {
    HTTP        struct { Port int; MetricsPort int }
    QuerySvc    struct { Addr string; Timeout time.Duration }
    SearchSvc   struct { Addr string }
    AISvc       struct { Addr string }
    Redis       struct { Addr string }
    CORS        struct { AllowedOrigins []string }
    RateLimit   struct { RequestsPerMin int; BurstSize int }
    Telemetry   struct { OTLPEndpoint string }
}
```

## Rate Limiter (IP-based)
```go
// infra/ratelimit/redis_rate_limiter.go
// Key: "osv:ratelimit:{sha256(ip)}:{minute_timestamp}"
// Sliding window: INCR + EXPIRE via Lua script
// 30 req/min per IP; 429 Too Many Requests nếu exceeded
```

## CORS Config
```
Allowed origins: osv.dev, *.osv.dev, localhost:*
Methods: GET, POST, OPTIONS
Headers: Content-Type, X-API-Key
MaxAge: 86400s
```

## SLO Targets
- Availability: 99.95%
- Homepage P50: <50ms (cache hit)
- Search P50: <100ms
- Vuln detail P50: <80ms
- Cache hit rate (homepage): >95%

## Checklist Thực Thi

> **Status: ✅ COMPLETED (Core)** — 2026-06-01

- [x] Implement `QueryServiceClient` interface (GetByID, GetEcosystemStats)
- [x] Implement `SearchServiceClient` interface (Search, Autocomplete)
- [x] Implement `AIEnrichmentClient` interface (GetEnrichment)
- [x] Implement `GetHomepageStatsHandler` (stale-while-revalidate: hard TTL=24h, soft=30min)
- [x] Implement `GetVulnerabilityDetailHandler` (parallel fetch QuerySvc + AI, graceful AI degradation)
- [x] Implement `SearchVulnerabilitiesHandler` (proxy + transform)
- [x] Implement `AutocompleteHandler` (proxy, degrade gracefully)
- [x] Implement `LintOSVHandler` (JSON Schema validation in-process, required fields check)
- [x] Implement IP-based rate limiter (Redis Lua sliding window, 30 req/min)
- [x] CORS middleware (osv.dev, *.osv.dev, localhost:*; Methods: GET,POST,OPTIONS)
- [x] Logging middleware (request/response interceptor)
- [x] Setup chi router with full middleware chain (RequestID, RealIP, CORS, RateLimit)
- [x] Health handlers: live (always 200) + ready (check QuerySvc reachability)
- [x] 301 redirect: `/{ID}` → `/vulnerability/{ID}`
- [x] `Dockerfile` (multi-stage → distroless)
- [x] `go.mod` + workspace entry
- [ ] `infra/client/query_service_client.go` — real gRPC client implementation
- [ ] `infra/client/search_service_client.go` — real gRPC client
- [ ] `infra/client/ai_enrichment_client.go` — real gRPC client
- [ ] `infra/cache/redis/ecosystem_stats_cache.go` — stale-while-revalidate Redis adapter
- [ ] `infra/cache/redis/popular_vuln_cache.go`
- [ ] `list_vulnerabilities` handler + route (`GET /api/v1/list`)
- [ ] `get_vulnerability_related` handler + route (`GET /api/v1/vulns/{id}/related`)
- [ ] Unit tests: linter validation, response transformation
- [ ] Integration tests: mock upstream gRPC servers
- [ ] Makefile
