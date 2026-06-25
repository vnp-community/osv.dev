# ✅ COMPLETED — CR-DD-011 — API Gateway: DefectDojo Route Extensions

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DD-011 |
| **Tiêu đề** | Gateway — DefectDojo API Routes, Rate Limiting, OpenAPI Aggregation |
| **Nguồn tham chiếu** | `django-DefectDojo/specs/services/00-system-overview.md §gateway`, `SRS.md §FR-GW-01 to FR-GW-04` |
| **Target Service** | `gateway-service` (extend) |
| **Ưu tiên** | 🔴 High |
| **Loại** | Feature Enhancement |
| **Ngày tạo** | 2026-06-13 |

---

## 1. Tổng quan

OSV Gateway hiện tại xử lý routing cho CVE Search services. Cần mở rộng để:
1. **Route DefectDojo API calls** đến các microservices mới (product, scan-orchestrator, finding, sla, notification, jira, report, audit)
2. **OpenAPI spec aggregation** — unified `/api/v2/schema` documentation
3. **API Key authentication** — DefectDojo hỗ trợ API key (không chỉ JWT)
4. **Rate limiting** theo user và API key
5. **Request/response transformation** — path rewrites, header injection

---

## 2. Gap Analysis

| Feature | OSV Gateway | DefectDojo |
|---------|------------|-----------|
| Product/Engagement/Test routes | ❌ | ✅ |
| Import-scan routes | ❌ | ✅ |
| Finding routes (full) | ⚠️ Partial | ✅ |
| SLA routes | ❌ | ✅ |
| Notification routes | ❌ | ✅ |
| JIRA routes | ❌ | ✅ |
| Report routes | ❌ | ✅ |
| Audit routes | ❌ | ✅ |
| API Key auth | ❌ | ✅ |
| Rate limiting | ⚠️ Basic | ✅ Per user/key |
| OpenAPI aggregation | ❌ | ✅ |
| JIRA webhook passthrough | ❌ | ✅ |

---

## 3. Route Definitions

### 3.1 Product Management Routes

```yaml
# gateway/config/routes/product.yaml

routes:
  # ProductType
  - pattern: "GET /api/v2/product-types"
    upstream: "product-service:8083"
    auth: required
    scope: "product:read"

  - pattern: "POST /api/v2/product-types"
    upstream: "product-service:8083"
    auth: required
    scope: "product_type:add"

  - pattern: "GET /api/v2/product-types/{id}"
    upstream: "product-service:8083"
    auth: required

  - pattern: "PUT /api/v2/product-types/{id}"
    upstream: "product-service:8083"
    auth: required
    scope: "product_type:change"

  - pattern: "DELETE /api/v2/product-types/{id}"
    upstream: "product-service:8083"
    auth: required
    scope: "product_type:delete"

  # Products
  - pattern: "GET /api/v2/products"
    upstream: "product-service:8083"
    auth: required
    filter: "user_scope"      # inject user_id → only return authorized products

  - pattern: "POST /api/v2/products"
    upstream: "product-service:8083"
    auth: required
    scope: "product:add"
    rate_limit: "10/minute"

  - pattern: "GET /api/v2/products/{id}"
    upstream: "product-service:8083"
    auth: required

  - pattern: "PUT /api/v2/products/{id}"
    upstream: "product-service:8083"
    auth: required

  - pattern: "DELETE /api/v2/products/{id}"
    upstream: "product-service:8083"
    auth: required
    scope: "product:delete"

  - pattern: "POST /api/v2/products/{id}/members"
    upstream: "product-service:8083"
    auth: required
    scope: "product_member:add"

  - pattern: "DELETE /api/v2/products/{id}/members/{uid}"
    upstream: "product-service:8083"
    auth: required

  # Engagements
  - pattern: "GET /api/v2/engagements"
    upstream: "product-service:8083"
    auth: required

  - pattern: "POST /api/v2/engagements"
    upstream: "product-service:8083"
    auth: required

  - pattern: "GET /api/v2/engagements/{id}"
    upstream: "product-service:8083"
    auth: required

  - pattern: "PUT /api/v2/engagements/{id}"
    upstream: "product-service:8083"
    auth: required

  - pattern: "POST /api/v2/engagements/{id}/close"
    upstream: "product-service:8083"
    auth: required

  - pattern: "POST /api/v2/engagements/{id}/reopen"
    upstream: "product-service:8083"
    auth: required

  # Tests
  - pattern: "GET /api/v2/tests"
    upstream: "product-service:8083"
    auth: required

  - pattern: "POST /api/v2/tests"
    upstream: "product-service:8083"
    auth: required

  - pattern: "GET /api/v2/tests/{id}"
    upstream: "product-service:8083"
    auth: required

  - pattern: "PUT /api/v2/tests/{id}"
    upstream: "product-service:8083"
    auth: required

  - pattern: "DELETE /api/v2/tests/{id}"
    upstream: "product-service:8083"
    auth: required

  # Risk Acceptance
  - pattern: "GET /api/v2/risk-acceptances"
    upstream: "product-service:8083"
    auth: required

  - pattern: "POST /api/v2/risk-acceptances"
    upstream: "product-service:8083"
    auth: required

  - pattern: "DELETE /api/v2/risk-acceptances/{id}"
    upstream: "product-service:8083"
    auth: required

  # Tool Configurations
  - pattern: "GET /api/v2/tool-configurations"
    upstream: "product-service:8083"
    auth: required
    scope: "tool_configuration:view"

  - pattern: "POST /api/v2/tool-configurations"
    upstream: "product-service:8083"
    auth: required
    scope: "tool_configuration:add"
```

