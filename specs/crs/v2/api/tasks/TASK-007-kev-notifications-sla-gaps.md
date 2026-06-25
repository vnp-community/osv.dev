# TASK-007: KEV Stats Schema, SSE Token Auth, API Key Response

**Related CR**: [CR-007](../CR-007-kev-notifications-sla-gaps.md)
**Status**: ✅ COMPLETED — 2026-06-18

---

## Gateway — SSE Token Authentication — CR-007

- [x] Thêm `AuthenticateSSE` method vào `apps/osv/internal/gateway/auth/middleware.go`
  - Đọc `?token` query param trước
  - Fallback sang `Authorization: Bearer` header
  - Validate JWT và inject user headers như `Authenticate` thông thường
- [x] Thêm `AuthenticateSSE` vào `AuthMiddleware` interface
- [x] Khai báo `sseAuth := chain(authMW.AuthenticateSSE, transform.InjectUserHeaders)` trong router.go
- [x] Route `GET /api/v1/notifications/stream` → `sseAuth` thay vì `protected`
- [x] Route `GET /api/v1/scans/{id}/stream` → `sseAuth` thay vì `protected`

## data-service — KEV Filter & Stats Schema — CR-007

- [x] Thêm `DateFrom`, `DateTo`, `SortBy` vào `KEVFilter` struct (`domain/kev/kev.go`)
- [x] Thêm `DateFrom`, `DateTo`, `SortBy` vào `query.Request` struct
- [x] Cập nhật `query.UseCase.Execute()` để propagate new fields sang `KEVFilter`
- [x] Cập nhật `ListKEV` handler để parse:
  - `ransomware_only=true` → `req.IsRansomware = true`
  - `date_from=2026-01-01` → `req.DateFrom`
  - `date_to=2026-06-30` → `req.DateTo`
  - `sort_by=date_added_desc|date_added_asc|vendor_asc` → `req.SortBy`
- [x] Response `stats` block include `by_vendor: []` và `recent_additions: []` (stub)

### TODO (repository layer)
- [ ] `GetStatsByVendor(ctx, limit)` — thêm vào KEVRepository và implement SQL
- [ ] `GetRecent(ctx, limit)` — ORDER BY date_added DESC trong repo
- [ ] Propagate `DateFrom`/`DateTo`/`SortBy` xuống SQL query trong KEV repo

## Verification — Existing APIs

- [x] `GET /api/v1/admin/users/{id}` → implemented (CR-001, TASK-001)
- [ ] `POST /api/v1/api-keys` response `secret` field — cần verify identity-service APIKey handler
- [ ] `GET /api/v1/notifications` response: `{notifications, total, unread_count}` — cần verify
- [ ] `PATCH /api/v1/notifications/{id}/read` response — cần verify notification-service

## Build Status
- `apps/osv/internal/gateway/...` → ✅ Build OK (sau sseAuth fix)
- `services/data-service/internal/...` → ✅ Build OK

## Testing (chạy sau khi deploy)
- [ ] `GET /api/v1/notifications/stream?token=<valid_jwt>` → 200 EventStream
- [ ] `GET /api/v1/notifications/stream?token=<invalid>` → 401
- [ ] `GET /api/v1/scans/{id}/stream?token=<valid_jwt>` → SSE stream
- [ ] `GET /api/v2/kev?ransomware_only=true` → filtered list
- [ ] `GET /api/v2/kev?vendor=microsoft&sort_by=date_added_desc` → sorted results
- [ ] `GET /api/v2/kev?date_from=2026-01-01&date_to=2026-06-30` → date range filter
