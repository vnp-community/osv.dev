# CR-UI-001 — Authentication & User API

**Series:** UI-API v2  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🟢 Mock Layer Complete / Backend Pending  
**Ưu tiên:** P0 — Critical (blocking toàn bộ UI)  
**Nguồn yêu cầu:** `ui/specs/TDD.md` §2, `ui/specs/architecture.md` §6  
**Services ảnh hưởng:** `gateway (apps/osv :8080)`, `identity-service (:8081)`

---

## 1. Bối cảnh

Frontend UI (React SPA) cần một tập Auth API hoàn chỉnh để:
1. Login bằng email/password, trả về JWT RS256 access token (15min TTL) + httpOnly refresh cookie.
2. Refresh token tự động khi access token hết hạn (Axios interceptor).
3. Lấy thông tin user hiện tại để điền vào auth store.
4. OAuth2 redirect flow (Google / GitHub).
5. MFA (TOTP) setup và verify.
6. Logout an toàn (revoke refresh cookie, blacklist access token).

Hiện tại `identity-service` hỗ trợ LDAP + HS256 JWT. CR này yêu cầu **mở rộng** để hỗ trợ thêm RS256, MFA, OAuth2 (xem CR-OVS-003). Với phase hiện tại (v2.2), **tối thiểu phải có** các endpoint login/refresh/me/logout dùng HS256.

---

## 2. Endpoints yêu cầu

### 2.1 POST /api/v1/auth/login

**Mô tả:** Đăng nhập bằng email + password (local auth hoặc LDAP).

**Request Body:**
```json
{
  "email": "bob@company.com",
  "password": "s3cret",
  "mfa_code": "123456"
}
```

| Field | Type | Required | Mô tả |
|-------|------|----------|-------|
| `email` | string | ✅ | Email hoặc username |
| `password` | string | ✅ | Plaintext password (HTTPS only) |
| `mfa_code` | string | ❌ | 6-digit TOTP (chỉ khi MFA enabled) |

**Response 200:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 900,
  "user": {
    "id": "usr_abc123",
    "email": "bob@company.com",
    "name": "Bob Smith",
    "role": "user",
    "permissions": ["scan:read", "finding:write", "finding:read", "report:download"],
    "mfa_enabled": false,
    "avatar_url": null,
    "created_at": "2026-01-15T08:00:00Z"
  }
}
```

**Response 200 (MFA required — chưa provide `mfa_code`):**
```json
{
  "mfa_required": true,
  "access_token": null,
  "user": null
}
```

**Response Errors:**
| Status | Error Code | Mô tả |
|--------|-----------|-------|
| 400 | `VALIDATION_ERROR` | Missing email/password |
| 401 | `INVALID_CREDENTIALS` | Sai email/password |
| 401 | `MFA_REQUIRED` | Cần TOTP code |
| 401 | `INVALID_MFA_CODE` | TOTP code sai |
| 423 | `ACCOUNT_LOCKED` | Quá 5 lần sai liên tiếp |

**Headers:**
- Response phải set `Set-Cookie: refresh_token=...; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth/refresh; Max-Age=604800`

---

### 2.2 POST /api/v1/auth/refresh

**Mô tả:** Đổi refresh token (httpOnly cookie) lấy access token mới. Không cần body.

**Request:** Cookie `refresh_token` tự động gửi kèm.

**Response 200:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 900
}
```

**Response Errors:**
| Status | Error Code | Mô tả |
|--------|-----------|-------|
| 401 | `REFRESH_TOKEN_INVALID` | Token không hợp lệ/hết hạn |
| 401 | `REFRESH_TOKEN_REUSED` | Token đã được dùng (replay attack) |

**Notes:**
- Refresh token rotation: mỗi lần refresh → revoke token cũ → issue token mới + set cookie mới.
- Reuse detection: nếu refresh token đã revoked được dùng lại → revoke toàn bộ family tokens.

---

### 2.3 GET /api/v1/auth/me

**Mô tả:** Lấy thông tin user đang đăng nhập. Dùng để khởi tạo auth store khi app load.

**Request Header:** `Authorization: Bearer {access_token}`

