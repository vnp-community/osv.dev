# Data Models — identity-service

> **Service**: `services/identity-service`  
> **Mô tả**: Xác thực và phân quyền người dùng. Quản lý JWT access tokens, refresh tokens (session family), TOTP MFA, OAuth2 (Google/GitHub), API keys và RBAC roles.  
> **Storage**: PostgreSQL (users, sessions, API keys, role assignments), Redis (rate limiting, session blacklist)  
> **Go package**: `services/identity-service/internal/domain/entity`, `domain/identity`  
> **Cập nhật:** 2026-06-24 — Thêm PlatformSetting, UserInvitation, RBACRoleMeta (TASK-HC-009, HC-010, HC-014)

---

## 1. User

Entity xác thực trung tâm — mỗi người dùng trong hệ thống.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `email` | string | No | Unique, login identifier chính |
| `username` | string | No | Unique, dùng cho display |
| `hashed_password` | string | Yes | nil khi dùng OAuth-only |
| `role` | string | No | Global role: `admin` \| `user` \| `readonly` \| `agent` |
| `auth_provider` | AuthProvider | No | Cơ chế xác thực |
| `mfa_enabled` | bool | No | TOTP 2FA đang kích hoạt |
| `mfa_totp_secret` | *string | Yes | TOTP secret mã hóa AES-256-GCM; nil khi MFA off |
| `is_active` | bool | No | false = không được đăng nhập |
| `is_verified` | bool | No | Email đã được xác nhận |
| `failed_login_attempts` | int | No | Số lần đăng nhập sai liên tiếp |
| `last_login_at` | *timestamp | Yes | Lần đăng nhập thành công cuối |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums — AuthProvider**:

| Giá trị | Mô tả |
|---------|-------|
| `local` | Email/password |
| `google` | OAuth2 Google |
| `github` | OAuth2 GitHub |

---

## 2. Session

Refresh token session với replay attack detection qua token families.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `user_id` | UUID | No | FK → User |
| `refresh_token_hash` | string | No | SHA-256(refreshToken) — raw token không bao giờ lưu |
| `token_family` | string | No | UUID nhóm các refresh tokens liên quan; nếu revoked token trong family bị dùng lại → revoke cả family |
| `ip_address` | string | Yes | IP client lúc tạo session |
| `user_agent` | string | Yes | Browser/app user agent |
| `expires_at` | timestamp | No | Thời điểm hết hạn (thường 7 ngày) |
| `revoked_at` | *timestamp | Yes | nil = session còn hoạt động |
| `created_at` | timestamp | No | |

---

## 3. APIKey

Long-lived credential cho agent và automation access.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `user_id` | UUID | No | FK → User |
| `name` | string | No | Human-readable label |
| `key_hash` | string | No | hex(sha256(fullKey)) — không bao giờ lưu raw key |
| `prefix` | string | No | 12 ký tự đầu của key, e.g. `ovs_Ab3xYz9q` (dùng để lookup) |
| `permissions` | []string | Yes | Legacy permissions list |
| `scopes` | []string | No | Granular scopes (xem bảng dưới) |
| `created_at` | timestamp | No | |
| `last_used_at` | *timestamp | Yes | |
| `expires_at` | *timestamp | Yes | nil = không hết hạn |
| `revoked_at` | *timestamp | Yes | nil = còn hoạt động |

**Scopes hợp lệ**:

| Scope | Mô tả |
|-------|-------|
| `cve:read` | Đọc CVE data |
| `cve:write` | Ghi CVE data |
| `search:read` | Tìm kiếm CVE |
| `search:manage` | Quản lý search (xóa history) |
| `ingest:admin` | Trigger data ingestion |
| `finding:read` | Đọc findings |
| `finding:write` | Ghi findings |
| `scan:execute` | Chạy scans |
| `scan:manage` | Quản lý scan schedules |
| `ranking:read` | Đọc CPE rankings |
| `ranking:write` | Quản lý CPE rankings |
| `admin:*` | Full admin (wildcard) |

**Scope matching rules**:
1. Exact match: `scope == requested`
2. Admin wildcard: `admin:*` grants everything
3. Resource wildcard: `cve:*` grants `cve:read` và `cve:write`

