# Change Request 011: Admin — RBAC Matrix & User Management Schema Updates

**Tạo:** 2026-06-19  
**Status:** New — RBAC endpoint cần schema upgrade, AdminUser cần thêm fields.  
**Nguồn:** openapi.yaml schemas `RBACMatrixResponse`, `RBACRole`, `AdminUser`, `AdminUsersResponse`  
**Target directory:** `specs/crs/v2/api-ui-v1/`

---

## 1. Bối cảnh

`AdminRBAC.tsx` sau khi fix hardcode không còn hardcode permission matrix. Thay vào đó, nó gọi `GET /api/v1/admin/roles` để lấy full RBAC matrix (roles + permissions categorized).

Đồng thời, `AdminUser` schema cần bổ sung `login_attempts` và `is_locked` để hiển thị lock status trong User Management UI.

| Thay đổi | Endpoint | Service | Trạng thái |
|---|---|---|---|
| Schema upgrade | `GET /api/v1/admin/roles` → `RBACMatrixResponse` | identity-service | ⚠️ Response schema không đủ |
| Schema update | `AdminUser` — thêm `login_attempts`, `is_locked` | identity-service | ⚠️ Fields thiếu trong response |
| Schema update | `GET /api/v1/admin/users` → `AdminUsersResponse` | identity-service | ⚠️ Response không wrap đúng |
| Schema update | `POST /api/v1/admin/users/invite` → `InviteUserRequest` | identity-service | ⚠️ Request body cần `name` field |

---

## 2. Chi tiết Thay đổi

### 2.1 [HIGH] `GET /api/v1/admin/roles` — RBAC Matrix Response

Frontend cần toàn bộ permission matrix để render bảng checkbox read-only. Endpoint hiện tại trả về:
```json
{ "roles": [{ "name": "admin", "permissions": [...] }] }
```

Cần upgrade lên `RBACMatrixResponse` schema:

```json
{
  "roles": [
    {
      "id": "role-admin",
      "name": "admin",
      "display_name": "Administrator",
      "description": "Full access to all platform features",
      "user_count": 3,
      "color": "#8B5CF6",
      "permissions": ["scan:create", "scan:read", "asset:write", "asset:read", "user:manage", "report:download", "system:configure", "finding:write", "finding:read", "agent:report"]
    },
    {
      "id": "role-user",
      "name": "user",
      "display_name": "Security Analyst",
      "description": "Standard analyst access",
      "user_count": 12,
      "color": "#3B82F6",
      "permissions": ["scan:create", "scan:read", "asset:read", "finding:write", "finding:read", "report:download"]
    },
    {
      "id": "role-readonly",
      "name": "readonly",
      "display_name": "Read-Only Viewer",
      "description": "View-only access to findings and assets",
      "user_count": 5,
      "color": "#6B7280",
      "permissions": ["scan:read", "asset:read", "finding:read"]
    },
    {
      "id": "role-agent",
      "name": "agent",
      "display_name": "Scan Agent",
      "description": "Automated scan agent access",
      "user_count": 2,
      "color": "#10B981",
      "permissions": ["agent:report", "finding:write"]
    }
  ],
  "permission_categories": [
    { "category": "Dashboard", "items": ["scan:read", "finding:read"] },
    { "category": "Scanning", "items": ["scan:create", "scan:read"] },
    { "category": "Findings", "items": ["finding:write", "finding:read"] },
    { "category": "Assets", "items": ["asset:write", "asset:read"] },
    { "category": "Reports", "items": ["report:download"] },
    { "category": "AI Center", "items": ["finding:write"] },
    { "category": "Administration", "items": ["user:manage", "system:configure"] },
    { "category": "Agent", "items": ["agent:report"] }
  ]
}
```

**Identity-service — Handler update:**

```go
// services/identity-service/internal/handler/admin.go
func (h *Handler) GetRBACMatrix(w http.ResponseWriter, r *http.Request) {
    roles, _ := h.repo.GetAllRoles(r.Context())
    
    // Enrich với user_count cho mỗi role
    for i, role := range roles {
        count, _ := h.repo.CountUsersByRole(r.Context(), role.Name)
        roles[i].UserCount = count
    }
    
    json.NewEncoder(w).Encode(RBACMatrixResponse{
        Roles:                 roles,
        PermissionCategories: h.config.PermissionCategories,
    })
}
```

