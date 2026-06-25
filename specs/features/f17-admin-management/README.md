# F17 — Administration & System Management

> **Spec Folder:** `specs/features/f17-admin-management/`  
> **Feature Doc:** [`docs/features/F17-admin-management.md`](../../../docs/features/F17-admin-management.md)  
> **Status:** ✅ v2.0 Implemented

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | User management, LDAP config, system config, data management |
| [dataflow.md](./dataflow.md) | Admin CRUD flows, system health, data purge flows |

---

## Services

| Service | Port | Role |
|---------|------|------|
| `identity-service` | 8081 | User CRUD, LDAP config, API key admin |
| `data-service` | 8082 | CVE source config, sync control |
| `finding-service` | 8085 | Product type admin, system settings |
| All services | — | Health endpoints, metrics |

---

## Admin Operations

| Category | Operations |
|----------|-----------|
| **User Management** | Create/edit/delete users, change roles, force logout |
| **LDAP Config** | Add/test/edit LDAP server configs and group mappings |
| **API Keys** | View all users' API keys, revoke any key |
| **CVE Sources** | Enable/disable fetchers, trigger manual sync, view sync status |
| **System Config** | SLA defaults, notification settings, rate limits |
| **Data Management** | Purge old findings, force re-index, vacuum DB |

---

## Quick Reference: API Endpoints

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET/POST | `/api/v1/admin/users` | User management |
| PATCH | `/api/v1/admin/users/{id}` | Edit user (role, active) |
| DELETE | `/api/v1/admin/users/{id}` | Deactivate user |
| GET/POST | `/api/v1/admin/ldap` | LDAP configs |
| POST | `/api/v1/admin/ldap/{id}/test` | Test LDAP connection |
| GET | `/api/v1/admin/api-keys` | All API keys |
| DELETE | `/api/v1/admin/api-keys/{id}` | Revoke any API key |
| POST | `/api/v1/admin/sync/{source}` | Trigger manual CVE sync |
| GET | `/api/v1/admin/sync/status` | All source sync status |
| GET | `/api/v1/admin/health` | System-wide health |
| POST | `/api/v1/admin/data/reindex` | Trigger OpenSearch reindex |
