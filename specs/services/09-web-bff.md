# Service 09 — Web BFF (Backend for Frontend)

> **Version:** 1.0 | **Status:** Proposed | **Priority:** P1  
> **Language:** Go  
> **Pattern:** BFF Pattern + Clean Architecture  
> **Communication:** gRPC (upstream calls) + HTTP/REST (frontend)

---

## 1. Trách Nhiệm

**Backend for Frontend** — thay thế Flask website backend (`gcp/website/`). Tối ưu hóa cho website/UI consumers, aggregate data từ multiple services, không expose business logic.

**Responsibilities:**
- Serve website pages data (homepage, search, vulnerability detail, list)
- Aggregate data từ Query Service + Search Service + AI Enrichment
- Ecosystem statistics cache
- OSV JSON Linter tool endpoint
- Blog metadata
- Rate limiting per IP (30 req/min for web)
- CORS handling
- Server-Side Rendering data (for Next.js or Hugo)
- Redirect `/{id}` → `/vulnerability/{id}`

**NOT Responsible for:**
- Business logic (delegates to backend services)
- Auth (delegated to API Gateway)
- Direct database access

---

## 2. Clean Architecture Layers

```
Domain:
  ├── EcosystemStats entity (counts per ecosystem)
  ├── VulnerabilityListItem entity (lightweight display model)
  └── LinterResult entity

Application:
  ├── GetHomepageStatsQuery + Handler
  ├── SearchVulnerabilitiesQuery + Handler (wraps Search Service)
  ├── GetVulnerabilityDetailQuery + Handler (wraps Query + AI)
  ├── ListVulnerabilitiesQuery + Handler
  └── LintOSVQuery + Handler

Infrastructure:
  ├── QueryServiceGrpcClient
  ├── SearchServiceGrpcClient
  ├── AIEnrichmentServiceGrpcClient
  ├── RedisCache (ecosystem counts, popular vulns)
  └── RateLimiter (Redis sliding window)

Interface:
  ├── HTTP handler (website REST API)
  └── HTTP middleware (CORS, rate limit, logging)
```

---

## 3. Directory Structure

```
services/web-bff/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── entity/
│   │   │   ├── ecosystem_stats.go          # Counts per ecosystem
│   │   │   ├── vuln_list_item.go           # Lightweight listing model
│   │   │   └── linter_result.go
│   │   └── valueobject/
│   │       ├── ecosystem.go
│   │       └── search_params.go
│   ├── application/
│   │   └── query/
│   │       ├── get_homepage_stats/
│   │       │   ├── query.go
│   │       │   └── handler.go
│   │       ├── search_vulnerabilities/
│   │       │   ├── query.go
│   │       │   └── handler.go
│   │       ├── get_vulnerability_detail/
│   │       │   ├── query.go
│   │       │   └── handler.go
│   │       ├── list_vulnerabilities/
│   │       │   ├── query.go
│   │       │   └── handler.go
│   │       └── lint_osv/
│   │           ├── query.go
│   │           └── handler.go
│   └── infra/
│       ├── client/
│       │   ├── query_service_client.go
│       │   ├── search_service_client.go
│       │   └── ai_enrichment_client.go
│       ├── cache/
│       │   └── redis/
│       │       ├── ecosystem_stats_cache.go
│       │       └── popular_vuln_cache.go
│       └── ratelimit/
│           └── redis_rate_limiter.go        # 30 req/min per IP
├── interface/
│   └── http/
│       ├── handler/
│       │   ├── homepage_handler.go
│       │   ├── search_handler.go
│       │   ├── vulnerability_handler.go
│       │   ├── list_handler.go
│       │   ├── linter_handler.go
│       │   └── health_handler.go
│       └── middleware/
│           ├── cors_middleware.go
│           ├── rate_limit_middleware.go
│           ├── logging_middleware.go
│           └── tracing_middleware.go
├── config/config.go
├── Dockerfile
└── go.mod
```

---

## 4. HTTP API Routes