`permission_categories` là **static config** trong identity-service (không cần DB).

---

### 2.2 [HIGH] `AdminUser` — Thêm `login_attempts` và `is_locked`

**DB update** (nếu columns chưa tồn tại):
```sql
-- Trong bảng users của identity-service
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS login_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS is_locked BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS locked_at TIMESTAMPTZ;
```

**Handler update** — đảm bảo 2 fields có trong `AdminUser` response:
```go
// services/identity-service/internal/model/admin_user.go
type AdminUser struct {
    ID            string     `json:"id"`
    Email         string     `json:"email"`
    Name          string     `json:"name"`
    Role          string     `json:"role"`
    IsActive      bool       `json:"is_active"`
    MFAEnabled    bool       `json:"mfa_enabled"`
    CreatedAt     time.Time  `json:"created_at"`
    LastLoginAt   *time.Time `json:"last_login_at"`
    LoginAttempts int        `json:"login_attempts"`  // ← MỚI
    IsLocked      bool       `json:"is_locked"`       // ← MỚI
}
```

**Unlock logic** — endpoint `POST /api/v1/admin/users/{id}/unlock` đã tồn tại nhưng cần thực hiện:
```go
func (h *Handler) UnlockUser(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    err := h.repo.UpdateUser(r.Context(), id, UserUpdate{
        LoginAttempts: 0,
        IsLocked:      false,
        LockedAt:      nil,
    })
    // ...
}
```

**Auto-lock logic** — sau N lần đăng nhập thất bại:
```go
// services/identity-service/internal/service/auth.go
const MaxLoginAttempts = 5

func (s *Service) RecordFailedLogin(ctx context.Context, userID string) error {
    attempts, _ := s.repo.IncrementLoginAttempts(ctx, userID)
    if attempts >= MaxLoginAttempts {
        s.repo.LockUser(ctx, userID)
    }
    return nil
}
```

---

### 2.3 [MEDIUM] `GET /api/v1/admin/users` — AdminUsersResponse Wrapper

Frontend expect response format:
```json
{
  "users": [...],
  "total": 47,
  "page": 1,
  "page_size": 20
}
```

Kiểm tra backend hiện tại có trả đúng format này không. Nếu đang trả array trực tiếp `[...]`, cần wrap.

**Query params được hỗ trợ:**
- `search` — tìm theo email, name
- `role` — filter: `admin | user | readonly | agent`
- `status` — filter: `active | inactive | locked`
- `page`, `page_size`

---

### 2.4 [LOW] `POST /api/v1/admin/users/invite` — Thêm `name` field

Request body cần bổ sung `name`:
```json
{
  "email": "newuser@company.com",
  "name": "Alice Johnson",
  "role": "user"
}
```

Backend hiện tại có thể chỉ yêu cầu `email` và `role`. Cần thêm `name` field và populate vào user record khi create.

---

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `GET /api/v1/admin/roles` trả về `RBACMatrixResponse` với `roles` và `permission_categories` — HTTP 200.
2. Mỗi role có `user_count` phản ánh số users thực tế đang có role đó.
3. `GET /api/v1/admin/users` response có `login_attempts` và `is_locked` trong mỗi `AdminUser`.
4. Sau 5 lần đăng nhập thất bại liên tiếp, `is_locked = true` và user không thể login.
5. `POST /api/v1/admin/users/{id}/unlock` reset `login_attempts = 0`, `is_locked = false`.
6. `GET /api/v1/admin/users` trả về `{ users: [...], total: N, page: P, page_size: PS }`.
7. `GET /api/v1/admin/users?role=admin` chỉ trả users có role admin.
8. `GET /api/v1/admin/users?status=locked` chỉ trả locked users.
9. `POST /api/v1/admin/users/invite` với `{ email, name, role }` gửi invite email — HTTP 201.
