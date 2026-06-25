# TASK-BE-P1-04 — Fix Admin API Schema (Health, Users, Roles)

**Phase:** Sprint 2 — P1 Schema Fixes  
**Nguồn giải pháp:** [`solutions/SOL-004_fix-schema-mismatches.md — FIX 6, 7, 8`](../solutions/SOL-004_fix-schema-mismatches.md)  
**Ưu tiên:** 🟠 P1 — Admin panel thiếu thông tin  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19

---

## Mục tiêu

- `GET /api/v1/admin/health` — `services` phải là **map** không phải array
- `GET /api/v1/admin/users` — User object thiếu `name` và `mfa_enabled`
- `GET /api/v1/admin/roles` — field names sai (`role_name`→`name`, `perms`→`permissions`)

---

## Files cần sửa

### Part A — Fix Health BFF (gateway)

**File**: [`apps/osv/internal/gateway/bff/health.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/health.go)

```bash
# Xem nội dung health.go hiện tại
cat apps/osv/internal/gateway/bff/health.go
```

Tìm và sửa phần build response trong `HandleAdminHealth`:

```go
// TRƯỚC — services là ARRAY:
services := []map[string]interface{}{
    {"name": "postgres", "status": "up"},
    {"name": "redis", "status": "up"},
}
respondJSON(w, http.StatusOK, map[string]interface{}{
    "status":   "healthy",
    "services": services,  // ← ARRAY
})

// SAU — services phải là MAP:
services := map[string]interface{}{
    "postgres": map[string]interface{}{
        "status":     postgresStatus,   // "up" | "down"
        "latency_ms": postgresLatency,  // int
    },
    "redis": map[string]interface{}{
        "status":     redisStatus,
        "latency_ms": redisLatency,
    },
    "nats": map[string]interface{}{
        "status": natsStatus,
    },
    "elasticsearch": map[string]interface{}{
        "status": esStatus,
    },
}

// Tính overall status
overallStatus := "healthy"
for _, svc := range services {
    if s, ok := svc.(map[string]interface{}); ok {
        if s["status"] != "up" && s["status"] != "ok" {
            overallStatus = "degraded"
            break
        }
    }
}

respondJSON(w, http.StatusOK, map[string]interface{}{
    "status":   overallStatus,
    "services": services,  // ← MAP
})
```

Kiểm tra cách health.go hiện tại ping các services:
```bash
# Xem toàn bộ health.go
cat apps/osv/internal/gateway/bff/health.go
```

### Part B — Fix Admin Users (identity-service)

```bash
# Tìm admin users handler
grep -r "ListUsers\|admin/users\|AdminUsers" \
  services/identity-service/ --include="*.go" -l
```

#### [FIND & MODIFY] Admin users list handler

```go
// Thêm DTO mapping để include name và mfa_enabled:
type AdminUserDTO struct {
    ID         string    `json:"id"`
    Email      string    `json:"email"`
    Name       string    `json:"name"`         // THÊM
    Role       string    `json:"role"`
    IsActive   bool      `json:"is_active"`
    MFAEnabled bool      `json:"mfa_enabled"`  // THÊM
    CreatedAt  string    `json:"created_at"`
}

func toAdminUserDTO(u *domain.User) AdminUserDTO {
    return AdminUserDTO{
        ID:         u.ID.String(),
        Email:      u.Email,
        Name:       u.Username,       // map Username → Name
        Role:       string(u.Role),
        IsActive:   u.IsActive,
        MFAEnabled: u.TOTPEnabled,    // map TOTPEnabled → MFAEnabled
        CreatedAt:  u.CreatedAt.Format(time.RFC3339),
    }
}

// Trong ListUsers handler — apply DTO:
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
    users, total, err := h.userRepo.List(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list users")
        return
    }

    dtos := make([]AdminUserDTO, len(users))
    for i, u := range users {
        dtos[i] = toAdminUserDTO(&u)
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "users":     dtos,
        "total":     total,
        "page":      page,
        "page_size": pageSize,
    })
}
```

### Part C — Fix Admin Roles (identity-service)

```bash
# Tìm roles handler
grep -r "ListRoles\|admin/roles\|role_name\|perms" \
  services/identity-service/ --include="*.go"
```

```go
// Sửa JSON tags trong Role DTO:
type RoleDTO struct {
    Name        string   `json:"name"`        // ĐỔI từ "role_name"
    Permissions []string `json:"permissions"` // ĐỔI từ "perms"
    Description string   `json:"description,omitempty"`
}

// Nếu handler đang build response trực tiếp:
respondJSON(w, http.StatusOK, map[string]interface{}{
    "roles": []RoleDTO{
        {Name: "admin",    Permissions: []string{"*"}},
        {Name: "analyst",  Permissions: []string{"cve:read", "finding:read"}},
        {Name: "operator", Permissions: []string{"scan:create", "finding:write"}},
        {Name: "readonly", Permissions: []string{"cve:read"}},
    },
})
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/admin/health` response: `services` là **object/map**, không phải array
- [ ] `services.postgres`, `services.redis` tồn tại với `status` và `latency_ms`
- [ ] `GET /api/v1/admin/users` — mỗi user có `name` và `mfa_enabled`
- [ ] `GET /api/v1/admin/roles` — mỗi role có `name` (không phải `role_name`) và `permissions` (không phải `perms`)

## Verification

```bash
# Health — services phải là map
curl -H "Authorization: Bearer <admin-token>" \
  https://c12.openledger.vn/api/v1/admin/health | jq '.services | type'
# Expected: "object"
# NOT: "array"

# Health — có key "postgres"
curl -H "Authorization: Bearer <admin-token>" \
  https://c12.openledger.vn/api/v1/admin/health | jq '.services.postgres'
# Expected: { "status": "up", "latency_ms": 2 }

# Users — có name và mfa_enabled
curl -H "Authorization: Bearer <admin-token>" \
  https://c12.openledger.vn/api/v1/admin/users | jq '.users[0] | keys'
# Expected: includes "name", "mfa_enabled"

# Roles — correct field names
curl -H "Authorization: Bearer <admin-token>" \
  https://c12.openledger.vn/api/v1/admin/roles | jq '.roles[0] | keys'
# Expected: ["description", "name", "permissions"]
# NOT: ["perms", "role_name"]
```
