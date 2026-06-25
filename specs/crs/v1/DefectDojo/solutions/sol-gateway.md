# ✅ COMPLETED — Solution: Gateway Extension (apps/osv + gateway-service)

> **Covers**: CR-DD-011  
> **Nguyên tắc**: `apps/osv` là **gateway monolith point** — KHÔNG chứa business logic, chỉ routing + auth + rate-limit. `gateway-service` chứa implementation gateway logic (route rules, middleware, proxy).

---

## Kiến trúc Gateway

```
External Traffic
      │
      ▼
┌─────────────────────────────────────────────┐
│              apps/osv (port 8080)            │
│  Entry Point — tất cả external requests     │
│                                             │
│  ┌─────────────┐  ┌──────────────────────┐  │
│  │ Auth Layer  │  │ Rate Limiting Layer  │  │
│  │ JWT + Token │  │ Redis-backed per user│  │
│  └─────────────┘  └──────────────────────┘  │
│                                             │
│  ┌─────────────────────────────────────────┐ │
│  │         Reverse Proxy Router            │ │
│  │  /api/v2/* → downstream services       │ │
│  │  /webhooks/* → passthrough (no auth)   │ │
│  │  /health → local                       │ │
│  └─────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
      │ reverse proxy
      ├── finding-service:8085
      ├── scan-service:8084
      ├── sla-service:8086
      ├── notification-service:8087
      ├── jira-service:8088
      └── audit-service:8090
```

---

## apps/osv Changes (gateway role)

`apps/osv` hiện tại chỉ có gRPC server. Cần mở rộng thành HTTP reverse proxy + gateway.

### Internal packages mới trong apps/osv

```
apps/osv/internal/
├── api/               # ĐÃ CÓ — gRPC server
├── config/            # ĐÃ CÓ
├── orchestrator/      # ĐÃ CÓ
└── gateway/           # 🆕 MỚI — HTTP gateway layer
    ├── server.go      # HTTP server setup
    ├── router.go      # Route registration
    ├── proxy.go       # Reverse proxy logic
    ├── auth/
    │   ├── middleware.go      # Auth middleware (JWT + API Key)
    │   ├── jwt.go             # JWT validation
    │   └── apikey.go          # Token <key> validation via identity-service
    ├── ratelimit/
    │   └── middleware.go      # Redis-backed rate limiting
    ├── transform/
    │   └── headers.go         # Inject X-User-ID, X-User-Email, X-User-Roles
    ├── openapi/
    │   └── aggregator.go      # Merge OpenAPI specs from all services
    └── routes/
        ├── finding.go         # /api/v2/findings/*, /api/v2/products/*, etc.
        ├── scan.go            # /api/v2/import-scan, /api/v2/parsers
        ├── sla.go             # /api/v2/sla-configurations/*
        ├── notification.go    # /api/v2/alerts/*, /api/v2/notification-rules/*
        ├── jira.go            # /api/v2/jira-*/*, /webhooks/jira/*
        ├── audit.go           # /api/v2/audit-log/*
        └── health.go          # /health, /readyz
```

### Route Registration (router.go)

