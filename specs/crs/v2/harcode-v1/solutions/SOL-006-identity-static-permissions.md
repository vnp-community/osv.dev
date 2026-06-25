# SOL-006: RBAC Matrix từ DB — identity-service

**CR:** CR-HC-006 | **Priority:** 🟡 Medium | **Sprint:** 2  
**Service:** `services/identity-service`

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-010
**Note:** RBAC Matrix đọc từ bảng rbac_roles_metadata (PostgreSQL)
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**File:** `identity-service/adapter/handler/http/admin_handler.go:260-285`

Hiện tại `GetRBACMatrix` có 2 vùng hardcode:
1. `permissionCategories` — list static
2. `roleMeta` — display name, color, description static

`valueobject.PermissionsFor(name)` ✅ đã implement thật (đọc từ domain).

**Chiến lược tối giản:** Chỉ cần DB table cho `role_metadata` (display_name, color). 
`permission_categories` vẫn có thể là config-driven (từ DB) nhưng ít critical hơn.

---

## Solution

### Bước 1: Migration

**File mới:** `identity-service/migrations/006_role_metadata.sql`

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

-- Seed từ hardcoded values hiện tại
INSERT INTO role_metadata (role, display_name, color, description, is_system, sort_order) VALUES
    ('admin',    'Administrator',    '#8B5CF6', 'Full system access',       true,  1),
    ('user',     'Security Analyst', '#3B82F6', 'Standard user access',     true,  2),
    ('readonly', 'Read-Only Viewer', '#6B7280', 'View-only access',         true,  3),
    ('agent',    'Scan Agent',       '#10B981', 'Automated scanner access',  true,  4)
ON CONFLICT (role) DO NOTHING;

CREATE TABLE IF NOT EXISTS permission_categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    sort_order  INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS permission_category_items (
    category_id UUID        NOT NULL REFERENCES permission_categories(id) ON DELETE CASCADE,
    permission  VARCHAR(100) NOT NULL,
    PRIMARY KEY (category_id, permission)
);

-- Seed permission categories từ hardcoded values
WITH cats AS (
    INSERT INTO permission_categories (name, sort_order) VALUES
        ('Dashboard',      1),
        ('Scanning',       2),
        ('Findings',       3),
        ('Reports',        4),
        ('AI Center',      5),
        ('Administration', 6),
        ('Agent',          7)
    ON CONFLICT (name) DO UPDATE SET sort_order = EXCLUDED.sort_order
    RETURNING id, name
)
INSERT INTO permission_category_items (category_id, permission)
SELECT c.id, p.permission FROM cats c
CROSS JOIN (VALUES
    ('Dashboard',      'scan:read'),
    ('Dashboard',      'finding:read'),
    ('Scanning',       'scan:create'),
    ('Scanning',       'scan:read'),
    ('Scanning',       'scan:delete'),
    ('Findings',       'finding:write'),
    ('Findings',       'finding:read'),
    ('Reports',        'report:download'),
    ('AI Center',      'finding:write'),
    ('Administration', 'user:manage'),
    ('Administration', 'system:configure'),
    ('Agent',          'agent:report')
) AS p(cat_name, permission)
WHERE c.name = p.cat_name
ON CONFLICT DO NOTHING;
```

### Bước 2: Repository interfaces

**File mới:** `identity-service/internal/domain/repository/role_metadata.go`

```go
package repository

import "context"

type RoleMetadata struct {
    Role        string `db:"role"`
    DisplayName string `db:"display_name"`
    Color       string `db:"color"`
    Description string `db:"description"`
    IsSystem    bool   `db:"is_system"`
    SortOrder   int    `db:"sort_order"`
}

type PermissionCategory struct {
    ID          string   `db:"id"`
    Name        string   `db:"name"`
    Description string   `db:"description"`
    SortOrder   int      `db:"sort_order"`
    Permissions []string // loaded separately
}

type RoleMetadataRepository interface {
    List(ctx context.Context) ([]*RoleMetadata, error)
    Update(ctx context.Context, role string, meta *RoleMetadata) error
}