---

## 4. OAuthAccount

Liên kết external OAuth identity với user.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `user_id` | UUID | No | FK → User |
| `provider` | string | No | `google` \| `github` |
| `provider_id` | string | No | External provider's user ID |
| `email` | string | No | Email từ OAuth provider |
| `name` | string | Yes | Display name từ provider |
| `avatar_url` | string | Yes | Avatar URL |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

---

## 5. RoleID (RBAC)

Numeric role IDs tương thích với Django DefectDojo RBAC.  
Package: `domain/entity` (role.go)

| RoleID | Tên | Mô tả |
|--------|-----|-------|
| 1 | `API Importer` | Import scan results |
| 2 | `Writer` | Tạo và sửa findings/engagements |
| 3 | `Maintainer` | Bao gồm Writer + risk acceptance, user management |
| 4 | `Owner` | Full product management |
| 5 | `Reader` | Read-only access |

**Permission matrix**:

| Permission | Reader | API Importer | Writer | Maintainer | Owner |
|-----------|--------|--------------|--------|------------|-------|
| `product:view` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `product:add/edit/delete` | | | | | ✓ |
| `engagement:view` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `engagement:add/edit` | | | ✓ | ✓ | ✓ |
| `finding:view` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `finding:add` | | ✓ | ✓ | ✓ | ✓ |
| `finding:edit/close` | | | ✓ | ✓ | ✓ |
| `finding:delete` | | | | ✓ | ✓ |
| `import:scan_result` | | ✓ | ✓ | ✓ | ✓ |
| `risk_acceptance:*` | | | | ✓ | ✓ |
| `user:view` | | | | ✓ | ✓ |
| `user:add/edit` | | | | | ✓ |
| `system:configure` | | | | | ✓ |
| `report:download` | ✓ | | ✓ | ✓ | ✓ |

---

## 6. RoleAssignment

Gán role cho user theo scope global hoặc product-level.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `user_id` | UUID | No | FK → User |
| `role_id` | int | No | FK → RoleID |
| `scope` | string | No | `global` \| `product` |
| `resource_id` | *UUID | Yes | nil khi scope = `global`; ProductID khi scope = `product` |
| `assigned_by` | UUID | No | FK → User (người gán) |

---

## 7. UserCreateInput & UserCreateResult

Bulk create users cho admin operations (SEED-001).

**UserCreateInput**:

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `email` | string | |
| `username` | string | |
| `password` | string | Plaintext — hashed bcrypt cost=12 bởi use case |
| `role` | string | `admin` \| `user` \| `readonly` \| `agent` |
| `is_active` | bool | |
| `is_verified` | bool | |

**UserCreateResult**:

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `email` | string | |
| `status` | string | `created` \| `error` |
| `id` | *UUID | UUID nếu created |
| `message` | string | Error message nếu failed |

---

## 8. PlatformSetting *(NEW — TASK-HC-009)*

Key-value store cho cấu hình nền tảng. Đọc/ghi từ bảng `platform_settings` — không còn hardcode.

> **Table:** `platform_settings`  
> **Migration:** `migrations/004_platform_settings.sql`  
> **API:** `GET /api/v1/admin/settings`, `PUT /api/v1/admin/settings`, `PATCH /api/v1/admin/settings`

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `key` | string | No | Unique config key, e.g. `max_scan_concurrent` |
| `value` | JSONB | No | Giá trị (string, number, object, array) |
| `description` | string | Yes | Mô tả setting |
| `updated_by` | UUID | Yes | FK → User (người cập nhật cuối) |
| `updated_at` | timestamp | No | |

**Ví dụ keys phổ biến**:

| Key | Type | Mô tả |
|-----|------|-------|
| `max_scan_concurrent` | int | Số scans chạy song song tối đa |
| `default_scan_timeout` | int | Timeout scan (giây) |
| `ai_enrichment_enabled` | bool | Bật/tắt AI enrichment tự động |
| `smtp_from_address` | string | Địa chỉ email gửi |
| `invitation_expiry_hours` | int | Thời gian hết hạn invitation token |
| `session_max_devices` | int | Số sessions tối đa mỗi user |

