# TASK-HC-008: ProductTypes từ DB

**Status:** ✅ DONE  
**Sprint:** 2 | **Ước lượng:** 4 giờ  
**Solution:** [SOL-003](../solutions/SOL-003-gateway-product-types.md)  
**Services:** `services/product-service`, `services/gateway-service`

---

## Mô tả

`GET /api/v1/products/types` đang trả hardcode list trong gateway. Cần tạo `product_types` table, endpoint trong product-service, và gateway proxy đến đó.

---

## Acceptance Criteria

- [x] Table `product_types` tồn tại với seed data (6 types)
- [x] `GET /api/v1/products/types` trả data từ DB (có field `id` UUID)
- [x] Thêm type mới qua API → tồn tại sau restart
- [x] Gateway `ProductTypes` handler proxy đến product-service (không hardcode)
- [x] `go build ./...` pass trong cả hai services

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| NEW | `services/product-service/migrations/002_product_types.sql` | Schema + seed |
| NEW | `services/product-service/internal/infra/postgres/product_type_repo.go` | Repository |
| MODIFY | `services/product-service/internal/delivery/http/handlers.go` | Thêm ProductTypes handler |
| MODIFY | `services/product-service/embedded.go` | Wire productTypeRepo |
| MODIFY | `services/gateway-service/internal/bff/handlers/handler_ui_api.go` | Proxy thay hardcode |

---

## Bước thực thi

### 1. Tạo migration

**File:** `services/product-service/migrations/002_product_types.sql`

```sql
CREATE TABLE IF NOT EXISTS product_types (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(50)  NOT NULL UNIQUE,
    display_name VARCHAR(100) NOT NULL,
    description  TEXT,
    icon         VARCHAR(50),
    is_active    BOOLEAN NOT NULL DEFAULT true,
    sort_order   INT     NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_product_types_active ON product_types(is_active, sort_order);

INSERT INTO product_types (name, display_name, description, sort_order) VALUES
    ('web_app',        'Web Application',    'Web-based applications and portals', 1),
    ('api',            'API / Microservice',  'REST, GraphQL, or gRPC APIs',        2),
    ('infrastructure', 'Infrastructure',      'Servers, network devices, cloud',    3),
    ('mobile',         'Mobile App',         'iOS and Android applications',       4),
    ('desktop',        'Desktop Application','Native desktop software',            5),
    ('iot',            'IoT / Embedded',     'Embedded systems and IoT devices',   6)
ON CONFLICT (name) DO NOTHING;
```

Chạy migration:
```bash
psql $DATABASE_URL -f services/product-service/migrations/002_product_types.sql
psql $DATABASE_URL -c "SELECT name, display_name FROM product_types ORDER BY sort_order;"
```

### 2. Tạo ProductType entity

Kiểm tra domain entities:
```bash
cat services/product-service/internal/domain/entity/entities.go | head -30
```

Nếu chưa có ProductType:
**File:** `services/product-service/internal/domain/entity/product_type.go`

```go
package entity

import (
    "context"
    "time"
    "github.com/google/uuid"
)

type ProductType struct {
    ID          uuid.UUID `json:"id"           db:"id"`
    Name        string    `json:"name"         db:"name"`
    DisplayName string    `json:"display_name" db:"display_name"`
    Description string    `json:"description"  db:"description"`
    Icon        string    `json:"icon"         db:"icon"`
    IsActive    bool      `json:"is_active"    db:"is_active"`
    SortOrder   int       `json:"sort_order"   db:"sort_order"`
    CreatedAt   time.Time `json:"created_at"   db:"created_at"`
}

type ProductTypeRepository interface {
    List(ctx context.Context, activeOnly bool) ([]*ProductType, error)
}
```

### 3. Tạo ProductTypeRepo

**File:** `services/product-service/internal/infra/postgres/product_type_repo.go`

```go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/google/osv.dev/services/product-service/internal/domain/entity"
)

type ProductTypeRepo struct {
    pool *pgxpool.Pool
}

func NewProductTypeRepo(pool *pgxpool.Pool) *ProductTypeRepo {
    return &ProductTypeRepo{pool: pool}
}

func (r *ProductTypeRepo) List(ctx context.Context, activeOnly bool) ([]*entity.ProductType, error) {
    query := `
        SELECT id, name, display_name, COALESCE(description,''), COALESCE(icon,''),
               is_active, sort_order, created_at
        FROM product_types
    `
    if activeOnly {
        query += " WHERE is_active = true"
    }
    query += " ORDER BY sort_order, name"

    rows, err := r.pool.Query(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("product_type_repo.List: %w", err)
    }
    defer rows.Close()

    var types []*entity.ProductType
    for rows.Next() {
        pt := &entity.ProductType{}
        if err := rows.Scan(&pt.ID, &pt.Name, &pt.DisplayName, &pt.Description,
            &pt.Icon, &pt.IsActive, &pt.SortOrder, &pt.CreatedAt); err != nil {
            return nil, fmt.Errorf("product_type_repo.List scan: %w", err)
        }
        types = append(types, pt)
    }
    return types, rows.Err()
}
```

### 4. Thêm ProductTypes handler trong product-service

```bash
grep -n "func.*Handler\|Router\|route\|Get\|Post" services/product-service/internal/delivery/http/handlers.go | head -20
```

Thêm handler và route:
```go
func (h *Handler) ProductTypes(w http.ResponseWriter, r *http.Request) {
    types, err := h.productTypeRepo.List(r.Context(), true)
    if err != nil {
        h.log.Error().Err(err).Msg("ProductTypes: list failed")
        writeError(w, http.StatusInternalServerError, "failed to list product types")
        return
    }
    if types == nil { types = []*entity.ProductType{} }
    writeJSON(w, http.StatusOK, map[string]interface{}{"types": types, "total": len(types)})
}
```

### 5. Wire productTypeRepo trong product-service embedded.go

```bash
grep -n "func WireEmbedded\|handler.NewHandler" services/product-service/embedded.go
```

```go
// Thêm productTypeRepo
productTypeRepo := postgres.NewProductTypeRepo(db)
h := handler.NewHandler(db, logger, handler.WithProductTypeRepo(productTypeRepo))
// Route:
r.Get("/api/v1/products/types", h.ProductTypes)
```

### 6. Gateway: proxy thay hardcode

```bash
grep -n "ProductTypes\|products/types" \
  services/gateway-service/internal/bff/handlers/handler_ui_api.go
```

Sửa handler:
```go
// [FIX CR-HC-003] Proxy to product-service instead of hardcode
func (h *UIAPIHandler) ProductTypes(w http.ResponseWriter, r *http.Request) {
    h.proxyRequest(w, r, h.productServiceURL+"/api/v1/products/types")
}
```

### 7. Build check cả hai services
```bash
cd services/product-service && go build ./...
cd services/gateway-service && go build ./...
```

---

## Verification

```bash
# List product types từ DB
curl -s -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/products/types" | jq '.total, .types[0]'
# PASS nếu total = 6 và types[0].id là UUID (không phải string "web_app")

# Verify không còn hardcode
curl -s -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/products/types" | jq '.types[0] | has("id")'
# PASS nếu = true (hardcode không có id field)
```
