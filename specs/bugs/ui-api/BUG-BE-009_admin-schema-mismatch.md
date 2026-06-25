# BUG-BE-009 — Admin API Response Schema Sai (health, users, roles)

| Trường | Giá trị |
|---|---|
| **ID** | BUG-BE-009 |
| **Severity** | 🟡 Medium |
| **Priority** | P1 |
| **Component** | Backend / API Gateway / Admin Handlers |
| **Endpoints** | `GET /api/v1/admin/health`, `GET /api/v1/admin/users`, `GET /api/v1/admin/roles` |
| **Phát hiện** | 2026-06-19 |
| **Server** | https://c12.openledger.vn |
| **Status** | Open |

---

## Mô tả

Ba Admin endpoints trả về HTTP 200 nhưng có **sai lệch schema** so với OpenAPI spec:
1. `/admin/health`: `services` là **array** thay vì **object/map**
2. `/admin/users`: User object thiếu field `name` và `mfa_enabled`
3. `/admin/roles`: Role object dùng tên field khác spec

## Chi Tiết Sai Lệch

### 1. `GET /api/v1/admin/health` — `services` phải là object

**Actual:**
```json
{
  "status": "healthy",
  "services": [
    { "name": "postgres", "status": "up" },
    { "name": "redis", "status": "up" }
  ]
}
```

**Expected (theo spec):**
```json
{
  "status": "healthy",
  "services": {
    "postgres": { "status": "up", "latency_ms": 2 },
    "redis":    { "status": "up", "latency_ms": 1 },
    "elasticsearch": { "status": "up" }
  }
}
```

### 2. `GET /api/v1/admin/users` — Thiếu `name`, `mfa_enabled`

**Actual user object:**
```json
{
  "id": "uuid",
  "email": "admin@openvulnscan.io",
  "role": "admin",
  "is_active": true,
  "created_at": "..."
}
```

**Expected (theo spec):**
```json
{
  "id": "uuid",
  "email": "admin@openvulnscan.io",
  "name": "Admin User",          ← THIẾU
  "role": "admin",
  "is_active": true,
  "mfa_enabled": false,          ← THIẾU
  "created_at": "..."
}
```

### 3. `GET /api/v1/admin/roles` — Field names sai

**Actual role object:**
```json
{
  "role_name": "admin",
  "perms": ["scan:create", "user:manage"]
}
```

**Expected (theo spec):**
```json
{
  "name": "admin",               ← key phải là "name"
  "permissions": ["scan:create"] ← key phải là "permissions"
}
```

## Fix

### health handler
```go
// Chuyển services array → map
servicesMap := make(map[string]ServiceStatus)
for _, svc := range services {
    servicesMap[svc.Name] = svc
}
c.JSON(200, gin.H{"status": status, "services": servicesMap})
```

### users handler
Thêm `name` và `mfa_enabled` vào User response DTO:
```go
type AdminUserResponse struct {
    ID         string `json:"id"`
    Email      string `json:"email"`
    Name       string `json:"name"`        // ← thêm
    Role       string `json:"role"`
    IsActive   bool   `json:"is_active"`
    MFAEnabled bool   `json:"mfa_enabled"` // ← thêm
    CreatedAt  string `json:"created_at"`
}
```

### roles handler
Đổi JSON tags trong Role DTO:
```go
type RoleResponse struct {
    Name        string   `json:"name"`        // thay vì "role_name"
    Permissions []string `json:"permissions"` // thay vì "perms"
}
```

## Ảnh hưởng

- Admin System Health panel hiển thị sai (array thay vì status map)
- User Management thiếu Name và MFA status column
- Role Management display sai field names
