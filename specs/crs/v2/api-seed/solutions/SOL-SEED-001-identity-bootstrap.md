# SOL-SEED-001: Giải pháp thực thi — Identity Bootstrap

> **CR:** [SEED-001-identity-bootstrap.md](../SEED-001-identity-bootstrap.md)  
> **Cập nhật:** 2026-06-18  
> **Domain:** `services/identity-service` + `apps/osv` (gateway)  
> **Priority:** 🔴 CRITICAL

---

## 1. Phân tích kiến trúc hiện tại

Theo `01-architecture.md §3.4`, `identity-service` tuân theo **Clean Architecture 4 layers**:
- `adapter/handler/http/` — HTTP handlers (đã có: `auth_handler.go`, `admin_handler.go`, `api_key_handler.go`)
- `internal/usecase/` — Business logic (đã có: `register`, `login`, `refresh_token`, `validate`)
- `internal/domain/` — Entities + Repository interfaces
- `internal/infra/` — PostgreSQL implementations

Schema DB: `osv_identity` — bảng `users`, `api_keys`, `sessions`.

**Auth chain gateway**: `protected` → `adminOnly` middleware inject `X-User-Role: Admin` vào header.

---

## 2. Các thay đổi cần thực hiện

### 2.1 Domain Layer — `services/identity-service/internal/domain/`

#### Thêm method vào UserRepository interface

**File**: `internal/domain/repository/user_repository.go`

```go
// Thêm 2 methods mới vào interface UserRepository
type UserRepository interface {
    // ... existing methods ...
    
    // CreateDirect tạo user với password đã biết (không qua invite flow)
    CreateDirect(ctx context.Context, user *entity.User, hashedPassword string) error
    
    // CreateBulk tạo nhiều users trong một transaction
    // Trả về slice results với status per-user
    CreateBulk(ctx context.Context, users []entity.UserCreateInput) ([]entity.UserCreateResult, error)
    
    // FindByID lấy user theo ID (nếu chưa có)
    FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
    
    // AssignRole gán role cho user (product-scoped hoặc global)
    AssignRole(ctx context.Context, assignment entity.RoleAssignment) error
}
```

**File**: `internal/domain/entity/user.go` — Thêm types:

```go
// UserCreateInput là input cho bulk create
type UserCreateInput struct {
    Email       string
    Username    string
    Password    string   // plaintext, sẽ được hash bởi usecase
    Role        string   // "admin" | "user" | "readonly"
    IsActive    bool
    IsVerified  bool
}

// UserCreateResult là kết quả per-item của bulk create
type UserCreateResult struct {
    Email   string
    Status  string  // "created" | "error"
    ID      *uuid.UUID
    Message string
}

// RoleAssignment là input gán role
type RoleAssignment struct {
    UserID     uuid.UUID
    RoleID     int
    Scope      string     // "global" | "product"
    ResourceID *uuid.UUID // nil nếu scope = global
}
```

---

### 2.2 Use Case Layer — `services/identity-service/internal/usecase/`

#### Tạo usecase `admin_user`

**File**: `internal/usecase/admin_user/usecase.go`

```go
package adminuser

type UseCase struct {
    userRepo   repository.UserRepository
    apiKeyRepo repository.APIKeyRepository
    hasher     crypto.Hasher
    generator  crypto.KeyGenerator
}

// CreateUser tạo user trực tiếp với password
// Pub: NATS identity.user.created (→ audit-service)
func (uc *UseCase) CreateUser(ctx context.Context, in CreateUserInput) (*entity.User, error)

// CreateBulkUsers tạo nhiều users — partial failure OK
func (uc *UseCase) CreateBulkUsers(ctx context.Context, users []UserCreateInput) (BulkCreateResult, error)

// CreateAPIKeyForUser admin tạo API key thay cho user khác
func (uc *UseCase) CreateAPIKeyForUser(ctx context.Context, adminID, targetUserID uuid.UUID, in APIKeyInput) (*entity.APIKeyWithSecret, error)

// AssignRole gán role cho user
func (uc *UseCase) AssignRole(ctx context.Context, in entity.RoleAssignment) error
```

