# SOL-005 — Implement Missing Endpoints (BUG-BE-010)

| Trường | Giá trị |
|---|---|
| **Bug** | [BUG-BE-010](../BUG-BE-010_missing-endpoints-404.md) |
| **Services** | `services/data-service` (CWE/CAPEC/EPSS/Vendors), `apps/osv` (BFF) |
| **Priority** | P1 |
| **Estimated effort** | 16–24h |
| **Kiến trúc** | [architecture.md §3.2](../../../01-architecture.md) — data-service là nơi serve CWE/CAPEC/EPSS/Vendor data |

---

## Phân Tích Hiện Tại

Từ `apps/osv/internal/gateway/router.go`, tất cả routes đã được mount (Sprint 4):

```go
// Line 228-235 — ĐÃ CÓ trong gateway
mux.Handle("GET /api/v2/epss/top", ...)      // → data-service:8082
mux.Handle("GET /api/v2/epss/distribution", ...) // → data-service:8082
mux.Handle("GET /api/v2/epss/{cveId}", ...)  // → data-service:8082
mux.Handle("GET /api/v2/cwe", ...)           // → data-service:8082
mux.Handle("GET /api/v2/cwe/{id}", ...)      // → data-service:8082
mux.Handle("GET /api/v2/capec/{id}", ...)    // → data-service:8082
mux.Handle("GET /api/v2/vendors", ...)       // → data-service:8082
```

**Kết luận**: Gateway routes **đã đúng**. Vấn đề là `data-service:8082` chưa implement các HTTP handlers tương ứng.

Từ `services/data-service/internal/delivery/http/`:
- `cwe_handler.go` — `CWEHandler.ListCWE` đã có nhưng **chưa được register vào router**
- `vendor_handler.go` — có file nhưng cần kiểm tra
- `epss_handler.go` — cần kiểm tra
- Taxonomy (CWE/{id}, CAPEC/{id}) — trong `taxonomy/` subdirectory

---

## Giải Pháp

### Step 1 — Kiểm tra data-service router

```bash
# Tìm router setup trong data-service
grep -r "chi.NewRouter\|mux.Handle\|r.Get\|r.Post" \
  services/data-service/internal/ --include="*.go" -l
```

### Step 2 — Register handlers trong data-service router

Tìm file router của data-service và thêm các routes còn thiếu:

```go
// services/data-service/internal/delivery/http/router.go (hoặc tương đương)
// THÊM các routes chưa có:

func SetupRouter(
    cveHandler *CVEHandler,
    kevHandler *KevHandler,
    cweHandler *CWEHandler,
    epssHandler *EPSSHandler,
    vendorHandler *VendorHandler,
    taxonomyHandler *TaxonomyHandler,
) http.Handler {
    r := chi.NewRouter()

    // KEV — đã có
    r.Get("/api/v2/kev", kevHandler.ListKEV)
    r.Get("/api/v2/kev/stats", kevHandler.GetStats)
    r.Get("/api/v2/kev/ransomware", kevHandler.GetRansomware)
    r.Get("/api/v2/kev/{cveId}", kevHandler.GetKEVEntry)

    // CWE — CÓ handler, CẦN REGISTER
    r.Get("/api/v2/cwe", cweHandler.ListCWE)              // ← THÊM
    r.Get("/api/v2/cwe/{id}", taxonomyHandler.GetCWE)     // ← THÊM

    // CAPEC — CẦN REGISTER
    r.Get("/api/v2/capec/{id}", taxonomyHandler.GetCAPEC) // ← THÊM

    // EPSS — CẦN REGISTER
    r.Get("/api/v2/epss/top", epssHandler.GetTop)         // ← THÊM
    r.Get("/api/v2/epss/distribution", epssHandler.GetDistribution) // ← THÊM
    r.Get("/api/v2/epss/{cveId}", epssHandler.GetByCVE)   // ← THÊM

    // Vendors — CẦN REGISTER
    r.Get("/api/v2/vendors", vendorHandler.ListVendors)   // ← THÊM
    r.Get("/api/v2/browse", vendorHandler.Browse)         // ← THÊM
    r.Get("/api/v2/browse/{vendor}", vendorHandler.BrowseVendor)    // ← THÊM
    r.Get("/api/v2/browse/{vendor}/{product}", vendorHandler.BrowseProduct) // ← THÊM

    // DBInfo — CẦN REGISTER
    r.Get("/api/v2/dbinfo", infoHandler.GetDBInfo)        // ← THÊM

    return r
}
```

### Step 3 — Implement EPSSHandler (nếu còn thiếu)

File: `services/data-service/internal/delivery/http/epss_handler.go`

Dựa trên architecture, EPSS data được fetch daily từ FIRST.org và stored trong PostgreSQL (`cves` table với `epss_score`, `epss_percentile`).

