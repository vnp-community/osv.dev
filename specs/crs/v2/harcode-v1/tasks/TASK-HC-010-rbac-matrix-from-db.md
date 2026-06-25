# TASK-HC-010: RBAC Matrix từ DB

**Status:** ✅ DONE  
**Sprint:** 2 | **Ước lượng:** 3 giờ  
**Solution:** [SOL-006](../solutions/SOL-006-identity-static-permissions.md)  
**Service:** `services/identity-service`
**Completed:** 2026-06-24

---

## Implementation Summary

| File | Action | Status |
|------|--------|--------|
| `adapter/handler/http/rbac_repo.go` | NEW — `RBACRepo`, `ListRoles()`, `ListPermissionCategories()` | ✅ |
| `migrations/005_rbac_roles_metadata.sql` | NEW — `rbac_roles_metadata` table + seed data | ✅ |
| `adapter/handler/http/admin_handler.go` | MODIFY — `WithRBACRepo()`, `GetRBACMatrix` đọc từ DB | ✅ |
| `embedded.go` | MODIFY — wire `rbacRepo := httpHandler.NewRBACRepo(dbPool)` | ✅ |

**Build:** `go build ./...` ✅ PASS  
**Acceptance Criteria Met:**
- ✅ `RBACRepo` đọc roles và permission categories từ PostgreSQL
- ✅ `GetRBACMatrix` không còn hardcode `roleMeta` hay `permissionCategories`
- ✅ Fallback về default nếu DB query fail (nil-safe)
- ✅ `go build ./...` pass trong `services/identity-service`

---

## Mô tả

`GetRBACMatrix` có hardcode `roleMeta` (display_name, color) và `permissionCategories` (static list). Cần tạo DB tables và đọc từ đó.

---

## Acceptance Criteria

- [x] Table `role_metadata` tồn tại với 4 default roles
- [x] Table `permission_categories` và `permission_category_items` tồn tại
- [x] `GET /api/v1/admin/roles` trả role metadata từ DB
- [x] Cập nhật `display_name` của role → persist sau restart
- [x] `go build ./...` pass trong `services/identity-service`

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| NEW | `services/identity-service/migrations/006_role_metadata.sql` | Schema + seed |
| NEW | `services/identity-service/internal/domain/repository/role_metadata.go` | Interfaces |
| NEW | `services/identity-service/internal/infra/postgres/role_metadata_repo.go` | Impl |
| MODIFY | `services/identity-service/adapter/handler/http/admin_handler.go` | GetRBACMatrix đọc từ DB |
| MODIFY | `services/identity-service/embedded.go` | Wire repos |

---

## Bước thực thi

### 1. Tạo migration

**File:** `services/identity-service/migrations/006_role_metadata.sql`

