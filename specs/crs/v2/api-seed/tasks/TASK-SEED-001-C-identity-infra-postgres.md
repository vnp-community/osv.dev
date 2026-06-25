# TASK-SEED-001-C: Infra Layer — PostgreSQL implementations (identity-service)

> **Solution:** [SOL-SEED-001](../solutions/SOL-SEED-001-identity-bootstrap.md)  
> **Service:** `services/identity-service`  
> **Depends on:** TASK-SEED-001-B (cần interfaces đã định nghĩa)  
> **Blocking:** TASK-SEED-001-D  
> **Status:** ✅ COMPLETED — 2026-06-18  
> **File sửa:** `adapter/repository/postgres/user_repo.go`

## Mục tiêu

Implement các methods mới trong `UserRepository` PostgreSQL implementation.

## Bước 1: Tìm file postgres repo

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service/internal/infra \
  -name "*.go" | xargs grep -l "UserRepository\|user_repo\|userRepo" 2>/dev/null
```

## Bước 2: Implement `CreateDirect`

**Thêm vào file postgres user repository:**

```go
// CreateDirect tạo user với password hash đã biết.
// Dùng ON CONFLICT DO NOTHING để idempotent.
func (r *userRepo) CreateDirect(ctx context.Context, user *entity.User, hashedPassword string) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO osv_identity.users
            (id, email, username, password_hash, role, is_active, is_verified, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
        ON CONFLICT (email) DO NOTHING
    `, user.ID, user.Email, user.Username, hashedPassword,
       user.Role, user.IsActive, user.IsVerified)
    if err != nil {
        return fmt.Errorf("CreateDirect: %w", err)
    }
    return nil
}
```

## Bước 3: Implement `CreateBulk`

```go
// CreateBulk tạo nhiều users trong một transaction, partial failure OK.
func (r *userRepo) CreateBulk(ctx context.Context, inputs []entity.UserCreateInput) ([]entity.UserCreateResult, error) {
    // Import crypto package từ identity-service
    results := make([]entity.UserCreateResult, 0, len(inputs))

    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return nil, fmt.Errorf("CreateBulk begin tx: %w", err)
    }
    defer tx.Rollback()

    stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO osv_identity.users
            (id, email, username, password_hash, role, is_active, is_verified, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
        ON CONFLICT (email) DO NOTHING
        RETURNING id
    `)
    if err != nil {
        return nil, err
    }
    defer stmt.Close()

    for _, in := range inputs {
        // Validate email
        if !isValidEmail(in.Email) {
            results = append(results, entity.UserCreateResult{
                Email: in.Email, Status: "error", Message: "invalid email format",
            })
            continue
        }

        // Hash password (bcrypt cost=12)
        hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
        if err != nil {
            results = append(results, entity.UserCreateResult{
                Email: in.Email, Status: "error", Message: "password hashing failed",
            })
            continue
        }

        id := uuid.New()
        var returnedID uuid.UUID
        err = stmt.QueryRowContext(ctx, id, in.Email, in.Username,
            string(hashed), in.Role, in.IsActive, in.IsVerified).Scan(&returnedID)

        if err == sql.ErrNoRows {
            // ON CONFLICT DO NOTHING — email đã tồn tại
            results = append(results, entity.UserCreateResult{
                Email: in.Email, Status: "error", Message: "email already exists",
            })
            continue
        }
        if err != nil {
            results = append(results, entity.UserCreateResult{
                Email: in.Email, Status: "error", Message: err.Error(),
            })
            continue
        }

        results = append(results, entity.UserCreateResult{
            Email: in.Email, Status: "created", ID: &returnedID,
        })
    }

    if err := tx.Commit(); err != nil {
        return nil, fmt.Errorf("CreateBulk commit: %w", err)
    }
    return results, nil
}
```

## Bước 4: Implement `FindByID` (nếu chưa có)

```go
func (r *userRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
    var u entity.User
    err := r.db.QueryRowContext(ctx, `
        SELECT id, email, username, role, is_active, is_verified, mfa_enabled, created_at, updated_at
        FROM osv_identity.users WHERE id = $1
    `, id).Scan(&u.ID, &u.Email, &u.Username, &u.Role,
        &u.IsActive, &u.IsVerified, &u.MFAEnabled, &u.CreatedAt, &u.UpdatedAt)
    if err == sql.ErrNoRows {
        return nil, domain.ErrNotFound
    }
    return &u, err
}
```

## Bước 5: Implement `AssignRole`

```go
func (r *userRepo) AssignRole(ctx context.Context, a entity.RoleAssignment) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO osv_identity.role_assignments
            (user_id, role_id, scope, resource_id, assigned_by, assigned_at)
        VALUES ($1, $2, $3, $4, $5, NOW())
        ON CONFLICT (user_id, role_id, scope, COALESCE(resource_id, '00000000-0000-0000-0000-000000000000'::UUID))
        DO UPDATE SET assigned_at = NOW(), assigned_by = EXCLUDED.assigned_by
    `, a.UserID, a.RoleID, a.Scope, a.ResourceID, a.AssignedBy)
    return err
}
```

## Acceptance Criteria

- [x] 4 methods được implement đầy đủ trong postgres repo (`CreateDirect`, `CreateBulk`, `AssignRole`, `FindByID` đã có sẵn)
- [x] `CreateBulk` dùng single transaction
- [x] `CreateBulk` xử lý email trùng → status "error", không abort toàn bộ transaction
- [x] `go build ./adapter/... ./internal/domain/...` trong identity-service thành công
- [x] `go vet ./...` không cảnh báo cho các package đã sửa
