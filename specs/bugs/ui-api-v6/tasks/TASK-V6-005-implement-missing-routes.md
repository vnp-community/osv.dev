# TASK-V6-005: Implement Missing Routes — Nhóm A (13 routes chưa có)

**Bug IDs:** BUG-V6-001 → BUG-V6-013  
**Solution:** [SOL-V6-005](../solutions/SOL-V6-005-implement-missing-routes.md)  
**Priority:** 🟡 P2  
**Status:** ✅ DONE (tất cả 13 routes đã được đăng ký hoặc xác nhận)

## Mô tả

13 routes trả về 404 Not Found — chưa được đăng ký trong gateway hoặc backend service.

## Thực thi — kết quả kiểm tra từng route

| Bug | Endpoint | Trạng thái | File / Line |
|-----|----------|-----------|-------------|
| V6-001 | `GET /auth/mfa/setup` | ✅ DONE | router.go:101 — ForwardRewrite → /totp/setup |
| V6-002 | `POST /auth/mfa/confirm` | ✅ DONE | router.go:107 — ForwardRewrite → /totp/verify |
| V6-003 | `GET /api/v2/browse` | ✅ DONE | router.go:324 — search-service:8083 |
| V6-004 | `GET /api/v2/dbinfo` | ✅ DONE | router.go:304 — data-service:8082 |
| V6-005 | `GET /products/grades` | ✅ DONE | router.go:214 — finding-service:8085 |
| V6-006 | `GET /ai/insights` | ✅ DONE | **Added** router.go:383 — ai-service:9103 |
| V6-007 | `GET /jira/config` | ✅ DONE | router.go:282 — jira-service:8088 |
| V6-008 | `POST /jira/config/test` | ✅ DONE | router.go:285 — jira-service:8088 |
| V6-009 | `GET /integrations/jira` | ✅ DONE | **Added** router.go:288 — ForwardRewrite → /jira/config |
| V6-010 | `PUT /integrations/jira` | ✅ DONE | **Added** router.go:293 — ForwardRewrite → /jira/config |
| V6-011 | `GET /audit-log` | ✅ DONE | router.go:291 — audit-service:8090 |
| V6-012 | `GET /search/recent` | ✅ DONE | router.go:319 — search-service:8083 |
| V6-013 | `GET /search/suggested` | ✅ DONE | router.go:320 — search-service:8083 |

## Code Changes

Thêm vào `apps/osv/internal/gateway/router.go`:

```go
// BUG-V6-006: AI insights summary dashboard
mux.Handle("GET /api/v1/ai/insights",
    protected(proxy.Forward("ai-service:9103")))

// BUG-V6-009, BUG-V6-010: /integrations/jira — alias → rewrite to /jira/config
mux.Handle("GET /api/v1/integrations/jira", adminOnly(proxy.ForwardRewrite(
    "jira-service:8088",
    "/api/v1/integrations/jira",
    "/api/v1/jira/config",
)))
mux.Handle("PUT /api/v1/integrations/jira", adminOnly(proxy.ForwardRewrite(
    "jira-service:8088",
    "/api/v1/integrations/jira",
    "/api/v1/jira/config",
)))
```

## Build Verification

```
✅ go build ./... — apps/osv (PASS)
✅ go build ./... — services/identity-service (PASS)
✅ go build ./... — services/finding-service (PASS)
✅ go build ./... — services/scan-service (PASS)
✅ go build ./... — services/notification-service (PASS)
✅ go build ./... — services/sla-service (PASS)
```

## Acceptance Criteria

- [x] Tất cả 13 routes đã được đăng ký trong gateway router
- [x] Literal paths đăng ký trước wildcard paths (không conflict)
- [x] Build pass sau khi thêm routes
- [ ] Verify live endpoints trả đúng code (cần deploy)
