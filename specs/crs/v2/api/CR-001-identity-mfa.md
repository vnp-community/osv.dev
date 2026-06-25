# Change Request 001: Hỗ trợ Multi-Factor Authentication (MFA) & Auth Gaps

**Cập nhật:** 2026-06-18  
**Status:** In Progress — một số endpoints đã có trong backend, một số vẫn thiếu.

## 1. Bối cảnh

Frontend (openapi.yaml) yêu cầu các API sau trong nhóm Auth:

| Endpoint frontend | Path backend hiện tại | Trạng thái |
|---|---|---|
| `GET /api/v1/auth/mfa/setup` | `GET /api/v1/auth/totp/setup` | ⚠️ **Path mismatch** — backend dùng `/totp/setup`, frontend gọi `/mfa/setup` |
| `POST /api/v1/auth/mfa/confirm` | `POST /api/v1/auth/totp/verify` | ⚠️ **Path mismatch** — backend `/totp/verify`, frontend `/mfa/confirm` |
| `DELETE /api/v1/auth/totp` | `DELETE /api/v1/auth/totp` | ✅ Đúng |
| `POST /api/v1/auth/login` với `mfa_code` | `POST /api/v1/auth/login` | ⚠️ Chưa rõ response schema (`mfa_required` field) |
| `GET /api/v1/auth/me` | `GET /api/v1/auth/me` | ⚠️ Cần kiểm tra `mfa_enabled` có trong response không |
| `GET /api/v1/admin/users/{id}` | **THIẾU** | ❌ Backend chỉ có `GET /api/v1/admin/users` (list), không có get-by-id |
| `PATCH /api/v1/admin/users/{id}` | `PATCH /api/v1/admin/users/{id}` | ✅ Có |
| `GET /api/v1/auth/oauth/google` | `GET /api/v1/auth/oauth/google` | ✅ Có |
| `GET /api/v1/auth/oauth/github` | `GET /api/v1/auth/oauth/github` | ✅ Có |
| `GET /api/v1/auth/callback` | Chưa tường minh | ⚠️ Cần xác nhận callback path |

## 2. Thay đổi Đề Xuất

### 2.1 [CRITICAL] Thêm alias path `/api/v1/auth/mfa/setup` tại Gateway

Frontend gọi `GET /api/v1/auth/mfa/setup` nhưng backend identity-service định nghĩa `POST /api/v1/auth/totp/setup`. **Giải pháp**: Thêm alias route tại Gateway hoặc đổi tên handler trong identity-service.

**Phương án khuyến nghị — thêm alias tại `apps/osv/internal/gateway/router.go`:**
```go
// MFA aliases (frontend dùng /mfa/*, backend có /totp/*)
mux.Handle("GET /api/v1/auth/mfa/setup",   protected(proxy.Forward("identity-service:8081")))
mux.Handle("POST /api/v1/auth/mfa/confirm", protected(proxy.Forward("identity-service:8081")))
// Gateway rewrite /mfa/setup -> /totp/setup, /mfa/confirm -> /totp/verify
```

Hoặc đổi tên endpoint trong identity-service để match frontend:
- `GET /api/v1/auth/totp/setup` → `GET /api/v1/auth/mfa/setup`
- `POST /api/v1/auth/totp/verify` → `POST /api/v1/auth/mfa/confirm` (response: `{success, mfa_enabled}`)

### 2.2 [HIGH] Thêm `GET /api/v1/admin/users/{id}`

Frontend gọi `GET /api/v1/admin/users/{id}` để lấy chi tiết user (AdminUser schema). Backend hiện chỉ có danh sách.

```go
// apps/osv/internal/gateway/router.go
mux.Handle("GET /api/v1/admin/users/{id}", adminOnly(proxy.Forward("identity-service:8081")))
```

Identity-service cần thêm handler `GET /api/v1/admin/users/{id}` trả về `AdminUser` schema.

### 2.3 [HIGH] Login response cần field `mfa_required`

Khi user đã bật MFA mà không truyền `mfa_code`, `POST /api/v1/auth/login` phải trả về:
```json
{
  "access_token": null,
  "expires_in": 0,
  "user": null,
  "mfa_required": true
}
```

### 2.4 [MEDIUM] `GET /api/v1/auth/me` response cần `mfa_enabled`

Frontend schema `User` yêu cầu field `mfa_enabled: boolean`. Backend cần đảm bảo field này có trong response.

### 2.5 [MEDIUM] MFA setup response cần `backup_codes`

Frontend schema `MFASetupResponse` yêu cầu:
```json
{
  "secret": "string",
  "qr_url": "string",
  "backup_codes": ["string"]
}
```
Hiện backend chưa rõ có trả về `backup_codes` không.

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `GET /api/v1/auth/mfa/setup` (hoặc alias) trả về `{secret, qr_url, backup_codes}` — HTTP 200.
2. `POST /api/v1/auth/mfa/confirm` với TOTP đúng trả về `{success: true, mfa_enabled: true}`.
3. `POST /api/v1/auth/login` với account MFA nhưng không có `mfa_code` → trả về `{mfa_required: true}`.
4. `POST /api/v1/auth/login` với `mfa_code` đúng → trả về `access_token`.
5. `GET /api/v1/auth/me` response có field `mfa_enabled`.
6. `GET /api/v1/admin/users/{id}` trả về `AdminUser` object — HTTP 200, 404 nếu không tồn tại.