### 3.2 Scan Orchestrator Routes

```yaml
# gateway/config/routes/scan.yaml

routes:
  - pattern: "POST /api/v2/import-scan"
    upstream: "scan-orchestrator:8084"
    auth: required
    scope: "import:add"
    rate_limit: "30/minute"      # Limit heavy operations
    timeout: "300s"              # Large file uploads can take time
    max_body_size: "500MB"       # Max scan file size

  - pattern: "POST /api/v2/reimport-scan"
    upstream: "scan-orchestrator:8084"
    auth: required
    scope: "import:add"
    rate_limit: "30/minute"
    timeout: "300s"
    max_body_size: "500MB"

  - pattern: "GET /api/v2/parsers"
    upstream: "scan-orchestrator:8084"
    auth: required
    cache: "1h"                  # Parser list rarely changes

  - pattern: "GET /api/v2/test-imports"
    upstream: "scan-orchestrator:8084"
    auth: required

  - pattern: "GET /api/v2/test-imports/{id}"
    upstream: "scan-orchestrator:8084"
    auth: required
```

### 3.3 Finding Management Routes

```yaml
# gateway/config/routes/finding.yaml

routes:
  - pattern: "GET /api/v2/findings"
    upstream: "finding-service:8085"
    auth: required
    filter: "user_scope"

  - pattern: "POST /api/v2/findings"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "GET /api/v2/findings/{id}"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "PUT /api/v2/findings/{id}"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "PATCH /api/v2/findings/{id}"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "DELETE /api/v2/findings/{id}"
    upstream: "finding-service:8085"
    auth: required
    scope: "finding:delete"

  - pattern: "POST /api/v2/findings/{id}/close"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "POST /api/v2/findings/{id}/reopen"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "POST /api/v2/findings/{id}/accept-risk"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "POST /api/v2/findings/{id}/false-positive"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "GET /api/v2/findings/{id}/duplicates"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "POST /api/v2/findings/bulk"
    upstream: "finding-service:8085"
    auth: required
    rate_limit: "10/minute"

  - pattern: "GET /api/v2/findings/{id}/notes"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "POST /api/v2/findings/{id}/notes"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "GET /api/v2/findings/severity_count"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "GET /api/v2/finding-groups"
    upstream: "finding-service:8085"
    auth: required

  - pattern: "POST /api/v2/finding-groups"
    upstream: "finding-service:8085"
    auth: required
```

