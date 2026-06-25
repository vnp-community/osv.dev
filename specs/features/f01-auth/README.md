# F01 — Authentication & Authorization

> **Spec Folder:** `specs/features/f01-auth/`  
> **Feature Doc:** [`docs/features/F01-auth.md`](../../../docs/features/F01-auth.md)  
> **SRS Refs:** FR-05-01 → FR-05-06  
> **Status:** ✅ v2.0–2.2 Implemented | 🔵 v3.0 Planned (MFA, RS256, OAuth2)

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | Auth chain, RBAC rules, token lifecycle, lockout logic |
| [dataflow.md](./dataflow.md) | Login sequence, API Key validation, refresh flow, OAuth2 flow |

---

## Services

| Service | Port | Role |
|---------|------|------|
| `identity-service` | 8081 | User store, LDAP, API key management, token issuance |
| `apps/osv` (gateway) | 8080 | Auth middleware — validates every inbound request |

---

## Quick Reference: API Endpoints

### v2.x Implemented
| Method | Endpoint | Auth | Mô tả |
|--------|----------|------|-------|
| POST | `/api/v1/auth/login` | None | Login email + password → JWT |
| POST | `/api/v1/auth/refresh` | Cookie | Refresh access token |
| GET | `/api/v1/auth/me` | JWT | Current user info + permissions |
| POST | `/api/v1/auth/logout` | JWT | Revoke token + clear cookie |
| GET | `/api/v1/api-keys` | JWT | List user's API keys (masked) |
| POST | `/api/v1/api-keys` | JWT | Create new API key |
| DELETE | `/api/v1/api-keys/{id}` | JWT | Revoke API key |

### v3.0 Planned
| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v1/auth/mfa/setup` | MFA setup QR code |
| POST | `/api/v1/auth/mfa/confirm` | Confirm TOTP code |
| GET | `/auth/google` | Redirect to Google OAuth2 |
| GET | `/auth/github` | Redirect to GitHub OAuth2 |
| GET | `/auth/callback` | OAuth2 callback handler |

---

## Roles & Permissions (RBAC)

| Role | Scopes |
|------|--------|
| `admin` | All operations |
| `user` | `cve:read`, `finding:read`, `finding:write`, `scan:read`, `report:download` |
| `readonly` | `cve:read`, `finding:read`, `scan:read`, `report:download` |

### API Key Scopes
`cve:read` · `finding:read` · `finding:write` · `scan:read` · `scan:write` · `report:download` · `agent:report`

---

## Database Schema (`osv_identity`)

| Table | Key Fields | Mô tả |
|-------|-----------|-------|
| `users` | `id`, `email`, `role`, `password_hash`, `locked_until` | User accounts |
| `api_keys` | `id`, `user_id`, `prefix` (12 chars), `hash_sha256`, `scopes[]`, `revoked` | API keys |
| `sessions` | `id`, `user_id`, `refresh_token_hash`, `expires_at` | Refresh token store |
| `ldap_configs` | `id`, `host`, `port`, `bind_dn`, `group_mapping` | LDAP server configs |

---

## Key Constants

| Parameter | Value |
|-----------|-------|
| JWT algorithm (v2.x) | HS256 |
| JWT algorithm (v3.0) | RS256 |
| Access token TTL | 15 phút (v3.0) |
| Refresh token TTL | 7 ngày |
| Login rate limit | 5 req/min per IP |
| Account lockout threshold | 5 consecutive failures |
| Lockout duration | 15 phút |
| API key format | `osv_{base58_32_bytes}` |
| API key lookup | SHA-256(key) match by 12-char prefix |
