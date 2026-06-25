# ✅ COMPLETED — TASK-DD-013 — Gateway Route Rules (100+ routes)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-013 |
| **Service** | `apps/osv` |
| **CR** | CR-DD-011 |
| **Phase** | 1 — Foundation |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-012 |
| **Estimated effort** | 1 ngày |

## Context

Implement `ReverseProxy` và `SetupRouter` trong `apps/osv`. Đây là nơi đăng ký toàn bộ 100+ route rules, forwarding đến đúng upstream service với timeout và maxBody phù hợp. JIRA webhook path `/webhooks/jira/*` không qua auth middleware.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/
```

## Files to Create

```
internal/gateway/
├── proxy.go            # ReverseProxy implementation
├── router.go           # SetupRouter — tất cả route registrations
└── middleware/
    └── timeout.go      # Per-route timeout middleware
```

## Implementation Spec

### `internal/gateway/proxy.go`

```go
package gateway

import (
    "encoding/json"
    "net/http"
    "net/http/httputil"
    "net/url"
    "time"
)

type ReverseProxy struct {
    proxies map[string]*httputil.ReverseProxy // upstream → proxy
    client  *http.Client
}

func NewReverseProxy() *ReverseProxy {
    return &ReverseProxy{
        proxies: make(map[string]*httputil.ReverseProxy),
        client:  &http.Client{Timeout: 60 * time.Second},
    }
}

// Forward returns an http.HandlerFunc that proxies to the given upstream
func (rp *ReverseProxy) Forward(upstream string) http.HandlerFunc {
    target, _ := url.Parse("http://" + upstream)
    proxy := httputil.NewSingleHostReverseProxy(target)

    // Error handler for when upstream is unavailable
    proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{
            "detail": "Service temporarily unavailable.",
        })
    }

    // Director: preserve original path, add upstream target
    originalDirector := proxy.Director
    proxy.Director = func(req *http.Request) {
        originalDirector(req)
        req.Host = target.Host
        // Remove gateway-internal headers
        req.Header.Del("X-Forwarded-Proto")
    }

    rp.proxies[upstream] = proxy
    return proxy.ServeHTTP
}

// ForwardWithTimeout wraps Forward with a specific timeout
func (rp *ReverseProxy) ForwardWithTimeout(upstream string, timeout time.Duration) http.HandlerFunc {
    forward := rp.Forward(upstream)
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), timeout)
        defer cancel()
        forward(w, r.WithContext(ctx))
    }
}

// ForwardWithMaxBody wraps Forward with a max request body size check
func (rp *ReverseProxy) ForwardWithMaxBody(upstream string, maxBytes int64) http.HandlerFunc {
    forward := rp.Forward(upstream)
    return func(w http.ResponseWriter, r *http.Request) {
        r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
        forward(w, r)
    }
}
```

### `internal/gateway/router.go`

```go
package gateway

import (
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/osv/apps/osv/internal/gateway/auth"
    "github.com/osv/apps/osv/internal/gateway/ratelimit"
    "github.com/osv/apps/osv/internal/gateway/transform"
)

const (
    MB500 = 500 * 1024 * 1024  // 500MB max import file size
)

