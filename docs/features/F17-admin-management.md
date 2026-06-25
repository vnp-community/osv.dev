# F17 — Administration & System Management

**Status:** 🔶 Planned v3.1 (CR-UI-010)  
**CR References:** CR-UI-010, CR-007 (cve-search)  
**Services:** `identity-service`, `gateway-service`, `jira-service`, `audit-service`  
**UI Routes:** `/admin/users`, `/admin/roles`, `/admin/audit`, `/admin/health`, `/admin/settings`, `/integrations/api-keys`  
**UI Components:** `UserManagement`, `RBACManagement`, `AuditLogs`, `SystemHealth`, `SystemSettings`, `APIKeyManagement`

---

## 1. Mô tả

Administration Center cung cấp toàn bộ công cụ quản trị hệ thống: User Management, RBAC, System Health monitoring, Settings, và API Key management. Dành cho role `admin`.

---

## 2. User Management

**Route:** `/admin/users`  
**Component:** `UserManagement`

### 2.1 Features
- Danh sách users với role, status, last_login
- Invite user qua email (gửi invitation link)
- Lock/unlock user account
- Reset password (send reset email)
- Change user role
- View user activity (last login, API key usage)

### 2.2 APIs
```
GET /api/v1/admin/users                → List all users (paginated)
POST /api/v1/admin/users/invite        → Invite new user
GET /api/v1/admin/users/{id}           → User detail
PATCH /api/v1/admin/users/{id}         → Update role, lock/unlock
DELETE /api/v1/admin/users/{id}        → Deactivate user
POST /api/v1/admin/users/{id}/reset-pw → Trigger password reset
```

### 2.3 User Entity
```json
{
  "id": "user-001",
  "email": "bob@company.com",
  "name": "Bob Smith",
  "role": "user",
  "status": "active",
  "mfa_enabled": false,
  "last_login": "2026-06-18T08:00:00Z",
  "created_at": "2026-01-01T00:00:00Z",
  "api_key_count": 2,
  "ldap_user": false
}
```

---

## 3. RBAC Management

**Route:** `/admin/roles`  
**Component:** `RBACManagement`

### 3.1 Roles Overview

| Role | Description | Scope |
|------|-------------|-------|
| `admin` | Full system access | Platform-wide |
| `user` | Standard security analyst | Platform-wide |
| `readonly` | Read-only access | Platform-wide |

### 3.2 Permission Matrix

| Permission | admin | user | readonly |
|-----------|-------|------|----------|
| `finding:write` | ✅ | ✅ | ❌ |
| `finding:read` | ✅ | ✅ | ✅ |
| `scan:read` | ✅ | ✅ | ✅ |
| `scan:write` | ✅ | ✅ | ❌ |
| `report:download` | ✅ | ✅ | ✅ |
| `report:generate` | ✅ | ✅ | ❌ |
| `admin:users` | ✅ | ❌ | ❌ |
| `admin:settings` | ✅ | ❌ | ❌ |
| `webhook:manage` | ✅ | ✅ | ❌ |

### 3.3 Product-level RBAC
Xem [F06 — Product Security](./F06-product-security.md) §5 cho product-level roles.

---

## 4. API Key Management

**Route:** `/integrations/api-keys`  
**Component:** `APIKeyManagement`

### 4.1 Features
- List tất cả API keys của user (masked)
- Tạo key mới với name + scopes
- Revoke key
- View usage stats (last used, request count)

### 4.2 API Key Format
```
osv_{base58_random_32_bytes}
```

### 4.3 Available Scopes
| Scope | Mô tả |
|-------|-------|
| `cve:read` | Đọc CVE data |
| `finding:read` | Đọc findings |
| `finding:write` | Tạo/update findings |
| `scan:read` | Xem scans |
| `scan:write` | Tạo scan |
| `report:download` | Download reports |
| `agent:report` | Dùng cho remote agents |

### 4.4 APIs
```
GET /api/v1/api-keys               → List user's API keys (masked)
POST /api/v1/api-keys              → Create new API key (returns plaintext ONCE)
DELETE /api/v1/api-keys/{id}       → Revoke API key
GET /api/v1/api-keys/{id}/usage    → Usage stats
```

### 4.5 Security
- SHA-256 stored (plaintext key shown ONCE at creation)
- Không thể recover sau khi đóng dialog
- Rate limit: 5 API keys per user

---

## 5. System Health Dashboard

**Route:** `/admin/health`  
**Component:** `SystemHealth`

### 5.1 Health Fan-out API
```
GET /api/v1/admin/health
```

Gateway fan-out đến `/health` endpoint của tất cả microservices:

**Response:**
```json
{
  "timestamp": "2026-06-18T10:00:00Z",
  "overall_status": "degraded",
  "services": {
    "gateway": {"status": "healthy", "latency_ms": 2},
    "identity-service": {"status": "healthy", "latency_ms": 5},
    "data-service": {"status": "healthy", "latency_ms": 15},
    "search-service": {"status": "degraded", "latency_ms": 850, "reason": "OpenSearch slow"},
    "finding-service": {"status": "healthy", "latency_ms": 8},
    "sla-service": {"status": "healthy", "latency_ms": 6},
    "notification-service": {"status": "healthy", "latency_ms": 12},
    "jira-service": {"status": "healthy", "latency_ms": 20},
    "audit-service": {"status": "healthy", "latency_ms": 9}
  },
  "infrastructure": {
    "postgresql": {"status": "healthy", "pool_used": 45, "pool_max": 100},
    "redis": {"status": "healthy", "memory_used_mb": 512},
    "opensearch": {"status": "degraded", "cluster_health": "yellow"},
    "nats": {"status": "healthy", "pending_messages": 0}
  }
}
```

### 5.2 UI Features
- Status badges (green/yellow/red) per service
- Response time graphs
- Infrastructure stats
- Auto-refresh mỗi 30 giây

---

## 6. System Settings

**Route:** `/admin/settings`  
**Component:** `SystemSettings`

### 6.1 Setting Categories

**Authentication Settings:**
- JWT TTL (default: 15 min)
- Max login attempts (default: 5)
- Account lockout duration (default: 15 min)
- LDAP config

**Notification Settings:**
- SMTP config (server, port, from, auth)
- Slack webhook URL
- Teams webhook URL
- Default notification rules

**Integration Settings:**
- JIRA default config
- Webhook global settings (max retries, timeout)

**Data Settings:**
- CVE retention policy
- Embedding provider (Ollama/OpenAI)
- Report storage TTL (default: 90 days)

### 6.2 API
```
GET /api/v1/admin/settings          → Get all settings
PATCH /api/v1/admin/settings        → Update settings
GET /api/v1/admin/settings/{key}    → Get specific setting
```

---

## 7. Audit Log Viewer

**Route:** `/admin/audit`  
**Component:** `AuditLogs`

Xem chi tiết tại [F12 — Audit Trail](./F12-audit-trail.md).

---

## 8. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| Health fan-out | < 2 giây (parallel calls với timeout 1s per service) |
| User list | < 100ms |
| Settings update | < 100ms + validation |
| Access control | Admin-only endpoints enforce via RBAC middleware |
