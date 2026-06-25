# TASK-SEED-001-D: UseCase + HTTP Handlers + Gateway (identity-service)

> **Solution:** [SOL-SEED-001](../solutions/SOL-SEED-001-identity-bootstrap.md)  
> **Service:** `services/identity-service` + `apps/osv`  
> **Depends on:** TASK-SEED-001-C  
> **Blocking:** Không có (đây là task cuối của SEED-001)  
> **Status:** ✅ COMPLETED — 2026-06-18 (gateway routes verified 2026-06-19)  
> **Files đã tạo/sửa:**  
> - `internal/usecase/admin_user/usecase.go` (NEW)  
> - `adapter/handler/http/admin_handler.go` (thêm 3 handlers + cập nhật struct)  
> - `adapter/handler/http/router.go` (thêm 4 routes)  
> - `apps/osv/internal/gateway/router.go` (thêm 5 gateway prefix routes, bao gồm `POST /api/v1/admin/users/{id}/roles`)

## Mục tiêu

Tạo usecase `admin_user`, thêm HTTP handlers vào `admin_handler.go`, đăng ký routes trong router, và thêm gateway routes.

## Bước 1: Đọc cấu trúc hiện tại

```bash
# Xem structure identity-service
find /Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service \
  -name "admin_handler.go" -o -name "admin*.go" | head -10

# Xem router file
find /Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service \
  -name "router.go" | head -5
```

## Bước 2: Tạo UseCase `admin_user`

**File:** `services/identity-service/internal/usecase/admin_user/usecase.go` (NEW)

```go
package adminuser

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"

    "github.com/osv/identity-service/internal/domain/entity"
    "github.com/osv/identity-service/internal/domain/repository"
)

type UseCase struct {
    userRepo   repository.UserRepository
    apiKeyRepo repository.APIKeyRepository
}

func New(userRepo repository.UserRepository, apiKeyRepo repository.APIKeyRepository) *UseCase {
    return &UseCase{userRepo: userRepo, apiKeyRepo: apiKeyRepo}
}

// CreateUser tạo user trực tiếp với password
func (uc *UseCase) CreateUser(ctx context.Context, in entity.UserCreateInput) (*entity.User, error) {
    if err := validatePassword(in.Password); err != nil {
        return nil, err
    }
    hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
    if err != nil {
        return nil, fmt.Errorf("hash password: %w", err)
    }
    user := &entity.User{
        ID:         uuid.New(),
        Email:      in.Email,
        Username:   in.Username,
        Role:       in.Role,
        IsActive:   in.IsActive,
        IsVerified: in.IsVerified,
    }
    if err := uc.userRepo.CreateDirect(ctx, user, string(hashed)); err != nil {
        return nil, err
    }
    return user, nil
}

// CreateBulkUsers tạo nhiều users, partial failure OK
func (uc *UseCase) CreateBulkUsers(ctx context.Context, inputs []entity.UserCreateInput) ([]entity.UserCreateResult, error) {
    if len(inputs) > 100 {
        return nil, fmt.Errorf("bulk limit exceeded: max 100 users per request")
    }
    return uc.userRepo.CreateBulk(ctx, inputs)
}

// CreateAPIKeyForUser admin tạo API key cho user khác
func (uc *UseCase) CreateAPIKeyForUser(ctx context.Context, targetUserID uuid.UUID, name string, scopes []string) (*entity.APIKeyWithPlaintext, error) {
    // Tái dùng logic từ existing API key usecase
    return uc.apiKeyRepo.CreateForUser(ctx, targetUserID, name, scopes)
}

// AssignRole gán role cho user
func (uc *UseCase) AssignRole(ctx context.Context, in entity.RoleAssignment) error {
    return uc.userRepo.AssignRole(ctx, in)
}

func validatePassword(p string) error {
    if len(p) < 8 {
        return fmt.Errorf("password must be at least 8 characters")
    }
    return nil
}
```

## Bước 3: Thêm handlers vào `admin_handler.go`

```bash
# Đọc file hiện tại để biết cấu trúc AdminHandler
cat /Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service/adapter/handler/http/admin_handler.go
```

**Thêm vào cuối `admin_handler.go`** (sau khi đọc và hiểu struct hiện tại):

```go
// Thêm adminUserUC vào AdminHandler struct:
// adminUserUC *adminuser.UseCase

// CreateUser handles POST /api/v1/admin/users
func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req entity.UserCreateInput
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, errResp("invalid_body", err.Error()))
        return
    }
    user, err := h.adminUserUC.CreateUser(r.Context(), req)
    if err != nil {
        if strings.Contains(err.Error(), "exists") || strings.Contains(err.Error(), "duplicate") {
            writeJSON(w, http.StatusConflict, errResp("email_exists", "email already registered"))
            return
        }
        writeJSON(w, http.StatusInternalServerError, errResp("internal", err.Error()))
        return
    }
    // Never return password_hash
    writeJSON(w, http.StatusCreated, map[string]any{
        "id":          user.ID,
        "email":       user.Email,
        "username":    user.Username,
        "role":        user.Role,
        "is_active":   user.IsActive,
        "is_verified": user.IsVerified,
        "created_at":  user.CreatedAt,
    })
}

// CreateBulkUsers handles POST /api/v1/admin/users/bulk
func (h *AdminHandler) CreateBulkUsers(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Users []entity.UserCreateInput `json:"users"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, errResp("invalid_body", err.Error()))
        return
    }
    results, err := h.adminUserUC.CreateBulkUsers(r.Context(), req.Users)
    if err != nil {
        writeJSON(w, http.StatusBadRequest, errResp("validation_error", err.Error()))
        return
    }
    created, failed := 0, 0
    for _, r := range results {
        if r.Status == "created" { created++ } else { failed++ }
    }
    writeJSON(w, http.StatusMultiStatus, map[string]any{
        "created_count": created,
        "failed_count":  failed,
        "results":       results,
    })
}

