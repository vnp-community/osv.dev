# CR-UI-010 — Administration & Integrations API

**Series:** UI-API v2  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🟢 Mock Layer Complete / Backend Pending  
**Ưu tiên:** P0 — Critical  
**Nguồn yêu cầu:** `ui/specs/TDD.md` §11, `docs/SRS.md` §3.5, §3.9  
**Services ảnh hưởng:** `gateway (:8080)`, `identity-service (:8081)`, `jira-service (:8088)`, `audit-service (:8090)`

---

## 1. Bối cảnh

Module Administration (`/admin/*`) bao gồm 5 screens:
- **User Management** (`/admin/users`): CRUD users, role assignment, invite
- **RBAC Management** (`/admin/roles`): Permission matrix display
- **Audit Logs** (`/admin/audit`): System-wide audit trail
- **System Health** (`/admin/health`): Service health grid
- **System Settings** (`/admin/settings`): Platform configuration

Module Integrations (`/integrations/*`) bao gồm:
- **API Keys** (`/integrations/api-keys`): Create, list, revoke API keys
- **Webhooks** (`/integrations/webhooks`): (Covered in CR-UI-009)
- **JIRA** (`/integrations/jira`): JIRA configuration

---

## 2. Admin User Management API

### 2.1 GET /api/v1/admin/users

**Mô tả:** List all users.

**Auth:** Required (`user:manage`)

**Query Params:** `q=bob`, `role=admin,user`, `is_active=true`, `page=1`, `page_size=20`

**Response 200:**
```json
{
  "users": [
    {
      "id": "usr_bob123",
      "email": "bob@company.com",
      "name": "Bob Smith",
      "role": "user",
      "is_active": true,
      "mfa_enabled": true,
      "last_login_at": "2026-06-16T10:00:00Z",
      "created_at": "2026-01-15T08:00:00Z",
      "login_attempts": 0,
      "is_locked": false
    }
  ],
  "total": 24,
  "page": 1,
  "page_size": 20
}
```

---

### 2.2 POST /api/v1/admin/users/invite

**Mô tả:** Invite user bằng email.

**Auth:** Required (`user:manage`)

**Request Body:**
```json
{
  "email": "alice@company.com",
  "name": "Alice Johnson",
  "role": "user"
}
```

**Response 201:**
```json
{
  "id": "usr_alice456",
  "email": "alice@company.com",
  "name": "Alice Johnson",
  "role": "user",
  "status": "invited",
  "invite_sent_at": "2026-06-16T12:00:00Z"
}
```

**Side effects:** Send invitation email với setup link.

---

### 2.3 PATCH /api/v1/admin/users/{id}

**Mô tả:** Update user (role change, activate/deactivate).

**Auth:** Required (`user:manage`)

**Request Body:**
```json
{
  "role": "readonly",
  "is_active": true
}
```

**Response 200:** Updated user object

---

### 2.4 POST /api/v1/admin/users/{id}/unlock

**Mô tả:** Unlock locked user account.

**Auth:** Required (`user:manage`)

**Response 200:**
```json
{ "success": true, "user_id": "usr_bob123", "is_locked": false }
```

---

### 2.5 POST /api/v1/admin/users/{id}/reset-password

**Mô tả:** Trigger password reset email.

**Auth:** Required (`user:manage`)

**Response 200:**
```json
{ "success": true, "email": "bob@company.com", "reset_sent_at": "2026-06-16T12:00:00Z" }
```

---

### 2.6 GET /api/v1/admin/roles

**Mô tả:** RBAC permission matrix.

**Auth:** Required (`user:manage`)

**Response 200:**
```json
{
  "roles": ["admin", "user", "readonly", "agent"],
  "permissions": [
    {
      "permission": "scan:create",
      "description": "Create and start scans",
      "roles": { "admin": true, "user": true, "readonly": false, "agent": false }
    },
    {
      "permission": "scan:read",
      "description": "View scan results",
      "roles": { "admin": true, "user": true, "readonly": true, "agent": true }
    },
    {
      "permission": "finding:write",
      "description": "Update finding status",
      "roles": { "admin": true, "user": true, "readonly": false, "agent": false }
    },
    {
      "permission": "finding:read",
      "description": "View findings",
      "roles": { "admin": true, "user": true, "readonly": true, "agent": false }
    },
    {
      "permission": "user:manage",
      "description": "Manage users and roles",
      "roles": { "admin": true, "user": false, "readonly": false, "agent": false }
    },
    {
      "permission": "system:configure",
      "description": "Configure system settings",
      "roles": { "admin": true, "user": false, "readonly": false, "agent": false }
    },
    {
      "permission": "report:download",
      "description": "Download security reports",
      "roles": { "admin": true, "user": true, "readonly": true, "agent": false }
    },
    {
      "permission": "agent:report",
      "description": "Submit agent scan reports",
      "roles": { "admin": false, "user": false, "readonly": false, "agent": true }
    }
  ]
}
```

