# Service 01 — API Gateway

> **Version:** 1.0 | **Status:** Proposed | **Priority:** P0  
> **Language:** Go  
> **Pattern:** Reverse Proxy + API Gateway

---

## 1. Trách Nhiệm

API Gateway là **single entry point** cho toàn bộ external traffic. Không chứa business logic — chỉ xử lý cross-cutting concerns.

**Responsibilities:**
- Authentication & Authorization
- Rate Limiting (per client, per endpoint, per ecosystem)
- Request routing đến upstream services
- gRPC ↔ HTTP/JSON transcoding
- Circuit Breaker cho upstream failures
- Request/Response transformation (version negotiation)
- Distributed tracing injection
- API documentation (OpenAPI v3)
- TLS termination
- CORS handling

**NOT Responsible for:**
- Business logic của bất kỳ domain nào
- Data persistence
- Caching (chỉ metadata cache cho auth)

---

## 2. Clean Architecture

```
interface/grpc/handler/     → gRPC proxy handlers
interface/http/handler/     → HTTP REST handlers
application/command/        → (minimal - mostly passthrough)
application/query/          → Auth context, route resolution
domain/                     → Gateway policy rules (rate limits, routing rules)
infra/
  ├── auth/                 → JWT validation, API key lookup
  ├── ratelimit/            → Redis-backed rate limiter
  ├── proxy/                → gRPC reverse proxy
  ├── discovery/            → Service discovery (k8s / Consul)
  └── cache/                → Auth token cache (Redis)
```

---

## 3. Directory Structure

```
services/api-gateway/
├── cmd/server/
│   └── main.go
├── internal/
│   ├── domain/
│   │   ├── policy/
│   │   │   ├── rate_limit_policy.go    # Rate limit rules per endpoint
│   │   │   └── routing_policy.go      # Route → service mapping
│   │   └── auth/
│   │       ├── api_key.go              # API key value object
│   │       └── principal.go           # Authenticated principal
│   ├── application/
│   │   ├── command/
│   │   │   └── revoke_key/
│   │   │       └── handler.go
│   │   └── query/
│   │       ├── resolve_route/
│   │       │   └── handler.go
│   │       └── validate_auth/
│   │           └── handler.go
│   └── infra/
│       ├── auth/
│       │   ├── jwt_validator.go
│       │   ├── api_key_validator.go
│       │   └── oauth2_validator.go
│       ├── ratelimit/
│       │   └── redis_rate_limiter.go
│       ├── proxy/
│       │   ├── grpc_proxy.go
│       │   └── http_proxy.go
│       ├── discovery/
│       │   └── k8s_resolver.go
│       └── cache/
│           └── redis_token_cache.go
├── interface/
│   ├── grpc/
│   │   ├── handler/
│   │   │   └── gateway_handler.go
│   │   └── middleware/
│   │       ├── auth_interceptor.go
│   │       ├── ratelimit_interceptor.go
│   │       ├── tracing_interceptor.go
│   │       └── logging_interceptor.go
│   └── http/
│       ├── handler/
│       │   ├── gateway_handler.go     # HTTP → gRPC proxy
│       │   └── health_handler.go
│       └── middleware/
│           ├── auth_middleware.go
│           ├── cors_middleware.go
│           └── ratelimit_middleware.go
└── config/
    └── config.go
```

---

## 4. Domain Model

```go
// domain/policy/routing_policy.go
package policy

type Route struct {
    Path        string        // e.g., "/v1/vulns/{id}"
    Method      string        // GET, POST, *
    Upstream    ServiceRef    // Target service
    AuthRequired bool
    RateLimit   RateLimitRef
}

type ServiceRef struct {
    Name    string  // "vulnerability-query-service"
    Address string  // "vuln-query-svc:50051" or resolved via discovery
    TLS     bool
}

// domain/auth/principal.go
type Principal struct {
    ID          string
    Type        PrincipalType // API_KEY | OAUTH2 | SERVICE_ACCOUNT
    Roles       []Role
    RateLimitTier string     // "free" | "premium" | "internal"
    Metadata    map[string]string
}

type Role string
const (
    RoleReader   Role = "reader"
    RoleImporter Role = "importer"
    RoleAdmin    Role = "admin"
)
```

---

## 5. Rate Limiting Design

```go
// infra/ratelimit/redis_rate_limiter.go

type RateLimiter interface {
    Allow(ctx context.Context, key RateLimitKey) (bool, RateLimitResult, error)
}

type RateLimitKey struct {
    ClientID  string  // API key or IP
    Endpoint  string  // "/v1/query"
    Ecosystem string  // optional - per-ecosystem limiting
}

type RateLimitResult struct {
    Allowed    bool
    Limit      int
    Remaining  int
    ResetAt    time.Time
    RetryAfter time.Duration
}

// Sliding window algorithm via Redis Lua script
// Limits:
//   - Anonymous: 10 req/min
//   - Free tier: 100 req/min, 1000/hour
//   - Premium: 1000 req/min, 100K/hour
//   - Internal service: unlimited
```

