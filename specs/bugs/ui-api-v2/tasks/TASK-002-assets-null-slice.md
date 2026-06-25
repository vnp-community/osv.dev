# TASK-002 — asset-service: Fix `nil` → `[]` (Assets `null.filter`)

**Bug**: [BUG-005](../BUG-005-assets.md)  
**Solution**: [SOL-005](../solutions/SOL-005-assets-null.md)  
**Priority**: 🔴 P0  
**Effort**: ~10 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/assets` trả `{ "data": null }` khi không có assets. Frontend gọi `data.filter(...)` → crash.

Pattern fix: trong Go, `var x []T` serialize thành JSON `null`. Phải dùng `x := make([]T, 0)` để serialize thành `[]`.

---

## File cần sửa

Tìm handler và repository của asset-service:

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/internal -name "*.go" \
  | xargs grep -l "List\|assets" | head -10
```

**Dự kiến file**:
- `services/asset-service/internal/infra/postgres/asset_repo.go`
- `services/asset-service/internal/delivery/http/asset_handler.go`

---

## Thay đổi — Repository Layer

**Tìm pattern** trong repo List function:

```go
var assets []Asset        // ← SAI: serialize thành null
// hoặc
var assets []*Asset
```

**Thay bằng**:

```go
assets := make([]*Asset, 0)    // ← ĐÚNG: serialize thành []
```

Áp dụng cho **tất cả** slice được return trong `List`, `ListByTag`, `ListByType`:

```go
// Pattern chuẩn cho mọi repo List function:
func (r *AssetRepo) List(ctx context.Context, filter AssetFilter) ([]*Asset, error) {
    rows, err := r.pool.Query(ctx, q, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    assets := make([]*Asset, 0)  // ← never nil
    for rows.Next() {
        a := &Asset{}
        if err := rows.Scan(/* ... */); err != nil {
            return nil, err
        }
        assets = append(assets, a)
    }
    return assets, rows.Err()
}
```

---

## Thay đổi — Handler Layer

**Tìm pattern** trong handler List function:

```go
respondJSON(w, http.StatusOK, map[string]interface{}{
    "data":  result,
    "total": total,
})
```

**Thêm nil guard** trước respond:

```go
if result == nil {
    result = make([]*Asset, 0)
}

respondJSON(w, http.StatusOK, map[string]interface{}{
    "data":  result,   // luôn là []
    "total": len(result),
})
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/assets` trả `{"data": []}` khi không có assets (không phải `{"data": null}`)
- [ ] Trang `/assets` không còn `TypeError: Cannot read properties of null`
- [ ] Hiển thị empty state thay vì crash
- [ ] `go build ./...` trong asset-service không có lỗi

---

## Verify

```bash
# Build
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service
go build ./...

# Test response type
curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/assets" | jq '.data | type'
# Expected: "array"
```