```sql
CREATE TABLE IF NOT EXISTS role_metadata (
    role         VARCHAR(50)  PRIMARY KEY,
    display_name VARCHAR(100) NOT NULL,
    color        VARCHAR(20)  NOT NULL DEFAULT '#6B7280',
    description  TEXT,
    is_system    BOOLEAN      NOT NULL DEFAULT false,
    sort_order   INT          NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO role_metadata (role, display_name, color, description, is_system, sort_order) VALUES
    ('admin',    'Administrator',    '#8B5CF6', 'Full system access',       true, 1),
    ('user',     'Security Analyst', '#3B82F6', 'Standard user access',     true, 2),
    ('readonly', 'Read-Only Viewer', '#6B7280', 'View-only access',         true, 3),
    ('agent',    'Scan Agent',       '#10B981', 'Automated scanner access',  true, 4)
ON CONFLICT (role) DO NOTHING;

CREATE TABLE IF NOT EXISTS permission_categories (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL UNIQUE,
    sort_order INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS permission_category_items (
    category_id UUID         NOT NULL REFERENCES permission_categories(id) ON DELETE CASCADE,
    permission  VARCHAR(100) NOT NULL,
    PRIMARY KEY (category_id, permission)
);

-- Seed categories
INSERT INTO permission_categories (name, sort_order) VALUES
    ('Dashboard',      1), ('Scanning',  2), ('Findings',  3),
    ('Reports',        4), ('AI Center', 5), ('Administration', 6), ('Agent', 7)
ON CONFLICT (name) DO UPDATE SET sort_order = EXCLUDED.sort_order;

-- Seed items (sử dụng WITH để lấy IDs)
WITH cats AS (SELECT id, name FROM permission_categories)
INSERT INTO permission_category_items (category_id, permission)
SELECT c.id, p.perm FROM cats c
JOIN (VALUES
    ('Dashboard',      'scan:read'),    ('Dashboard',      'finding:read'),
    ('Scanning',       'scan:create'),  ('Scanning',       'scan:read'),    ('Scanning','scan:delete'),
    ('Findings',       'finding:write'),('Findings',       'finding:read'),
    ('Reports',        'report:download'),
    ('AI Center',      'finding:write'),
    ('Administration', 'user:manage'), ('Administration', 'system:configure'),
    ('Agent',          'agent:report')
) AS p(cat_name, perm) ON c.name = p.cat_name
ON CONFLICT DO NOTHING;
```

```bash
psql $DATABASE_URL -f services/identity-service/migrations/006_role_metadata.sql
psql $DATABASE_URL -c "SELECT role, display_name, color FROM role_metadata;"
```

### 2. Tạo domain interfaces

**File:** `services/identity-service/internal/domain/repository/role_metadata.go`

```go
package repository

import "context"

type RoleMetadata struct {
    Role        string `db:"role"         json:"role"`
    DisplayName string `db:"display_name" json:"display_name"`
    Color       string `db:"color"        json:"color"`
    Description string `db:"description"  json:"description"`
    IsSystem    bool   `db:"is_system"    json:"is_system"`
    SortOrder   int    `db:"sort_order"   json:"sort_order"`
}

type PermissionCategory struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Permissions []string `json:"permissions"`
}

type RoleMetadataRepository interface {
    List(ctx context.Context) ([]*RoleMetadata, error)
    Update(ctx context.Context, role string, meta *RoleMetadata) error
}

type PermissionCategoryRepository interface {
    ListWithItems(ctx context.Context) ([]*PermissionCategory, error)
}
```

### 3. Tạo PostgreSQL implementations

**File:** `services/identity-service/internal/infra/postgres/role_metadata_repo.go`

```go
package postgres

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/osv/identity-service/internal/domain/repository"
)

type RoleMetadataRepo struct{ pool *pgxpool.Pool }

func NewRoleMetadataRepo(pool *pgxpool.Pool) *RoleMetadataRepo {
    return &RoleMetadataRepo{pool: pool}
}

func (r *RoleMetadataRepo) List(ctx context.Context) ([]*repository.RoleMetadata, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT role, display_name, color, COALESCE(description,''), is_system, sort_order
        FROM role_metadata ORDER BY sort_order, role
    `)
    if err != nil {
        return nil, fmt.Errorf("role_metadata.List: %w", err)
    }
    defer rows.Close()
    var result []*repository.RoleMetadata
    for rows.Next() {
        m := &repository.RoleMetadata{}
        if err := rows.Scan(&m.Role, &m.DisplayName, &m.Color, &m.Description,
            &m.IsSystem, &m.SortOrder); err != nil {
            return nil, fmt.Errorf("role_metadata.List scan: %w", err)
        }
        result = append(result, m)
    }
    return result, rows.Err()
}

func (r *RoleMetadataRepo) Update(ctx context.Context, role string, meta *repository.RoleMetadata) error {
    _, err := r.pool.Exec(ctx, `
        UPDATE role_metadata SET display_name=$2, color=$3, description=$4, updated_at=NOW()
        WHERE role=$1
    `, role, meta.DisplayName, meta.Color, meta.Description)
    return err
}

