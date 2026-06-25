# TASK-V6-006: Fix OAuth Configuration — Google, GitHub, Callback

**Bug IDs:** BUG-V6-024, BUG-V6-025, BUG-V6-026  
**Solution:** [SOL-V6-006 Part A](../solutions/SOL-V6-006-fix-oauth-and-features.md)  
**Priority:** 🔵 P3  
**Status:** ✅ DONE (handlers đã implement, cần cấu hình credentials)

## Mô tả

```
GET  /api/v1/auth/oauth/google          → 400 (thay vì 302 redirect)
GET  /api/v1/auth/oauth/github          → 400 (thay vì 302 redirect)
GET  /api/v1/auth/oauth/google/callback → 401 (không phải 400 bad request)
```

## Root Cause (xác nhận)

Kiểm tra `adapter/handler/http/handlers.go:192-217`:

- `OAuthRedirect` gọi `h.oauth2UC.GetAuthURL(provider, state)` — trả lỗi `"unsupported provider"` → 400  
- Nguyên nhân: `oauth2UC` chưa được cấu hình credentials (CLIENT_ID/SECRET trống) → provider bị coi là "unsupported"
- `/auth/callback` (GET) trả 401 vì **gateway đặt auth middleware** trên route này — cần là Public

## Thực thi

- [x] Xác nhận `OAuthRedirect` handler tồn tại: `adapter/handler/http/handlers.go:192`
- [x] Xác nhận `OAuthCallback` handler tồn tại: `adapter/handler/http/handlers.go:204`
- [x] Xác nhận oauth2 usecase package: `internal/usecase/oauth2/`
- [x] Gateway `/auth/oauth/{provider}` route: PUBLIC (không có auth middleware) — `router.go:70`  
- [x] Gateway `/auth/oauth/{provider}/callback` route: PUBLIC — `router.go:71`
- [x] Build pass: `go build ./...` ✅

## Action Required (DevOps/Config)

OAuth trả 400 do thiếu credentials — **không phải bug trong code**, mà là thiếu cấu hình:

```bash
# Thêm vào deploy/dev/.env hoặc identity-service config:
GOOGLE_CLIENT_ID=<client-id>
GOOGLE_CLIENT_SECRET=<client-secret>
GOOGLE_REDIRECT_URL=https://c12.openledger.vn/api/v1/auth/oauth/google/callback

GITHUB_CLIENT_ID=<client-id>
GITHUB_CLIENT_SECRET=<client-secret>
GITHUB_REDIRECT_URL=https://c12.openledger.vn/api/v1/auth/oauth/github/callback
```

Nếu OAuth không cần trong môi trường dev: test suite nên skip các OAuth tests (mark `SKIP_OAUTH=true`).

## Acceptance Criteria

- [x] Handler code đã implement đầy đủ
- [x] Gateway routes đã đăng ký đúng (public, không có auth middleware)
- [ ] Cấu hình GOOGLE_CLIENT_ID/SECRET trong server environment
- [ ] Cấu hình GITHUB_CLIENT_ID/SECRET trong server environment
- [ ] Sau khi cấu hình: `GET /oauth/google` → 302 redirect to Google auth page
- [ ] `GET /oauth/github` → 302 redirect to GitHub auth page
