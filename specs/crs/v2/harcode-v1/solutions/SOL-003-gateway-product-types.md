# SOL-003: ProductTypes từ DB — gateway-service + product-service

**CR:** CR-HC-003 | **Priority:** 🟠 High | **Sprint:** 2  
**Services:** `services/gateway-service`, `services/product-service`

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-008
**Note:** ProductTypes đọc từ bảng product_types (PostgreSQL)
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**Gateway ProductTypes handler hiện tại:**
```go
// gateway-service/internal/bff/handlers/handler_ui_api.go:646
func (h *UIAPIHandler) ProductTypes(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "types": []map[string]string{
            {"value": "web_app",        "label": "Web Application"},
            {"value": "api",            "label": "API"},
            {"value": "infrastructure", "label": "Infrastructure"},
            {"value": "mobile",         "label": "Mobile"},
        },
    })
}
```

**Product-service đã có:**
- `product-service/internal/delivery/http/handlers.go` — `ProductTypeID` field là `// ignored — no product_types table`
- `product-service/embedded.go` — `WireEmbedded` với pool available
- Gateway đã có `productServiceURL` và `proxyRequest()` method

**Giải pháp đơn giản nhất:** Product-service thêm endpoint `/api/v1/products/types` đọc từ DB, gateway proxy đến đó.

---

## Solution

### Bước 1: Migration SQL trong product-service

**File mới:** `product-service/migrations/002_product_types.sql`

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

CREATE INDEX IF NOT EXISTS idx_product_types_active 
    ON product_types(is_active, sort_order);

-- Seed default types (từ hardcoded values trong gateway)
INSERT INTO product_types (name, display_name, description, sort_order) VALUES
    ('web_app',        'Web Application',      'Web-based applications and portals', 1),
    ('api',            'API / Microservice',   'REST, GraphQL, or gRPC APIs',       2),
    ('infrastructure', 'Infrastructure',        'Servers, network devices, cloud',   3),
    ('mobile',         'Mobile App',           'iOS and Android applications',      4),
    ('desktop',        'Desktop Application',  'Native desktop software',           5),
    ('iot',            'IoT / Embedded',       'Embedded systems and IoT devices',  6)
ON CONFLICT (name) DO NOTHING;
```

### Bước 2: Domain entity và interface trong product-service

**File mới:** `product-service/internal/domain/entity/product_type.go`

```go
package entity

import (
    "time"
    "github.com/google/uuid"
)

// ProductType is a configurable category of product.
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

// ProductTypeRepository provides access to product types.
type ProductTypeRepository interface {
    List(ctx context.Context, activeOnly bool) ([]*ProductType, error)
    Create(ctx context.Context, pt *ProductType) error
    Update(ctx context.Context, pt *ProductType) error
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### Bước 3: Repository implementation

**File mới:** `product-service/internal/infra/postgres/product_type_repo.go`

```go
package postgres

import (
    "context"
    "fmt"

    "github.com/google/uuid"
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
        if err := rows.Scan(
            &pt.ID, &pt.Name, &pt.DisplayName, &pt.Description,
            &pt.Icon, &pt.IsActive, &pt.SortOrder, &pt.CreatedAt,
        ); err != nil {
            return nil, fmt.Errorf("product_type_repo.List scan: %w", err)
        }
        types = append(types, pt)
    }
    return types, rows.Err()
}

func (r *ProductTypeRepo) Create(ctx context.Context, pt *entity.ProductType) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO product_types (id, name, display_name, description, icon, is_active, sort_order)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (name) DO UPDATE SET
            display_name = EXCLUDED.display_name,
            updated_at = NOW()
    `, pt.ID, pt.Name, pt.DisplayName, pt.Description, pt.Icon, pt.IsActive, pt.SortOrder)
    return err
}
```

### Bước 4: Handler trong product-service

**File sửa:** `product-service/internal/delivery/http/handlers.go`

```go
// [FIX CR-HC-003] ProductTypes reads from DB — không hardcode
func (h *Handler) ProductTypes(w http.ResponseWriter, r *http.Request) {
    types, err := h.productTypeRepo.List(r.Context(), true)
    if err != nil {
        h.log.Error().Err(err).Msg("ProductTypes: list failed")
        writeError(w, http.StatusInternalServerError, "failed to list product types")
        return
    }
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "types": types,
        "total": len(types),
    })
}

// Route registration:
r.Get("/api/v1/products/types", h.ProductTypes)
r.Post("/api/v1/products/types", h.CreateProductType)  // Admin only
r.Put("/api/v1/products/types/{id}", h.UpdateProductType)
r.Delete("/api/v1/products/types/{id}", h.DeleteProductType)
```

### Bước 5: Gateway proxy thay vì hardcode

**File sửa:** `gateway-service/internal/bff/handlers/handler_ui_api.go`

```go
// [FIX CR-HC-003] Proxy to product-service instead of hardcode
func (h *UIAPIHandler) ProductTypes(w http.ResponseWriter, r *http.Request) {
    h.proxyRequest(w, r, h.productServiceURL+"/api/v1/products/types")
}
```

### Bước 6: Wire ProductTypeRepo trong product-service embedded.go

**File sửa:** `product-service/embedded.go`

```go
import (
    pginfra "github.com/google/osv.dev/services/product-service/internal/infra/postgres"
)

func WireEmbedded(ctx context.Context, logger zerolog.Logger, db *pgxpool.Pool, mux *http.ServeMux) error {
    productTypeRepo := pginfra.NewProductTypeRepo(db)  // [FIX CR-HC-003]
    h := handler.NewHandler(db, logger, handler.WithProductTypeRepo(productTypeRepo))
    // ...
}
```

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| NEW | `product-service/migrations/002_product_types.sql` |
| NEW | `product-service/internal/domain/entity/product_type.go` |
| NEW | `product-service/internal/infra/postgres/product_type_repo.go` |
| MODIFY | `product-service/internal/delivery/http/handlers.go` — thêm ProductTypes handler |
| MODIFY | `product-service/embedded.go` — wire ProductTypeRepo |
| MODIFY | `gateway-service/internal/bff/handlers/handler_ui_api.go` — proxy thay hardcode |

---

## Verification

```bash
# Migration
psql $DATABASE_URL -f product-service/migrations/002_product_types.sql

# Test
curl -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/products/types"
# Expect: {"types":[{"id":"uuid","name":"web_app","display_name":"Web Application",...}],"total":6}

# Verify từ DB
psql $DATABASE_URL -c "SELECT name, display_name FROM product_types ORDER BY sort_order;"
```
