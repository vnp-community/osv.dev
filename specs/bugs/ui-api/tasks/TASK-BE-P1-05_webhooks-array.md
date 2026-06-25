# TASK-BE-P1-05 — Fix Webhooks Response Format

**Phase:** Sprint 2 — P1 Schema Fixes  
**Nguồn giải pháp:** [`solutions/SOL-004_fix-schema-mismatches.md — FIX 9`](../solutions/SOL-004_fix-schema-mismatches.md)  
**Ưu tiên:** 🟠 P1 — Integrations page crash  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19

---

## Mục tiêu

`GET /api/v1/webhooks` trả `{ "webhooks": [...] }` (object) thay vì `[...]` (array trực tiếp) như spec yêu cầu. Frontend `response.map is not a function`.

---

## Files cần sửa

### [FIND & MODIFY] `services/notification-service` — webhooks handler

```bash
# Tìm webhooks handler
grep -r "ListWebhooks\|GetWebhooks\|/api/v1/webhooks\|webhooks" \
  services/notification-service/ --include="*.go" -l

# Xem nội dung handler
grep -r "webhooks" services/notification-service/internal/ --include="*.go" -A 10
```

```go
// TRƯỚC — trả object wrapper (sai spec):
func (h *WebhookHandler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
    webhooks, err := h.webhookRepo.List(r.Context())
    // ...
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "webhooks": webhooks,  // ← BỊ WRAP
    })
}

// SAU — trả ARRAY trực tiếp theo spec:
func (h *WebhookHandler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
    webhooks, err := h.webhookRepo.List(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list webhooks")
        return
    }
    if webhooks == nil {
        webhooks = []WebhookResponse{} // empty array, không phải null
    }
    // Trả ARRAY trực tiếp
    respondJSON(w, http.StatusOK, webhooks)
}
```

**Webhook response type** (theo spec):
```go
type WebhookResponse struct {
    ID        string   `json:"id"`
    Name      string   `json:"name"`
    URL       string   `json:"url"`
    Events    []string `json:"events"`   // ["scan.completed", "finding.created"]
    Secret    string   `json:"secret"`   // masked: "***"
    Active    bool     `json:"active"`
    CreatedAt string   `json:"created_at"`
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/webhooks` trả HTTP 200 với **JSON array** trực tiếp
- [ ] `response[0].id` hoạt động (không phải `response.webhooks[0].id`)
- [ ] Response `[]` (empty array) khi chưa có webhook (không phải `null`)

## Verification

```bash
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/webhooks | jq 'type'
# Expected: "array"
# NOT: "object"

curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/webhooks | jq '.[0] | keys'
# Expected: ["active", "created_at", "events", "id", "name", "secret", "url"]
```

---

# TASK-BE-P1-06 — Register CWE/EPSS/Vendor Routes trong data-service

**Phase:** Sprint 2 — P1 Missing Routes  
**Nguồn giải pháp:** [`solutions/SOL-005_implement-missing-endpoints.md`](../solutions/SOL-005_implement-missing-endpoints.md)  
**Ưu tiên:** 🟠 P1 — CWE/Vendor/EPSS pages 404  
**Phụ thuộc:** TASK-BE-P1-07 (EPSS handler), TASK-BE-P1-08 (Vendor handler)

---

## Mục tiêu

Gateway routes đã đúng (routes đã mount). Vấn đề là data-service router chưa đăng ký các handlers cho CWE, EPSS, Vendors, CAPEC. Handler code đã có (cwe_handler.go, epss_handler.go...) nhưng chưa được register.

---

## Files cần sửa

### [FIND & MODIFY] data-service router

```bash
# Tìm router file
grep -r "chi.NewRouter\|mux.Handle\|r\.Get\b\|r\.Post\b" \
  services/data-service/ --include="*.go" -l

# Xem router setup
grep -r "SetupRouter\|NewRouter\|setupRoutes" \
  services/data-service/ --include="*.go" -l
```

Thêm các routes còn thiếu vào router:

```go
// services/data-service/internal/delivery/http/router.go (hoặc tương đương)
// THÊM các dòng sau vào hàm setup router:

// CWE — handler đã có trong cwe_handler.go
r.Get("/api/v2/cwe", cweHandler.ListCWE)           // THÊM
r.Get("/api/v2/cwe/{id}", taxonomyHandler.GetCWE)  // THÊM (nếu taxonomy handler có)

// CAPEC — cần implement hoặc dùng taxonomy handler
r.Get("/api/v2/capec/{id}", taxonomyHandler.GetCAPEC) // THÊM

// EPSS — sau khi TASK-BE-P1-07 tạo xong
r.Get("/api/v2/epss/top", epssHandler.GetTop)               // THÊM
r.Get("/api/v2/epss/distribution", epssHandler.GetDistribution) // THÊM
r.Get("/api/v2/epss/{cveId}", epssHandler.GetByCVE)          // THÊM

// Vendors + Browse — sau khi TASK-BE-P1-08 tạo xong
r.Get("/api/v2/vendors", vendorHandler.ListVendors)            // THÊM
r.Get("/api/v2/browse", vendorHandler.Browse)                   // THÊM
r.Get("/api/v2/browse/{vendor}", vendorHandler.BrowseVendor)    // THÊM
r.Get("/api/v2/browse/{vendor}/{product}", vendorHandler.BrowseProduct) // THÊM

// DBInfo — kiểm tra có handler chưa
r.Get("/api/v2/dbinfo", infoHandler.GetDBInfo)  // THÊM nếu chưa có
```

Kiểm tra taxonomy handler directory:
```bash
ls services/data-service/internal/delivery/http/taxonomy/
grep -r "GetCWE\|GetCAPEC" services/data-service/ --include="*.go"
```

---

## Acceptance Criteria

- [ ] `GET /api/v2/cwe?page=1&page_size=10` trả HTTP 200 (không còn 404)
- [ ] `GET /api/v2/capec/CAPEC-66` trả HTTP 200 hoặc 404 (không còn endpoint 404)
- [ ] `GET /api/v2/epss/top?limit=10` trả HTTP 200 (cần TASK-BE-P1-07 hoàn thành)
- [ ] `GET /api/v2/vendors?q=apache` trả HTTP 200 (cần TASK-BE-P1-08 hoàn thành)

## Verification

```bash
# CWE
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v2/cwe?page=1&page_size=5"
# Expected: HTTP 200 (không phải 404)

# CAPEC
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v2/capec/CAPEC-66
# Expected: HTTP 200 hoặc 404 (not found) — không phải gateway 404
```