// GetUserByID handles GET /api/v1/admin/users/{id}
func (h *AdminHandler) GetUserByID(w http.ResponseWriter, r *http.Request) {
    idStr := chi.URLParam(r, "id")  // hoặc r.PathValue("id") nếu stdlib
    id, err := uuid.Parse(idStr)
    if err != nil {
        writeJSON(w, http.StatusBadRequest, errResp("invalid_id", "invalid UUID"))
        return
    }
    user, err := h.userRepo.FindByID(r.Context(), id)
    if err != nil {
        writeJSON(w, http.StatusNotFound, errResp("not_found", "user not found"))
        return
    }
    writeJSON(w, http.StatusOK, user)
}

// CreateAPIKeyForUser handles POST /api/v1/admin/users/{id}/api-keys
func (h *AdminHandler) CreateAPIKeyForUser(w http.ResponseWriter, r *http.Request) {
    targetID, _ := uuid.Parse(chi.URLParam(r, "id"))
    var req struct {
        Name   string   `json:"name"`
        Scopes []string `json:"scopes"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    result, err := h.adminUserUC.CreateAPIKeyForUser(r.Context(), targetID, req.Name, req.Scopes)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errResp("internal", err.Error()))
        return
    }
    writeJSON(w, http.StatusCreated, result) // includes plaintext key — one time only
}

// AssignRole handles POST /api/v1/admin/users/{id}/roles
func (h *AdminHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
    userID, _ := uuid.Parse(chi.URLParam(r, "id"))
    actorID, _ := uuid.Parse(r.Header.Get("X-User-ID"))
    var req struct {
        RoleID     int        `json:"role_id"`
        Scope      string     `json:"scope"`
        ResourceID *uuid.UUID `json:"resource_id"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    err := h.adminUserUC.AssignRole(r.Context(), entity.RoleAssignment{
        UserID:     userID,
        RoleID:     req.RoleID,
        Scope:      req.Scope,
        ResourceID: req.ResourceID,
        AssignedBy: actorID,
    })
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errResp("internal", err.Error()))
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"status": "assigned"})
}
```

## Bước 4: Đăng ký routes trong router

**File:** identity-service router (tìm bằng `find ... -name "router.go"`)

```go
// Thêm vào admin route group (X-User-Role: Admin được enforce bởi gateway)
// QUAN TRỌNG: literal paths TRƯỚC wildcard {id}
r.Post("/api/v1/admin/users",               adminH.CreateUser)
r.Post("/api/v1/admin/users/bulk",          adminH.CreateBulkUsers)   // literal TRƯỚC /{id}
r.Get("/api/v1/admin/users/{id}",           adminH.GetUserByID)
r.Post("/api/v1/admin/users/{id}/api-keys", adminH.CreateAPIKeyForUser)
r.Post("/api/v1/admin/users/{id}/roles",    adminH.AssignRole)
```

## Bước 5: Thêm gateway routes

**File:** `apps/osv/internal/gateway/router.go`

```bash
# Đọc file gateway router để tìm vị trí thêm admin routes
grep -n "admin/users\|admin.*user\|identity-service" \
  /Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go | head -20
```

Thêm vào sau các admin routes hiện có, với chú ý literal trước wildcard:

```go
// SEED-001: Admin User Seed Routes
// Rate limit: 5/min cho bulk (nặng), 20/min cho single
mux.Handle("POST /api/v1/admin/users",
    adminOnly(ratelimit.Wrap(proxy.Forward("identity-service:8081"), 20)))
mux.Handle("POST /api/v1/admin/users/bulk",       // LITERAL trước /{id}
    adminOnly(ratelimit.Wrap(proxy.Forward("identity-service:8081"), 5)))
mux.Handle("GET /api/v1/admin/users/{id}",
    adminOnly(proxy.Forward("identity-service:8081")))
mux.Handle("POST /api/v1/admin/users/{id}/api-keys",
    adminOnly(proxy.Forward("identity-service:8081")))
mux.Handle("POST /api/v1/admin/users/{id}/roles",
    adminOnly(proxy.Forward("identity-service:8081")))
```

## Acceptance Criteria

- [x] `POST /api/v1/admin/users` → `201` với user object, không có `password_hash`
- [x] `POST /api/v1/admin/users` email trùng → `409 Conflict`
- [x] `POST /api/v1/admin/users/bulk` với 5 users → `207 {"created_count": 5}`
- [x] `POST /api/v1/admin/users/bulk` với 1 email trùng → `207 {"created_count": 4, "failed_count": 1}`
- [x] `GET /api/v1/admin/users/{id}` → `200` | `404` (handler đã có sẵn từ CR-001)
- [x] `POST /api/v1/admin/users/{id}/roles` → `200 {"status": "assigned"}`
- [x] Caller không phải Admin → `403 Forbidden` từ gateway (enforce qua `RequiredPerm: "system:configure"`)
- [x] `go build ./adapter/... ./internal/domain/...` thành công
- [x] `go build ./internal/proxy/...` (gateway) thành công
