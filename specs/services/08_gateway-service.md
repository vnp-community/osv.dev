# gateway-service

**Bounded Context**: API Gateway & Backend for Frontend (BFF)
**Go Module**: `github.com/osv/gateway-service`

---

## Merge từ

| Source | Trạng thái |
|--------|-----------|
| `services/unified-gateway` | ✅ Active — base chính |
| `archive/api-gateway` | 📦 Archive — merged |
| `archive/dd-api-gateway` | 📦 Archive — merged |
| `archive/web-bff` | 📦 Archive — merged |
| `archive/info-service` | 📦 Archive — merged |

---

## Chức năng

| # | Chức năng | Mô tả |
|---|-----------|-------|
| 1 | **Single Entry Point** | Tất cả external requests đi qua gateway |
| 2 | **JWT Authentication** | Validate JWT bằng cách gọi identity-service |
| 3 | **API Key Auth** | Validate API key cho programmatic access |
| 4 | **Authorization** | RBAC permission check cho từng endpoint |
| 5 | **Smart Routing** | Route requests đến đúng upstream service |
| 6 | **Rate Limiting** | Per-user và per-IP rate limiting |
| 7 | **BFF Aggregation** | Gom nhiều service calls thành 1 response |
| 8 | **Request Transform** | Transform request/response format nếu cần |
| 9 | **Health Aggregation** | Aggregate health status của tất cả services |
| 10 | **CORS & Security** | CORS headers, security headers |
| 11 | **Observability** | Request tracing, access logs, metrics |

---

## Clean Architecture Layout

```
gateway-service/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── domain/                         # ← Gateway domain rules
│   │   ├── auth/
│   │   │   ├── principal.go            # AuthenticatedPrincipal value object
│   │   │   ├── token.go                # Token value objects
│   │   │   └── repository.go           # Auth cache interface
│   │   ├── policy/
│   │   │   ├── access_policy.go        # AccessPolicy entity
│   │   │   ├── route_policy.go         # RoutePolicy (which roles can access)
│   │   │   └── evaluator.go            # Policy evaluation
│   │   └── entity/
│   │       └── upstream.go             # Upstream service entity
│   │
│   ├── usecase/                        # ← Application use cases
│   │   ├── authenticate/
│   │   │   ├── jwt.go                  # JWT validation
│   │   │   └── apikey.go               # API key validation
│   │   ├── authorize/
│   │   │   └── usecase.go              # RBAC permission check
│   │   └── aggregate_bff/
│   │       ├── dashboard.go            # Dashboard BFF (findings + stats + scans)
│   │       ├── cve_detail.go           # CVE detail BFF (cve + enrichment + findings)
│   │       └── asset_overview.go       # Asset overview BFF
│   │
│   ├── delivery/                       # ← Transport layer (HTTP only)
│   │   └── http/
│   │       ├── router.go               # Route registration
│   │       ├── middleware/
│   │       │   ├── auth.go             # Auth middleware
│   │       │   ├── ratelimit.go        # Rate limit middleware
│   │       │   ├── cors.go             # CORS middleware
│   │       │   ├── logger.go           # Request logging
│   │       │   └── tracing.go          # OpenTelemetry tracing
│   │       ├── proxy/
│   │       │   └── handler.go          # Reverse proxy handler
│   │       ├── bff/
│   │       │   ├── dashboard_handler.go
│   │       │   ├── cve_handler.go
│   │       │   └── asset_handler.go
│   │       └── health/
│   │           └── handler.go          # Aggregate health endpoint
│   │
│   ├── infra/                          # ← External systems
│   │   ├── redis/
│   │   │   ├── token_cache.go          # Cache validated tokens (TTL 1min)
│   │   │   └── ratelimit.go            # Redis-backed rate limiter
│   │   └── clients/                    # ← gRPC clients to upstream services
│   │       ├── identity_client.go      # identity-service gRPC client
│   │       ├── data_client.go          # data-service gRPC client
│   │       ├── search_client.go        # search-service gRPC client
│   │       ├── scan_client.go          # scan-service gRPC client
│   │       ├── finding_client.go       # finding-service gRPC client
│   │       ├── ai_client.go            # ai-service gRPC client
│   │       └── notification_client.go  # notification-service gRPC client
│   │
│   ├── proxy/
│   │   └── reverse_proxy.go            # HTTP reverse proxy logic
│   │
│   └── ratelimit/
│       └── limiter.go                  # Rate limit logic
│
├── config/
│   ├── routes.yaml                     # Route configuration
│   └── upstreams.yaml                  # Upstream service addresses
│
├── go.mod
└── Dockerfile
```

---

## Route Configuration

```yaml
# config/routes.yaml
routes:
  - prefix: "/api/v1/auth"
    upstream: "identity-service"
    auth: false                    # Public endpoints
    paths:
      - "POST /register"
      - "POST /login"
      - "GET  /oauth/{provider}"

  - prefix: "/api/v1/auth"
    upstream: "identity-service"
    auth: true
    paths:
      - "POST /logout"
      - "POST /refresh"
      - "GET  /me"
      - "CRUD /api-keys"

  - prefix: "/api/v1/cve"
    upstream: "data-service"
    auth: true
    roles: ["viewer", "analyst", "admin"]

  - prefix: "/api/v1/search"
    upstream: "search-service"
    auth: true
    roles: ["viewer", "analyst", "admin"]

  - prefix: "/api/v1/scan"
    upstream: "scan-service"
    auth: true
    roles: ["analyst", "admin"]

  - prefix: "/api/v1/findings"
    upstream: "finding-service"
    auth: true
    roles: ["analyst", "admin"]

  - prefix: "/api/v1/reports"
    upstream: "finding-service"
    auth: true
    roles: ["analyst", "admin"]

  - prefix: "/api/v1/products"
    upstream: "finding-service"
    auth: true
    roles: ["analyst", "admin"]

  - prefix: "/api/v1/ai"
    upstream: "ai-service"
    auth: true
    roles: ["analyst", "admin"]

  - prefix: "/api/v1/notifications"
    upstream: "notification-service"
    auth: true
    roles: ["analyst", "admin"]

  # BFF aggregated endpoints (handled by gateway itself)
  - prefix: "/api/v1/bff"
    upstream: "gateway-bff"
    auth: true

  - prefix: "/health"
    upstream: "gateway-health"
    auth: false
```

