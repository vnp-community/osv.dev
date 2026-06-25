# SOL-011: Admin RBAC Matrix & User Management Schema Updates

> **CR:** [CR-011](../CR-011-admin-rbac-matrix-user-schema.md)  
> **Priority:** 🔴 HIGH (Phase 2)  
> **Service(s):** `identity-service` (`:8081`)  
> **Tạo:** 2026-06-19  
> **Cập nhật:** 2026-06-22  
> **Trạng thái:** ⚠️ PARTIAL — Một phần đã implement, còn lại pending (xác nhận bởi API test 2026-06-22)

---

## 1. Tóm tắt Giải pháp

| Thay đổi | File chính | Loại |
|---|---|---|
| RBAC Matrix response upgrade | `delivery/http/admin_handler.go` | Schema extension |
| AdminUser thêm `login_attempts`, `is_locked` | `domain/user.go` + DB | DB migration + model update |
| Auto-lock sau 5 failed logins | `service/auth.go` | Business logic |
| AdminUsersResponse wrapper | `delivery/http/admin_handler.go` | Response format |
| InviteUser thêm `name` field | `delivery/http/admin_handler.go` | Request body update |

---

## 2. DB Migration

### File: `services/identity-service/migrations/20260619_002_users_lock.sql`

```sql
-- Thêm login tracking + lock fields vào bảng users
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS login_attempts INTEGER     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS is_locked      BOOLEAN     NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS locked_at      TIMESTAMPTZ;

-- Index để tìm locked users nhanh
CREATE INDEX IF NOT EXISTS idx_users_is_locked ON users(is_locked) WHERE is_locked = true;

-- Thêm display_name và color vào roles nếu chưa có
ALTER TABLE roles
    ADD COLUMN IF NOT EXISTS display_name  VARCHAR(100),
    ADD COLUMN IF NOT EXISTS description   TEXT,
    ADD COLUMN IF NOT EXISTS color         VARCHAR(7);   -- hex color "#RRGGBB"

-- Populate defaults cho các roles có sẵn
UPDATE roles SET
    display_name = CASE name
        WHEN 'admin'    THEN 'Administrator'
        WHEN 'user'     THEN 'Security Analyst'
        WHEN 'readonly' THEN 'Read-Only Viewer'
        WHEN 'agent'    THEN 'Scan Agent'
        ELSE name
    END,
    color = CASE name
        WHEN 'admin'    THEN '#8B5CF6'
        WHEN 'user'     THEN '#3B82F6'
        WHEN 'readonly' THEN '#6B7280'
        WHEN 'agent'    THEN '#10B981'
        ELSE '#9CA3AF'
    END,
    description = CASE name
        WHEN 'admin'    THEN 'Full access to all platform features'
        WHEN 'user'     THEN 'Standard analyst access'
        WHEN 'readonly' THEN 'View-only access to findings and assets'
        WHEN 'agent'    THEN 'Automated scan agent access'
        ELSE ''
    END
WHERE display_name IS NULL;
```

---

## 3. Domain Models

### File: `services/identity-service/internal/domain/user.go`

```go
package domain

import "time"

// AdminUser — extended với lock tracking fields
type AdminUser struct {
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
    LockedAt      *time.Time `json:"locked_at,omitempty"` // optional, chỉ trả nếu locked
}

// AdminUsersResponse — response wrapper
type AdminUsersResponse struct {
    Users    []AdminUser `json:"users"`
    Total    int         `json:"total"`
    Page     int         `json:"page"`
    PageSize int         `json:"page_size"`
}

// Role — enriched với display info và user count
type Role struct {
    ID                string   `json:"id"`
    Name              string   `json:"name"`
    DisplayName       string   `json:"display_name"`
    Description       string   `json:"description"`
    UserCount         int      `json:"user_count"`
    Color             string   `json:"color"`
    Permissions       []string `json:"permissions"`
}

// PermissionCategory — static config cho RBAC matrix UI
type PermissionCategory struct {
    Category string   `json:"category"`
    Items    []string `json:"items"`
}

// RBACMatrixResponse — full matrix cho Admin UI
type RBACMatrixResponse struct {
    Roles                []Role               `json:"roles"`
    PermissionCategories []PermissionCategory `json:"permission_categories"`
}
```

---

