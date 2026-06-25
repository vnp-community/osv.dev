# SOL-005 — Asset Inventory: API trả `null` thay vì `[]` (P0 — CRITICAL)

**Bug**: [BUG-005](../BUG-005-assets.md)  
**Service**: `asset-service` (`services/asset-service`)  
**Endpoint**: `GET /api/v1/assets`  
**Lỗi frontend**: `TypeError: Cannot read properties of null (reading 'filter')`

**Status**: `✅ Implemented` — via [TASK-002](../../tasks/TASK-002-*.md)

---

## Root Cause Analysis

### Routing (Gateway)

Route đúng trong [`router.go:315`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L315):

```go
mux.Handle("GET /api/v1/assets", protected(proxy.Forward("asset-service:8091")))
```

### Vấn đề

Asset-service đang trả response với field `assets` hoặc `data` là `null` khi không có assets, thay vì `[]` (empty array). Frontend component `AssetInventory` gọi `.filter()` trực tiếp trên giá trị nhận được → crash.

---

## Giải pháp

### Bước 1: Kiểm tra response hiện tại

```bash
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/assets"
```

### Bước 2: Tìm và sửa trong asset-service

Tìm handler List trong asset-service:

```
services/asset-service/internal/delivery/http/
```

**Pattern Fix — Repository Layer**

```go
// services/asset-service/internal/infra/postgres/asset_repo.go (hoặc tương đương)

func (r *AssetRepo) List(ctx context.Context, filter AssetFilter) (*AssetListResult, error) {
    rows, err := r.pool.Query(ctx, q, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    // FIX: khởi tạo slice ngay từ đầu, không dùng var assets []Asset
    assets := make([]*Asset, 0)   // ← khởi tạo empty slice, không phải nil

    for rows.Next() {
        a := &Asset{}
        if err := rows.Scan(/* ... */); err != nil {
            return nil, err
        }
        assets = append(assets, a)
    }

    return &AssetListResult{
        Assets: assets,  // ← luôn là [], không bao giờ null
        Total:  len(assets),
    }, rows.Err()
}
```

**Pattern Fix — Handler Layer**

```go
// services/asset-service/internal/delivery/http/asset_handler.go

func (h *AssetHandler) List(w http.ResponseWriter, r *http.Request) {
    result, err := h.repo.List(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list assets")
        return
    }

    // FIX: defensive nil check
    if result == nil {
        result = &AssetListResult{Assets: make([]*Asset, 0)}
    }
    if result.Assets == nil {
        result.Assets = make([]*Asset, 0)
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":       result.Assets,  // luôn là []
        "total":      result.Total,
        "pagination": result.Pagination,
    })
}
```

### Bước 3: Kiểm tra response schema

Response **phải** theo format:

```json
{
  "data": [],
  "total": 0,
  "pagination": {
    "page": 1,
    "page_size": 50,
    "total": 0,
    "total_pages": 0
  }
}
```

**Không được** trả:
```json
{
  "data": null,
  "total": 0
}
```

### Bước 4: Fix migration nếu schema chưa khớp

```bash
# Kiểm tra asset table
docker exec -it postgres psql -U postgres -d osv -c "
  SELECT column_name, data_type, is_nullable
  FROM information_schema.columns
  WHERE table_schema = 'osv_asset'
    AND table_name = 'assets'
  ORDER BY ordinal_position;
"
```

---

## Files cần sửa

| File | Thay đổi |
|------|----------|
| `services/asset-service/internal/infra/postgres/asset_repo.go` | `var assets []Asset` → `assets := make([]*Asset, 0)` |
| `services/asset-service/internal/delivery/http/asset_handler.go` | Thêm nil guard trước khi respond |

---

## Verification

```bash
# Sau fix — phải trả 200 với data là []
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/assets" | jq '.data | type'
# Expected: "array"

# Không được là:
# Expected: "null"
```
