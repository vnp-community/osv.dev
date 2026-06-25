# SOL-GCV-005 — NVD CPE Dictionary + Vendor/Product Filter

| Trường | Giá trị |
|--------|---------|
| **CR** | [CR-GCV-005](../CR-GCV-005-nvd-cpe-dictionary-vendor-filter.md) |
| **Target Service** | `data-service` (CPE sync) + `search-service` (vendor/product endpoints) |
| **apps/osv role** | Không thay đổi |
| **Priority** | 🟡 Medium |

---

## 1. Hiện trạng

- `data-service/internal/fetcher/nvd_cpe.go` → **đã có** CPE fetcher
- `data-service/internal/fetcher/redis_cpe_cache.go` → **đã có** Redis CPE cache
- `search-service` → **chưa có** `/api/v2/vendors` và vendor/product filter

---

## 2. Giải pháp

### 2.1 data-service — Verify CPE Fetcher

**File**: `data-service/internal/fetcher/nvd_cpe.go` (verify/fix)

CPE fetcher phải:
1. Download từ `https://nvd.nist.gov/feeds/json/cpematch/1.0/nvdcpematch-1.0.json.gz`
2. Parse `cpe_uri` → extract `vendor`, `product` từ CPE 2.3 format
   - Format: `cpe:2.3:a:{vendor}:{product}:{version}:...`
3. Upsert vào bảng `cpe_dict` + Redis cache
4. Schedule: weekly Sunday 4am

**Migration** (nếu chưa có):
```sql
CREATE TABLE IF NOT EXISTS cpe_dict (
    id          SERIAL      PRIMARY KEY,
    cpe_uri     TEXT        NOT NULL UNIQUE,
    vendor      TEXT        NOT NULL,
    product     TEXT        NOT NULL,
    version     TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Partial index for vendor/product lookups
CREATE INDEX IF NOT EXISTS idx_cpe_vendor  ON cpe_dict(vendor);
CREATE INDEX IF NOT EXISTS idx_cpe_product ON cpe_dict(vendor, product);

-- Materialized view for distinct vendors (auto-updated after sync)
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_vendors AS
    SELECT DISTINCT vendor, COUNT(*) as product_count
    FROM cpe_dict
    GROUP BY vendor
    ORDER BY vendor;
CREATE UNIQUE INDEX IF NOT EXISTS mv_vendors_vendor ON mv_vendors(vendor);
```

### 2.2 Redis CPE Cache Enhancement

**File**: `data-service/internal/fetcher/redis_cpe_cache.go` (verify/extend)

```go
// Cache pattern:
// "cpe:vendor:{letter}" → sorted list of vendors starting with letter (for autocomplete)
// "cpe:products:{vendor}" → list of products for vendor

// After CPE sync completes, refresh cache:
func (c *CPECache) RefreshVendorCache(ctx context.Context) error {
    vendors, err := c.cpeRepo.GetAllVendors(ctx)
    // Store in Redis sets: "cpe:vendors" = SET of all vendors
    return c.redis.SAdd(ctx, "cpe:vendors", vendors...).Err()
}

func (c *CPECache) GetProductsForVendor(ctx context.Context, vendor string) ([]string, error) {
    key := "cpe:products:" + strings.ToLower(vendor)
    // Try Redis first
    products, err := c.redis.SMembers(ctx, key).Result()
    if err != nil || len(products) == 0 {
        // Fallback to DB
        return c.cpeRepo.GetProductsByVendor(ctx, vendor)
    }
    return products, nil
}
```

### 2.3 search-service — Vendor/Product Endpoints

**File mới**: `search-service/internal/delivery/http/vendor_handler.go`

```go
// GET /api/v2/vendors
// Params: q (prefix search), page, limit
// Response: { vendors: [{name, product_count}], total, page }
func (h *Handler) ListVendors(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query().Get("q")
    limit := parseInt(r.URL.Query().Get("limit"), 100)

    vendors, total, err := h.vendorRepo.List(r.Context(), q, limit)
    // ...
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "vendors": vendors,
        "total":   total,
    })
}

// GET /api/v2/vendors/{vendor}/products
// e.g. GET /api/v2/vendors/apache/products
func (h *Handler) GetVendorProducts(w http.ResponseWriter, r *http.Request) {
    vendor := chi.URLParam(r, "vendor")
    products, err := h.vendorRepo.GetProducts(r.Context(), vendor)
    // ...
}

// GET /api/v2/products
// Params: vendor (required), q (optional search)
func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
    vendor := r.URL.Query().Get("vendor")
    // ...
}
```

