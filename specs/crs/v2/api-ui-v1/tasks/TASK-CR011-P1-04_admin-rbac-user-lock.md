# TASK-CR011-P1-04 — Admin: RBAC Matrix + User Auto-Lock Schema

**Phase:** Phase 2 — Blocking UI  
**Nguồn giải pháp:** [`solutions/SOL-011`](../solutions/SOL-011-admin-rbac-matrix-user-schema.md)  
**Ưu tiên:** 🔴 P1 — Permission matrix không hiển thị, UI crash  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19  

---

## Mục tiêu

1. `GET /api/v1/admin/roles` → trả `RBACMatrixResponse` đầy đủ (roles + permission_categories)
2. `GET /api/v1/admin/users` → mỗi user có `login_attempts` + `is_locked`
3. Auto-lock user sau 5 lần đăng nhập thất bại
4. `POST /api/v1/admin/users/{id}/unlock` thực sự hoạt động

---

## Điều tra trước khi code

```bash
# 1. Xem response hiện tại của admin/roles
curl -s -H "Authorization: Bearer <admin-token>" \
  https://c12.openledger.vn/api/v1/admin/roles | jq .

# 2. Tìm handler roles
grep -rn "admin/roles\|ListRoles\|GetRoles\|RBACMatrix" \
  services/identity-service/ --include="*.go"

# 3. Xem table users có login_attempts chưa
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "SELECT column_name FROM information_schema.columns \
      WHERE table_name='users' \
      AND column_name IN ('login_attempts','is_locked','locked_at');"

# 4. Xem table roles
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "\d roles"
```

---

## Bước 1: DB Migration

**File:** `services/identity-service/migrations/20260619_002_users_lock.sql`

```sql
-- Thêm lock tracking vào users
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS login_attempts INTEGER     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS is_locked      BOOLEAN     NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS locked_at      TIMESTAMPTZ;

-- Thêm display info vào roles
ALTER TABLE roles
    ADD COLUMN IF NOT EXISTS display_name VARCHAR(100),
    ADD COLUMN IF NOT EXISTS description  TEXT,
    ADD COLUMN IF NOT EXISTS color        VARCHAR(7);

-- Populate defaults
UPDATE roles SET
    display_name = CASE name
        WHEN 'admin'    THEN 'Administrator'
        WHEN 'user'     THEN 'Security Analyst'
        WHEN 'analyst'  THEN 'Security Analyst'
        WHEN 'readonly' THEN 'Read-Only Viewer'
        WHEN 'agent'    THEN 'Scan Agent'
        ELSE initcap(name)
    END,
    color = CASE name
        WHEN 'admin'    THEN '#8B5CF6'
        WHEN 'user'     THEN '#3B82F6'
        WHEN 'analyst'  THEN '#3B82F6'
        WHEN 'readonly' THEN '#6B7280'
        WHEN 'agent'    THEN '#10B981'
        ELSE '#9CA3AF'
    END
WHERE display_name IS NULL;
```

```bash
# Chạy migration
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -f /migrations/20260619_002_users_lock.sql

# Verify
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "SELECT column_name FROM information_schema.columns \
      WHERE table_name='users' AND column_name IN ('login_attempts','is_locked');"
```

---

## Bước 2: Update AdminUser DTO

**Tìm DTO:**
```bash
grep -rn "AdminUser\|AdminUserDTO\|type.*User.*struct" \
  services/identity-service/ --include="*.go" | grep -i "dto\|admin\|response"
```

**Thêm fields mới vào DTO:**
```go
type AdminUserDTO struct {
    ID            string     `json:"id"`
    Email         string     `json:"email"`
    Name          string     `json:"name"`
    Role          string     `json:"role"`
    IsActive      bool       `json:"is_active"`
    MFAEnabled    bool       `json:"mfa_enabled"`
    CreatedAt     time.Time  `json:"created_at"`
    LastLoginAt   *time.Time `json:"last_login_at"`
    LoginAttempts int        `json:"login_attempts"` // ← MỚI
    IsLocked      bool       `json:"is_locked"`       // ← MỚI
}
```

**Cập nhật query lấy users:**
```go
// Thêm login_attempts và is_locked vào SELECT + Scan
rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.IsActive, &u.MFAEnabled,
    &u.CreatedAt, &u.LastLoginAt, &u.LoginAttempts, &u.IsLocked)
```

---

## Bước 3: Auto-Lock Logic trong Login Handler

**Tìm login handler:**
```bash
grep -rn "Login\|Authenticate\|password.*compare\|bcrypt" \
  services/identity-service/internal/ --include="*.go" -l
```

**Thêm vào login flow:**
```go
const MaxLoginAttempts = 5

// Sau khi verify password thất bại:
func (s *AuthService) recordFailedLogin(ctx context.Context, userID string) {
    // Tăng counter và lock nếu vượt ngưỡng
    var attempts int
    s.db.QueryRow(ctx, `
        UPDATE users SET login_attempts = login_attempts + 1
        WHERE id = $1 RETURNING login_attempts
    `, userID).Scan(&attempts)

    if attempts >= MaxLoginAttempts {
        s.db.Exec(ctx, `
            UPDATE users SET is_locked = true, locked_at = NOW()
            WHERE id = $1
        `, userID)
    }
}

// Kiểm tra locked TRƯỚC khi verify password:
user, err := repo.GetByEmail(ctx, email)
if err != nil {
    return nil, ErrInvalidCredentials
}
if user.IsLocked {
    return nil, fmt.Errorf("account locked after %d failed attempts", MaxLoginAttempts)
}
// ... verify password ...
// Nếu thành công, reset counter:
s.db.Exec(ctx, `UPDATE users SET login_attempts = 0 WHERE id = $1`, user.ID)
```