```go
// apps/osv/internal/gateway/router.go
func SetupRouter(cfg *Config, proxy *ReverseProxy, auth *AuthMiddleware, rl *RateLimitMiddleware) http.Handler {
    mux := chi.NewRouter()

    // Global middleware
    mux.Use(middleware.RequestID)
    mux.Use(middleware.RealIP)
    mux.Use(middleware.Logger)
    mux.Use(middleware.Recoverer)

    // Health (no auth)
    mux.Get("/health", handleHealth)
    mux.Get("/readyz", handleReadyz)

    // OpenAPI aggregation (no auth)
    mux.Get("/api/v2/schema", aggregator.HandleSchema)

    // JIRA webhook — no auth (HMAC verified by jira-service)
    mux.With(rl.Limit("100/minute")).
        Post("/webhooks/jira/{config_id}", proxy.Forward("jira-service:8088"))

    // Protected routes — require auth
    mux.Group(func(r chi.Router) {
        r.Use(auth.Authenticate)
        r.Use(transform.InjectUserHeaders)

        // === finding-service routes ===
        // Products
        r.Get("/api/v2/product-types", proxy.Forward("finding-service:8085"))
        r.With(rl.Limit("10/minute")).Post("/api/v2/product-types", proxy.Forward("finding-service:8085"))
        r.Route("/api/v2/product-types/{id}", func(r chi.Router) {
            r.Get("/", proxy.Forward("finding-service:8085"))
            r.Put("/", proxy.Forward("finding-service:8085"))
            r.Delete("/", proxy.Forward("finding-service:8085"))
        })

        r.With(transform.UserScopeFilter).Get("/api/v2/products", proxy.Forward("finding-service:8085"))
        r.With(rl.Limit("10/minute")).Post("/api/v2/products", proxy.Forward("finding-service:8085"))
        r.Route("/api/v2/products/{id}", func(r chi.Router) {
            r.Get("/", proxy.Forward("finding-service:8085"))
            r.Put("/", proxy.Forward("finding-service:8085"))
            r.Delete("/", proxy.Forward("finding-service:8085"))
            r.Post("/members", proxy.Forward("finding-service:8085"))
            r.Delete("/members/{uid}", proxy.Forward("finding-service:8085"))
        })

        // Engagements
        r.Get("/api/v2/engagements", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/engagements", proxy.Forward("finding-service:8085"))
        r.Route("/api/v2/engagements/{id}", func(r chi.Router) {
            r.Get("/", proxy.Forward("finding-service:8085"))
            r.Put("/", proxy.Forward("finding-service:8085"))
            r.Post("/close", proxy.Forward("finding-service:8085"))
            r.Post("/reopen", proxy.Forward("finding-service:8085"))
        })

        // Tests
        r.Get("/api/v2/tests", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/tests", proxy.Forward("finding-service:8085"))
        r.Route("/api/v2/tests/{id}", func(r chi.Router) {
            r.Get("/", proxy.Forward("finding-service:8085"))
            r.Put("/", proxy.Forward("finding-service:8085"))
            r.Delete("/", proxy.Forward("finding-service:8085"))
        })

        // Risk Acceptances
        r.Get("/api/v2/risk-acceptances", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/risk-acceptances", proxy.Forward("finding-service:8085"))
        r.Route("/api/v2/risk-acceptances/{id}", func(r chi.Router) {
            r.Get("/", proxy.Forward("finding-service:8085"))
            r.Put("/", proxy.Forward("finding-service:8085"))
            r.Delete("/", proxy.Forward("finding-service:8085"))
            r.Post("/findings/{fid}/remove", proxy.Forward("finding-service:8085"))
        })

        // Tool Configurations
        r.Get("/api/v2/tool-configurations", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/tool-configurations", proxy.Forward("finding-service:8085"))
        r.Route("/api/v2/tool-configurations/{id}", func(r chi.Router) {
            r.Get("/", proxy.Forward("finding-service:8085"))
            r.Put("/", proxy.Forward("finding-service:8085"))
            r.Delete("/", proxy.Forward("finding-service:8085"))
        })

        // Findings
        r.With(transform.UserScopeFilter).Get("/api/v2/findings", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/findings", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/findings/severity_count", proxy.Forward("finding-service:8085"))
        r.With(rl.Limit("10/minute")).Post("/api/v2/findings/bulk", proxy.Forward("finding-service:8085"))
        r.Route("/api/v2/findings/{id}", func(r chi.Router) {
            r.Get("/", proxy.Forward("finding-service:8085"))
            r.Put("/", proxy.Forward("finding-service:8085"))
            r.Patch("/", proxy.Forward("finding-service:8085"))
            r.Delete("/", proxy.Forward("finding-service:8085"))
            r.Post("/close", proxy.Forward("finding-service:8085"))
            r.Post("/reopen", proxy.Forward("finding-service:8085"))
            r.Post("/accept-risk", proxy.Forward("finding-service:8085"))
            r.Post("/false-positive", proxy.Forward("finding-service:8085"))
            r.Get("/duplicates", proxy.Forward("finding-service:8085"))
            r.Get("/notes", proxy.Forward("finding-service:8085"))
            r.Post("/notes", proxy.Forward("finding-service:8085"))
        })

        // Finding Groups
        r.Get("/api/v2/finding-groups", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/finding-groups", proxy.Forward("finding-service:8085"))

        // Reports & Metrics
        r.With(rl.Limit("5/minute")).Post("/api/v2/reports", proxy.Forward("finding-service:8085"))
        r.With(transform.UserScopeFilter).Get("/api/v2/reports", proxy.Forward("finding-service:8085"))
        r.Route("/api/v2/reports/{id}", func(r chi.Router) {
            r.Get("/", proxy.Forward("finding-service:8085"))
            r.With(timeout("30s")).Get("/download", proxy.Forward("finding-service:8085"))
            r.Delete("/", proxy.Forward("finding-service:8085"))
        })
        r.With(cache("5m")).Get("/api/v2/metrics/products", proxy.Forward("finding-service:8085"))
        r.With(cache("5m")).Get("/api/v2/metrics/products/{id}", proxy.Forward("finding-service:8085"))
        r.With(cache("15m")).Get("/api/v2/metrics/findings/trends", proxy.Forward("finding-service:8085"))
        r.With(cache("5m")).Get("/api/v2/metrics/sla-compliance", proxy.Forward("finding-service:8085"))
        r.With(cache("5m")).Get("/api/v2/product-grades", proxy.Forward("finding-service:8085"))
        r.With(cache("5m")).Get("/api/v2/product-grades/{id}", proxy.Forward("finding-service:8085"))

        // === scan-service routes ===
        r.With(rl.Limit("30/minute"), maxBody("500MB"), timeout("300s")).
            Post("/api/v2/import-scan", proxy.Forward("scan-service:8084"))
        r.With(rl.Limit("30/minute"), maxBody("500MB"), timeout("300s")).
            Post("/api/v2/reimport-scan", proxy.Forward("scan-service:8084"))
        r.With(cache("1h")).Get("/api/v2/parsers", proxy.Forward("scan-service:8084"))
        r.Get("/api/v2/test-imports", proxy.Forward("scan-service:8084"))
        r.Get("/api/v2/test-imports/{id}", proxy.Forward("scan-service:8084"))

        // === sla-service routes ===
        r.Get("/api/v2/sla-configurations", proxy.Forward("sla-service:8086"))
        r.Post("/api/v2/sla-configurations", proxy.Forward("sla-service:8086"))
        r.Route("/api/v2/sla-configurations/{id}", func(r chi.Router) {
            r.Get("/", proxy.Forward("sla-service:8086"))
            r.Put("/", proxy.Forward("sla-service:8086"))
            r.Delete("/", proxy.Forward("sla-service:8086"))
            r.Post("/assign/{product_id}", proxy.Forward("sla-service:8086"))
        })
        r.With(cache("5m")).Get("/api/v2/sla-dashboard", proxy.Forward("sla-service:8086"))
        r.Get("/api/v2/sla-violations", proxy.Forward("sla-service:8086"))
        r.Get("/api/v2/sla-violations/{product_id}", proxy.Forward("sla-service:8086"))

        // === notification-service routes ===
        r.Get("/api/v2/notification-rules", proxy.Forward("notification-service:8087"))
        r.Post("/api/v2/notification-rules", proxy.Forward("notification-service:8087"))
        r.Route("/api/v2/notification-rules/{id}", func(r chi.Router) {
            r.Put("/", proxy.Forward("notification-service:8087"))
            r.Delete("/", proxy.Forward("notification-service:8087"))
        })
        r.Get("/api/v2/system-notification-rules", proxy.Forward("notification-service:8087"))
        r.Put("/api/v2/system-notification-rules", proxy.Forward("notification-service:8087"))
        r.With(transform.UserScopeFilter).Get("/api/v2/alerts", proxy.Forward("notification-service:8087"))
        r.Get("/api/v2/alerts/count", proxy.Forward("notification-service:8087"))
        r.Post("/api/v2/alerts/{id}/read", proxy.Forward("notification-service:8087"))
        r.Post("/api/v2/alerts/read-all", proxy.Forward("notification-service:8087"))
        r.Get("/api/v2/notification-deliveries", proxy.Forward("notification-service:8087"))

        // === jira-service routes ===
        r.Get("/api/v2/jira-configurations", proxy.Forward("jira-service:8088"))
        r.Post("/api/v2/jira-configurations", proxy.Forward("jira-service:8088"))
        r.Route("/api/v2/jira-configurations/{id}", func(r chi.Router) {
            r.Get("/", proxy.Forward("jira-service:8088"))
            r.Put("/", proxy.Forward("jira-service:8088"))
            r.Delete("/", proxy.Forward("jira-service:8088"))
        })
        r.Get("/api/v2/jira-issues", proxy.Forward("jira-service:8088"))
        r.With(rl.Limit("20/minute")).Post("/api/v2/jira-issues", proxy.Forward("jira-service:8088"))
        r.Get("/api/v2/jira-issues/{finding_id}", proxy.Forward("jira-service:8088"))
        r.Delete("/api/v2/jira-issues/{id}", proxy.Forward("jira-service:8088"))

        // === audit-service routes ===
        r.Get("/api/v2/audit-log", proxy.Forward("audit-service:8090"))
        r.Get("/api/v2/audit-log/{id}", proxy.Forward("audit-service:8090"))
        r.Get("/api/v2/audit-log/resource/{type}/{id}", proxy.Forward("audit-service:8090"))
        r.Get("/api/v2/audit-log/actor/{user_id}", proxy.Forward("audit-service:8090"))
        r.With(rl.Limit("2/minute"), timeout("120s")).
            Get("/api/v2/audit-log/export", proxy.Forward("audit-service:8090"))
    })

    return mux
}
```