---

## 6. gRPC Middleware Chain

```go
// interface/grpc/middleware/

// Order matters - applied in sequence:
chain := grpc.ChainUnaryInterceptor(
    middleware.Recovery(),           // 1. Panic recovery
    middleware.RequestID(),          // 2. Inject request ID
    middleware.Tracing(tracer),      // 3. OpenTelemetry trace
    middleware.Logging(logger),      // 4. Structured logging
    middleware.Auth(authValidator),  // 5. Authentication
    middleware.RateLimit(limiter),   // 6. Rate limiting
    middleware.Timeout(30*time.Second), // 7. Deadline
    middleware.CircuitBreaker(),     // 8. Circuit breaker
)
```

---

## 7. Route Configuration

```yaml
# config/routes.yaml
routes:
  # Vulnerability Query Service
  - path: "/v1/vulns/{id}"
    method: GET
    upstream: vulnerability-query-service
    auth_required: false
    rate_limit_tier: public

  - path: "/v1/query"
    method: POST
    upstream: vulnerability-query-service
    auth_required: false
    rate_limit_tier: public

  - path: "/v1/querybatch"
    method: POST
    upstream: vulnerability-query-service
    auth_required: false
    rate_limit_tier: public
    body_size_limit: 1MB

  # Version Index Service
  - path: "/v1experimental/determineversion"
    method: POST
    upstream: version-index-service
    auth_required: false
    rate_limit_tier: public

  # Import Findings
  - path: "/v1experimental/importfindings/{source}"
    method: GET
    upstream: ingestion-service
    auth_required: false
    rate_limit_tier: public

  # Admin APIs (require auth)
  - path: "/v1/admin/sources"
    method: POST
    upstream: source-sync-service
    auth_required: true
    required_role: admin

  # Internal - not exposed externally
  - path: "/internal/*"
    auth_required: true
    internal_only: true
```

---

## 8. Observability

```go
// Every request emits:
// - Span with: route, upstream, status_code, latency
// - Metric: gateway_requests_total{route, method, status, upstream}
// - Metric: gateway_request_duration_seconds{route, upstream} (histogram)
// - Metric: gateway_upstream_errors_total{upstream, error_type}
// - Metric: gateway_rate_limited_total{tier, endpoint}
// - Log: {level, request_id, trace_id, route, client_id, status, latency_ms}
```

---

## 9. Health & Readiness

```go
// GET /health/live → always 200 if process running
// GET /health/ready → checks:
//   - Redis connectivity (rate limiter)
//   - Auth service connectivity
//   - At least 1 upstream reachable

// GET /metrics → Prometheus metrics
// GET /debug/pprof → Go profiling (internal only)
```

---

## 10. gRPC Service Routing (Proto)

```protobuf
// The gateway does NOT define its own proto.
// It proxies to upstream services' protos transparently.
// Uses grpc-gateway for HTTP transcoding.
// Service discovery resolves upstream addresses.
```

---

## 11. SLO/SLA Targets

| Metric | Target |
|--------|--------|
| Availability | 99.99% |
| P50 latency | < 5ms (gateway overhead only) |
| P99 latency | < 50ms (gateway overhead only) |
| Error rate | < 0.01% |
| Rate limit accuracy | > 99.9% |

---

## 12. Implementation Status

> **Status:** ✅ Core Implemented | **Updated:** 2026-06-01

### Implemented
- [x] `domain/policy/routing_policy.go` — Route, ServiceRef, RateLimitTier, DefaultRoutes()
- [x] `domain/auth/principal.go` — Principal, PrincipalType, Role, AnonymousPrincipal()
- [x] `infra/ratelimit/redis_rate_limiter.go` — Sliding window Lua script (sliding window per client+endpoint)
- [x] `infra/proxy/grpc_proxy.go` — Transparent gRPC reverse proxy with connection pool
- [x] `infra/auth/jwt_validator.go` — JWKS-based JWT validation with 1h cache
- [x] `infra/auth/api_key_validator.go` — Redis-backed API key validation (5min TTL)
- [x] `cmd/server/main.go` — gRPC server + HTTP health, graceful shutdown, interceptor chain
- [x] `Dockerfile` — Multi-stage → distroless

### Pending
- [ ] `infra/discovery/k8s_resolver.go` — K8s service discovery (currently uses static addresses)
- [ ] `interface/grpc/middleware/` — Full 8-interceptor chain (circuit breaker, full auth, timeout)
- [ ] `interface/http/handler/gateway_handler.go` — HTTP→gRPC transcoding (grpc-gateway)
- [ ] `config/routes.yaml` — Move routes from DefaultRoutes() to YAML-loaded config
- [ ] Unit tests (rate limiter, routing, auth)
- [ ] Integration tests (with upstream stubs)
- [ ] Helm chart / K8s manifests

### Deviations from Spec
- Auth validators implemented as standalone infra adapters (not application layer use cases)
- gRPC transcoding stubs only — full grpc-gateway transcoding requires proto-gen toolchain