## 4. Auth Service — Auto-Lock Logic

### File: `services/identity-service/internal/service/auth.go`

```go
package service

import (
    "context"
    "fmt"
    "time"
)

const MaxLoginAttempts = 5

// RecordFailedLogin tăng login_attempts, lock user nếu vượt ngưỡng
func (s *AuthService) RecordFailedLogin(ctx context.Context, userID string) error {
    attempts, err := s.userRepo.IncrementLoginAttempts(ctx, userID)
    if err != nil {
        return fmt.Errorf("increment login attempts: %w", err)
    }

    if attempts >= MaxLoginAttempts {
        now := time.Now().UTC()
        if err := s.userRepo.LockUser(ctx, userID, now); err != nil {
            return fmt.Errorf("lock user: %w", err)
        }
        // Notify admin via NATS
        s.nats.Publish("identity.user.locked", &UserLockedEvent{
            UserID:   userID,
            LockedAt: now,
            Reason:   fmt.Sprintf("Exceeded %d failed login attempts", MaxLoginAttempts),
        })
    }
    return nil
}

// RecordSuccessfulLogin reset attempts counter sau khi đăng nhập thành công
func (s *AuthService) RecordSuccessfulLogin(ctx context.Context, userID string) error {
    return s.userRepo.ResetLoginAttempts(ctx, userID)
}

// Login — cập nhật để gọi tracking
func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
    user, err := s.userRepo.GetByEmail(ctx, email)
    if err != nil {
        return nil, ErrInvalidCredentials
    }

    // Kiểm tra locked trước
    if user.IsLocked {
        return nil, ErrAccountLocked
    }

    // Verify password
    if !verifyPassword(password, user.PasswordHash) {
        s.RecordFailedLogin(ctx, user.ID) // async OK
        return nil, ErrInvalidCredentials
    }

    // Success
    s.RecordSuccessfulLogin(ctx, user.ID)

    token, err := s.issueJWT(user)
    if err != nil {
        return nil, err
    }

    return &LoginResult{Token: token, User: user}, nil
}
```

### File: `services/identity-service/internal/infra/postgres/user_repo.go`

```go
// IncrementLoginAttempts tăng counter và trả về giá trị mới
func (r *UserRepo) IncrementLoginAttempts(ctx context.Context, userID string) (int, error) {
    var attempts int
    err := r.db.QueryRow(ctx, `
        UPDATE users
        SET login_attempts = login_attempts + 1
        WHERE id = $1
        RETURNING login_attempts
    `, userID).Scan(&attempts)
    return attempts, err
}

// LockUser đặt is_locked = true, ghi thời điểm lock
func (r *UserRepo) LockUser(ctx context.Context, userID string, lockedAt time.Time) error {
    _, err := r.db.Exec(ctx, `
        UPDATE users
        SET is_locked = true, locked_at = $2
        WHERE id = $1
    `, userID, lockedAt)
    return err
}

// ResetLoginAttempts về 0 sau login thành công
func (r *UserRepo) ResetLoginAttempts(ctx context.Context, userID string) error {
    _, err := r.db.Exec(ctx, `
        UPDATE users
        SET login_attempts = 0
        WHERE id = $1
    `, userID)
    return err
}

// UnlockUser reset lock status
func (r *UserRepo) UnlockUser(ctx context.Context, userID string) error {
    _, err := r.db.Exec(ctx, `
        UPDATE users
        SET is_locked = false, locked_at = NULL, login_attempts = 0
        WHERE id = $1
    `, userID)
    return err
}
```

---

## 5. Admin Handler

### File: `services/identity-service/internal/delivery/http/admin_handler.go`