---

## API Key Authentication

```go
// apps/osv/internal/gateway/auth/apikey.go
// DefectDojo-compatible: Authorization: Token <api_key>

type APIKeyAuthProvider struct {
    identityClient identityv1.IdentityServiceClient
}

func (p *APIKeyAuthProvider) Authenticate(r *http.Request) (*AuthClaims, error) {
    authHeader := r.Header.Get("Authorization")

    switch {
    case strings.HasPrefix(authHeader, "Token "):
        // API Key auth (DefectDojo style)
        apiKey := strings.TrimPrefix(authHeader, "Token ")
        resp, err := p.identityClient.ValidateAPIKey(r.Context(), &identityv1.ValidateAPIKeyRequest{
            ApiKey: apiKey,
        })
        if err != nil || !resp.Valid {
            return nil, ErrInvalidAPIKey
        }
        return &AuthClaims{
            UserID:   resp.UserId,
            Email:    resp.Email,
            Roles:    resp.Roles,
            AuthType: "api_key",
        }, nil

    case strings.HasPrefix(authHeader, "Bearer "):
        // JWT auth
        return p.jwtAuth.Authenticate(r)

    default:
        return nil, ErrNoCredentials
    }
}
```

---

## Rate Limiting

```go
// apps/osv/internal/gateway/ratelimit/middleware.go
// Redis-backed sliding window rate limiter

type RateLimiter struct {
    redis  *redis.Client
}

func (rl *RateLimiter) Limit(spec string) func(http.Handler) http.Handler {
    // Parse "30/minute", "5/minute", etc.
    limit, window := parseSpec(spec)

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userID := getUserID(r) // từ auth middleware
            path := r.URL.Path

            key := fmt.Sprintf("rate:%s:%s", userID, path)
            count, _ := rl.redis.Incr(r.Context(), key).Result()
            if count == 1 {
                rl.redis.Expire(r.Context(), key, window)
            }

            // Set rate limit headers
            w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
            w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(max(0, limit-int(count))))

            if int(count) > limit {
                http.Error(w, `{"detail":"Request was throttled."}`, http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## Request Transform (Header Injection)

```go
// apps/osv/internal/gateway/transform/headers.go
// Inject user context headers vào requests xuống downstream services
// Downstream services không cần re-validate JWT

func InjectUserHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := getAuthClaims(r.Context())
        if claims != nil {
            r.Header.Set("X-User-ID", claims.UserID)
            r.Header.Set("X-User-Email", claims.Email)
            r.Header.Set("X-User-Roles", strings.Join(claims.Roles, ","))
            r.Header.Set("X-Auth-Type", claims.AuthType)
        }
        next.ServeHTTP(w, r)
    })
}

// UserScopeFilter: inject user_id query param cho list endpoints
// Để downstream chỉ trả về resources user có quyền truy cập
func UserScopeFilter(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := getAuthClaims(r.Context())
        if claims != nil && !claims.IsAdmin {
            q := r.URL.Query()
            q.Set("_user_id", claims.UserID)
            r.URL.RawQuery = q.Encode()
        }
        next.ServeHTTP(w, r)
    })
}
```

---

## OpenAPI Aggregation

```go
// apps/osv/internal/gateway/openapi/aggregator.go
// Merge OpenAPI specs từ tất cả downstream services

type OpenAPIAggregator struct {
    services []ServiceEndpoint
    cache    *sync.Map  // cached merged spec (refresh every 1h)
}

type ServiceEndpoint struct {
    Name    string
    SpecURL string
}

var serviceEndpoints = []ServiceEndpoint{
    {"finding", "http://finding-service:8085/openapi.json"},
    {"scan", "http://scan-service:8084/openapi.json"},
    {"sla", "http://sla-service:8086/openapi.json"},
    {"notification", "http://notification-service:8087/openapi.json"},
    {"jira", "http://jira-service:8088/openapi.json"},
    {"audit", "http://audit-service:8090/openapi.json"},
}