```go
// services/data-service/internal/delivery/http/epss_handler.go
package http

import (
    "net/http"
    "strconv"
)

type EPSSHandler struct {
    cveRepo CVERepository // có epss_score, epss_percentile fields
}

// GET /api/v2/epss/top?limit=20
// Returns top N CVEs by EPSS score
func (h *EPSSHandler) GetTop(w http.ResponseWriter, r *http.Request) {
    limit := 20
    if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 100 {
        limit = l
    }

    results, err := h.cveRepo.GetTopEPSS(r.Context(), limit)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to get top EPSS")
        return
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":  results,
        "total": len(results),
        "limit": limit,
    })
}

// GET /api/v2/epss/distribution
// Returns histogram of EPSS score distribution
func (h *EPSSHandler) GetDistribution(w http.ResponseWriter, r *http.Request) {
    // Phân phối theo buckets: 0-0.1, 0.1-0.2, ..., 0.9-1.0
    buckets, err := h.cveRepo.GetEPSSDistribution(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to get EPSS distribution")
        return
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "distribution": buckets,
        "model_date":   "current",
    })
}

// GET /api/v2/epss/{cveId}
func (h *EPSSHandler) GetByCVE(w http.ResponseWriter, r *http.Request) {
    cveID := chi.URLParam(r, "cveId")
    result, err := h.cveRepo.GetEPSSByCVE(r.Context(), cveID)
    if err != nil {
        respondError(w, http.StatusNotFound, "CVE not found or no EPSS data")
        return
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "cve_id":     cveID,
        "epss_score": result.EPSSScore,
        "percentile": result.EPSSPercentile,
        "model_date": result.ModelDate,
    })
}
```

**Repository method** cần thêm:
```go
// services/data-service/internal/infra/postgres/cve_repo.go
func (r *PostgresCVERepository) GetTopEPSS(ctx context.Context, limit int) ([]EPSSEntry, error) {
    rows, err := r.db.Query(ctx, `
        SELECT cve_id, epss_score, epss_percentile, severity_v3, description
        FROM cves
        WHERE epss_score IS NOT NULL
        ORDER BY epss_score DESC
        LIMIT $1
    `, limit)
    // ...
}
```

### Step 4 — Implement VendorHandler (nếu còn thiếu)

File: `services/data-service/internal/delivery/http/vendor_handler.go`

Vendors và browse endpoints dùng data từ `cves` table (vendor, product fields) hoặc `cpe_dictionary`.

```go
// services/data-service/internal/delivery/http/vendor_handler.go
package http

import "net/http"

type VendorHandler struct {
    cveRepo CVERepository
}

// GET /api/v2/vendors?q=apache&limit=20
func (h *VendorHandler) ListVendors(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query().Get("q")
    limit := parseInt(r.URL.Query().Get("limit"), 50)

    vendors, err := h.cveRepo.GetVendors(r.Context(), q, limit)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list vendors")
        return
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "vendors": vendors,
        "total":   len(vendors),
    })
}

// GET /api/v2/browse?vendor=apache — list products for vendor
func (h *VendorHandler) Browse(w http.ResponseWriter, r *http.Request) {
    vendor := r.URL.Query().Get("vendor")
    products, err := h.cveRepo.GetProductsByVendor(r.Context(), vendor)
    // ...
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "vendor":   vendor,
        "products": products,
    })
}
```

**Repository SQL** (từ `cves` table):
```sql
-- GetVendors: distinct vendors matching query
SELECT DISTINCT vendor, COUNT(*) as cve_count
FROM cves
WHERE vendor ILIKE '%' || $1 || '%'
GROUP BY vendor
ORDER BY cve_count DESC
LIMIT $2;

-- GetProductsByVendor
SELECT DISTINCT product, COUNT(*) as cve_count
FROM cves
WHERE vendor = $1
GROUP BY product
ORDER BY cve_count DESC;
```

### Step 5 — Đảm bảo CWE data được sync

CWE data cần được sync từ MITRE (weekly, theo architecture §3.2 `CWEFetcher`):

```bash
# Kiểm tra CWE table có data không:
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "SELECT COUNT(*) FROM cwe_weaknesses;"

# Nếu 0, trigger manual sync:
curl -X POST http://localhost:8082/internal/cwe/sync
```

---

## Xác Nhận Fix

```bash
# CWE list
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v2/cwe?page=1&page_size=10"
# Expected: HTTP 200 { "cwe_list": [...], "total": N }

# EPSS top
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v2/epss/top?limit=10"
# Expected: HTTP 200 { "data": [...], "total": 10 }

# Vendors
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v2/vendors?q=apache"
# Expected: HTTP 200 { "vendors": [...], "total": N }

# CAPEC
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v2/capec/CAPEC-66
# Expected: HTTP 200 với CAPEC detail
```

## Không Thay Đổi

- `apps/osv/internal/gateway/router.go` — routes đã đúng, KHÔNG sửa
- nginx config — KHÔNG sửa

## Ưu Tiên Implementation

| Endpoint | Ưu tiên | Lý do |
|---|---|---|
| `GET /api/v2/vendors` | P1 | Vendor Browse page bị broken |
| `GET /api/v2/cwe` | P1 | CWE Library page bị broken |
| `GET /api/v2/epss/top` | P1 | EPSS Analytics page bị broken |
| `GET /api/v2/epss/{id}` | P2 | Dùng trong CVE detail |
| `GET /api/v2/capec/{id}` | P2 | CAPEC không click được |
| `GET /api/v2/epss/distribution` | P3 | Chart data |
| `GET /api/v2/cves/export` | P3 | Export feature |