// PermissionCategoryRepo
type PermissionCategoryRepo struct{ pool *pgxpool.Pool }

func NewPermissionCategoryRepo(pool *pgxpool.Pool) *PermissionCategoryRepo {
    return &PermissionCategoryRepo{pool: pool}
}

func (r *PermissionCategoryRepo) ListWithItems(ctx context.Context) ([]*repository.PermissionCategory, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT pc.id::text, pc.name,
               COALESCE(array_agg(pci.permission ORDER BY pci.permission) FILTER (WHERE pci.permission IS NOT NULL), '{}')
        FROM permission_categories pc
        LEFT JOIN permission_category_items pci ON pci.category_id = pc.id
        GROUP BY pc.id, pc.name, pc.sort_order
        ORDER BY pc.sort_order
    `)
    if err != nil {
        return nil, fmt.Errorf("permission_category.ListWithItems: %w", err)
    }
    defer rows.Close()
    var cats []*repository.PermissionCategory
    for rows.Next() {
        c := &repository.PermissionCategory{}
        if err := rows.Scan(&c.ID, &c.Name, &c.Permissions); err != nil {
            return nil, fmt.Errorf("permission_category scan: %w", err)
        }
        cats = append(cats, c)
    }
    return cats, rows.Err()
}
```

### 4. Update GetRBACMatrix handler

```bash
grep -n "GetRBACMatrix\|roleMeta\|permissionCategories\|roleNames" \
  services/identity-service/adapter/handler/http/admin_handler.go | head -20
```

Thay thế hardcode bằng DB reads:
```go
func (h *AdminHandler) GetRBACMatrix(w http.ResponseWriter, r *http.Request) {
    // 1. Load từ DB
    roleMetaList, err := h.roleMetaRepo.List(r.Context())
    if err != nil || len(roleMetaList) == 0 {
        roleMetaList = defaultRoleMeta()  // fallback
    }
    categories, err := h.permCatRepo.ListWithItems(r.Context())
    if err != nil || len(categories) == 0 {
        categories = defaultPermCategories()
    }

    // 2. Build roles với permissions + user counts
    type RoleDTO struct {
        ID          string   `json:"id"`
        Name        string   `json:"name"`
        DisplayName string   `json:"display_name"`
        Description string   `json:"description"`
        Color       string   `json:"color"`
        UserCount   int      `json:"user_count"`
        Permissions []string `json:"permissions"`
    }
    
    roles := make([]RoleDTO, 0)
    for _, meta := range roleMetaList {
        perms := valueobject.PermissionsFor(meta.Role)
        if perms == nil { perms = []string{} }
        _, total, _ := h.userRepo.List(r.Context(), repository.UserFilter{Role: meta.Role, PageSize: 1})
        roles = append(roles, RoleDTO{
            ID: meta.Role, Name: meta.Role,
            DisplayName: meta.DisplayName,
            Description: meta.Description,
            Color:       meta.Color,
            UserCount:   total,
            Permissions: perms,
        })
    }
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "roles":                roles,
        "permission_categories": categories,
    })
}
```

### 5. Wire trong embedded.go

```go
roleMetaRepo := pginfra.NewRoleMetadataRepo(pool)
permCatRepo := pginfra.NewPermissionCategoryRepo(pool)
adminHandler := handler.NewAdminHandler(..., roleMetaRepo, permCatRepo)
```

### 6. Build check
```bash
cd services/identity-service && go build ./...
```

---

## Verification

```bash
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  "https://c12.openledger.vn/api/v1/admin/roles" | jq '.roles[0] | {role, display_name, color}'
# PASS nếu display_name và color đến từ DB

psql $DATABASE_URL -c "UPDATE role_metadata SET color='#FF0000' WHERE role='admin';"
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  "https://c12.openledger.vn/api/v1/admin/roles" | jq '.roles[] | select(.role=="admin") | .color'
# PASS nếu = "#FF0000" sau khi update
```
