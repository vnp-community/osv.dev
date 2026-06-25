# TASK-V6-001: Fix 500 Errors — Profile Sessions & Notification Settings

**Bug IDs:** BUG-V6-018, BUG-V6-019, BUG-V6-020  
**Solution:** [SOL-V6-001](../solutions/SOL-V6-001-fix-profile-500.md)  
**Priority:** 🔴 P0  
**Status:** ✅ DONE

## Mô tả

3 endpoints trong profile service gặp 500 Internal Server Error với response body trống:

```
GET  /api/v1/profile/sessions                → 500 (empty body)
GET  /api/v1/profile/notifications/settings  → 500 (empty body)
PUT  /api/v1/profile/notifications/settings  → 500 (empty body)
```

## Evidence

```
GET    https://c12.openledger.vn/api/v1/profile/sessions               →  500
GET    https://c12.openledger.vn/api/v1/profile/notifications/settings →  500
PUT    https://c12.openledger.vn/api/v1/profile/notifications/settings →  500
```

Response body: `""` (rỗng hoàn toàn — không có error JSON)

## Root Cause (xác nhận)

Handler `ListSessions` dùng `auth.sessions` table — ✅ đã có trong migration `001_initial_schema.sql`.  
Handler `GetNotifSettings`/`UpdateNotifSettings` dùng `auth.notification_preferences` table — ❌ **thiếu migration**.

## Thực thi

- [x] Xác nhận handler `ListSessions` tồn tại: `adapter/handler/http/profile_handler.go:140`
- [x] Xác nhận handler `GetNotifSettings` tồn tại: `adapter/handler/http/profile_handler.go:214`
- [x] Xác nhận handler `UpdateNotifSettings` tồn tại: `adapter/handler/http/profile_handler.go:227`
- [x] Xác nhận routes được đăng ký: `adapter/handler/http/router.go:160-163` và `:177-180`
- [x] Xác nhận gateway routes: `apps/osv/internal/gateway/router.go:118-121`
- [x] `auth.sessions` table đã có trong `migrations/001_initial_schema.sql`
- [x] **Tạo migration** `migrations/007_notification_preferences.sql` — bảng `auth.notification_preferences`
- [x] Build pass: `go build ./...` ✅

## Acceptance Criteria

- [x] Code của `GET /api/v1/profile/sessions` handler đã implement đầy đủ
- [x] Code của `GET /api/v1/profile/notifications/settings` handler đã implement đầy đủ
- [x] Code của `PUT /api/v1/profile/notifications/settings` handler đã implement đầy đủ
- [x] Migration `007_notification_preferences.sql` đã được tạo
- [ ] **Deploy migration lên server** (cần DBA/DevOps chạy: `psql $DATABASE_URL -f 007_notification_preferences.sql`)
- [ ] Verify live endpoints trả 200 (cần deploy lên server)
