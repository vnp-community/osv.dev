# TASK-BE-022 — gateway: Register All New Routes (Master Route Table)

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-022 |
| **Service** | `apps/osv` (gateway) |
| **Solution Ref** | [SOL-UI-001–004 (all)](../solutions/README.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-BE-001 through TASK-BE-021 (all services phải deploy trước) |
| **Estimated** | 2h |

---

## Context

Tất cả services đã có handlers sau các tasks trước. Task cuối này đăng ký **toàn bộ routes mới** vào gateway router trong `apps/osv/internal/gateway/router.go`.

Gateway pattern:
- Inject `X-User-ID`, `X-User-Role`, `X-User-Email` từ JWT vào request trước khi forward
- `am.Authenticate()` — verify JWT, inject headers
- `am.RequireRole("admin")` — check X-User-Role
- `p.Forward("service:port")` — reverse proxy
- `p.ForwardSSE("service:port")` — SSE-aware proxy (no timeout, no buffering)
- `p.ForwardWithMaxBody("service:port", maxBytes)` — for file upload

---

## Target Files

| Action | File Path |
|--------|-----------|
| MODIFY | `apps/osv/internal/gateway/router.go` |

---

## Implementation

```go
// apps/osv/internal/gateway/router.go
// ADD all new routes to the existing SetupRoutes() or equivalent function

func SetupRoutes(mux *http.ServeMux, deps *Dependencies) {
    am  := deps.AuthMiddleware
    p   := deps.Proxy
    bff := deps.BFF

    // ═══════════════════════════════════════════════════════
    // SPRINT 1 — Auth (identity-service:8081)
    // ═══════════════════════════════════════════════════════

    // Public auth routes — NO JWT required
    mux.Handle("POST /api/v1/auth/login",     p.Forward("identity-service:8081"))
    mux.Handle("POST /api/v1/auth/refresh",   p.ForwardWithTimeout("identity-service:8081", 5*time.Second))

    // Protected auth routes
    mux.Handle("GET  /api/v1/auth/me",        am.Authenticate(p.Forward("identity-service:8081")))
    mux.Handle("POST /api/v1/auth/logout",    am.Authenticate(p.Forward("identity-service:8081")))

    // Profile routes
    mux.Handle("GET  /api/v1/profile",        am.Authenticate(p.Forward("identity-service:8081")))
    mux.Handle("PATCH /api/v1/profile",       am.Authenticate(p.Forward("identity-service:8081")))
    mux.Handle("POST  /api/v1/profile/change-password", am.Authenticate(p.Forward("identity-service:8081")))

    // API Keys routes
    mux.Handle("GET    /api/v1/api-keys",     am.Authenticate(p.Forward("identity-service:8081")))
    mux.Handle("POST   /api/v1/api-keys",     am.Authenticate(p.Forward("identity-service:8081")))
    mux.Handle("DELETE /api/v1/api-keys/",    am.Authenticate(p.Forward("identity-service:8081")))

    // Admin routes — role:admin required
    adminOnly := func(h http.Handler) http.Handler {
        return am.Authenticate(am.RequireRole("admin")(h))
    }
    mux.Handle("GET  /api/v1/admin/users",              adminOnly(p.Forward("identity-service:8081")))
    mux.Handle("POST /api/v1/admin/users/invite",       adminOnly(p.Forward("identity-service:8081")))
    mux.Handle("PATCH /api/v1/admin/users/",            adminOnly(p.Forward("identity-service:8081")))
    mux.Handle("POST  /api/v1/admin/users/",            adminOnly(p.Forward("identity-service:8081"))) // /{id}/unlock, /{id}/reset-password
    mux.Handle("GET   /api/v1/admin/roles",             adminOnly(p.Forward("identity-service:8081")))

    // ═══════════════════════════════════════════════════════
    // SPRINT 1 — Dashboard BFF (in-gateway)
    // ═══════════════════════════════════════════════════════

    mux.Handle("GET /api/v1/dashboard",     am.Authenticate(http.HandlerFunc(bff.Dashboard.HandleDashboard)))
    mux.Handle("GET /api/v1/dashboard/sla", am.Authenticate(http.HandlerFunc(bff.Dashboard.HandleDashboardSLA)))

    // ═══════════════════════════════════════════════════════
    // SPRINT 2 — Finding Extensions (finding-service:8085)
    // ═══════════════════════════════════════════════════════

    // Stats
    mux.Handle("GET  /api/v1/findings/stats",            am.Authenticate(p.Forward("finding-service:8085")))
    // Bulk operations (order matters: more specific first)
    mux.Handle("POST /api/v1/findings/bulk/reopen",      am.Authenticate(p.Forward("finding-service:8085")))
    mux.Handle("POST /api/v1/findings/bulk/assign",      am.Authenticate(p.Forward("finding-service:8085")))
    mux.Handle("POST /api/v1/findings/bulk/close",       am.Authenticate(p.Forward("finding-service:8085")))
    // Notes
    mux.Handle("POST /api/v1/findings/{id}/notes",       am.Authenticate(p.Forward("finding-service:8085")))
    mux.Handle("GET  /api/v1/findings/{id}/notes",       am.Authenticate(p.Forward("finding-service:8085")))
    // Audit trail
    mux.Handle("GET  /api/v1/findings/{id}/audit",       am.Authenticate(p.Forward("finding-service:8085")))

    // ═══════════════════════════════════════════════════════
    // SPRINT 2 — Product Extensions (finding-service:8085)
    // ═══════════════════════════════════════════════════════

    mux.Handle("GET  /api/v1/products/grades",           am.Authenticate(p.Forward("finding-service:8085")))
    mux.Handle("GET  /api/v1/products/types",            am.Authenticate(p.Forward("finding-service:8085")))

    // ═══════════════════════════════════════════════════════
    // SPRINT 2 — Reports (finding-service:8085)
    // ═══════════════════════════════════════════════════════

    mux.Handle("GET  /api/v1/reports/{id}/download",     am.Authenticate(p.Forward("finding-service:8085")))

    // ═══════════════════════════════════════════════════════
    // SPRINT 2 — Notifications (notification-service:8087)
    // ═══════════════════════════════════════════════════════

    // SSE stream — special proxy (no timeout, no buffering)
    mux.Handle("GET /api/v1/notifications/stream",          am.Authenticate(p.ForwardSSE("notification-service:8087")))
    // REST notifications
    mux.Handle("GET  /api/v1/notifications",                am.Authenticate(p.Forward("notification-service:8087")))
    mux.Handle("PATCH /api/v1/notifications/",              am.Authenticate(p.Forward("notification-service:8087"))) // /{id}/read
    mux.Handle("POST  /api/v1/notifications/mark-all-read", am.Authenticate(p.Forward("notification-service:8087")))
    mux.Handle("GET   /api/v1/notifications/unread-count",  am.Authenticate(p.Forward("notification-service:8087")))
    // Webhooks
    mux.Handle("GET    /api/v1/webhooks",                   am.Authenticate(p.Forward("notification-service:8087")))
    mux.Handle("POST   /api/v1/webhooks",                   am.Authenticate(p.Forward("notification-service:8087")))
    mux.Handle("DELETE /api/v1/webhooks/",                  am.Authenticate(p.Forward("notification-service:8087")))
    mux.Handle("POST   /api/v1/webhooks/{id}/test",         adminOnly(p.Forward("notification-service:8087")))

    // ═══════════════════════════════════════════════════════
    // SPRINT 3 — Admin Health + Settings (in-gateway BFF)
    // ═══════════════════════════════════════════════════════

    mux.Handle("GET   /api/v1/admin/health",   adminOnly(http.HandlerFunc(bff.Health.HandleAdminHealth)))
    mux.Handle("GET   /api/v1/admin/settings", adminOnly(http.HandlerFunc(bff.Settings.GetSettings)))
    mux.Handle("PATCH /api/v1/admin/settings", adminOnly(http.HandlerFunc(bff.Settings.UpdateSettings)))

    // ═══════════════════════════════════════════════════════
    // SPRINT 3 — JIRA Config (jira-service:8088)
    // ═══════════════════════════════════════════════════════

    mux.Handle("GET  /api/v1/jira/config",      adminOnly(p.Forward("jira-service:8088")))
    mux.Handle("POST /api/v1/jira/config",      adminOnly(p.Forward("jira-service:8088")))
    mux.Handle("POST /api/v1/jira/config/test", adminOnly(p.Forward("jira-service:8088")))

    // ═══════════════════════════════════════════════════════
    // SPRINT 3 — Audit Log (audit-service:8090)
    // ═══════════════════════════════════════════════════════

    mux.Handle("GET /api/v1/audit-log", adminOnly(p.Forward("audit-service:8090")))

    // ═══════════════════════════════════════════════════════
    // SPRINT 4 — CVE Intel New Endpoints (data-service:8082)
    // ═══════════════════════════════════════════════════════

    mux.Handle("GET /api/v2/epss/top",          am.Authenticate(p.Forward("data-service:8082")))
    mux.Handle("GET /api/v2/epss/distribution", am.Authenticate(p.Forward("data-service:8082")))
    mux.Handle("GET /api/v2/cwe",               am.Authenticate(p.Forward("data-service:8082"))) // list only; /{id} existing
    mux.Handle("GET /api/v2/vendors",           am.Authenticate(p.Forward("data-service:8082")))

    // ═══════════════════════════════════════════════════════
    // RISK ACCEPTANCES (finding-service — existing path, keep)
    // ═══════════════════════════════════════════════════════

    mux.Handle("GET    /api/v1/risk-acceptances",    am.Authenticate(p.Forward("finding-service:8085")))
    mux.Handle("POST   /api/v1/risk-acceptances",    am.Authenticate(p.Forward("finding-service:8085")))
    mux.Handle("DELETE /api/v1/risk-acceptances/",   am.Authenticate(p.Forward("finding-service:8085")))

    // ═══════════════════════════════════════════════════════
    // SLA Config (sla-service:8086 — existing + dashboard)
    // ═══════════════════════════════════════════════════════

    mux.Handle("GET /api/v1/sla/config", am.Authenticate(p.Forward("sla-service:8086")))
    mux.Handle("PUT /api/v1/sla/config", adminOnly(p.Forward("sla-service:8086")))

    // ═══════════════════════════════════════════════════════
    // v3.0 PLANNED — Scan Service (scan-service:8058)
    // Register now with 503 fallback if service not deployed
    // ═══════════════════════════════════════════════════════

    // scanProxy := p.ForwardWithFallback("scan-service:8058", serviceUnavailableHandler)
    // mux.Handle("GET  /api/v1/scans",              am.Authenticate(scanProxy))
    // mux.Handle("POST /api/v1/scans",              am.Authenticate(scanProxy))
    // ... add when scan-service is deployed
}

// ─── Required gateway middleware interface ────────────────────────

// AuthMiddleware injects user context from JWT into request headers
// All services rely on X-User-ID, X-User-Role, X-User-Email being set correctly

// RequireRole returns middleware that checks X-User-Role header
// This MUST be applied after Authenticate (which sets X-User-Role)
func (m *authMiddleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userRole := r.Header.Get("X-User-Role")
            for _, role := range roles {
                if userRole == role {
                    next.ServeHTTP(w, r)
                    return
                }
            }
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(403)
            w.Write([]byte(`{"error":"FORBIDDEN","message":"Insufficient permissions"}`))
        })
    }
}
```

---

## Route Conflict Resolution

Go 1.22+ ServeMux phân biệt các paths dựa trên độ cụ thể:
- `GET /api/v1/findings/stats` — exact, checked BEFORE `GET /api/v1/findings/{id}`
- `POST /api/v1/findings/bulk/reopen` — more specific than `POST /api/v1/findings/{id}/notes`
- `GET /api/v2/cwe` — exact, separate from `GET /api/v2/cwe/{id}` (wildcard)

---

## Path Rewriting

Các services có internal paths khác với public paths. Gateway cần strip prefix:

```go
// proxy.go — Forward function strips /api/v1 or /api/v2 prefix

func (p *Proxy) Forward(target string) http.Handler {
    upstreamURL, _ := url.Parse("http://" + target)
    proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

    proxy.Director = func(req *http.Request) {
        req.URL.Scheme = upstreamURL.Scheme
        req.URL.Host = upstreamURL.Host

        // Strip /api/v1 or /api/v2 prefix
        // /api/v1/auth/login → /auth/login (identity-service)
        // /api/v2/cves/search → /api/v2/cves/search (data-service keeps v2 prefix)

        if strings.HasPrefix(req.URL.Path, "/api/v1/") {
            req.URL.Path = strings.TrimPrefix(req.URL.Path, "/api/v1")
        }
        // /api/v2/ is kept for data-service and search-service
    }
    return proxy
}
```

---

## Verification

```bash
cd apps/osv

# Build gateway
go build ./...

# Run gateway
./osv-gateway &

# Test all new auth routes
curl -X POST http://localhost:8080/api/v1/auth/login \
  -d '{"email":"admin@test.com","password":"test"}' | jq .status
# Expected: not "404"

# Test dashboard BFF
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/dashboard | jq 'keys | length'
# Expected: 7

# Test SSE connection
curl -N -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/notifications/stream &
sleep 2; kill %1
# Expected: "event: connected" received before kill

# Test admin route without admin role
USER_TOKEN=$(login_as_user)
curl -H "Authorization: Bearer $USER_TOKEN" \
  http://localhost:8080/api/v1/admin/users | jq .error
# Expected: "FORBIDDEN"
```

---

## Checklist

- [x] Auth routes: login + refresh có NO auth middleware
- [x] Tất cả `/api/v1/admin/*` require `role:admin` middleware
- [x] SSE route `GET /api/v1/notifications/stream` dùng `ForwardSSE` (không timeout)
- [x] BFF routes (dashboard, health, settings) xử lý trong gateway process (không forward)
- [x] Path rewriting: `/api/v1/` stripped khi forward đến services (trừ data-service giữ `/api/v2/`)
- [x] `GET /api/v2/cwe` (exact) không conflict với `GET /api/v2/cwe/{id}` (wildcard)
- [x] `RequireRole` middleware check `X-User-Role` header (được inject bởi `Authenticate`)
- [x] `go build ./...` thành công không có errors
- [x] `go vet ./...` không có warnings

## Notes for AI

- Thứ tự đăng ký routes quan trọng: exact paths phải đăng ký TRƯỚC wildcard paths
- File upload route `POST /api/v1/scans/import` cần `p.ForwardWithMaxBody(100MB)` để tránh OOM
- `p.ForwardSSE` phải set `X-Accel-Buffering: no` header và không có `ResponseTimeout`
- Khi deploy từng service theo Sprint, uncomment routes tương ứng để tránh 502 gateway errors
