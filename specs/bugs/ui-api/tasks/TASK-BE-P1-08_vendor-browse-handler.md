# TASK-BE-P1-08 — Implement Vendor/Browse Handler (data-service)

**Phase:** Sprint 3 — P1 Implement Missing  
**Nguồn giải pháp:** [`solutions/SOL-005_implement-missing-endpoints.md — Step 4`](../solutions/SOL-005_implement-missing-endpoints.md)  
**Ưu tiên:** 🟠 P1 — Vendor Browse page 404  
**Status:** ✅ **DONE** — 2026-06-19
**Phụ thuộc:** Không có

---

## Mục tiêu

Implement `VendorHandler` cho `GET /api/v2/vendors` và `GET /api/v2/browse/*`. Data đến từ `cves` table trong PostgreSQL (distinct vendor, product fields).

---

## Files cần tạo/sửa

### [CHECK/MODIFY] `services/data-service/internal/delivery/http/vendor_handler.go`

```bash
# Kiểm tra file hiện tại
cat services/data-service/internal/delivery/http/vendor_handler.go
```

Implement đầy đủ:

```go
// services/data-service/internal/delivery/http/vendor_handler.go
package http

import (
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "github.com/rs/zerolog"
)

type VendorRepository interface {
    GetVendors(ctx context.Context, q string, limit int) ([]VendorEntry, error)
    GetProductsByVendor(ctx context.Context, vendor string, page, pageSize int) ([]ProductEntry, int, error)
    GetCVEsByVendorProduct(ctx context.Context, vendor, product string, page, pageSize int) ([]CVEBrowseEntry, int, error)
}

type VendorEntry struct {
    Vendor   string `json:"vendor"`
    CVECount int64  `json:"cve_count"`
}

type ProductEntry struct {
    Product  string `json:"product"`
    Vendor   string `json:"vendor"`
    CVECount int64  `json:"cve_count"`
}

type CVEBrowseEntry struct {
    CveID      string  `json:"cve_id"`
    SeverityV3 string  `json:"severity_v3"`
    CVSSScore  float64 `json:"cvss_v3_score"`
    IsKEV      bool    `json:"is_kev"`
    PublishedAt string `json:"published_at"`
}

type VendorHandler struct {
    vendorRepo VendorRepository
    log        zerolog.Logger
}

func NewVendorHandler(repo VendorRepository, log zerolog.Logger) *VendorHandler {
    return &VendorHandler{vendorRepo: repo, log: log}
}

// GET /api/v2/vendors?q=apache&limit=50
func (h *VendorHandler) ListVendors(w http.ResponseWriter, r *http.Request) {
    q     := r.URL.Query().Get("q")
    limit := 50
    if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 200 {
        limit = l
    }

    vendors, err := h.vendorRepo.GetVendors(r.Context(), q, limit)
    if err != nil {
        h.log.Error().Err(err).Msg("list vendors failed")
        respondError(w, http.StatusInternalServerError, "failed to list vendors")
        return
    }
    if vendors == nil {
        vendors = []VendorEntry{}
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "vendors": vendors,
        "total":   len(vendors),
    })
}

// GET /api/v2/browse?vendor=apache — list vendors summary
func (h *VendorHandler) Browse(w http.ResponseWriter, r *http.Request) {
    // Alias: same as ListVendors but filtered by vendor query param
    vendor := r.URL.Query().Get("vendor")
    vendors, err := h.vendorRepo.GetVendors(r.Context(), vendor, 50)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "browse failed")
        return
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "vendors": vendors,
        "total":   len(vendors),
    })
}

// GET /api/v2/browse/{vendor} — list products for a specific vendor
func (h *VendorHandler) BrowseVendor(w http.ResponseWriter, r *http.Request) {
    vendor   := chi.URLParam(r, "vendor")
    page     := parseInt(r.URL.Query().Get("page"), 1)
    pageSize := parseInt(r.URL.Query().Get("page_size"), 20)

    products, total, err := h.vendorRepo.GetProductsByVendor(r.Context(), vendor, page, pageSize)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list products")
        return
    }
    if products == nil {
        products = []ProductEntry{}
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "vendor":    vendor,
        "products":  products,
        "total":     total,
        "page":      page,
        "page_size": pageSize,
    })
}

// GET /api/v2/browse/{vendor}/{product} — list CVEs for vendor/product
func (h *VendorHandler) BrowseProduct(w http.ResponseWriter, r *http.Request) {
    vendor  := chi.URLParam(r, "vendor")
    product := chi.URLParam(r, "product")
    page     := parseInt(r.URL.Query().Get("page"), 1)
    pageSize := parseInt(r.URL.Query().Get("page_size"), 20)

    cves, total, err := h.vendorRepo.GetCVEsByVendorProduct(r.Context(), vendor, product, page, pageSize)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list CVEs")
        return
    }
    if cves == nil {
        cves = []CVEBrowseEntry{}
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "vendor":    vendor,
        "product":   product,
        "cves":      cves,
        "total":     total,
        "page":      page,
        "page_size": pageSize,
    })
}
```