func SetupRouter(
    proxy     *ReverseProxy,
    authMW    auth.AuthMiddleware,
    rl        *ratelimit.RateLimiter,
) http.Handler {
    r := chi.NewRouter()

    // Global middleware
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.Compress(5))

    // ─── Public routes (no auth) ───────────────────────────────────────
    r.Get("/health", handleHealth)
    r.Get("/readyz", handleReadyz)
    r.Get("/api/v2/schema", handleOpenAPISchema)

    // JIRA webhook — no auth (HMAC verified by jira-service)
    r.With(rl.Limit("100/minute")).
        Post("/webhooks/jira/{config_id}", proxy.Forward("jira-service:8088"))

    // ─── Protected routes (require auth) ──────────────────────────────
    r.Group(func(r chi.Router) {
        r.Use(authMW.Authenticate)
        r.Use(transform.InjectUserHeaders)

        // ════════════════════════════════════
        // finding-service:8085
        // ════════════════════════════════════

        // Product Types
        r.Get("/api/v2/product-types", proxy.Forward("finding-service:8085"))
        r.With(rl.Limit("10/minute")).Post("/api/v2/product-types", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/product-types/{id}", proxy.Forward("finding-service:8085"))
        r.Put("/api/v2/product-types/{id}", proxy.Forward("finding-service:8085"))
        r.Delete("/api/v2/product-types/{id}", proxy.Forward("finding-service:8085"))

        // Products (with user scope filter for list)
        r.With(transform.UserScopeFilter).Get("/api/v2/products", proxy.Forward("finding-service:8085"))
        r.With(rl.Limit("10/minute")).Post("/api/v2/products", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/products/{id}", proxy.Forward("finding-service:8085"))
        r.Put("/api/v2/products/{id}", proxy.Forward("finding-service:8085"))
        r.Delete("/api/v2/products/{id}", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/products/{id}/members", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/products/{id}/members", proxy.Forward("finding-service:8085"))
        r.Delete("/api/v2/products/{id}/members/{uid}", proxy.Forward("finding-service:8085"))

        // Engagements
        r.Get("/api/v2/engagements", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/engagements", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/engagements/{id}", proxy.Forward("finding-service:8085"))
        r.Put("/api/v2/engagements/{id}", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/engagements/{id}/close", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/engagements/{id}/reopen", proxy.Forward("finding-service:8085"))

        // Tests
        r.Get("/api/v2/tests", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/tests", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/tests/{id}", proxy.Forward("finding-service:8085"))
        r.Put("/api/v2/tests/{id}", proxy.Forward("finding-service:8085"))
        r.Delete("/api/v2/tests/{id}", proxy.Forward("finding-service:8085"))

        // Risk Acceptances
        r.Get("/api/v2/risk-acceptances", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/risk-acceptances", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/risk-acceptances/{id}", proxy.Forward("finding-service:8085"))
        r.Put("/api/v2/risk-acceptances/{id}", proxy.Forward("finding-service:8085"))
        r.Delete("/api/v2/risk-acceptances/{id}", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/risk-acceptances/{id}/findings/{fid}/remove", proxy.Forward("finding-service:8085"))

        // Tool Configurations
        r.Get("/api/v2/tool-configurations", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/tool-configurations", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/tool-configurations/{id}", proxy.Forward("finding-service:8085"))
        r.Put("/api/v2/tool-configurations/{id}", proxy.Forward("finding-service:8085"))
        r.Delete("/api/v2/tool-configurations/{id}", proxy.Forward("finding-service:8085"))

        // Findings
        r.With(transform.UserScopeFilter).Get("/api/v2/findings", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/findings", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/findings/severity_count", proxy.Forward("finding-service:8085"))
        r.With(rl.Limit("10/minute")).Post("/api/v2/findings/bulk", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/findings/{id}", proxy.Forward("finding-service:8085"))
        r.Put("/api/v2/findings/{id}", proxy.Forward("finding-service:8085"))
        r.Patch("/api/v2/findings/{id}", proxy.Forward("finding-service:8085"))
        r.Delete("/api/v2/findings/{id}", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/findings/{id}/close", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/findings/{id}/reopen", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/findings/{id}/accept-risk", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/findings/{id}/false-positive", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/findings/{id}/out-of-scope", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/findings/{id}/duplicates", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/findings/{id}/notes", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/findings/{id}/notes", proxy.Forward("finding-service:8085"))

        // Finding Groups
        r.Get("/api/v2/finding-groups", proxy.Forward("finding-service:8085"))
        r.Post("/api/v2/finding-groups", proxy.Forward("finding-service:8085"))

        // Reports
        r.With(rl.Limit("5/minute")).Post("/api/v2/reports", proxy.Forward("finding-service:8085"))
        r.With(transform.UserScopeFilter).Get("/api/v2/reports", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/reports/{id}", proxy.Forward("finding-service:8085"))
        r.With(rl.Limit("5/minute")).Get("/api/v2/reports/{id}/download",
            proxy.ForwardWithTimeout("finding-service:8085", 30*time.Second))
        r.Delete("/api/v2/reports/{id}", proxy.Forward("finding-service:8085"))

        // Metrics & Grades (with cache headers)
        r.Get("/api/v2/metrics/products", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/metrics/products/{id}", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/metrics/findings/trends", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/metrics/sla-compliance", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/product-grades", proxy.Forward("finding-service:8085"))
        r.Get("/api/v2/product-grades/{id}", proxy.Forward("finding-service:8085"))

        // ════════════════════════════════════
        // scan-service:8084
        // ════════════════════════════════════
        r.With(rl.Limit("30/minute")).
            Post("/api/v2/import-scan",
                proxy.ForwardWithMaxBody("scan-service:8084", MB500))
        r.With(rl.Limit("30/minute")).
            Post("/api/v2/reimport-scan",
                proxy.ForwardWithMaxBody("scan-service:8084", MB500))
        r.Get("/api/v2/parsers", proxy.Forward("scan-service:8084"))
        r.Get("/api/v2/test-imports", proxy.Forward("scan-service:8084"))
        r.Get("/api/v2/test-imports/{id}", proxy.Forward("scan-service:8084"))

        // ════════════════════════════════════
        // sla-service:8086
        // ════════════════════════════════════
        r.Get("/api/v2/sla-configurations", proxy.Forward("sla-service:8086"))
        r.Post("/api/v2/sla-configurations", proxy.Forward("sla-service:8086"))
        r.Get("/api/v2/sla-configurations/{id}", proxy.Forward("sla-service:8086"))
        r.Put("/api/v2/sla-configurations/{id}", proxy.Forward("sla-service:8086"))
        r.Delete("/api/v2/sla-configurations/{id}", proxy.Forward("sla-service:8086"))
        r.Post("/api/v2/sla-configurations/{id}/assign/{product_id}", proxy.Forward("sla-service:8086"))
        r.Get("/api/v2/sla-dashboard", proxy.Forward("sla-service:8086"))
        r.Get("/api/v2/sla-violations", proxy.Forward("sla-service:8086"))
        r.Get("/api/v2/sla-violations/{product_id}", proxy.Forward("sla-service:8086"))

        // ════════════════════════════════════
        // notification-service:8087
        // ════════════════════════════════════
        r.Get("/api/v2/notification-rules", proxy.Forward("notification-service:8087"))
        r.Post("/api/v2/notification-rules", proxy.Forward("notification-service:8087"))
        r.Put("/api/v2/notification-rules/{id}", proxy.Forward("notification-service:8087"))
        r.Delete("/api/v2/notification-rules/{id}", proxy.Forward("notification-service:8087"))
        r.Get("/api/v2/system-notification-rules", proxy.Forward("notification-service:8087"))
        r.Put("/api/v2/system-notification-rules", proxy.Forward("notification-service:8087"))
        r.With(transform.UserScopeFilter).Get("/api/v2/alerts", proxy.Forward("notification-service:8087"))
        r.Get("/api/v2/alerts/count", proxy.Forward("notification-service:8087"))
        r.Post("/api/v2/alerts/{id}/read", proxy.Forward("notification-service:8087"))
        r.Post("/api/v2/alerts/read-all", proxy.Forward("notification-service:8087"))

        // ════════════════════════════════════
        // jira-service:8088
        // ════════════════════════════════════
        r.Get("/api/v2/jira-configurations", proxy.Forward("jira-service:8088"))
        r.Post("/api/v2/jira-configurations", proxy.Forward("jira-service:8088"))
        r.Get("/api/v2/jira-configurations/{id}", proxy.Forward("jira-service:8088"))
        r.Put("/api/v2/jira-configurations/{id}", proxy.Forward("jira-service:8088"))
        r.Delete("/api/v2/jira-configurations/{id}", proxy.Forward("jira-service:8088"))
        r.Get("/api/v2/jira-issues", proxy.Forward("jira-service:8088"))
        r.With(rl.Limit("20/minute")).Post("/api/v2/jira-issues", proxy.Forward("jira-service:8088"))
        r.Get("/api/v2/jira-issues/{finding_id}", proxy.Forward("jira-service:8088"))
        r.Delete("/api/v2/jira-issues/{id}", proxy.Forward("jira-service:8088"))

        // ════════════════════════════════════
        // audit-service:8090
        // ════════════════════════════════════
        r.Get("/api/v2/audit-log", proxy.Forward("audit-service:8090"))
        r.Get("/api/v2/audit-log/{id}", proxy.Forward("audit-service:8090"))
        r.Get("/api/v2/audit-log/resource/{type}/{id}", proxy.Forward("audit-service:8090"))
        r.Get("/api/v2/audit-log/actor/{user_id}", proxy.Forward("audit-service:8090"))
        r.With(rl.Limit("2/minute")).
            Get("/api/v2/audit-log/export",
                proxy.ForwardWithTimeout("audit-service:8090", 120*time.Second))
    })

    return r
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"status":"ok"}`))
}

func handleReadyz(w http.ResponseWriter, r *http.Request) {
    // Check all downstream services are reachable (or return cached health)
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"status":"ready"}`))
}
```

## Acceptance Criteria

- [x] `GET /health` → 200 `{"status":"ok"}` without auth
- [x] `POST /api/v2/import-scan` without auth → 401
- [x] `POST /api/v2/import-scan` with valid auth → proxied to scan-service:8084
- [x] `POST /api/v2/findings/{id}/close` → proxied to finding-service:8085
- [x] `POST /webhooks/jira/{id}` → proxied WITHOUT auth check
- [x] `GET /api/v2/products` injects `_user_id` query param
- [x] `GET /api/v2/alerts` injects `_user_id` query param
- [x] scan-service down → 503 `{"detail": "Service temporarily unavailable."}`
- [x] `POST /api/v2/import-scan` accepts 100MB file (no 413 error)
- [x] `GET /api/v2/reports/{id}/download` timeout 30s
- [x] `GET /api/v2/audit-log/export` timeout 120s
- [x] All 100+ routes registered correctly (integration test with mock upstreams)

## Implementation Status: ✅ DONE

> `apps/osv/internal/gateway/proxy.go` — ReverseProxy, ForwardWithTimeout, ForwardWithMaxBody
> `apps/osv/internal/gateway/router.go` — 100+ routes across finding-service, scan-service, sla-service, notification-service, jira-service, audit-service
> Public routes: /health, /readyz, /api/v2/schema, /webhooks/jira/* (no auth)
> Protected group: all other routes (auth + UserHeaders middleware)