---

## 9. UserInvitation *(NEW — TASK-HC-014)*

Invitation token để mời người dùng mới vào hệ thống. Single-use, expire sau 48h.

> **Table:** `user_invitations`  
> **Migration:** `migrations/006_user_invitations.sql`  
> **API:** `POST /api/v1/admin/users/invite`, `GET /api/v1/auth/accept-invite?token=...`

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `user_id` | UUID | No | FK → User (user được mời, `is_active=false` cho đến khi accept) |
| `token` | string | No | Unique secure random token (32 bytes hex, crypto/rand) |
| `email` | string | No | Email người được mời |
| `invited_by` | UUID | Yes | FK → User (admin gửi invite) |
| `expires_at` | timestamp | No | Hết hạn sau 48 giờ kể từ tạo |
| `accepted_at` | *timestamp | Yes | nil = chưa accept; non-nil = đã activate |
| `created_at` | timestamp | No | |

**Flow**:
1. Admin gọi `POST /api/v1/admin/users/invite` → User tạo với `is_active=false`
2. Nếu SMTP cấu hình: Email gửi với link `GET /api/v1/auth/accept-invite?token=<token>`
3. Nếu SMTP chưa cấu hình: Invitation vẫn tạo, chỉ log warning
4. User click link → `is_active=true`, `is_verified=true`, `accepted_at` được set

---

## 10. RBACRoleMeta & RBACPermissionCategory *(NEW — TASK-HC-010)*

Metadata cho roles và permission categories hiển thị trong UI RBAC matrix. Thay thế hardcode static maps trong `GetRBACMatrix` handler.

> **Tables:** `rbac_roles`, `rbac_permission_categories`, `rbac_category_permissions`  
> **Migration:** `migrations/005_rbac_roles_metadata.sql`  
> **API:** `GET /api/v1/admin/roles`

**RBACRoleMeta** (`rbac_roles` table):

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | serial | No | Auto-increment PK |
| `name` | string | No | Unique identifier, e.g. `admin`, `user`, `readonly`, `agent` |
| `display_name` | string | No | UI label, e.g. `Administrator` |
| `description` | string | No | Mô tả quyền hạn |
| `color` | string | No | Hex color cho UI badge, e.g. `#8B5CF6` |
| `is_system` | bool | No | `true` = role hệ thống, không thể xóa |
| `created_at` | timestamp | No | |

**RBACPermissionCategory** (`rbac_permission_categories` + `rbac_category_permissions`):

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | serial | No | |
| `category` | string | No | Unique, e.g. `Dashboard`, `Scanning` |
| `sort_order` | int | No | Thứ tự hiển thị trong RBAC matrix UI |
| `permissions` | []string | — | Aggregated từ `rbac_category_permissions` join |

**Default permission categories (seed)**:
`Dashboard`, `Scanning`, `Findings`, `Reports`, `AI Center`, `Administration`, `Agent`

---

## 11. Relationships *(Updated)*

```
User ──────────── Session (1:N, via user_id)
User ──────────── APIKey (1:N, via user_id)
User ──────────── OAuthAccount (1:N, via user_id)
User ──────────── RoleAssignment (1:N, global or product-scoped)
User ──────────── UserInvitation (1:1, invited user; accepted_at nil until clicked)
Session ────────── Token family (N:1 grouping, for replay attack detection)
RBACRoleMeta ─── RBACPermissionCategory (via rbac_category_permissions join)
PlatformSetting ─ (standalone KV store, no FK)
```

---

## 12. Security Notes

- **Refresh tokens**: Chỉ hash (SHA-256) được lưu, raw token chỉ trả về 1 lần
- **API keys**: Chỉ prefix + hash lưu, full key chỉ hiển thị lúc tạo
- **TOTP secrets**: Mã hóa AES-256-GCM trước khi lưu DB
- **Token rotation**: Mỗi refresh tạo token mới + revoke token cũ trong cùng family
- **Replay detection**: Nếu revoked token bị dùng lại → revoke toàn bộ family
- **Invitation tokens**: 32 bytes `crypto/rand` hex; expire 48h; single-use (`accepted_at` set on use)