```go
// interface/http/handler/router.go

func SetupRouter(handlers *Handlers) http.Handler {
    mux := chi.NewRouter()
    
    // Middleware chain
    mux.Use(middleware.Tracing())
    mux.Use(middleware.Logging())
    mux.Use(middleware.CORS())
    mux.Use(middleware.RateLimit(30, time.Minute)) // 30 req/min per IP
    
    // Homepage
    mux.Get("/api/v1/stats", handlers.Homepage.GetStats)
    
    // Search
    mux.Get("/api/v1/search", handlers.Search.Search)
    mux.Get("/api/v1/search/autocomplete", handlers.Search.Autocomplete)
    
    // Vulnerability listing
    mux.Get("/api/v1/list", handlers.List.List)
    
    // Vulnerability detail
    mux.Get("/api/v1/vulns/{id}", handlers.Vulnerability.GetDetail)
    mux.Get("/api/v1/vulns/{id}/related", handlers.Vulnerability.GetRelated)
    
    // Redirect: /{id} → /vulnerability/{id}
    mux.Get("/{id:[A-Z]+-[0-9].*}", handlers.Redirect.RedirectToVuln)
    
    // Linter
    mux.Post("/api/v1/lint", handlers.Linter.Lint)
    
    // Health
    mux.Get("/health/live", handlers.Health.Live)
    mux.Get("/health/ready", handlers.Health.Ready)
    mux.Get("/metrics", handlers.Metrics.Serve)
    
    return mux
}
```

---

## 5. Homepage Stats Handler

```go
// interface/http/handler/homepage_handler.go

type HomepageHandler struct {
    getStats *get_homepage_stats.Handler
}

func (h *HomepageHandler) GetStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    result, err := h.getStats.Handle(ctx, get_homepage_stats.Query{})
    if err != nil {
        writeError(w, err)
        return
    }
    
    writeJSON(w, http.StatusOK, result)
}

// application/query/get_homepage_stats/handler.go
type Handler struct {
    queryClient  port.QueryServiceClient
    statsCache   port.StatsCache
    tracer       trace.Tracer
}

func (h *Handler) Handle(ctx context.Context, q Query) (*Result, error) {
    // Cache with hard=24h, soft=30min (same as original)
    if cached, ok := h.statsCache.Get(ctx, "homepage_stats"); ok {
        return cached, nil
    }
    
    stats, err := h.queryClient.GetEcosystemStats(ctx)
    if err != nil {
        // Return stale cache on error (graceful degradation)
        if stale, ok := h.statsCache.GetStale(ctx, "homepage_stats"); ok {
            return stale, nil
        }
        return nil, err
    }
    
    h.statsCache.Set(ctx, "homepage_stats", stats, 24*time.Hour)
    return stats, nil
}
```

---

## 6. Vulnerability Detail Aggregation

```go
// application/query/get_vulnerability_detail/handler.go
// Aggregates: Query Service + AI Enrichment Service

type Handler struct {
    queryClient     port.QueryServiceClient
    aiClient        port.AIEnrichmentClient
    tracer          trace.Tracer
}

func (h *Handler) Handle(ctx context.Context, q Query) (*Result, error) {
    ctx, span := h.tracer.Start(ctx, "GetVulnerabilityDetail")
    defer span.End()
    
    // Parallel fetch: core data + AI metadata
    var (
        vuln      *entity.Vulnerability
        aiMeta    *entity.AIMetadata
        vulnErr   error
        aiErr     error
    )
    
    var wg sync.WaitGroup
    wg.Add(2)
    
    go func() {
        defer wg.Done()
        vuln, vulnErr = h.queryClient.GetByID(ctx, q.VulnID)
    }()
    
    go func() {
        defer wg.Done()
        aiMeta, aiErr = h.aiClient.GetEnrichment(ctx, q.VulnID)
    }()
    
    wg.Wait()
    
    if vulnErr != nil {
        return nil, vulnErr
    }
    
    result := &Result{Vulnerability: vuln}
    if aiErr == nil {
        result.AIMetadata = aiMeta
    }
    // AI metadata is optional — degrade gracefully
    
    return result, nil
}
```