---

## Bước 4: Unlock Handler

**Tìm unlock endpoint:**
```bash
grep -rn "unlock\|Unlock" services/identity-service/ --include="*.go"
```

**Implement hoặc fix unlock handler:**
```go
// POST /api/v1/admin/users/{id}/unlock
func (h *AdminHandler) UnlockUser(w http.ResponseWriter, r *http.Request) {
    // Lấy user ID từ path
    id := r.PathValue("id") // Go 1.22+ hoặc chi.URLParam(r, "id")

    _, err := h.db.Exec(r.Context(), `
        UPDATE users
        SET is_locked = false, locked_at = NULL, login_attempts = 0
        WHERE id = $1
    `, id)
    if err != nil {
        respondError(w, 500, "failed to unlock user")
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
```

---

## Bước 5: RBAC Matrix Response

**Tìm roles handler:**
```bash
grep -rn "admin/roles\|ListRoles\|GetRoles" \
  services/identity-service/ --include="*.go" -n
```

**Upgrade response:**
```go
// Static config — permission categories
var permissionCategories = []map[string]interface{}{
    {"category": "Dashboard",      "items": []string{"scan:read", "finding:read"}},
    {"category": "Scanning",       "items": []string{"scan:create", "scan:read"}},
    {"category": "Findings",       "items": []string{"finding:write", "finding:read"}},
    {"category": "Assets",         "items": []string{"asset:write", "asset:read"}},
    {"category": "Reports",        "items": []string{"report:download"}},
    {"category": "AI Center",      "items": []string{"finding:write"}},
    {"category": "Administration", "items": []string{"user:manage", "system:configure"}},
    {"category": "Agent",          "items": []string{"agent:report"}},
}

// GET /api/v1/admin/roles
func (h *AdminHandler) GetRBACMatrix(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Lấy roles từ DB
    rows, err := h.db.Query(ctx, `
        SELECT id, name, COALESCE(display_name, name),
               COALESCE(description, ''), COALESCE(color, '#9CA3AF')
        FROM roles ORDER BY name
    `)
    if err != nil {
        respondError(w, 500, "failed to fetch roles")
        return
    }
    defer rows.Close()

    type RoleDTO struct {
        ID          string   `json:"id"`
        Name        string   `json:"name"`
        DisplayName string   `json:"display_name"`
        Description string   `json:"description"`
        UserCount   int      `json:"user_count"`
        Color       string   `json:"color"`
        Permissions []string `json:"permissions"`
    }

    var roles []RoleDTO
    for rows.Next() {
        var r RoleDTO
        rows.Scan(&r.ID, &r.Name, &r.DisplayName, &r.Description, &r.Color)

        // Get permissions for this role
        permRows, _ := h.db.Query(ctx,
            `SELECT permission FROM role_permissions WHERE role_name = $1`, r.Name)
        for permRows.Next() {
            var p string
            permRows.Scan(&p)
            r.Permissions = append(r.Permissions, p)
        }
        permRows.Close()
        if r.Permissions == nil {
            r.Permissions = []string{}
        }

        // Count users
        h.db.QueryRow(ctx,
            `SELECT COUNT(*) FROM users WHERE role = $1 AND is_active = true`, r.Name,
        ).Scan(&r.UserCount)

        roles = append(roles, r)
    }
    if roles == nil {
        roles = []RoleDTO{}
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "roles":                 roles,
        "permission_categories": permissionCategories,
    })
}
```

> **Note:** Nếu `role_permissions` table không tồn tại, kiểm tra xem permissions lưu ở đâu:
> ```bash
> grep -rn "permissions\|role_perm" \
>   services/identity-service/internal/infra/ --include="*.go"
> docker exec osv-backend-postgres-1 psql -U osv -d osv \
>   -c "\dt" | grep -i "role\|perm"
> ```

---

## Bước 6: Verify Gateway Routes

```bash
grep -n "admin/roles\|admin/users\|unlock" \
  apps/osv/internal/gateway/router.go
```

Thêm nếu thiếu:
```go
// Unlock user — cần đăng ký
mux.Handle("POST /api/v1/admin/users/{id}/unlock",
    adminOnly(proxy.Forward("identity-service:8081")))
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/admin/roles` → `{ roles: [...], permission_categories: [...] }` — HTTP 200
- [ ] Mỗi role có `user_count` phản ánh số users thực tế
- [ ] `GET /api/v1/admin/users` → mỗi user có `login_attempts` và `is_locked`
- [ ] Sau 5 lần login thất bại → `is_locked = true`, login bị block
- [ ] `POST /api/v1/admin/users/{id}/unlock` → `login_attempts = 0`, `is_locked = false`
- [ ] `GET /api/v1/admin/users?status=locked` → chỉ locked users

## Verification

```bash
TOKEN="<admin-token>"

# RBAC Matrix
curl -s https://c12.openledger.vn/api/v1/admin/roles \
  -H "Authorization: Bearer $TOKEN" | jq '{roles: (.roles | length), cats: (.permission_categories | length)}'
# Expected: roles >= 3, cats >= 5

# Users với lock fields
curl -s https://c12.openledger.vn/api/v1/admin/users \
  -H "Authorization: Bearer $TOKEN" | jq '.users[0] | {login_attempts, is_locked}'
# Expected: { "login_attempts": 0, "is_locked": false }

# Unlock user
curl -s -X POST \
  https://c12.openledger.vn/api/v1/admin/users/<id>/unlock \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: { "success": true }
```
