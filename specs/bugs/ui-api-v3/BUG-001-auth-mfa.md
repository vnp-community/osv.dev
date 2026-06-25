# BUG-001: Auth MFA — Route Chưa Implement

**ID**: BUG-001  
**Domain**: Authentication  
**Mức độ**: 🟡 HIGH  
**Loại**: `404 Not Found`  
**Phát hiện**: 2026-06-23  
**Trạng thái**: OPEN  

## Endpoints Bị Lỗi

| Method | Endpoint | HTTP Status | URL Thực Tế |
|---|---|---|---|
| `GET`  | `/api/v1/auth/mfa/setup`   | **404** | `https://c12.openledger.vn/api/v1/auth/mfa/setup` |
| `POST` | `/api/v1/auth/mfa/confirm` | **404** | `https://c12.openledger.vn/api/v1/auth/mfa/confirm` |

## Mô Tả

Hai endpoint MFA (Multi-Factor Authentication) hoàn toàn chưa được implement trên server:

- `GET /auth/mfa/setup`: Dùng để lấy QR code và secret key cho TOTP app (Google Authenticator, Authy,...).
- `POST /auth/mfa/confirm`: Dùng để verify TOTP code và kích hoạt MFA cho tài khoản.

UI hiện đang hiển thị trạng thái MFA "Active" trong `UserProfile.tsx` nhưng button thiết lập MFA sẽ fail khi được nhấn.

## Tác Động

- User không thể kích hoạt MFA mới.
- Giảm security posture của platform.

## Endpoint Spec Đề Xuất

```
GET /api/v1/auth/mfa/setup
→ 200 OK
{
  "secret": "JBSWY3DPEHPK3PXP",
  "qr_url": "otpauth://totp/...",
  "backup_codes": ["xxx-xxx", ...]
}

POST /api/v1/auth/mfa/confirm
Body: { "code": "123456" }
→ 200 OK { "enabled": true }
→ 400 { "error": "Invalid TOTP code" }
```

## Các Endpoints Đang Hoạt Động (Tham Khảo)

- ✅ `POST /api/v1/auth/login` → 400 (route OK)
- ✅ `POST /api/v1/auth/refresh` → 200
- ✅ `GET /api/v1/auth/me` → 200