```go
package http

import (
    "encoding/json"
    "net/http"
    "strconv"
)

// Static config — permission categories cho RBAC matrix UI
var permissionCategories = []domain.PermissionCategory{
    {Category: "Dashboard",      Items: []string{"scan:read", "finding:read"}},
    {Category: "Scanning",       Items: []string{"scan:create", "scan:read"}},
    {Category: "Findings",       Items: []string{"finding:write", "finding:read"}},
    {Category: "Assets",         Items: []string{"asset:write", "asset:read"}},
    {Category: "Reports",        Items: []string{"report:download"}},
    {Category: "AI Center",      Items: []string{"finding:write"}},
    {Category: "Administration", Items: []string{"user:manage", "system:configure"}},
    {Category: "Agent",          Items: []string{"agent:report"}},
}

// GetRBACMatrix godoc
// GET /api/v1/admin/roles
// Trả về full RBAC matrix: roles + permissions per role + categories
func (h *AdminHandler) GetRBACMatrix(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Lấy tất cả roles từ DB
    roles, err := h.roleRepo.GetAll(ctx)
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to fetch roles")
        return
    }

    // Enrich: thêm user_count cho mỗi role
    for i, role := range roles {
        count, _ := h.userRepo.CountByRole(ctx, role.Name)
        roles[i].UserCount = count
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(domain.RBACMatrixResponse{
        Roles:                roles,
        PermissionCategories: permissionCategories,
    })
}

// ListUsers godoc
// GET /api/v1/admin/users
// Query params: search, role, status, page, page_size
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    q := r.URL.Query()

    // Parse query params
    filter := UserFilter{
        Search:   q.Get("search"),
        Role:     q.Get("role"),
        Status:   q.Get("status"), // "active" | "inactive" | "locked"
        Page:     parseIntDefault(q.Get("page"), 1),
        PageSize: parseIntDefault(q.Get("page_size"), 20),
    }

    users, total, err := h.userRepo.ListAdmin(ctx, filter)
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to list users")
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(domain.AdminUsersResponse{
        Users:    users,
        Total:    total,
        Page:     filter.Page,
        PageSize: filter.PageSize,
    })
}

// UnlockUser godoc
// POST /api/v1/admin/users/{id}/unlock
func (h *AdminHandler) UnlockUser(w http.ResponseWriter, r *http.Request) {
    userID := r.PathValue("id")
    if err := h.userRepo.UnlockUser(r.Context(), userID); err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to unlock user")
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// InviteUser godoc
// POST /api/v1/admin/users/invite
// Body: { email, name, role }
func (h *AdminHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Email string `json:"email"`
        Name  string `json:"name"`  // ← MỚI
        Role  string `json:"role"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, 400, "INVALID_BODY", "Invalid request body")
        return
    }
    if req.Email == "" || req.Role == "" {
        jsonError(w, 400, "VALIDATION_ERROR", "email and role are required")
        return
    }

    // Tạo user với name được cung cấp
    user, err := h.userSvc.Invite(r.Context(), InviteRequest{
        Email: req.Email,
        Name:  req.Name, // populate vào user record
        Role:  req.Role,
    })
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to invite user")
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(user)
}

// Repository query with filter support
type UserFilter struct {
    Search   string
    Role     string
    Status   string // "active" | "inactive" | "locked"
    Page     int
    PageSize int
}
```

### Repository: ListAdmin với filter

```go
// services/identity-service/internal/infra/postgres/user_repo.go

func (r *UserRepo) ListAdmin(ctx context.Context, f UserFilter) ([]domain.AdminUser, int, error) {
    query := `
        SELECT id, email, name, role, is_active, mfa_enabled,
               created_at, last_login_at, login_attempts, is_locked, locked_at
        FROM users
        WHERE 1=1
    `
    args := []interface{}{}
    argIdx := 1

    if f.Search != "" {
        query += fmt.Sprintf(" AND (email ILIKE $%d OR name ILIKE $%d)", argIdx, argIdx)
        args = append(args, "%"+f.Search+"%")
        argIdx++
    }

    if f.Role != "" {
        query += fmt.Sprintf(" AND role = $%d", argIdx)
        args = append(args, f.Role)
        argIdx++
    }

    switch f.Status {
    case "active":
        query += " AND is_active = true AND is_locked = false"
    case "inactive":
        query += " AND is_active = false"
    case "locked":
        query += " AND is_locked = true"
    }

    // Count total
    countQuery := strings.Replace(query,
        "SELECT id, email, name, role, is_active, mfa_enabled, created_at, last_login_at, login_attempts, is_locked, locked_at",
        "SELECT COUNT(*)", 1)
    var total int
    r.db.QueryRow(ctx, countQuery, args...).Scan(&total)

    // Paginate
    offset := (f.Page - 1) * f.PageSize
    query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
    args = append(args, f.PageSize, offset)

    rows, err := r.db.Query(ctx, query, args...)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var users []domain.AdminUser
    for rows.Next() {
        var u domain.AdminUser
        rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.IsActive, &u.MFAEnabled,
            &u.CreatedAt, &u.LastLoginAt, &u.LoginAttempts, &u.IsLocked, &u.LockedAt)
        users = append(users, u)
    }
    return users, total, nil
}
```

---

## 6. Gateway Routing

```go
// apps/osv/internal/gateway/router.go
// Routes đã tồn tại — verify chúng proxy đúng vào identity-service:8081

