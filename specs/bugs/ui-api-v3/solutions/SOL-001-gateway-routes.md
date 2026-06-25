# SOL-001: Gateway Route Registration

> **Bugs giải quyết**: BUG-001, 002, 005, 007, 010, 013, 014, 016, 017 (và một phần tất cả bugs còn lại)  
> **Service**: `apps/osv` — API Gateway  
> **File chính**: `apps/osv/internal/gateway/routes.go` (hoặc file setup routes tương đương)  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**Đã hoàn thành** trong `apps/osv/internal/gateway/router.go`:

| Route | Trạng thái |
|---|---|
| `GET /api/v1/auth/mfa/setup` → identity-service (rewrite) | ✅ Đã có |
| `POST /api/v1/auth/mfa/confirm` → identity-service (rewrite) | ✅ Đã có |
| `GET /api/v1/profile`, `PATCH /api/v1/profile`, `POST .../change-password` | ✅ Đã có |
| `GET /api/v1/profile/sessions` | ✅ Thêm mới (SOL-001 fix) |
| `DELETE /api/v1/profile/sessions/{sessionId}` | ✅ Thêm mới (SOL-001 fix) |
| `GET /api/v1/profile/notifications/settings` | ✅ Thêm mới (SOL-001 fix) |
| `PUT /api/v1/profile/notifications/settings` | ✅ Thêm mới (SOL-001 fix) |
| `GET /api/v1/notifications`, `unread-count`, `mark-all-read` | ✅ Đã có |
| `GET /api/v1/scans/history` (trước `/{id}`) | ✅ Đã có (TASK-009) |
| `GET /api/v1/risk-acceptances`, `POST`, `DELETE /{id}` | ✅ Đã có |
| `GET /api/v1/jira/config`, `PUT`, `POST /test` | ✅ Đã có |
| `GET /api/v1/audit-log` | ✅ Đã có |
| `GET /api/v1/products/grades` | ✅ Đã có |
| `GET /api/v1/webhooks/stats/hourly` | ✅ Đã có |
| `GET /api/v1/ai/triage/queue`, `GET /ai/enrichment`, `POST /trigger`, `GET /insights` | ✅ Đã có (TASK-006) |
| `GET /api/v1/search/recent` | ✅ Thêm mới (SOL-001 fix) |
| `GET /api/v1/search/suggested` | ✅ Thêm mới (SOL-001 fix) |
| `GET /api/v2/cves/search/semantic/suggestions` (trước `/semantic`) | ✅ Thêm mới (SOL-001 fix) |
| `GET /api/v2/browse`, `GET /api/v2/dbinfo` | ✅ Đã có |

**Build verify**: `go build ./...` ✅ apps/osv

## Nguyên Nhân Gốc

Theo `01-architecture.md` §3.1, Gateway đã được thiết kế với route table Sprint 1–4. Tuy nhiên, nhiều routes chưa được register trong implementation thực tế tại `apps/osv`. Đây là nguyên nhân gây ra 404 cho các static routes.

**Quan trọng**: Theo architecture, gateway chỉ là **reverse proxy** — nó forward request đến upstream service. Nếu upstream service ĐÃ implement endpoint, việc fix chỉ cần thêm route vào gateway.

## Các Routes Cần Thêm vào Gateway

### Sprint 1 Routes (identity-service:8081)

```go
// apps/osv/internal/gateway/setup.go

// ── MFA (BUG-001) ──────────────────────────────────────────────────────────
// Architecture §3.4: MFA aliases — BFF path rewrite
// GET /api/v1/auth/mfa/setup   → rewrite → /api/v1/auth/totp/setup   → identity:8081
// POST /api/v1/auth/mfa/confirm → rewrite → /api/v1/auth/totp/verify  → identity:8081
mux.Handle("GET /api/v1/auth/mfa/setup",
    protected(rewrite("/api/v1/auth/mfa/setup", "/api/v1/auth/totp/setup",
        proxy.Forward("identity-service:8081"))))

mux.Handle("POST /api/v1/auth/mfa/confirm",
    protected(rewrite("/api/v1/auth/mfa/confirm", "/api/v1/auth/totp/verify",
        proxy.Forward("identity-service:8081"))))

// ── Profile (BUG-014) ────────────────────────────────────────────────────
// Architecture §3.1 Sprint 1: /api/v1/profile → identity-service:8081
mux.Handle("GET /api/v1/profile",
    protected(proxy.Forward("identity-service:8081")))
mux.Handle("PATCH /api/v1/profile",
    protected(proxy.Forward("identity-service:8081")))
mux.Handle("POST /api/v1/profile/change-password",
    protected(proxy.Forward("identity-service:8081")))
mux.Handle("GET /api/v1/profile/sessions",
    protected(proxy.Forward("identity-service:8081")))
mux.Handle("DELETE /api/v1/profile/sessions/{sessionId}",
    protected(proxy.Forward("identity-service:8081")))
mux.Handle("GET /api/v1/profile/notifications/settings",
    protected(proxy.Forward("identity-service:8081")))
mux.Handle("PUT /api/v1/profile/notifications/settings",
    protected(proxy.Forward("identity-service:8081")))
```

### Sprint 2 Routes (notification-service:8087)

