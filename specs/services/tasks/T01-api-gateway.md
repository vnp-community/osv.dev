# Task T01 — API Gateway Service

> **Priority:** P0 | **Phase:** 2 | **Spec:** `specs/services/01-api-gateway.md`  
> **Depends on:** T00-shared-libs, T12-infrastructure (Redis, NATS running)

## Mục Tiêu
Xây dựng single entry point cho toàn bộ external traffic. Không có business logic — chỉ cross-cutting concerns.

## Trách Nhiệm
- Auth (JWT, API Key, OAuth2) + Authorization (RBAC: reader/importer/admin)
- Rate limiting (Redis sliding window, per-client per-endpoint)
- gRPC reverse proxy đến upstream services
- HTTP ↔ gRPC transcoding (grpc-gateway)
- Circuit breaker cho upstream failures
- TLS termination, CORS
- OpenTelemetry trace injection
- Service discovery (K8s resolver)

## Không Làm
- Business logic, data persistence, response caching (chỉ auth token cache)

## Cấu Trúc File

```
services/api-gateway/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── policy/
│   │   │   ├── rate_limit_policy.go    # RateLimitRule per endpoint/tier
│   │   │   └── routing_policy.go       # Route{Path, Method, Upstream, AuthRequired}
│   │   └── auth/
│   │       ├── api_key.go              # APIKey value object (validated, hashed)
│   │       └── principal.go           # Principal{ID, Type, Roles, RateLimitTier}
│   ├── application/
│   │   ├── command/revoke_key/handler.go
│   │   └── query/
│   │       ├── resolve_route/handler.go
│   │       └── validate_auth/handler.go
│   └── infra/
│       ├── auth/
│       │   ├── jwt_validator.go        # Validate Bearer JWT
│       │   ├── api_key_validator.go    # Lookup API key từ Redis/Firestore
│       │   └── oauth2_validator.go
│       ├── ratelimit/redis_rate_limiter.go
│       ├── proxy/
│       │   ├── grpc_proxy.go           # Transparent gRPC proxy
│       │   └── http_proxy.go
│       ├── discovery/k8s_resolver.go   # Resolve service names → gRPC addresses
│       └── cache/redis_token_cache.go
├── interface/
│   ├── grpc/
│   │   ├── handler/gateway_handler.go
│   │   └── middleware/
│   │       ├── auth_interceptor.go
│   │       ├── ratelimit_interceptor.go
│   │       ├── tracing_interceptor.go
│   │       └── logging_interceptor.go
│   └── http/
│       ├── handler/
│       │   ├── gateway_handler.go      # HTTP → gRPC proxy
│       │   └── health_handler.go
│       └── middleware/
│           ├── auth_middleware.go
│           ├── cors_middleware.go
│           └── ratelimit_middleware.go
└── config/config.go
```

## Domain Models Chính

```go
// domain/policy/routing_policy.go
type Route struct {
    Path         string      // "/v1/vulns/{id}"
    Method       string      // GET | POST | *
    Upstream     ServiceRef  // Target service
    AuthRequired bool
    RequiredRole Role        // "" = any authenticated
    RateLimit    RateLimitRef
    BodySizeLimit int64      // 0 = no limit
    InternalOnly bool
}
type ServiceRef struct {
    Name    string  // "vulnerability-query-service"
    Address string  // resolved via discovery
    TLS     bool
}

// domain/auth/principal.go
type PrincipalType string
const (PrincipalAPIKey PrincipalType = "API_KEY"; PrincipalOAuth2 = "OAUTH2"; PrincipalService = "SERVICE_ACCOUNT")

type Principal struct {
    ID            string
    Type          PrincipalType
    Roles         []Role
    RateLimitTier string  // "anonymous" | "free" | "premium" | "internal"
    Metadata      map[string]string
}
type Role string
const (RoleReader Role = "reader"; RoleImporter = "importer"; RoleAdmin = "admin")
```

## Rate Limiter Design

