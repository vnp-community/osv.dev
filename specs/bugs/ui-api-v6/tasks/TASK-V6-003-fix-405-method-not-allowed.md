# TASK-V6-003: Fix 405 Method Not Allowed — 4 Endpoints

**Bug IDs:** BUG-V6-014, BUG-V6-015, BUG-V6-016, BUG-V6-017  
**Solution:** [SOL-V6-003](../solutions/SOL-V6-003-fix-405-method-not-allowed.md)  
**Priority:** 🟠 P1  
**Status:** ✅ DONE

## Mô tả

4 endpoints trả về 405 Method Not Allowed:

```
GET  /api/v1/findings/{id}/notes → 405
PUT  /api/v1/sla/config          → 405
GET  /api/v1/webhooks/stats      → 405 (thực tế là /webhooks/stats/hourly)
GET  /api/v1/admin/users/{id}    → 405
```

## Thực thi

### BUG-V6-014: GET /findings/{id}/notes
- [x] Route đã được đăng ký trong gateway: `router.go:176` (`GET /api/v1/findings/{id}/notes`)
- [x] `GET /api/v1/findings/{id}/notes` BEFORE wildcard `/{id}` — đúng thứ tự
- [x] finding-service có handler (forwarded proxy)

### BUG-V6-015: PUT /sla/config
- [x] Route đã được đăng ký trong gateway: `router.go:395` (`PUT /api/v1/sla/config`)
- [x] `UpdateConfig` handler đã implement đầy đủ: `sla-service/internal/delivery/http/config_handler.go:85`
- [x] Validation logic (days ordering) đúng
- [x] Build pass: `go build ./...` ✅

### BUG-V6-016: GET /webhooks/stats/hourly
- [x] Route đã được đăng ký trong gateway: `router.go:263` (`GET /api/v1/webhooks/stats/hourly`)
- [x] Literal path `/stats/hourly` đăng ký trước wildcard `/{id}` — comment CR-009 xác nhận
- [x] `GetWebhookHourlyStats` handler tồn tại: `notification-service/internal/delivery/http/delivery_handler.go:216`

### BUG-V6-017: GET /admin/users/{id}
- [x] Route đã được đăng ký trong gateway: `router.go:150` (`GET /api/v1/admin/users/{id}`) — comment CR-001
- [x] Literal `/users/invite`, `/users/bulk` đăng ký TRƯỚC wildcard `/users/{id}` — đúng thứ tự
- [x] `GetUser` handler tồn tại: `adapter/handler/http/admin_handler.go:96`
- [x] Build pass: `go build ./...` ✅

## Acceptance Criteria

- [x] Gateway router đăng ký đúng method cho cả 4 endpoints
- [x] Literal paths được đăng ký trước wildcard paths trong mọi router
- [x] `UpdateConfig` (sla-service) implement đầy đủ với validation
- [x] `GetWebhookHourlyStats` (notification-service) implement đầy đủ
- [x] `GetUser` (identity-service) implement đầy đủ
- [ ] Verify live endpoints trả đúng code (cần deploy)
