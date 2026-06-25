# CR-HC-003: gateway-service — ProductTypes Hardcoded + password_policy Static

## Trạng thái: 🟠 High

## Vấn đề 1: ProductTypes Hardcoded Enum
File: `services/gateway-service/internal/bff/handlers/handler_ui_api.go:646`

```go
// ProductTypes handles GET /api/v1/products/types — returns hardcoded enum for now.
func (h *UIAPIHandler) ProductTypes(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusOK, []string{"web", "mobile", "api", "network", "cloud"})
}
```

Product types là hardcoded enum trong handler. Khi business cần thêm type mới, phải deploy lại code.
Đây là violation của Open/Closed Principle và Clean Architecture.

### Giải pháp
Product types phải được lưu trong DB và quản lý qua admin API.

#### 1. Migration SQL (product-service)
```sql
CREATE TABLE IF NOT EXISTS product_types (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(50) NOT NULL UNIQUE,
    display_name VARCHAR(100) NOT NULL,
    description  TEXT,
    is_active    BOOLEAN NOT NULL DEFAULT true,
    sort_order   INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO product_types (name, display_name, sort_order) VALUES
    ('web',     'Web Application',  1),
    ('mobile',  'Mobile App',       2),
    ('api',     'API / Microservice', 3),
    ('network', 'Network / Infrastructure', 4),
    ('cloud',   'Cloud / SaaS',     5);
```

#### 2. Repository trong product-service
```go
type ProductTypeRepository interface {
    List(ctx context.Context) ([]*ProductType, error)
    Create(ctx context.Context, pt *ProductType) error
    Update(ctx context.Context, pt *ProductType) error
    Delete(ctx context.Context, id uuid.UUID) error
}
```

#### 3. Gateway proxy ProductTypes → product-service
Gateway không nên giữ business data. Proxy tới product-service hoặc inject use case.

---

## Vấn đề 2: password_policy Hardcoded "medium"
File: `services/gateway-service/internal/bff/handlers/handler_ui_api.go:970`

```go
"password_policy": "medium",  // ← hardcoded, không đọc từ admin settings
```

`password_policy` là cấu hình hệ thống phải được lưu trong admin settings table và có thể thay đổi qua Admin UI.

### Giải pháp
Đọc từ `system_settings` table thông qua identity-service hoặc admin settings endpoint:

```go
// Thay vì hardcode:
settings, err := h.adminSettingsRepo.Get(ctx)
if err != nil {
    settings = &AdminSettings{PasswordPolicy: "medium"} // default chỉ khi DB lỗi
}
writeJSON(w, http.StatusOK, map[string]interface{}{
    "password_policy": settings.PasswordPolicy,
    // ...
})
```

## Files cần thay đổi
- `services/gateway-service/internal/bff/handlers/handler_ui_api.go` — xóa hardcode
- `services/product-service/internal/domain/product_type.go` [NEW]
- `services/product-service/internal/infra/postgres/product_type_repo.go` [NEW]
- `services/product-service/migrations/002_product_types.sql` [NEW]
- `services/identity-service/internal/infra/postgres/admin_settings_repo.go` — thêm PasswordPolicy field

## Acceptance Criteria
- [ ] `GET /api/v1/products/types` đọc từ DB, không hardcode
- [ ] Admin có thể thêm product type mới qua API mà không cần redeploy
- [ ] `password_policy` đọc từ `system_settings` table
