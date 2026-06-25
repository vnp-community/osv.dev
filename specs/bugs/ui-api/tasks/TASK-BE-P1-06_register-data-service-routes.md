# TASK-BE-P1-06 — Register CWE/EPSS/Vendor Routes trong data-service

**Phase:** Sprint 2/3 — P1 Missing Routes  
**Nguồn giải pháp:** [`solutions/SOL-005_implement-missing-endpoints.md`](../solutions/SOL-005_implement-missing-endpoints.md)  
**Ưu tiên:** 🟠 P1 — CWE/Vendor/EPSS pages 404  
**Status:** ✅ **DONE** — 2026-06-19
**Phụ thuộc:** TASK-BE-P1-07, TASK-BE-P1-08

---

## Mục tiêu

Gateway routes đã đúng — data-service router chưa đăng ký handlers cho CWE, EPSS, Vendors, CAPEC. Handler code đã có (cwe_handler.go, epss_handler.go, vendor_handler.go) nhưng chưa được register vào router.

---

## Điều tra trước khi code

```bash
# 1. Tìm router file
grep -r "chi.NewRouter\|mux.Handle\|SetupRouter" \
  services/data-service/ --include="*.go" -l

# 2. Xem handlers đã được register chưa
grep -r "\.Get\|\.Post\|\.Handle" \
  services/data-service/internal/delivery/ --include="*.go"

# 3. Kiểm tra taxonomy subdirectory
ls services/data-service/internal/delivery/http/taxonomy/

# 4. Kiểm tra CWE data có trong DB không
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "SELECT COUNT(*) FROM cwe_weaknesses;"
```

---

## Files cần sửa

### [FIND & MODIFY] data-service router setup

```bash
# Tìm file
find services/data-service/ -name "*.go" | \
  xargs grep -l "chi.NewRouter\|NewServeMux\|setupRouter"
```

Thêm các routes vào router function:

```go
// Trong router setup — THÊM:

// === CWE ===
r.Get("/api/v2/cwe", cweHandler.ListCWE)          // Handler: cwe_handler.go
r.Get("/api/v2/cwe/{id}", cweHandler.GetCWEByID)  // Tìm hoặc tạo GetCWEByID

// === CAPEC ===
// Kiểm tra taxonomy/ subpackage
r.Get("/api/v2/capec/{id}", taxonomyHandler.GetCAPEC)

// === EPSS (sau TASK-BE-P1-07) ===
r.Get("/api/v2/epss/top", epssHandler.GetTop)
r.Get("/api/v2/epss/distribution", epssHandler.GetDistribution)
r.Get("/api/v2/epss/{cveId}", epssHandler.GetByCVE)  // ← literal trước wildcard

// === Vendors/Browse (sau TASK-BE-P1-08) ===
r.Get("/api/v2/vendors", vendorHandler.ListVendors)
r.Get("/api/v2/browse", vendorHandler.Browse)
r.Get("/api/v2/browse/{vendor}", vendorHandler.BrowseVendor)
r.Get("/api/v2/browse/{vendor}/{product}", vendorHandler.BrowseProduct)

// === DBInfo ===
r.Get("/api/v2/dbinfo", infoHandler.GetDBInfo)
```

**Quan trọng — thứ tự route (literal trước wildcard):**
```go
// ĐÚNG (literal /epss/top trước wildcard /epss/{cveId}):
r.Get("/api/v2/epss/top", epssHandler.GetTop)
r.Get("/api/v2/epss/distribution", epssHandler.GetDistribution)
r.Get("/api/v2/epss/{cveId}", epssHandler.GetByCVE)

// ĐÚNG (literal /browse trước wildcards):
r.Get("/api/v2/browse", vendorHandler.Browse)
r.Get("/api/v2/browse/{vendor}", vendorHandler.BrowseVendor)
r.Get("/api/v2/browse/{vendor}/{product}", vendorHandler.BrowseProduct)
```

### Thêm GetCWEByID nếu chưa có

```go
// services/data-service/internal/delivery/http/cwe_handler.go — THÊM:
func (h *CWEHandler) GetCWEByID(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id") // e.g. "CWE-79"
    
    item, total, err := h.cweRepo.List(r.Context(), id, 1, 1)
    if err != nil || total == 0 {
        writeJSONData(w, http.StatusNotFound, map[string]string{"error": "CWE not found"})
        return
    }
    writeJSONData(w, http.StatusOK, item[0])
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v2/cwe?page=1&page_size=10` trả HTTP 200 (không còn 404)
- [ ] `GET /api/v2/cwe/CWE-79` trả HTTP 200 hoặc 404 (không phải gateway 404)
- [ ] `GET /api/v2/epss/top?limit=10` trả HTTP 200 (sau P1-07)
- [ ] `GET /api/v2/vendors?q=apache` trả HTTP 200 (sau P1-08)
- [ ] `GET /api/v2/dbinfo` trả HTTP 200

## Verification

```bash
# CWE list
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v2/cwe?page=1&page_size=5"
# Expected: HTTP 200 { "cwe_list": [...], "total": N }

# CWE detail
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v2/cwe/CWE-79
# Expected: HTTP 200 { "cwe_id": "CWE-79", ... }

# EPSS (sau P1-07)
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v2/epss/top?limit=5"
# Expected: HTTP 200

# Vendors (sau P1-08)
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v2/vendors?q=apache"
# Expected: HTTP 200
```
