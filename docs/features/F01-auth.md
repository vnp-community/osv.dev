# F01 — Authentication & Authorization

**Status:** ✅ v2.0 Implemented + 🔵 v3.0 Planned  
**CR References:** CR-007 (cve-search), CR-DD-011, CR-GCV-008, CR-OVS-003  
**Services:** `identity-service`, `gateway-service`  
**UI Routes:** `/login`, `/profile`, `/onboarding`, `/auth/callback`

---

## 1. Mô tả

Hệ thống xác thực và phân quyền toàn diện hỗ trợ nhiều phương thức đăng nhập, quản lý phiên làm việc an toàn, và kiểm soát truy cập dựa trên vai trò (RBAC). Gateway thực hiện xác thực tập trung cho tất cả requests trước khi dispatch đến microservices.

---

## 2. Tính năng đã triển khai (v2.0 — v2.2)

### 2.1 JWT Authentication (HS256)
- Access token JWT HS256, stateless auth
- Token được lưu trong Zustand memory store (không lưu localStorage)
- Refresh token lưu trong httpOnly cookie (Secure, SameSite=Strict)
- Gateway middleware validate JWT trong mọi request

### 2.2 API Key Management
- **Format:** prefix + base58 random bytes (vd: `osv_...`)
- **Storage:** SHA-256 hash only — plaintext chỉ hiển thị 1 lần khi tạo
- **Scoped permissions:** mỗi key có tập quyền riêng (vd: `scan:read`, `finding:write`)
- **Use case:** CI/CD pipelines, remote agents không cần username/password
- Endpoint: `GET/POST/DELETE /api/v1/api-keys`

### 2.3 LDAP Authentication
- Provider LDAP cho enterprise users
- Auth chain: local → LDAP (configurable order)
- LDAP groups mapping → OSV roles
- Config qua environment variables

### 2.4 RBAC — Role-Based Access Control

| Role | Permissions |
|------|------------|
| `admin` | Tất cả operations |
| `user` | `scan:read`, `finding:write`, `finding:read`, `report:download` |
| `readonly` | `scan:read`, `finding:read`, `report:download` |

### 2.5 Rate Limiting
- Per-IP Redis token bucket
- Login: 5 req/min per IP
- Gateway middleware: < 5ms validation latency

### 2.6 Dual Auth Gateway
- JWT + API Key xử lý trong cùng một middleware
- `X-User-*` headers được inject vào downstream services
- SSRF protection cho webhook delivery

---

## 3. Planned Features (v3.0 — CR-OVS-003)

### 3.1 JWT RS256 (Asymmetric)
- Algorithm: RS256 thay thế HS256
- Access token TTL: **15 phút**
- Refresh token rotation với reuse-attack detection
- Account lockout: **5 consecutive failures → 15 phút lockout**

### 3.2 MFA — Multi-Factor Authentication (TOTP)
- RFC 6238 compliant
- 30-second window, ±1 period tolerance
- 8 backup codes generate tại setup
- UI flow: Setup → QR code → Verify → Enable

### 3.3 OAuth2 Social Login
- Providers: **Google**, **GitHub**
- Flow: redirect → callback → upsert user → return JWT tokens
- Endpoint: `/auth/google`, `/auth/github`, `/auth/callback`

### 3.4 Argon2id Password Hashing
- Thay thế bcrypt hiện tại
- Memory: 64MB, iterations: 3, parallelism: 4

---

## 4. UI Components (React SPA)

| Component | Route | Mô tả |
|-----------|-------|-------|
| `LoginScreen` | `/login` | Email/password form, OAuth2 buttons |
| `OAuthCallback` | `/auth/callback` | Xử lý OAuth redirect |
| `UserProfile` | `/profile` | Thông tin user, đổi password, MFA setup |
| `OnboardingExperience` | `/onboarding` | First-time setup wizard |

---

## 5. API Contracts

### Implemented (v2.x)
| Endpoint | Mô tả |
|----------|-------|
| `POST /api/v1/auth/login` | Login với email + password |
| `POST /api/v1/auth/refresh` | Refresh access token từ httpOnly cookie |
| `GET /api/v1/auth/me` | Current user info + permissions |
| `POST /api/v1/auth/logout` | Revoke token + xóa cookie |
| `GET/POST/DELETE /api/v1/api-keys` | API Key management |

### Planned (v3.0)
| Endpoint | Mô tả |
|----------|-------|
| `GET /api/v1/auth/mfa/setup` | Bắt đầu MFA setup (QR code) |
| `POST /api/v1/auth/mfa/confirm` | Confirm TOTP code |
| `GET /auth/google` | Redirect to Google OAuth |
| `GET /auth/github` | Redirect to GitHub OAuth |

---

## 6. Error Codes

| HTTP | Code | Tình huống |
|------|------|-----------|
| 401 | `INVALID_CREDENTIALS` | Sai email/password |
| 401 | `TOKEN_EXPIRED` | JWT hết hạn |
| 401 | `REFRESH_TOKEN_REUSED` | Replay attack detected |
| 401 | `MFA_REQUIRED` | Cần TOTP code |
| 423 | `ACCOUNT_LOCKED` | > 5 login failures |
| 429 | `RATE_LIMIT_EXCEEDED` | Rate limit hit |

---

## 7. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| Auth middleware latency | < 5ms per request |
| Login rate limit | 5 req/min per IP |
| Account lockout | 5 consecutive failures → 15 min |
| Refresh token cookie | httpOnly, Secure, SameSite=Strict |
| Access token storage | Zustand memory (không localStorage) |
| API Key validation | < 5ms |
| TLS | Minimum TLS 1.3 |

---

## 8. Database Schema

**Schema:** `osv_identity`

| Table | Mô tả |
|-------|-------|
| `users` | User accounts, roles, password hash |
| `api_keys` | SHA-256 hashed keys, scopes, expiry |
| `ldap_configs` | LDAP server configs |
| `sessions` | Refresh token store (rotation) |