// GET /api/v2/schema → aggregated OpenAPI 3.0 JSON
```

---

## Error Response Standardization

```go
// apps/osv/internal/gateway/proxy.go
// Standardize error responses (DefectDojo format)
// 404: {"detail": "Not found."}
// 401: {"detail": "Authentication credentials were not provided."}
// 403: {"detail": "You do not have permission to perform this action."}
// 429: {"detail": "Request was throttled. Expected available in Xs."}
// 503: {"detail": "Service temporarily unavailable."}
```

---

## Acceptance Criteria

- [x] `POST /api/v2/import-scan` → routed correctly to scan-service:8084
- [x] `POST /api/v2/findings/{id}/close` → routed to finding-service:8085
- [x] `Authorization: Token <key>` → authenticated via API key (not JWT)
- [x] `POST /api/v2/import-scan` rate limit: 30/min → 429 after limit
- [x] `POST /api/v2/reports` rate limit: 5/min → 429
- [x] `POST /webhooks/jira/{id}` → passthrough WITHOUT auth check
- [x] `GET /api/v2/parsers` → cached 1h (second request hits cache)
- [x] `GET /api/v2/schema` → aggregated OpenAPI từ tất cả services
- [x] Unauthorized (no token) → 401 `{"detail": "Authentication credentials were not provided."}`
- [x] Service down → 503 với proper error message
- [x] Large import file 100MB → no 413 error (gateway passes through)
- [x] X-User-ID header injected vào tất cả downstream requests

## Implementation Status: ✅ DONE

> `apps/osv/internal/gateway/router.go` — 100+ routes: finding-service (products, engagements, tests, findings, reports, metrics), scan-service (import-scan, parsers, test-imports), sla-service, notification-service, jira-service, audit-service + /webhooks/jira (no auth)
> `apps/osv/internal/gateway/auth/middleware.go` — dual auth: JWT Bearer + `Token <api_key>` (DefectDojo-compatible)
> `apps/osv/internal/gateway/ratelimit/middleware.go` — Redis-backed sliding window per user/path
> `apps/osv/internal/gateway/transform/` — X-User-ID, X-User-Email, X-User-Roles injection + UserScopeFilter
> `apps/osv/internal/gateway/openapi/aggregator.go` — merge specs from 6 services, 1h cache
> `apps/osv/internal/gateway/gwerrors/` — standardized error format: {"detail": "..."} for 400/401/403/429/503
> `apps/osv/internal/gateway/proxy.go` — reverse proxy with configurable timeout/maxBody per route group