```go
// ── Notifications (BUG-002) ───────────────────────────────────────────────
// Architecture §3.1 Sprint 2: /api/v1/notifications/* → notification-service:8087
mux.Handle("GET /api/v1/notifications",
    protected(proxy.Forward("notification-service:8087")))
mux.Handle("GET /api/v1/notifications/unread-count",
    protected(proxy.Forward("notification-service:8087")))
mux.Handle("POST /api/v1/notifications/mark-all-read",
    protected(proxy.Forward("notification-service:8087")))

// ── Scan History (BUG-005) ────────────────────────────────────────────────
// Architecture §3.1 Sprint 2: /api/v1/scans/* → scan-service:8084
// CRITICAL: Register BEFORE /api/v1/scans/{id} to avoid routing conflict
mux.Handle("GET /api/v1/scans/history",
    protected(proxy.Forward("scan-service:8084")))

// ── Risk Acceptances (BUG-007) ────────────────────────────────────────────
// finding-service:8085 manages RiskAcceptance domain
mux.Handle("GET /api/v1/risk-acceptances",
    protected(proxy.Forward("finding-service:8085")))
mux.Handle("POST /api/v1/risk-acceptances",
    protected(proxy.Forward("finding-service:8085")))
```

### Sprint 3 Routes

```go
// ── JIRA (BUG-013) ────────────────────────────────────────────────────────
// Architecture §3.1 Sprint 3: /api/v1/jira/* → jira-service:8088
// Chuẩn hóa sang /integrations/jira
mux.Handle("GET /api/v1/integrations/jira",
    adminOnly(proxy.Forward("jira-service:8088")))
mux.Handle("PUT /api/v1/integrations/jira",
    adminOnly(proxy.Forward("jira-service:8088")))
mux.Handle("POST /api/v1/integrations/jira/test",
    adminOnly(proxy.Forward("jira-service:8088")))
// Alias cũ (redirect sang path mới để không break clients)
mux.Handle("GET /api/v1/jira/config",
    adminOnly(redirectTo("/api/v1/integrations/jira")))

// ── Audit Log (BUG-016) ───────────────────────────────────────────────────
// Architecture §3.1 Sprint 3: /api/v1/audit-log → audit-service:8090
mux.Handle("GET /api/v1/audit-log",
    adminOnly(proxy.Forward("audit-service:8090")))
```

### Sprint 4 Routes

```go
// ── Product Grades (BUG-010) ──────────────────────────────────────────────
// finding-service:8085 — ComputeGrade already implemented (§3.5, §5.4)
// CRITICAL: Register BEFORE /api/v1/products/{id} to avoid routing conflict
mux.Handle("GET /api/v1/products/grades",
    protected(proxy.Forward("finding-service:8085")))

// ── Webhook Stats (BUG-012) ───────────────────────────────────────────────
mux.Handle("GET /api/v1/webhooks/stats",
    protected(proxy.Forward("notification-service:8087")))
// Alias với path cụ thể hơn (đồng bộ với endpoints.ts)
mux.Handle("GET /api/v1/webhooks/stats/hourly",
    protected(proxy.Forward("notification-service:8087")))

// ── AI Center (BUG-011) ───────────────────────────────────────────────────
// Architecture §3.11: ai-service đã có HTTP handlers cho tất cả endpoints
// Chỉ cần route trong gateway (hiện đã thiếu)
mux.Handle("GET /api/v1/ai/triage/queue",
    protected(proxy.Forward("ai-service:9103")))
mux.Handle("GET /api/v1/ai/enrichment",
    protected(proxy.Forward("ai-service:9103")))
mux.Handle("POST /api/v1/ai/enrichment/trigger",
    protected(proxy.Forward("ai-service:9103")))
mux.Handle("GET /api/v1/ai/insights",
    protected(proxy.Forward("ai-service:9103")))

// ── Search (BUG-017) ─────────────────────────────────────────────────────
// search-service:8083 → phần recent/suggested query
mux.Handle("GET /api/v1/search/recent",
    protected(proxy.Forward("search-service:8083")))
mux.Handle("GET /api/v1/search/suggested",
    protected(proxy.Forward("search-service:8083")))
// Semantic suggestions (BUG-003)
mux.Handle("GET /api/v2/cves/search/semantic/suggestions",
    protected(proxy.Forward("search-service:8083")))

// ── Browse Root + DBInfo (BUG-004) ───────────────────────────────────────
// Architecture §3.1 Public: /api/v2/browse → search-service:8083
mux.Handle("GET /api/v2/browse",
    proxy.Forward("search-service:8083"))  // Public — no auth
mux.Handle("GET /api/v2/dbinfo",
    proxy.Forward("data-service:8082"))    // Public — no auth
```

## Thứ Tự Đăng Ký Routes (QUAN TRỌNG)

Go 1.22 ServeMux ưu tiên routes cụ thể hơn, nhưng vẫn cần đăng ký đúng thứ tự khi có cả static và dynamic segments:

```go
// ✅ ĐÚNG — static route trước dynamic
mux.Handle("GET /api/v1/scans/history",  ...)   // PHẢI trước
mux.Handle("GET /api/v1/scans/{id}",     ...)   // PHẢI sau

mux.Handle("GET /api/v1/products/grades", ...)  // PHẢI trước
mux.Handle("GET /api/v1/products/{id}",   ...)  // PHẢI sau
```

> **Note**: Go 1.22+ ServeMux tự động ưu tiên exact match và longer prefix trước wildcard. Tuy nhiên cần kiểm tra version Go đang dùng.

## Rewrite Helper (BFF Pattern cho MFA)

```go
// apps/osv/internal/gateway/middleware.go

// rewritePath tạo middleware chuyển đổi path trước khi forward
func rewritePath(from, to string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        r.URL.Path = strings.Replace(r.URL.Path, from, to, 1)
        r.RequestURI = r.URL.RequestURI()
        next.ServeHTTP(w, r)
    })
}
```

## Checklist Verification

Sau khi thêm routes, chạy lại test:

```bash
cd tests/client && python3 test_all_endpoints.py 2>&1 | grep "✗"
```

Expected: 0 failures từ các routes đã liệt kê ở trên.