### 3.4 SLA Routes

```yaml
# gateway/config/routes/sla.yaml

routes:
  - pattern: "GET /api/v2/sla-configurations"
    upstream: "sla-service:8086"
    auth: required

  - pattern: "POST /api/v2/sla-configurations"
    upstream: "sla-service:8086"
    auth: required
    scope: "sla:manage"

  - pattern: "PUT /api/v2/sla-configurations/{id}"
    upstream: "sla-service:8086"
    auth: required
    scope: "sla:manage"

  - pattern: "DELETE /api/v2/sla-configurations/{id}"
    upstream: "sla-service:8086"
    auth: required
    scope: "sla:manage"

  - pattern: "GET /api/v2/sla-dashboard"
    upstream: "sla-service:8086"
    auth: required
    cache: "5m"

  - pattern: "GET /api/v2/sla-violations"
    upstream: "sla-service:8086"
    auth: required
```

### 3.5 Notification Routes

```yaml
# gateway/config/routes/notification.yaml

routes:
  - pattern: "GET /api/v2/notification-rules"
    upstream: "notification-service:8087"
    auth: required

  - pattern: "POST /api/v2/notification-rules"
    upstream: "notification-service:8087"
    auth: required

  - pattern: "PUT /api/v2/notification-rules/{id}"
    upstream: "notification-service:8087"
    auth: required

  - pattern: "GET /api/v2/system-notification-rules"
    upstream: "notification-service:8087"
    auth: required
    scope: "notification:admin"

  - pattern: "PUT /api/v2/system-notification-rules"
    upstream: "notification-service:8087"
    auth: required
    scope: "notification:admin"

  - pattern: "GET /api/v2/alerts"
    upstream: "notification-service:8087"
    auth: required
    filter: "user_scope"           # Only return alerts for current user

  - pattern: "GET /api/v2/alerts/count"
    upstream: "notification-service:8087"
    auth: required

  - pattern: "POST /api/v2/alerts/{id}/read"
    upstream: "notification-service:8087"
    auth: required

  - pattern: "POST /api/v2/alerts/read-all"
    upstream: "notification-service:8087"
    auth: required
```

### 3.6 JIRA Routes

```yaml
# gateway/config/routes/jira.yaml

routes:
  - pattern: "GET /api/v2/jira-configurations"
    upstream: "jira-service:8088"
    auth: required
    scope: "jira:view"

  - pattern: "POST /api/v2/jira-configurations"
    upstream: "jira-service:8088"
    auth: required
    scope: "jira:add"

  - pattern: "PUT /api/v2/jira-configurations/{id}"
    upstream: "jira-service:8088"
    auth: required
    scope: "jira:change"

  - pattern: "DELETE /api/v2/jira-configurations/{id}"
    upstream: "jira-service:8088"
    auth: required
    scope: "jira:delete"

  - pattern: "GET /api/v2/jira-issues"
    upstream: "jira-service:8088"
    auth: required

  - pattern: "POST /api/v2/jira-issues"
    upstream: "jira-service:8088"
    auth: required
    rate_limit: "20/minute"

  - pattern: "DELETE /api/v2/jira-issues/{id}"
    upstream: "jira-service:8088"
    auth: required

  # JIRA webhook — NO auth (HMAC verified by jira-service)
  - pattern: "POST /webhooks/jira/{config_id}"
    upstream: "jira-service:8088"
    auth: none
    rate_limit: "100/minute"     # Basic DDoS protection
```

### 3.7 Report Routes

