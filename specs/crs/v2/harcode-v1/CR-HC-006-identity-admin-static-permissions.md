# CR-HC-006: identity-service — Static Permission Categories và Role Metadata

## Trạng thái: 🟡 Medium

## Vấn đề
File: `services/identity-service/adapter/handler/http/admin_handler.go:264-285`

```go
// Static permission categories as required by the frontend matrix UI
permissionCategories := []map[string]interface{}{
    {"name": "Scan Management", "permissions": []string{"scan:create","scan:read","scan:delete"}},
    {"name": "Asset Management", "permissions": []string{"asset:read","asset:write"}},
    // ...
}

// Static role metadata (display_name, color) — DB migration optional
roleMetadata := map[string]map[string]string{
    "admin":    {"display_name": "Administrator", "color": "#ef4444"},
    "user":     {"display_name": "User",          "color": "#3b82f6"},
    // ...
}
```

**Vấn đề:**
1. Khi thêm permission mới (`scan:execute`, `finding:manage`...) phải sửa code và redeploy
2. Role display metadata không thể thay đổi qua UI
3. Vi phạm Single Responsibility — handler giữ business config

## Giải pháp

### 1. Migration: `permission_categories` + `role_metadata` tables
```sql
CREATE TABLE IF NOT EXISTS permission_categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    sort_order  INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS permission_category_items (
    category_id UUID NOT NULL REFERENCES permission_categories(id) ON DELETE CASCADE,
    permission  VARCHAR(100) NOT NULL,
    PRIMARY KEY (category_id, permission)
);

CREATE TABLE IF NOT EXISTS role_metadata (
    role         VARCHAR(50) PRIMARY KEY,
    display_name VARCHAR(100) NOT NULL,
    color        VARCHAR(20) NOT NULL DEFAULT '#6b7280',
    description  TEXT,
    is_system    BOOLEAN NOT NULL DEFAULT false
);
```

### 2. Seed data (migration)
```sql
-- Seed permission categories
INSERT INTO permission_categories (name, sort_order) VALUES
    ('Scan Management',   1),
    ('Asset Management',  2),
    ('Finding Management',3),
    ('User Management',   4),
    ('Report Management', 5),
    ('System Administration', 6);

-- Seed role metadata
INSERT INTO role_metadata (role, display_name, color, is_system) VALUES
    ('admin',    'Administrator', '#ef4444', true),
    ('user',     'User',          '#3b82f6', true),
    ('readonly', 'Read Only',     '#6b7280', true),
    ('agent',    'Agent',         '#8b5cf6', true);
```

### 3. Repository
```go
type PermissionCategoryRepository interface {
    ListWithItems(ctx context.Context) ([]*PermissionCategory, error)
}

type RoleMetadataRepository interface {
    List(ctx context.Context) ([]*RoleMetadata, error)
    Update(ctx context.Context, role string, meta *RoleMetadata) error
}
```

### 4. UseCase + Handler
```go
func (h *AdminHandler) GetRBACMatrix(w http.ResponseWriter, r *http.Request) {
    categories, err := h.permCatRepo.ListWithItems(r.Context())
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to load permission categories")
        return
    }
    roles, err := h.roleMeta.List(r.Context())
    // ...
    writeJSON(w, http.StatusOK, RBACMatrixResponse{
        Roles:                 roles,
        PermissionCategories:  categories,
    })
}
```

## Files cần thay đổi
- `services/identity-service/adapter/handler/http/admin_handler.go` — wire repos
- `services/identity-service/internal/infra/postgres/permission_category_repo.go` [NEW]
- `services/identity-service/internal/infra/postgres/role_metadata_repo.go` [NEW]
- `services/identity-service/migrations/003_rbac_metadata.sql` [NEW]

## Acceptance Criteria
- [ ] `GET /admin/roles` trả data từ DB
- [ ] Admin có thể thêm permission vào category qua API
- [ ] Role display_name/color có thể cập nhật mà không cần deploy lại