---

## 7. OSV Linter Tool

```go
// application/query/lint_osv/handler.go

// Linter validates OSV JSON submitted by users (website tool).
// Pure in-process validation, no external calls needed.

type Handler struct {
    schemaValidator *jsonschema.Validator
}

func (h *Handler) Handle(ctx context.Context, q Query) (*Result, error) {
    // Validate against OSV schema
    errors, warnings := h.schemaValidator.Validate(q.RawJSON)
    
    return &Result{
        IsValid:  len(errors) == 0,
        Errors:   errors,
        Warnings: warnings,
    }, nil
}
```

---

## 8. Response Format (Website API)

```json
// GET /api/v1/vulns/CVE-2023-12345
{
  "id": "CVE-2023-12345",
  "summary": "Remote code execution in foo",
  "details": "...",
  "modified": "2024-01-15T00:00:00Z",
  "published": "2023-06-01T00:00:00Z",
  "severity": "CRITICAL",
  "cvss_score": 9.8,
  "ecosystems": ["PyPI"],
  "packages": ["requests"],
  "aliases": ["GHSA-xxxx-xxxx-xxxx"],
  "affected": [...],
  "references": [...],
  
  // AI-enriched fields (when available)
  "ai_metadata": {
    "technical_summary": "AI-generated concise explanation...",
    "remediation_advice": "Upgrade to version X or apply patch Y...",
    "attack_vector_tags": ["network", "unauthenticated"],
    "exploitability_score": 0.87
  },
  
  // Related vulnerabilities
  "related_vulns": [
    {"id": "GHSA-xxxx", "summary": "...", "score": 0.92}
  ]
}
```

---

## 9. SLO Targets

| Metric | Target |
|--------|--------|
| Availability | 99.95% |
| Homepage P50 latency | < 50ms (cache hit) |
| Search P50 latency | < 100ms |
| Vuln detail P50 latency | < 80ms |
| Cache hit rate (homepage) | > 95% |
| Rate limit enforcement | 100% accuracy |

---

## 10. Implementation Status

> **Status:** ✅ Core Implemented | **Updated:** 2026-06-01

### Implemented
- [x] `application/query/get_homepage_stats/handler.go` — Stale-while-revalidate (hard=24h, soft=30min)
- [x] `application/query/get_vulnerability_detail/handler.go` — Parallel Query + AI fetch (goroutines + graceful AI degradation)
- [x] `application/query/search_vulnerabilities/handler.go` — Wraps Search Service
- [x] `application/query/lint_osv/handler.go` — In-process JSON Schema validation
- [x] `interface/http/handler/handlers.go` — Full route handlers: homepage, search, vuln detail, redirect, linter, health
- [x] `interface/http/middleware/middleware.go` — CORS, logging, rate limiter (Redis Lua sliding window, 30 req/min per IP)
- [x] `cmd/server/main.go` — chi router, full middleware chain, health check, 301 redirect `/{id}` → `/vulnerability/{id}`
- [x] `Dockerfile`, `go.mod`

### Pending
- [ ] `infra/client/query_service_client.go` — Real gRPC client to Vulnerability Query Service
- [ ] `infra/client/search_service_client.go` — Real gRPC client to Search Service
- [ ] `infra/client/ai_enrichment_client.go` — Real gRPC client to AI Enrichment Service
- [ ] `infra/cache/redis/ecosystem_stats_cache.go` — Ecosystem stats cache
- [ ] `infra/cache/redis/popular_vuln_cache.go` — Popular vulnerability cache
- [ ] `application/query/list_vulnerabilities/handler.go` — List vulnerabilities handler
- [ ] `application/query/get_vulnerability_detail/` — Related vulnerabilities (GetRelated)
- [ ] Unit + integration tests, Makefile

### Deviations from Spec
- Rate limiter uses same Redis Lua sliding window pattern as API Gateway (30 req/min per IP vs Gateway's per-client)
- gRPC client implementations are interface stubs pending real service proto-gen