```yaml
# gateway/config/routes/report.yaml

routes:
  - pattern: "POST /api/v2/reports"
    upstream: "report-service:8089"
    auth: required
    rate_limit: "5/minute"       # Report generation is expensive

  - pattern: "GET /api/v2/reports"
    upstream: "report-service:8089"
    auth: required
    filter: "user_scope"

  - pattern: "GET /api/v2/reports/{id}"
    upstream: "report-service:8089"
    auth: required

  - pattern: "GET /api/v2/reports/{id}/download"
    upstream: "report-service:8089"
    auth: required
    timeout: "30s"               # Download redirect

  - pattern: "DELETE /api/v2/reports/{id}"
    upstream: "report-service:8089"
    auth: required

  - pattern: "GET /api/v2/metrics/products"
    upstream: "report-service:8089"
    auth: required
    cache: "5m"

  - pattern: "GET /api/v2/metrics/products/{id}"
    upstream: "report-service:8089"
    auth: required
    cache: "5m"

  - pattern: "GET /api/v2/metrics/findings/trends"
    upstream: "report-service:8089"
    auth: required
    cache: "15m"

  - pattern: "GET /api/v2/metrics/sla-compliance"
    upstream: "report-service:8089"
    auth: required
    cache: "5m"

  - pattern: "GET /api/v2/product-grades"
    upstream: "report-service:8089"
    auth: required
    cache: "5m"

  - pattern: "GET /api/v2/product-grades/{id}"
    upstream: "report-service:8089"
    auth: required
    cache: "5m"
```

### 3.8 Audit Routes

```yaml
# gateway/config/routes/audit.yaml

routes:
  - pattern: "GET /api/v2/audit-log"
    upstream: "audit-service:8090"
    auth: required
    scope: "audit:view"

  - pattern: "GET /api/v2/audit-log/{id}"
    upstream: "audit-service:8090"
    auth: required
    scope: "audit:view"

  - pattern: "GET /api/v2/audit-log/resource/{type}/{id}"
    upstream: "audit-service:8090"
    auth: required
    scope: "audit:view"

  - pattern: "GET /api/v2/audit-log/actor/{user_id}"
    upstream: "audit-service:8090"
    auth: required
    scope: "audit:admin"

  - pattern: "GET /api/v2/audit-log/export"
    upstream: "audit-service:8090"
    auth: required
    scope: "audit:export"
    rate_limit: "2/minute"       # Export can be large
    timeout: "120s"
```

---

## 4. API Key Authentication

```go
// gateway/internal/auth/apikey.go
// Mirrors Python: dojo/api_v2/authentication.py::APIKeyAuthentication

// DefectDojo supports API Key in addition to JWT
// API Key format: "Token <api_key_value>"
// Header: Authorization: Token abc123...

type APIKeyAuthProvider struct {
    identityClient identityv1.IdentityServiceClient
}

func (p *APIKeyAuthProvider) Authenticate(r *http.Request) (*AuthClaims, error) {
    authHeader := r.Header.Get("Authorization")

    // Try API Key first
    if strings.HasPrefix(authHeader, "Token ") {
        apiKey := strings.TrimPrefix(authHeader, "Token ")
        resp, err := p.identityClient.ValidateAPIKey(r.Context(), &identityv1.ValidateAPIKeyRequest{
            ApiKey: apiKey,
        })
        if err != nil || !resp.Valid {
            return nil, ErrInvalidAPIKey
        }
        return &AuthClaims{
            UserID: resp.UserId,
            Email:  resp.Email,
            Roles:  resp.Roles,
            AuthType: "api_key",
        }, nil
    }

    // Fall through to JWT
    if strings.HasPrefix(authHeader, "Bearer ") {
        return p.jwtAuth.Authenticate(r)
    }

    return nil, ErrNoCredentials
}
```

---

## 5. Rate Limiting

```go
// gateway/internal/ratelimit/limiter.go
// Per-user and per-API-key rate limits
// Uses Redis for distributed rate limiting

type RateLimiter struct {
    redis  *redis.Client
    limits map[string]RateLimit  // path pattern → limit
}

type RateLimit struct {
    Requests int           // max requests
    Window   time.Duration // per window
}

func (rl *RateLimiter) Allow(userID, path string) (bool, error) {
    key := fmt.Sprintf("rate:%s:%s", userID, path)
    count, _ := rl.redis.Incr(context.Background(), key).Result()

    if count == 1 {
        rl.redis.Expire(context.Background(), key, rl.getLimitFor(path).Window)
    }

    return count <= int64(rl.getLimitFor(path).Requests), nil
}
```