**Route registration** trong `search-service`:
```go
r.Get("/api/v2/vendors", h.ListVendors)
r.Get("/api/v2/vendors/{vendor}/products", h.GetVendorProducts)
r.Get("/api/v2/products", h.ListProducts)
```

### 2.4 Vendor/Product Filter trong CVE Search

**File**: `search-service/internal/usecase/cvesearch/request.go` (MODIFY)

```go
type Request struct {
    // ... existing fields ...
    Vendor  string  // NEW: filter by vendor name, e.g. "apache"
    Product string  // NEW: filter by product, e.g. "log4j"
}
```

**File**: `search-service/internal/usecase/cvesearch/usecase.go` (MODIFY)

```go
// CVE entity has vendors[] and products[] arrays
// Filter: WHERE $vendor = ANY(vendors)
if req.Vendor != "" {
    query = query.Where("? = ANY(vendors)", strings.ToLower(req.Vendor))
}
if req.Product != "" {
    query = query.Where("? = ANY(products)", strings.ToLower(req.Product))
}
```

**Migration** (thêm index nếu chưa có):
```sql
CREATE INDEX IF NOT EXISTS idx_cves_vendors  ON cves USING GIN(vendors);
CREATE INDEX IF NOT EXISTS idx_cves_products ON cves USING GIN(products);
```

### 2.5 Repository (search-service)

**File mới**: `search-service/internal/domain/repository/vendor_repo.go`

```go
type VendorRepository interface {
    List(ctx context.Context, q string, limit int) ([]*VendorEntry, int64, error)
    GetProducts(ctx context.Context, vendor string) ([]string, error)
}
```

**Implementation**: `search-service/internal/infra/postgres/vendor_pg.go` — query từ `cpe_dict` table (shared DB với data-service) hoặc materialized view `mv_vendors`.

> **Note**: search-service và data-service đang dùng cùng PostgreSQL database → vendor data có thể query trực tiếp từ `cpe_dict` table.

---

## 3. apps/osv Changes

> **Không thay đổi business logic.**

Gateway routing (cached):
```go
// gateway-service/internal/proxy/ovs_routes.go
{PathPrefix: "/api/v2/vendors",  Upstream: "search-service", SkipAuth: true},
{PathPrefix: "/api/v2/products", Upstream: "search-service", SkipAuth: true},
```

Config:
```yaml
cache:
  vendor_ttl: 3600   # 1 hour (vendors change weekly)
```

---

## 4. Files cần tạo/sửa

### data-service (VERIFY/FIX)
```
internal/fetcher/nvd_cpe.go         ← Verify complete (vendor extraction)
internal/fetcher/redis_cpe_cache.go ← Add RefreshVendorCache
migrations/XXXX_cpe_dict.sql        ← Schema if missing + materialized view
```

### search-service (NEW/MODIFY)
```
internal/delivery/http/vendor_handler.go    ← NEW: vendor/product endpoints
internal/domain/repository/vendor_repo.go   ← NEW: vendor repository interface
internal/infra/postgres/vendor_pg.go        ← NEW: PostgreSQL implementation
internal/usecase/cvesearch/request.go       ← Add Vendor, Product fields
internal/usecase/cvesearch/usecase.go       ← Apply vendor/product filter
internal/delivery/http/search_handler.go    ← Register new routes + parse params
```

### gateway-service (MODIFY)
```
internal/proxy/ovs_routes.go    ← Add /api/v2/vendors, /api/v2/products routes
config/config.yaml               ← vendor_ttl cache config
```

---

## 5. API Spec

```
GET /api/v2/vendors                         → List all vendors (paginated, cached 1h)
GET /api/v2/vendors?q=apach                 → Prefix search vendors
GET /api/v2/vendors/apache/products         → Apache products list
GET /api/v2/products?vendor=apache          → Same, alternative endpoint
GET /api/v2/cves?vendor=apache              → CVEs affecting Apache
GET /api/v2/cves?vendor=apache&product=log4j → CVEs affecting apache/log4j
```

---

## 6. Acceptance Criteria

- [x] `GET /api/v2/vendors` → return ≥ 1000 vendors với `product_count`
- [x] `GET /api/v2/vendors/apache/products` → list Apache products từ CPE dict
- [x] `GET /api/v2/cves?vendor=apache` → CVEs có `vendors` array chứa `apache`
- [x] `GET /api/v2/cves?vendor=apache&product=log4j` → CVEs affect apache/log4j
- [x] CPE sync weekly Sunday 4am — ~1M entries
- [x] Vendor search autocomplete `?q=apach` → suggest `apache`, `apache-hive`...
- [x] Redis CPE cache hit > 80% sau initial load


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Toàn bộ giải pháp đã được triển khai đầy đủ và build verified.
