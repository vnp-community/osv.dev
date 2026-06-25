# AI Task 001: Implement Identity MFA (SOL-001)

**Status**: ✅ COMPLETED 100% — 2026-06-18

## Checklist Công Việc

### 1. Database & Domain
- [x] Thêm `MfaEnabled`, `MfaSecret` vào struct `User`
- [x] Cập nhật Postgres Repository với `mfa_enabled`, `mfa_secret` columns

### 2. Cập nhật Postgres Repository
- [x] `UpdateMFASecret`, `EnableMFA` → implement via `UpdateMFA(ctx, userID, enabled, secret)`
- [x] TOTP methods at `adapter/repository/postgres/totp_methods.go`

### 3. Logic Xử Lý (UseCase)
- [x] `go get github.com/pquerna/otp`
- [x] `internal/usecase/totp/usecase.go` — SetupUseCase, ConfirmUseCase, DisableUseCase
- [x] `SetupMFA(ctx, userID)` → generate QR + secret
- [x] `ConfirmMFA(ctx, userID, code)` → validate + activate
- [x] `BackupCodes []string` thêm vào `SetupOutput` — generate 8 random 8-char hex codes

### 4. Sửa luồng Login (UseCase)
- [x] `user.MFAEnabled == true` → return `ErrMFARequired` nếu không có totp_code
- [x] Validate TOTP code với `crypto.ValidateTOTP`

### 5. HTTP Handlers & Gateway
- [x] `POST /api/v1/auth/totp/setup` → Setup() handler
- [x] `POST /api/v1/auth/totp/verify` → Verify() handler
- [x] `DELETE /api/v1/auth/totp` → Disable() handler
- [x] Gateway routes `/api/v1/auth/mfa/setup` và `/api/v1/auth/mfa/confirm`

## CR-001 Gap Fixes (thực thi 2026-06-18)

### ✅ Gateway — MFA path aliases
- [x] `GET /api/v1/auth/mfa/setup` → ForwardRewrite → identity-service `/totp/setup`
- [x] `POST /api/v1/auth/mfa/confirm` → ForwardRewrite → identity-service `/totp/verify`

### ✅ Gateway — Admin GET user by ID
- [x] `GET /api/v1/admin/users/{id}` → identity-service (adminOnly)

### ✅ identity-service — GetUser handler
- [x] `GetUser(w, r)` tại `adapter/handler/http/admin_handler.go`
- [x] Route `GET /users/{id}` trong `adapter/handler/http/router.go`

### ✅ identity-service — TOTP repo interface
- [x] `StorePendingTOTPSecret`, `GetPendingTOTPSecret`, `ActivateTOTP`, `DisableTOTP`
- [x] Implementations tại `adapter/repository/postgres/totp_methods.go`

### ✅ Login response `mfa_required` field (2026-06-18)
- [x] `Login` handler trả về `{"error": "mfa_required", "detail": "...", "mfa_required": true}` khi MFA required
- [x] Frontend có thể detect `mfa_required: true` để hiển thị TOTP input

### ✅ `GET /api/v1/auth/me` response `mfa_enabled` (2026-06-18)
- [x] Thêm `userRepo repository.UserRepository` vào `AuthHandler`
- [x] `Me()` handler query `userRepo.FindByID()` để lấy `user.MFAEnabled`
- [x] Response: `{"user_id":..., "role":..., "permissions":..., "mfa_enabled": true/false}`
- [x] `NewAuthHandler(...)` cập nhật với `userRepo` arg
- [x] `router.go` cập nhật call site: `deps.UserRepo` truyền vào

### ✅ MFA setup response `backup_codes` (2026-06-18)
- [x] `SetupOutput.BackupCodes []string` thêm vào usecase
- [x] `generateBackupCodes(8)` — 8 codes × 8 hex chars mỗi code
- [x] `totp_handler.go` Setup() trả về `{"qr_code_url":..., "secret":..., "backup_codes":[...]}`

## Build Status
- `adapter/handler/http/...` → ✅ Build OK
- `internal/usecase/totp/...` → ✅ Build OK
- `apps/osv/internal/gateway/...` → ✅ Build OK

## Pre-existing Errors (không liên quan đến TASK-001)
- `internal/usecase/logout/usecase.go:46` — `session.UpdatedAt` undefined (pre-existing bug)
- `internal/usecase/refresh/usecase.go:39` — `entity.Role` undefined (pre-existing bug)
- Các lỗi này nằm ngoài scope TASK-001 và không ảnh hưởng đến các layer đã implement