---

## BFF Endpoints (Aggregated)

### Dashboard BFF
```
GET /api/v1/bff/dashboard
```
Aggregates từ:
1. `finding-service` → recent findings, stats, SLA status
2. `scan-service` → recent scans, agent status
3. `data-service` → latest KEV additions

Response:
```json
{
  "findings": {
    "total": 1243,
    "by_severity": {"critical": 12, "high": 89},
    "sla_breached": 3,
    "recent": [...]
  },
  "scans": {
    "running": 2,
    "completed_today": 15,
    "agents_online": 4
  },
  "kev": {
    "total": 1189,
    "added_this_week": 5
  }
}
```

### CVE Detail BFF
```
GET /api/v1/bff/cve/{id}
```
Aggregates từ:
1. `data-service` → CVE data, KEV, aliases
2. `ai-service` → AI enrichment, EPSS score
3. `finding-service` → Active findings for this CVE

### Asset Overview BFF
```
GET /api/v1/bff/assets/{id}/overview
```
Aggregates từ:
1. `scan-service` → Asset info, recent scans
2. `finding-service` → Findings for this asset, SLA status

---

## Authentication Flow

```
Client Request
     │
     ▼
gateway-service
     │
     ├─ Extract token from header
     │
     ├─ Check Redis cache (validated tokens)
     │   ├─ HIT  → use cached principal → skip gRPC call
     │   └─ MISS → call identity-service gRPC ValidateToken
     │                   │
     │               ┌───▼───┐
     │               │identity│
     │               │service │
     │               └───┬───┘
     │                   │ Returns: userID, roles, scopes
     │
     ├─ Cache result in Redis (TTL: 1 minute)
     │
     ├─ Check RoutePolicy (role required for this route)
     │
     └─ Proxy to upstream service (with X-User-ID, X-Roles headers)
```

---

## Rate Limiting Strategy

```
# Per-user limits
Anonymous:    100 req/hour
Viewer:       1,000 req/hour
Analyst:      10,000 req/hour
Admin:        unlimited

# Per-endpoint limits
POST /search: 200 req/minute per user
POST /reports/generate: 10 req/hour per user
POST /admin/sync: 5 req/day per user

# Implementation: Redis token bucket
Key format: ratelimit:{user_id}:{window}
```

---

## Security Headers

```
Strict-Transport-Security: max-age=63072000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'self'
Referrer-Policy: strict-origin-when-cross-origin
```

---

## Health Aggregation

```
GET /health
```

```json
{
  "status": "degraded",
  "services": {
    "identity-service": {"status": "healthy", "latency_ms": 2},
    "data-service":     {"status": "healthy", "latency_ms": 5},
    "search-service":   {"status": "degraded", "latency_ms": 250},
    "scan-service":     {"status": "healthy", "latency_ms": 3},
    "finding-service":  {"status": "healthy", "latency_ms": 4},
    "ai-service":       {"status": "healthy", "latency_ms": 45},
    "notification-service": {"status": "healthy", "latency_ms": 3}
  },
  "version": "1.2.0",
  "uptime_seconds": 86400
}
```

---

## gRPC Client Pool

```go
// All upstream gRPC clients use connection pooling
type UpstreamClients struct {
    Identity     identitypb.AuthServiceClient
    Data         datapb.CVEServiceClient
    Search       searchpb.SearchServiceClient
    Scan         scanpb.ScanServiceClient
    Finding      findingpb.FindingServiceClient
    AI           aipb.AIEnrichmentServiceClient
    Notification notifpb.NotificationServiceClient
}
```

---

## Dependencies

```
github.com/go-chi/chi/v5       # HTTP router
github.com/redis/go-redis/v9   # Token cache, rate limiter
google.golang.org/grpc         # gRPC clients to all services
go.opentelemetry.io/otel/*     # Distributed tracing
github.com/osv/shared/pkg      # Shared utilities
github.com/osv/shared/proto    # gRPC client contracts
```

---

## Configuration

```yaml
server:
  http_port: 8080
  read_timeout: "30s"
  write_timeout: "30s"

redis:
  addr: "${REDIS_ADDR}"
  db: 4
  token_cache_ttl: "1m"

upstreams:
  identity_service:  "identity-service:50051"
  data_service:      "data-service:50052"
  search_service:    "search-service:50053"
  scan_service:      "scan-service:50054"
  finding_service:   "finding-service:50055"
  ai_service:        "ai-service:50056"
  notification_service: "notification-service:50057"

rate_limit:
  enabled: true
  anonymous_limit: 100      # requests per hour
  default_limit: 1000       # requests per hour per user

cors:
  allowed_origins: ["https://app.osv.dev", "http://localhost:3000"]
  allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
  allow_credentials: true

tracing:
  enabled: true
  endpoint: "${OTEL_EXPORTER_OTLP_ENDPOINT}"
  service_name: "gateway-service"
```
