# TASK-SEED-001-B: Domain Layer — UserRepository & entities (identity-service)

> **Solution:** [SOL-SEED-001](../solutions/SOL-SEED-001-identity-bootstrap.md)  
> **Service:** `services/identity-service`  
> **Depends on:** TASK-SEED-001-A  
> **Blocking:** TASK-SEED-001-C  
> **Status:** ✅ COMPLETED — 2026-06-18  
> **Files đã sửa:** `internal/domain/entity/user.go`, `internal/domain/repository/repositories.go`

## Mục tiêu

Thêm methods mới vào `UserRepository` interface và thêm entity types cần thiết cho bulk user creation và role assignment.

## Bước 1: Đọc file hiện tại

```bash
# Tìm UserRepository interface
grep -rn "type UserRepository interface\|UserRepository" \
  /Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service/internal/domain/ \
  --include="*.go" | head -20

# Đọc file entity user
find /Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service/internal/domain \
  -name "*.go" | xargs grep -l "type User struct"
```

## Bước 2: Thêm types vào entity file

**File:** `internal/domain/entity/user.go` (hoặc file chứa `User struct`)

Thêm các types sau (nếu chưa tồn tại):

```go
// UserCreateInput là input để admin tạo user trực tiếp
type UserCreateInput struct {
    Email      string
    Username   string
    Password   string // plaintext — sẽ được hash bởi usecase
    Role       string // "admin" | "user" | "readonly"
    IsActive   bool
    IsVerified bool
}

// UserCreateResult là kết quả per-item của bulk create
type UserCreateResult struct {
    Email   string
    Status  string // "created" | "error"
    ID      *uuid.UUID
    Message string
}

// RoleAssignment là input để gán role cho user
type RoleAssignment struct {
    UserID     uuid.UUID
    RoleID     int
    Scope      string     // "global" | "product"
    ResourceID *uuid.UUID // nil nếu scope = global
    AssignedBy uuid.UUID
}
```

## Bước 3: Thêm methods vào UserRepository interface

**File:** `internal/domain/repository/user_repository.go` (hoặc tương đương)

```go
// Thêm vào interface UserRepository (nếu chưa có):

// CreateDirect tạo user với password hash đã cho
CreateDirect(ctx context.Context, user *entity.User, hashedPassword string) error

// CreateBulk tạo nhiều users, partial failure OK
// Trả về slice results với status per-user
CreateBulk(ctx context.Context, inputs []entity.UserCreateInput) ([]entity.UserCreateResult, error)

// FindByID lấy user theo UUID
FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error)

// AssignRole gán role cho user (tạo record trong role_assignments)
AssignRole(ctx context.Context, assignment entity.RoleAssignment) error
```

## Bước 4: Thêm RoleAssignmentRepository interface (nếu cần tách riêng)

```go
// File: internal/domain/repository/role_assignment_repository.go (NEW nếu cần)
type RoleAssignmentRepository interface {
    Create(ctx context.Context, a entity.RoleAssignment) error
    FindByUser(ctx context.Context, userID uuid.UUID) ([]entity.RoleAssignment, error)
}
```

## Acceptance Criteria

- [x] `UserCreateInput`, `UserCreateResult`, `RoleAssignment` types tồn tại trong domain
- [x] `UserRepository` interface có đủ 3 methods mới (`CreateDirect`, `CreateBulk`, `AssignRole`)
- [x] Code compile không lỗi (`go build ./adapter/... ./internal/domain/...` thành công)
- [x] Không phá vỡ implementations hiện tại