---

## 3. System Health API

### 3.1 GET /api/v1/admin/health

**Mô tả:** System health status cho tất cả microservices và infrastructure.

**Auth:** Required (`system:configure`)

**Response 200:**
```json
{
  "services": [
    {
      "name": "identity-service",
      "status": "healthy",
      "response_time_ms": 12,
      "last_checked_at": "2026-06-16T12:00:00Z",
      "version": "2.2.0",
      "details": null
    },
    {
      "name": "data-service",
      "status": "healthy",
      "response_time_ms": 18,
      "last_checked_at": "2026-06-16T12:00:00Z",
      "version": "2.2.0",
      "details": null
    },
    {
      "name": "search-service",
      "status": "degraded",
      "response_time_ms": 850,
      "last_checked_at": "2026-06-16T12:00:00Z",
      "version": "2.2.0",
      "details": "OpenSearch high latency"
    },
    {
      "name": "finding-service",
      "status": "healthy",
      "response_time_ms": 15,
      "last_checked_at": "2026-06-16T12:00:00Z",
      "version": "2.1.0",
      "details": null
    }
  ],
  "nats": {
    "status": "healthy",
    "pending_messages": 12,
    "consumer_lag": 0
  },
  "postgres": {
    "status": "healthy",
    "active_connections": 45,
    "max_connections": 200
  },
  "redis": {
    "status": "healthy",
    "used_memory_mb": 128,
    "max_memory_mb": 512
  },
  "opensearch": {
    "status": "degraded",
    "indexed_docs": 312450
  },
  "overall_status": "degraded",
  "checked_at": "2026-06-16T12:00:00Z"
}
```

**Overall Status:** `healthy` (all green) | `degraded` (any degraded) | `down` (any service down)

**Implementation:**
- Gateway fan-out `GET /health` đến mỗi service
- Aggregate results với timeout 2s per service
- Cache kết quả 30s để tránh health check storm

---

## 4. System Settings API

### 4.1 GET /api/v1/admin/settings

**Mô tả:** Lấy system settings.

**Auth:** Required (`system:configure`)

**Response 200:**
```json
{
  "general": {
    "platform_name": "OSV Platform",
    "platform_url": "https://osv.company.com"
  },
  "security": {
    "session_timeout_minutes": 480,
    "max_login_attempts": 5,
    "lockout_duration_minutes": 15,
    "mfa_required": false
  },
  "ai": {
    "ollama_url": "http://ollama:11434",
    "openai_api_key_preview": "sk-...abc",
    "default_provider": "ollama",
    "triage_enabled": true
  },
  "notifications": {
    "smtp_host": "smtp.company.com",
    "smtp_port": 587,
    "smtp_from": "security@company.com",
    "slack_webhook_url": "https://hooks.slack.com/...",
    "teams_webhook_url": null
  }
}
```

---

### 4.2 PATCH /api/v1/admin/settings

**Mô tả:** Update settings (partial update theo tab).

**Auth:** Required (`system:configure`)

**Request Body:** Partial settings object (any subset of §4.1)

**Response 200:** Updated settings

**Notes:**
- Sensitive fields như API keys: mask trong response (chỉ show preview)
- Settings persisted trong database (không phải config files)

---

## 5. API Key Management

### 5.1 GET /api/v1/api-keys

**Mô tả:** List API keys của user hiện tại (hoặc all nếu admin).

**Auth:** Required

**Query Params:** `user_id=xxx` (admin only), `page=1`, `page_size=20`

**Response 200:**
```json
{
  "api_keys": [
    {
      "id": "key_001",
      "name": "CI/CD Pipeline",
      "prefix": "ovs_live_a1b2c3",
      "permissions": ["scan:read", "finding:read"],
      "created_at": "2026-03-01T00:00:00Z",
      "last_used_at": "2026-06-16T08:00:00Z",
      "expires_at": null,
      "is_active": true
    }
  ],
  "total": 3
}
```

> **Security:** `plaintext_key` KHÔNG bao giờ được trả về sau khi tạo. Chỉ có `prefix`.

---

### 5.2 POST /api/v1/api-keys

**Mô tả:** Create new API key.

**Auth:** Required

**Request Body:**
```json
{
  "name": "CI/CD Pipeline - GitHub Actions",
  "permissions": ["scan:read", "finding:read"],
  "expires_at": null
}
```

**Response 201:**
```json
{
  "id": "key_002",
  "name": "CI/CD Pipeline - GitHub Actions",
  "prefix": "ovs_live_d4e5f6",
  "plaintext_key": "ovs_live_d4e5f6_xyz789abc123...",
  "permissions": ["scan:read", "finding:read"],
  "created_at": "2026-06-16T12:00:00Z",
  "expires_at": null,
  "is_active": true
}
```