**Response 200:**
```json
{
  "user": {
    "id": "usr_abc123",
    "email": "bob@company.com",
    "name": "Bob Smith",
    "role": "user",
    "permissions": ["scan:read", "finding:write", "finding:read", "report:download"],
    "mfa_enabled": false,
    "avatar_url": null,
    "created_at": "2026-01-15T08:00:00Z"
  }
}
```

---

### 2.4 POST /api/v1/auth/logout

**Mô tả:** Đăng xuất — revoke refresh token cookie, blacklist access token.

**Request Header:** `Authorization: Bearer {access_token}`

**Response 200:**
```json
{ "success": true }
```

**Side effects:**
- Clear `refresh_token` cookie (set Max-Age=0).
- Add access token JTI vào Redis blacklist (TTL = thời gian còn lại của token).

---

### 2.5 GET /api/v1/auth/mfa/setup

**Mô tả:** Bắt đầu setup MFA — trả về TOTP secret + QR code URL.

**Auth:** `Authorization: Bearer {access_token}` (user đã login, chưa enable MFA)

**Response 200:**
```json
{
  "secret": "JBSWY3DPEHPK3PXP",
  "qr_url": "otpauth://totp/OSV:bob@company.com?secret=JBSWY3DPEHPK3PXP&issuer=OSV%20Platform",
  "backup_codes": [
    "a1b2-c3d4", "e5f6-g7h8", "i9j0-k1l2",
    "m3n4-o5p6", "q7r8-s9t0", "u1v2-w3x4",
    "y5z6-a7b8", "c9d0-e1f2"
  ]
}
```

---

### 2.6 POST /api/v1/auth/mfa/confirm

**Mô tả:** Xác nhận setup MFA bằng TOTP code từ authenticator app.

**Request Body:**
```json
{ "code": "123456" }
```

**Response 200:**
```json
{ "success": true, "mfa_enabled": true }
```

**Response 400:**
```json
{ "error": "INVALID_MFA_CODE", "message": "TOTP code is invalid or expired" }
```

---

### 2.7 GET /api/v1/auth/oauth/google

**Mô tả:** Initiate Google OAuth2 flow — redirect đến Google.

**Response 302:** Redirect đến `https://accounts.google.com/o/oauth2/auth?...`

---

### 2.8 GET /api/v1/auth/oauth/github

**Mô tả:** Initiate GitHub OAuth2 flow.

**Response 302:** Redirect đến `https://github.com/login/oauth/authorize?...`

---

### 2.9 GET /api/v1/auth/callback

**Mô tả:** OAuth2 callback — nhận code từ provider, exchange lấy tokens, upsert user.

**Query Params:** `code`, `state`, `provider` (google/github)

**Response 302:** Redirect về UI với access token:
```
Location: /auth/callback?access_token={jwt}&expires_in=900
```
Đồng thời set `refresh_token` httpOnly cookie.

---

## 3. Data Models

### User Object
```json
{
  "id": "usr_abc123",          // string, prefix "usr_"
  "email": "string",
  "name": "string",
  "role": "admin | user | readonly | agent",
  "permissions": ["string"],   // array of permission strings
  "mfa_enabled": false,        // boolean
  "avatar_url": null,          // string | null
  "created_at": "ISO8601"
}
```

### Permission Set mỗi Role

| Permission | admin | user | readonly | agent |
|-----------|-------|------|----------|-------|
| `scan:create` | ✅ | ✅ | ❌ | ❌ |
| `scan:read` | ✅ | ✅ | ✅ | ✅ |
| `asset:write` | ✅ | ✅ | ❌ | ❌ |
| `asset:read` | ✅ | ✅ | ✅ | ❌ |
| `finding:write` | ✅ | ✅ | ❌ | ❌ |
| `finding:read` | ✅ | ✅ | ✅ | ❌ |
| `report:download` | ✅ | ✅ | ✅ | ❌ |
| `user:manage` | ✅ | ❌ | ❌ | ❌ |
| `system:configure` | ✅ | ❌ | ❌ | ❌ |
| `agent:report` | ❌ | ❌ | ❌ | ✅ |

---

## 4. API Error Format (Standard)