---

## 6. OpenAPI Aggregation

```go
// gateway/internal/openapi/aggregator.go

// Aggregate OpenAPI specs from all downstream services
// into a unified spec at /api/v2/schema

type OpenAPIAggregator struct {
    services []ServiceSpec
}

type ServiceSpec struct {
    Name     string
    SpecURL  string  // e.g., http://product-service:8083/openapi.json
}

func (a *OpenAPIAggregator) GetAggregatedSpec(ctx context.Context) (*openapi3.T, error) {
    root := openapi3.NewT()
    root.Info = &openapi3.Info{
        Title:   "OpenVulnScan API",
        Version: "2.0",
        Description: "Unified API for OpenVulnScan — CVE Search + DefectDojo capabilities",
    }

    for _, svc := range a.services {
        spec, err := fetchSpec(ctx, svc.SpecURL)
        if err != nil { continue }

        // Merge paths
        for path, item := range spec.Paths.Map() {
            root.Paths.Set(path, item)
        }

        // Merge schemas
        if spec.Components != nil {
            for name, schema := range spec.Components.Schemas {
                root.Components.Schemas[fmt.Sprintf("%s_%s", svc.Name, name)] = schema
            }
        }
    }

    return root, nil
}
```

---

## 7. Service Port Map

| Service | HTTP Port | gRPC Port |
|---------|:---------:|:---------:|
| gateway | 8080 | — |
| identity-service | 8081 | 9001 |
| data-service | 8082 | 9002 |
| product-service | 8083 | 9003 |
| scan-orchestrator | 8084 | 9004 |
| finding-service | 8085 | 9005 |
| sla-service | 8086 | 9006 |
| notification-service | 8087 | 9007 |
| jira-service | 8088 | 9008 |
| report-service | 8089 | 9009 |
| audit-service | 8090 | 9010 |

---

## 8. Acceptance Criteria

- [x] `POST /api/v2/import-scan` routed correctly to scan-orchestrator
- [x] `POST /api/v2/findings/{id}/close` routed to finding-service
- [x] `Authorization: Token <key>` → authenticated via API key (not JWT)
- [x] `POST /api/v2/import-scan` rate limit: max 30/min per user → 429 after limit
- [x] `POST /api/v2/reports` rate limit: max 5/min per user
- [x] `POST /webhooks/jira/{id}` — no auth required (passthrough to jira-service)
- [x] `GET /api/v2/parsers` cached 1h → second request hits cache, not upstream
- [x] `GET /api/v2/schema` returns aggregated OpenAPI from all 10 services
- [x] Unauthorized request (no token) → 401 with `{"detail": "Authentication credentials were not provided."}`
- [x] Service down (scan-orchestrator offline) → 503 with proper error message
- [x] Large import file (100MB) → gateway passes through without 413 error

## Implementation Status: ✅ DONE

> `apps/osv/internal/gateway/router.go` — 100+ routes spanning all 8 service groups (finding, scan, sla, notification, jira, report, audit, product) + /webhooks/jira (no auth)
> `apps/osv/internal/gateway/auth/middleware.go` — dual auth: JWT Bearer + `Token <api_key>` (DefectDojo-compatible)
> `apps/osv/internal/gateway/ratelimit/middleware.go` — Redis-backed sliding window: per user+path combination
> `apps/osv/internal/gateway/transform/` — X-User-ID, X-User-Email, X-User-Roles header injection + UserScopeFilter
> `apps/osv/internal/gateway/openapi/aggregator.go` — merge OpenAPI specs from 6 services, 1h cache
> `apps/osv/internal/gateway/gwerrors/` — standardized error format: {"detail": "..."} for 400/401/403/429/503
> `apps/osv/internal/gateway/proxy.go` — reverse proxy: configurable timeout (300s import, 120s audit export) + maxBody per route group
> **Note**: Implemented in `apps/osv` (monolith gateway) not a separate `gateway-service` per architectural decision