**Quy trình `CreateBulkUsers`:**
```
1. Open DB transaction
2. For each UserCreateInput:
   a. Validate email format, password strength
   b. Check email uniqueness (SELECT ... FOR UPDATE)
   c. bcrypt(password, cost=12)
   d. INSERT INTO users
   e. Append to results: {email, status:"created", id}
3. Commit transaction
4. Publish NATS: identity.user.bulk_created{count, actor_id}
5. Return BulkCreateResult{created_count, failed_count, results}
```

---

### 2.3 Adapter Layer — `services/identity-service/adapter/handler/http/`

#### Thêm handler vào `admin_handler.go`

**File**: `adapter/handler/http/admin_handler.go`

```go
// CreateUser handles POST /api/v1/admin/users
// Requires X-User-Role: Admin (injected by gateway)
func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Email      string `json:"email"`
        Username   string `json:"username"`
        Password   string `json:"password"`
        Role       string `json:"role"`
        IsActive   bool   `json:"is_active"`
        IsVerified bool   `json:"is_verified"`
    }
    // decode → validate → usecase.CreateUser → 201
}

// CreateBulkUsers handles POST /api/v1/admin/users/bulk
func (h *AdminHandler) CreateBulkUsers(w http.ResponseWriter, r *http.Request) {
    // decode array → usecase.CreateBulkUsers → 207
}

// GetUserByID handles GET /api/v1/admin/users/{id}
func (h *AdminHandler) GetUserByID(w http.ResponseWriter, r *http.Request) {
    // parse path param id → userRepo.FindByID → 200 | 404
}

// CreateAPIKeyForUser handles POST /api/v1/admin/users/{id}/api-keys
func (h *AdminHandler) CreateAPIKeyForUser(w http.ResponseWriter, r *http.Request) {
    // parse {id} → decode scopes → usecase.CreateAPIKeyForUser → 201
}

// AssignRole handles POST /api/v1/admin/users/{id}/roles
func (h *AdminHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
    // parse {id} → decode RoleAssignment → usecase.AssignRole → 200
}
```

#### Router registration

**File**: `adapter/handler/http/router.go` (hoặc tương đương)

```go
// Admin routes (X-User-Role: Admin enforced by gateway)
r.Post("/api/v1/admin/users",           adminH.CreateUser)
r.Post("/api/v1/admin/users/bulk",      adminH.CreateBulkUsers)
r.Get("/api/v1/admin/users/{id}",       adminH.GetUserByID)
r.Post("/api/v1/admin/users/{id}/api-keys", adminH.CreateAPIKeyForUser)
r.Post("/api/v1/admin/users/{id}/roles", adminH.AssignRole)
```

---

### 2.4 Infra Layer — `services/identity-service/internal/infra/`

#### PostgreSQL implementations

**File**: `internal/infra/postgres/user_repo.go`

```go
// CreateDirect: INSERT INTO osv_identity.users (...)
func (r *userRepo) CreateDirect(ctx context.Context, user *entity.User, hashedPwd string) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO osv_identity.users
            (id, email, username, password_hash, role, is_active, is_verified, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
        ON CONFLICT (email) DO NOTHING
        RETURNING id
    `, user.ID, user.Email, user.Username, hashedPwd, user.Role, user.IsActive, user.IsVerified)
    return err
}