### [FIND & MODIFY] PostgreSQL repository — thêm Vendor methods

Thêm vào CVE repo hoặc tạo vendor-specific repo:

```go
// services/data-service/internal/infra/postgres/vendor_repo.go

func (r *PostgresVendorRepo) GetVendors(ctx context.Context, q string, limit int) ([]VendorEntry, error) {
    query := `
        SELECT vendor, COUNT(*) as cve_count
        FROM cves
        WHERE vendor IS NOT NULL AND vendor != ''
          AND ($1 = '' OR vendor ILIKE '%' || $1 || '%')
        GROUP BY vendor
        ORDER BY cve_count DESC
        LIMIT $2
    `
    rows, err := r.db.Query(ctx, query, q, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var vendors []VendorEntry
    for rows.Next() {
        var v VendorEntry
        rows.Scan(&v.Vendor, &v.CVECount)
        vendors = append(vendors, v)
    }
    return vendors, nil
}

func (r *PostgresVendorRepo) GetProductsByVendor(ctx context.Context, vendor string, page, pageSize int) ([]ProductEntry, int, error) {
    offset := (page - 1) * pageSize
    
    var total int
    r.db.QueryRow(ctx, `
        SELECT COUNT(DISTINCT product) FROM cves 
        WHERE vendor = $1 AND product IS NOT NULL
    `, vendor).Scan(&total)

    rows, err := r.db.Query(ctx, `
        SELECT product, vendor, COUNT(*) as cve_count
        FROM cves
        WHERE vendor = $1 AND product IS NOT NULL AND product != ''
        GROUP BY product, vendor
        ORDER BY cve_count DESC
        LIMIT $2 OFFSET $3
    `, vendor, pageSize, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var products []ProductEntry
    for rows.Next() {
        var p ProductEntry
        rows.Scan(&p.Product, &p.Vendor, &p.CVECount)
        products = append(products, p)
    }
    return products, total, nil
}

func (r *PostgresVendorRepo) GetCVEsByVendorProduct(ctx context.Context, vendor, product string, page, pageSize int) ([]CVEBrowseEntry, int, error) {
    offset := (page - 1) * pageSize
    
    var total int
    r.db.QueryRow(ctx, `
        SELECT COUNT(*) FROM cves WHERE vendor = $1 AND product = $2
    `, vendor, product).Scan(&total)

    rows, err := r.db.Query(ctx, `
        SELECT cve_id, severity_v3, cvss_v3_score, is_kev,
               TO_CHAR(published_at, 'YYYY-MM-DD') as published_at
        FROM cves
        WHERE vendor = $1 AND product = $2
        ORDER BY published_at DESC
        LIMIT $3 OFFSET $4
    `, vendor, product, pageSize, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var cves []CVEBrowseEntry
    for rows.Next() {
        var c CVEBrowseEntry
        rows.Scan(&c.CveID, &c.SeverityV3, &c.CVSSScore, &c.IsKEV, &c.PublishedAt)
        cves = append(cves, c)
    }
    return cves, total, nil
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v2/vendors?q=apache` trả HTTP 200 `{ "vendors": [...], "total": N }`
- [ ] `GET /api/v2/browse/apache` trả HTTP 200 `{ "vendor": "apache", "products": [...] }`
- [ ] `GET /api/v2/browse/apache/log4j` trả HTTP 200 `{ "cves": [...], "total": N }`
- [ ] Data thực từ DB (không hardcode)

## Verification

```bash
# Vendors search
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v2/vendors?q=microsoft" | jq '.vendors[0]'
# Expected: { "vendor": "microsoft", "cve_count": N }

# Browse vendor
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v2/browse/apache | jq 'keys'
# Expected: ["page", "page_size", "products", "total", "vendor"]

# Browse product
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v2/browse/apache/log4j" | jq '.total'
# Expected: N > 0 (nếu có data)
```