```go
// infra/ratelimit/redis_rate_limiter.go
// Sliding window via Redis Lua script (atomic)
type RateLimitKey struct {
    ClientID  string  // API key ID or IP
    Endpoint  string
    Ecosystem string  // optional
}
type RateLimitResult struct {
    Allowed    bool
    Limit      int
    Remaining  int
    ResetAt    time.Time
    RetryAfter time.Duration
}
// Tiers: anonymous=10/min, free=100/min+1000/h, premium=1000/min+100K/h, internal=unlimited
```

## gRPC Middleware Chain (Order Quan Trọng)
```go
grpc.ChainUnaryInterceptor(
    middleware.Recovery(),           // 1. panic recovery
    middleware.RequestID(),          // 2. inject X-Request-ID
    middleware.Tracing(tracer),      // 3. OTel span
    middleware.Logging(logger),      // 4. structured log
    middleware.Auth(authValidator),  // 5. validate auth
    middleware.RateLimit(limiter),   // 6. rate limit
    middleware.Timeout(30*time.Second),
    middleware.CircuitBreaker(),
)
```

## Route Config (routes.yaml)
```yaml
routes:
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
    body_size_limit: 1048576  # 1MB

  - path: "/v1experimental/determineversion"
    method: POST
    upstream: version-index-service
    auth_required: false

  - path: "/v1admin/sources"
    method: POST
    upstream: source-sync-service
    auth_required: true
    required_role: admin

  - path: "/internal/*"
    auth_required: true
    internal_only: true
```

## Config Struct
```go
type Config struct {
    Server   struct { GRPCPort int; HTTPPort int; MetricsPort int }
    Auth     struct { JWTSecret string; APIKeyHashSalt string }
    Redis    struct { Addr string; Password string; DB int }
    Routes   string  // path to routes.yaml
    Telemetry struct { OTLPEndpoint string; ServiceName string }
}
```

## Metrics Cần Expose
```
gateway_requests_total{route, method, status, upstream}
gateway_request_duration_seconds{route, upstream} histogram
gateway_upstream_errors_total{upstream, error_type}
gateway_rate_limited_total{tier, endpoint}
```

## Health Checks
```
GET /health/live  → 200 always
GET /health/ready → check Redis + at least 1 upstream reachable
GET /metrics      → Prometheus
```

## SLO Targets
- Availability: 99.99%
- P50 overhead: <5ms, P99 overhead: <50ms
- Error rate: <0.01%

## Checklist Thực Thi

> **Status: ✅ COMPLETED (Core)** — 2026-06-01

- [x] `go mod init github.com/osv/api-gateway` → `services/api-gateway/go.mod`
- [x] Implement domain: `Route`, `ServiceRef`, `Principal`, `RateLimitPolicy` (`domain/policy/routing_policy.go`)
- [x] Implement `infra/ratelimit/redis_rate_limiter.go` (Lua sliding window, atomic)
- [x] Implement `infra/proxy/grpc_proxy.go` (transparent gRPC proxy)
- [x] Load routes từ `routes.yaml` vào `RoutingPolicy` (with path param + wildcard matching)
- [x] Rate limit tiers: anonymous=10/min, public=100/min, premium=1000/min, internal=unlimited
- [x] `config/routes.yaml` với tất cả routes spec
- [x] `Dockerfile` (multi-stage → distroless)
- [ ] Implement `infra/auth/jwt_validator.go` (dùng `golang-jwt/jwt`)
- [ ] Implement `infra/auth/api_key_validator.go` (Redis lookup)
- [ ] Implement `infra/discovery/k8s_resolver.go`
- [ ] Implement gRPC middleware chain (8 interceptors đúng thứ tự)
- [ ] Implement HTTP handlers: gateway + health
- [ ] `cmd/server/main.go`: wire tất cả dependencies, graceful shutdown
- [ ] Unit test: rate limiter logic, route matching, auth validation
- [ ] Integration test: mock upstream, verify auth/rate-limit behavior
- [ ] Makefile
