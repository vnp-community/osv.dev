# TASK-BE-P1-03 — Fix Assets & Products Response Schema

**Phase:** Sprint 2 — P1 Schema Fixes  
**Nguồn giải pháp:** [`solutions/SOL-004_fix-schema-mismatches.md — FIX 4, 5`](../solutions/SOL-004_fix-schema-mismatches.md)  
**Ưu tiên:** 🟠 P1 — Trang Assets/Products không render  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19

---

## Mục tiêu

- `GET /api/v1/assets` trả object hoặc array trực tiếp → cần wrap trong `{ "assets": [...], "total": N }`
- `GET /api/v1/products` trả array trực tiếp → cần wrap trong `{ "products": [...], "total": N }`
- `GET /api/v1/products/types` trả `[{"id":1,"name":"web_application"}]` → cần trả `{ "types": ["web_application", ...] }`

---

## Files cần sửa

### Part A — Asset-service

```bash
# Tìm asset handler
grep -r "ListAssets\|/api/v1/assets\|assets" \
  services/asset-service/ --include="*.go" -l

ls services/asset-service/internal/
```

### [FIND & MODIFY] `services/asset-service/internal/delivery/http/asset_handler.go`

```go
// GET /api/v1/assets — FIX wrapper
func (h *AssetHandler) ListAssets(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    filter := AssetFilter{
        RiskLevel: q.Get("riskLevel"),
        Tags:      q["tags"],
        Page:      parseInt(q.Get("page"), 1),
        PageSize:  parseInt(q.Get("page_size"), 20),
    }

    assets, total, err := h.assetRepo.List(r.Context(), filter)
    if err != nil {
        log.Error().Err(err).Msg("list assets failed")
        respondError(w, http.StatusInternalServerError, "failed to list assets")
        return
    }

    // PHẢI wrap: { "assets": [...], "total": N }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "assets": assets,
        "total":  total,
    })
}
```

Kiểm tra các method cần có trong AssetRepository:
```bash
grep -r "type.*Repository interface\|AssetRepository" \
  services/asset-service/ --include="*.go"
```

### Part B — Finding-service (Products + Types)

```bash
# Tìm product handler trong finding-service
grep -r "ListProducts\|GetTypes\|/api/v1/products" \
  services/finding-service/ --include="*.go" -l
```

### [FIND & MODIFY] `services/finding-service/internal/delivery/http/` — product handler

```go
// GET /api/v1/products — FIX wrapper
func (h *ProductHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    page     := parseInt(q.Get("page"), 1)
    pageSize := parseInt(q.Get("page_size"), 20)
    filter   := q.Get("filter")

    products, total, err := h.productRepo.List(r.Context(), filter, page, pageSize)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list products")
        return
    }

    // PHẢI wrap: { "products": [...], "total": N }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "products": products,
        "total":    total,
    })
}

// GET /api/v1/products/types — trả STRING ARRAY
func (h *ProductHandler) GetProductTypes(w http.ResponseWriter, r *http.Request) {
    // Spec yêu cầu { "types": ["web_application", "api", ...] }
    // KHÔNG phải [{ "id": 1, "name": "web_application" }]

    // Option A: từ DB (nếu có product_types table)
    dbTypes, err := h.productRepo.GetTypes(r.Context())
    if err == nil && len(dbTypes) > 0 {
        typeNames := make([]string, len(dbTypes))
        for i, t := range dbTypes {
            typeNames[i] = t.Name // chỉ lấy Name string
        }
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "types": typeNames,
        })
        return
    }

    // Option B: hardcode default types (per architecture §3.5)
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "types": []string{
            "web_application",
            "api",
            "mobile",
            "infrastructure",
            "iot",
            "network",
        },
    })
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/assets` response có `{ "assets": [...], "total": N }` (không phải flat object)
- [ ] `GET /api/v1/products` response có `{ "products": [...], "total": N }` (không phải array)
- [ ] `GET /api/v1/products/types` response có `{ "types": ["web_application", "api", ...] }` (string array, không phải object array)
- [ ] Trang `/assets` trong UI render được (không còn `response.assets is undefined`)

## Verification

```bash
# Assets wrapper
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/assets | jq 'keys'
# Expected: ["assets", "total"]

# Products wrapper
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/products | jq 'keys'
# Expected: ["products", "total"]

# Types — string array
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/products/types | jq '.types[0]'
# Expected: "web_application" (string, không phải object)
```