type PermissionCategoryRepository interface {
    ListWithItems(ctx context.Context) ([]*PermissionCategory, error)
}
```

### Bước 3: Repository implementations

**File mới:** `identity-service/internal/infra/postgres/role_metadata_repo.go`

```go
package postgres

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/osv/identity-service/internal/domain/repository"
)

type RoleMetadataRepo struct {
    pool *pgxpool.Pool
}

func NewRoleMetadataRepo(pool *pgxpool.Pool) *RoleMetadataRepo {
    return &RoleMetadataRepo{pool: pool}
}

func (r *RoleMetadataRepo) List(ctx context.Context) ([]*repository.RoleMetadata, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT role, display_name, color, COALESCE(description,''), is_system, sort_order
        FROM role_metadata
        ORDER BY sort_order, role
    `)
    if err != nil {
        return nil, fmt.Errorf("role_metadata.List: %w", err)
    }
    defer rows.Close()

    var result []*repository.RoleMetadata
    for rows.Next() {
        m := &repository.RoleMetadata{}
        if err := rows.Scan(&m.Role, &m.DisplayName, &m.Color, &m.Description, &m.IsSystem, &m.SortOrder); err != nil {
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
```

**File mới:** `identity-service/internal/infra/postgres/permission_category_repo.go`

```go
func (r *PermissionCategoryRepo) ListWithItems(ctx context.Context) ([]*repository.PermissionCategory, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT pc.id::text, pc.name, COALESCE(pc.description,''), pc.sort_order,
               COALESCE(array_agg(pci.permission ORDER BY pci.permission), '{}')
        FROM permission_categories pc
        LEFT JOIN permission_category_items pci ON pci.category_id = pc.id
        GROUP BY pc.id, pc.name, pc.description, pc.sort_order
        ORDER BY pc.sort_order
    `)
    // ... scan with array
}
```

### Bước 4: Update GetRBACMatrix handler

**File sửa:** `identity-service/adapter/handler/http/admin_handler.go`

```go
// [FIX CR-HC-006] Read role metadata and permission categories from DB
func (h *AdminHandler) GetRBACMatrix(w http.ResponseWriter, r *http.Request) {
    // 1. Load role metadata from DB
    roleMetaList, err := h.roleMetaRepo.List(r.Context())
    if err != nil {
        h.log.Warn().Err(err).Msg("GetRBACMatrix: role metadata unavailable, using defaults")
        roleMetaList = defaultRoleMeta() // graceful fallback
    }

    // 2. Load permission categories from DB
    categories, err := h.permCatRepo.ListWithItems(r.Context())
    if err != nil {
        h.log.Warn().Err(err).Msg("GetRBACMatrix: categories unavailable, using defaults")
        categories = defaultPermCategories()
    }

    // 3. Build roles with user counts (existing logic stays)
    roles := make([]RoleDTO, 0)
    for _, meta := range roleMetaList {
        perms := valueobject.PermissionsFor(meta.Role)
        _, total, _ := h.userRepo.List(r.Context(), repository.UserFilter{Role: meta.Role, PageSize: 1})
        roles = append(roles, RoleDTO{
            ID:          meta.Role,
            Name:        meta.Role,
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

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| NEW | `identity-service/migrations/006_role_metadata.sql` |
| NEW | `identity-service/internal/domain/repository/role_metadata.go` |
| NEW | `identity-service/internal/infra/postgres/role_metadata_repo.go` |
| NEW | `identity-service/internal/infra/postgres/permission_category_repo.go` |
| MODIFY | `identity-service/adapter/handler/http/admin_handler.go` |
| MODIFY | `identity-service/embedded.go` — wire repos |

---

## Verification

```bash
psql $DATABASE_URL -f identity-service/migrations/006_role_metadata.sql

curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  "https://c12.openledger.vn/api/v1/admin/roles"
# Expect: roles từ DB, permission_categories từ DB

# Update role color
curl -X PUT -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"display_name":"Security Admin","color":"#FF0000"}' \
  "https://c12.openledger.vn/api/v1/admin/roles/admin"
# Verify persist sau restart
```