Mọi error response PHẢI theo format:
```json
{
  "error": "MACHINE_READABLE_CODE",
  "message": "Human readable message",
  "details": {},
  "trace_id": "abc123"
}
```

---

## 5. Security Requirements

| Requirement | Mô tả |
|------------|-------|
| HTTPS only | Tất cả auth endpoints phải TLS 1.3+ |
| Rate limit | Login: 5 req/min per IP; Refresh: 10 req/min per token |
| Account lockout | 5 consecutive failures → 15min lockout |
| Refresh token storage | httpOnly, Secure, SameSite=Strict cookie |
| Access token storage | Không lưu localStorage/sessionStorage (UI dùng Zustand memory) |
| CORS | Allow only whitelisted origins từ config |

---

## 6. Gateway Routes cần thêm

| Method | Path | Backend Service | Auth Required |
|--------|------|----------------|---------------|
| POST | `/api/v1/auth/login` | identity-service | ❌ |
| POST | `/api/v1/auth/refresh` | identity-service | ❌ (cookie) |
| GET | `/api/v1/auth/me` | identity-service | ✅ JWT |
| POST | `/api/v1/auth/logout` | identity-service | ✅ JWT |
| GET | `/api/v1/auth/mfa/setup` | identity-service | ✅ JWT |
| POST | `/api/v1/auth/mfa/confirm` | identity-service | ✅ JWT |
| GET | `/api/v1/auth/oauth/google` | identity-service | ❌ |
| GET | `/api/v1/auth/oauth/github` | identity-service | ❌ |
| GET | `/api/v1/auth/callback` | identity-service | ❌ |

---

## 7. Acceptance Criteria

> **Chú thích:** `[x]` = đã implement (UI mock layer + component); `[ ]` = backend pending

- [x] `POST /api/v1/auth/login` với valid credentials → 200 + `access_token` + `refresh_token` cookie _(mock: auth.handlers.ts, LoginScreen.tsx — updated: proper cookie header, mfa_required flow)_
- [x] `POST /api/v1/auth/login` với invalid credentials → 401 `INVALID_CREDENTIALS` _(mock: error code updated)_
- [x] `POST /api/v1/auth/refresh` với valid cookie → 200 + new `access_token` _(mock: auto-refresh via Axios interceptor)_
- [x] `POST /api/v1/auth/refresh` với expired/invalid cookie → 401 _(mock handled)_
- [x] `GET /api/v1/auth/me` với valid Bearer token → 200 + user object đầy đủ với permissions array _(mock: auth.handlers.ts)_
- [x] `POST /api/v1/auth/logout` → clear cookie + blacklist token _(mock: Max-Age=0 cookie set)_
- [x] Mọi user response đều có `permissions` array để UI render RBAC _(Zustand auth store)_
- [x] MFA setup flow: `GET /api/v1/auth/mfa/setup` → secret + QR URL + backup codes _(mock: auth.handlers.ts)_
- [x] MFA confirm: `POST /api/v1/auth/mfa/confirm` với valid 6-digit code → 200 success _(mock: auth.handlers.ts)_
- [x] OAuth initiation: `GET /api/v1/auth/oauth/google` và `GET /api/v1/auth/oauth/github` → redirect URL _(mock: auth.handlers.ts)_
- [x] OAuth callback: `GET /api/v1/auth/callback?code=...` → access_token + user _(mock: auth.handlers.ts)_
- [x] MFA setup flow hoạt động end-to-end (TDD §2.3) — _(mock: auth.handlers.ts, UserProfile.tsx)_
- [x] OAuth2 callback redirect đến UI với access token trong URL — _(mock: auth.handlers.ts, OAuthCallback.tsx)_

---

## 8. Phụ thuộc

| CR | Mô tả |
|----|-------|
| CR-007 (v1 cve-search) | RBAC roles — đã implement |
| CR-OVS-003 (v2) | JWT RS256, MFA TOTP, OAuth2 — planned |

> **Lưu ý:** Các endpoint MFA và OAuth (§2.5–2.9) là **v3.0 features** (CR-OVS-003). Phase hiện tại (v2.2) **bắt buộc** implement §2.1–2.4 (login/refresh/me/logout). §2.5–2.9 là **optional** cho v2.2, required cho v3.0.