// Admin routes (adminOnly chain)
// GET /api/v1/admin/roles → GetRBACMatrix
// GET /api/v1/admin/users → ListUsers (với filter support)
// POST /api/v1/admin/users/{id}/unlock → UnlockUser  ← cần verify đã có
// POST /api/v1/admin/users/invite → InviteUser      ← cần verify đã có
```

> **Lưu ý:** Nếu `/admin/users/{id}/unlock` và `/admin/users/invite` chưa có trong router, cần thêm vào.

---

## 7. Tests

```go
// TestGetRBACMatrix
func TestGetRBACMatrix_HasRolesAndCategories(t *testing.T) {
    resp := GET("/api/v1/admin/roles")
    var result domain.RBACMatrixResponse
    json.Unmarshal(resp.Body, &result)

    assert.NotEmpty(t, result.Roles)
    assert.NotEmpty(t, result.PermissionCategories)
    // Each role có user_count
    for _, role := range result.Roles {
        assert.GreaterOrEqual(t, role.UserCount, 0)
    }
}

// TestAutoLock
func TestLogin_AutoLockAfter5FailedAttempts(t *testing.T) {
    // Seed user
    for i := 0; i < 5; i++ {
        POST("/api/v1/auth/login", wrongPassword)
    }
    // 6th attempt phải fail với ErrAccountLocked
    resp := POST("/api/v1/auth/login", wrongPassword)
    assert.Equal(t, 403, resp.StatusCode)
    assert.Contains(t, resp.Body, "account_locked")
}

// TestUnlockUser
func TestUnlockUser_ResetsLoginAttempts(t *testing.T) {
    // Lock user
    // POST /api/v1/admin/users/{id}/unlock
    resp := POST("/api/v1/admin/users/"+userID+"/unlock", nil)
    assert.Equal(t, 200, resp.StatusCode)
    // User có thể login lại
}

// TestListUsers_Filter
func TestListUsers_FilterByStatus(t *testing.T) {
    resp := GET("/api/v1/admin/users?status=locked")
    var result domain.AdminUsersResponse
    json.Unmarshal(resp.Body, &result)
    for _, u := range result.Users {
        assert.True(t, u.IsLocked)
    }
}
```

---

## 8. Acceptance Criteria Checklist

> **Kết quả API Test (2026-06-22):**
> - `GET /api/v1/admin/roles` → ✅ **200** nhưng ⚠️ thiếu `permission_categories` trong response (CR-011 pending)
> - `GET /api/v1/admin/users` → ✅ **200** với `login_attempts` + `is_locked` đã có
> - Filter `?role=admin` → ✅ hoạt động
> - Filter `?status=locked` → ✅ hoạt động
> - `POST /api/v1/admin/users/{id}/unlock` → chưa kiểm tra
> - Auto-lock sau 5 failed logins → chưa kiểm tra

- [x] `GET /api/v1/admin/roles` → `RBACMatrixResponse` với `roles` — HTTP 200
- [ ] `GET /api/v1/admin/roles` → có `permission_categories` field trong response
- [x] Mỗi role có `user_count` phản ánh số users thực tế
- [x] `GET /api/v1/admin/users` → `login_attempts` + `is_locked` trong mỗi AdminUser
- [ ] Sau 5 lần đăng nhập thất bại → `is_locked = true`, user không login được
- [ ] `POST /api/v1/admin/users/{id}/unlock` → `login_attempts = 0`, `is_locked = false`
- [x] `GET /api/v1/admin/users` → `{ users: [...], total: N, page: P, page_size: PS }`
- [x] `?role=admin` filter hoạt động
- [x] `?status=locked` filter hoạt động
- [ ] `POST /api/v1/admin/users/invite` với `{ email, name, role }` → HTTP 201