// CreateBulk: sử dụng pgx COPY protocol cho hiệu năng cao
func (r *userRepo) CreateBulk(ctx context.Context, users []entity.UserCreateInput) ([]entity.UserCreateResult, error) {
    // Loop with individual inserts inside transaction (COPY không hỗ trợ ON CONFLICT)
}
```

**Migration**: `migrations/identity/0010_admin_user_seed.sql`

```sql
-- Thêm role_assignments table cho product-scoped roles
CREATE TABLE IF NOT EXISTS osv_identity.role_assignments (
    id          UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES osv_identity.users(id) ON DELETE CASCADE,
    role_id     INT NOT NULL,
    scope       VARCHAR(20) NOT NULL DEFAULT 'global',  -- 'global' | 'product'
    resource_id UUID,
    assigned_at TIMESTAMPTZ DEFAULT NOW(),
    assigned_by UUID REFERENCES osv_identity.users(id),
    UNIQUE(user_id, role_id, scope, resource_id)
);
CREATE INDEX ON osv_identity.role_assignments(user_id);
```

---

### 2.5 Gateway Layer — `apps/osv/internal/gateway/router.go`

Thêm routes mới vào route group `/api/v1/admin/*`:

```go
// ═══════════════════════════════════════════════
// ADMIN USER SEED ENDPOINTS (SEED-001)
// ═══════════════════════════════════════════════
// Rate limit: 20/min (bulk tạo users)
mux.Handle("POST /api/v1/admin/users",
    adminOnly(ratelimit.Wrap(proxy.Forward("identity-service:8081"), 20)))

mux.Handle("POST /api/v1/admin/users/bulk",
    adminOnly(ratelimit.Wrap(proxy.Forward("identity-service:8081"), 5)))

mux.Handle("GET /api/v1/admin/users/{id}",
    adminOnly(proxy.Forward("identity-service:8081")))

mux.Handle("POST /api/v1/admin/users/{id}/api-keys",
    adminOnly(proxy.Forward("identity-service:8081")))

mux.Handle("POST /api/v1/admin/users/{id}/roles",
    adminOnly(proxy.Forward("identity-service:8081")))
```

> ⚠️ **Route ordering**: `POST /api/v1/admin/users/bulk` phải đứng **TRƯỚC** `POST /api/v1/admin/users/{id}/*` để tránh shadowing trong Go `net/http` ServeMux.

---

## 3. NATS Events mới

| Subject | Publisher | Consumers | Payload |
|---------|-----------|----------|---------|
| `identity.user.created` | identity-service | audit-service | `{user_id, email, role, actor_id}` |
| `identity.user.bulk_created` | identity-service | audit-service | `{count, actor_id}` |
| `identity.apikey.created` | identity-service | audit-service | `{key_id, user_id, actor_id}` |

---

## 4. Security considerations

- **Password policy**: Tối thiểu 8 ký tự, ít nhất 1 chữ hoa + 1 số.
- **Bulk create limit**: Tối đa 100 users/request để tránh DoS.
- **API key display**: `key` field chỉ trả về 1 lần trong `201 Created` response. Sau đó chỉ lưu SHA-256 hash.
- **Audit**: Mọi admin action đều publish NATS event → audit-service ghi log với HMAC.
- **Rate limiting**: `POST /admin/users/bulk` → 5/min; `POST /admin/users` → 20/min.

---

## 5. File thay đổi tổng hợp

| File | Thay đổi |
|------|---------|
| `internal/domain/repository/user_repository.go` | Thêm `CreateDirect`, `CreateBulk`, `FindByID`, `AssignRole` |
| `internal/domain/entity/user.go` | Thêm `UserCreateInput`, `UserCreateResult`, `RoleAssignment` |
| `internal/usecase/admin_user/usecase.go` | **[NEW]** UseCase mới |
| `adapter/handler/http/admin_handler.go` | Thêm 5 handlers |
| `adapter/handler/http/router.go` | Đăng ký 5 routes mới |
| `internal/infra/postgres/user_repo.go` | Implement `CreateDirect`, `CreateBulk` |
| `migrations/identity/0010_admin_user_seed.sql` | **[NEW]** Migration cho `role_assignments` |
| `apps/osv/internal/gateway/router.go` | Thêm 5 gateway routes |

---

## 6. Acceptance Criteria

1. `POST /api/v1/admin/users` với valid body → `201` với user object không có `password_hash`.
2. `POST /api/v1/admin/users` với email trùng → `409 Conflict {"error": "email_exists"}`.
3. `POST /api/v1/admin/users/bulk` với 10 users hợp lệ → `207 {"created_count": 10}`.
4. `POST /api/v1/admin/users/bulk` với 1 email trùng trong 10 → `207 {"created_count": 9, "failed_count": 1}`.
5. `GET /api/v1/admin/users/{id}` trả về user; với ID không tồn tại → `404`.
6. `POST /api/v1/admin/users/{id}/api-keys` → `201` với `key` plaintext (1 lần duy nhất).
7. `POST /api/v1/admin/users/{id}/roles` với product scope → `200`.
8. Caller không phải Admin → `403 Forbidden` từ gateway.
