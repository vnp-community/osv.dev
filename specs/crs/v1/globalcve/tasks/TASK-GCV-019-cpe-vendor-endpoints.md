# TASK-GCV-019 — Vendor/Product Endpoints (search-service)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-019 |
| **Service** | `search-service` |
| **CR** | CR-GCV-005 |
| **Phase** | 2 — Enrichment |
| **Priority** | 🟡 Medium |
| **Prerequisites** | — |

## Context

Thêm 3 endpoints vào `search-service` để list vendors, list products per vendor, và search products. Data từ bảng `cpe_dict` (populated bởi data-service CPE fetcher — shared DB).

## Reference

- Solution: [SOL-GCV-005](../solutions/SOL-GCV-005-nvd-cpe-vendor-filter.md) §2.3, §2.4

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/domain/repository/vendor_repo.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/infra/postgres/vendor_pg.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/delivery/http/vendor_handler.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/delivery/http/
        (router — đăng ký routes)
```

**Đọc trước**: Cấu trúc search-service infra/postgres để match pattern hiện có.

## Implementation Spec

### repository/vendor_repo.go

```go
package repository

import "context"

// VendorEntry contains vendor metadata from CPE dictionary.
type VendorEntry struct {
    Vendor       string `db:"vendor"        json:"vendor"`
    ProductCount int    `db:"product_count" json:"product_count"`
}

// VendorRepository queries vendor/product data from CPE dictionary.
type VendorRepository interface {
    ListVendors(ctx context.Context, q string, limit int) ([]*VendorEntry, int64, error)
    GetProductsByVendor(ctx context.Context, vendor string) ([]string, error)
    ListProducts(ctx context.Context, vendor, q string, limit int) ([]string, error)
}
```

### infra/postgres/vendor_pg.go

```go
package postgres

type pgVendorRepository struct{ db *sqlx.DB }

func NewVendorRepository(db *sqlx.DB) repository.VendorRepository {
    return &pgVendorRepository{db: db}
}

func (r *pgVendorRepository) ListVendors(ctx context.Context, q string, limit int) ([]*repository.VendorEntry, int64, error) {
    args := []interface{}{limit}
    where := ""
    if q != "" {
        args = append(args, "%"+strings.ToLower(q)+"%")
        where = fmt.Sprintf("WHERE lower(vendor) LIKE $%d", len(args))
    }

    var entries []*repository.VendorEntry
    query := fmt.Sprintf(`
        SELECT vendor, COUNT(DISTINCT product) as product_count
        FROM cpe_dict
        %s
        GROUP BY vendor
        ORDER BY vendor
        LIMIT $1
    `, where)
    err := r.db.SelectContext(ctx, &entries, query, args...)

    var total int64
    r.db.QueryRowContext(ctx,
        fmt.Sprintf("SELECT COUNT(DISTINCT vendor) FROM cpe_dict %s", where),
        args[1:]...).Scan(&total)

    return entries, total, err
}

func (r *pgVendorRepository) GetProductsByVendor(ctx context.Context, vendor string) ([]string, error) {
    var products []string
    err := r.db.SelectContext(ctx, &products, `
        SELECT DISTINCT product
        FROM cpe_dict
        WHERE lower(vendor) = lower($1)
        ORDER BY product
    `, vendor)
    return products, err
}

func (r *pgVendorRepository) ListProducts(ctx context.Context, vendor, q string, limit int) ([]string, error) {
    args := []interface{}{strings.ToLower(vendor), limit}
    where := "WHERE lower(vendor) = $1"
    if q != "" {
        args = append(args, "%"+strings.ToLower(q)+"%")
        where += fmt.Sprintf(" AND lower(product) LIKE $%d", len(args))
    }

    var products []string
    err := r.db.SelectContext(ctx, &products,
        fmt.Sprintf(`SELECT DISTINCT product FROM cpe_dict %s ORDER BY product LIMIT $2`, where),
        args...)
    return products, err
}
```

### delivery/http/vendor_handler.go

```go
package http

type VendorHandler struct {
    vendorRepo repository.VendorRepository
}

func NewVendorHandler(repo repository.VendorRepository) *VendorHandler {
    return &VendorHandler{vendorRepo: repo}
}

// GET /api/v2/vendors?q=apach&limit=100
func (h *VendorHandler) ListVendors(w http.ResponseWriter, r *http.Request) {
    q     := r.URL.Query().Get("q")
    limit := parseInt(r.URL.Query().Get("limit"), 100)
    if limit > 500 { limit = 500 }

    vendors, total, err := h.vendorRepo.ListVendors(r.Context(), q, limit)
    if err != nil {
        respondError(w, 500, "failed to list vendors")
        return
    }
    respondJSON(w, 200, map[string]interface{}{
        "vendors": vendors,
        "total":   total,
    })
}

// GET /api/v2/vendors/{vendor}/products
func (h *VendorHandler) GetVendorProducts(w http.ResponseWriter, r *http.Request) {
    vendor := chi.URLParam(r, "vendor")
    products, err := h.vendorRepo.GetProductsByVendor(r.Context(), vendor)
    if err != nil {
        respondError(w, 500, "failed to get products")
        return
    }
    respondJSON(w, 200, map[string]interface{}{
        "vendor":   vendor,
        "products": products,
        "count":    len(products),
    })
}

// GET /api/v2/products?vendor=apache&q=log
func (h *VendorHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
    vendor := r.URL.Query().Get("vendor")
    if vendor == "" {
        respondError(w, 400, "vendor parameter is required")
        return
    }
    q     := r.URL.Query().Get("q")
    limit := parseInt(r.URL.Query().Get("limit"), 100)

    products, err := h.vendorRepo.ListProducts(r.Context(), vendor, q, limit)
    if err != nil {
        respondError(w, 500, "failed to list products")
        return
    }
    respondJSON(w, 200, map[string]interface{}{
        "vendor":   vendor,
        "products": products,
        "count":    len(products),
    })
}
```

### Route registration

```go
vendorHandler := http.NewVendorHandler(vendorRepo)
r.Get("/api/v2/vendors", vendorHandler.ListVendors)
r.Get("/api/v2/vendors/{vendor}/products", vendorHandler.GetVendorProducts)
r.Get("/api/v2/products", vendorHandler.ListProducts)
```

## Acceptance Criteria

- [x] `GET /api/v2/vendors` → list vendors với `vendor`, `product_count`
- [x] `GET /api/v2/vendors?q=apach` → filter vendors chứa "apach" (case-insensitive)
- [x] `GET /api/v2/vendors/apache/products` → list Apache products sorted
- [x] `GET /api/v2/products?vendor=apache&q=log` → Apache products chứa "log"
- [x] `GET /api/v2/products` without vendor → 400 "vendor parameter is required"
- [x] `GET /api/v2/vendors/nonexistent/products` → empty `products: []` (không 404)
- [x] limit max = 500 (cap không error)
- [x] Response có `total` field (vendor count)
- [x] `go build ./...` pass không lỗi