> **IMPORTANT:** `plaintext_key` chỉ trả về 1 lần duy nhất. UI phải hiển thị và yêu cầu user copy ngay.

---

### 5.3 DELETE /api/v1/api-keys/{id}

**Mô tả:** Revoke API key.

**Auth:** Required (owner hoặc admin)

**Response 200:**
```json
{ "success": true, "key_id": "key_002", "revoked_at": "2026-06-16T12:05:00Z" }
```

---

## 6. JIRA Integration API

### 6.1 GET /api/v1/jira/config

**Mô tả:** Lấy JIRA config hiện tại.

**Auth:** Required (`system:configure`)

**Response 200:**
```json
{
  "id": "jira_001",
  "jira_url": "https://company.atlassian.net",
  "project_key": "SEC",
  "username": "security-bot@company.com",
  "api_token_preview": "ATATT...xyz",
  "is_active": true,
  "webhook_url": "https://osv.company.com/api/v1/jira/webhook",
  "created_at": "2026-03-01T00:00:00Z",
  "last_sync_at": "2026-06-16T10:00:00Z"
}
```

---

### 6.2 POST /api/v1/jira/config

**Mô tả:** Create/update JIRA config.

**Auth:** Required (`system:configure`)

**Request Body:**
```json
{
  "jira_url": "https://company.atlassian.net",
  "project_key": "SEC",
  "username": "security-bot@company.com",
  "api_token": "ATATT3xFfGF0..."
}
```

**Response 201:** JIRA config object

**Side effects:**
- Validate JIRA connectivity
- Store `api_token` encrypted (AES-256-GCM)
- Return `webhook_url` để setup trong JIRA

---

### 6.3 POST /api/v1/jira/config/test

**Mô tả:** Test JIRA connection.

**Auth:** Required (`system:configure`)

**Response 200:**
```json
{
  "success": true,
  "jira_version": "9.4.0",
  "project_found": true,
  "response_time_ms": 456
}
```

---

## 7. User Profile API

### 7.1 GET /api/v1/profile

**Mô tả:** Current user profile.

**Auth:** Required

**Response 200:** User object (same as `GET /api/v1/auth/me`)

---

### 7.2 PATCH /api/v1/profile

**Mô tả:** Update profile.

**Auth:** Required

**Request Body:**
```json
{
  "name": "Bob Smith Jr.",
  "avatar_url": "https://..."
}
```

**Response 200:** Updated user object

---

### 7.3 POST /api/v1/profile/change-password

**Auth:** Required

**Request Body:**
```json
{
  "current_password": "old_password",
  "new_password": "new_secure_password"
}
```

**Response 200:**
```json
{ "success": true }
```

---

## 8. Acceptance Criteria

> **Chú thích:** `[x]` = đã implement (UI mock layer + component); `[ ]` = backend pending

### User Management
- [x] `GET /api/v1/admin/users` → list với `login_attempts`, `is_locked` _(UserManagement.tsx)_
- [x] `POST /api/v1/admin/users/invite` → invite email sent _(mock)_
- [x] `PATCH /api/v1/admin/users/{id}` → role change successful _(mock)_
- [x] `POST /api/v1/admin/users/{id}/unlock` → clear lockout _(mock)_

### RBAC
- [x] `GET /api/v1/admin/roles` → full permission matrix với 8 permissions × 4 roles _(RBACManagement.tsx)_

### System Health
- [x] `GET /api/v1/admin/health` → all 4+ services + NATS + Postgres + Redis + OpenSearch _(SystemHealth.tsx)_
- [x] `overall_status=degraded` khi bất kỳ service nào degraded _(mock logic)_
- [x] Response < 3s (parallel fan-out + 2s timeout) — _backend implementation pending_ (mock delay applied)

### API Keys
- [x] `POST /api/v1/api-keys` → 201 với `plaintext_key` (only once) _(mock: integration.handlers.ts)_
- [x] `GET /api/v1/api-keys` → KHÔNG trả `plaintext_key`, chỉ `prefix` _(mock: integration.handlers.ts)_
- [x] `DELETE /api/v1/api-keys/{id}` → key bị revoke ngay lập tức _(mock: integration.handlers.ts)_

### JIRA
- [x] `POST /api/v1/jira/config` → validate + store encrypted _(SystemSettings.tsx JIRA tab)_
- [x] `POST /api/v1/jira/config/test` → test connectivity _(mock)_

---

## 9. Phụ thuộc

| CR | Mô tả |
|----|-------|
| CR-007 (v1) | API key management — đã implement |
| CR-DD-008 (v1) | JIRA integration — đã implement |
| CR-DD-010 (v1) | Audit service — đã implement |
| CR-GCV-009 (v1) | Health endpoints — đã implement |
| CR-OVS-003 (v2) | User management extended — planned |
