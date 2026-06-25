# BUG-H2-001 — asset-service: GetAsset trả shell response, không query DB

## Metadata
- **ID**: BUG-H2-001
- **Service**: `asset-service`
- **File**: [`handlers.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/internal/delivery/http/handlers.go)
- **Lines**: 169–177
- **Severity**: 🔴 High
- **Category**: Stub Handler
- **Status**: ✅ Fixed

## Mô tả

Handler `GetAsset` parse `id` từ URL param nhưng không thực hiện bất kỳ query DB nào. Luôn trả về `{"id": "..."}` — một shell response thiếu toàn bộ asset data.

```go
// handlers.go L169-177 — BUG
func (h *Handler) GetAsset(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        jsonError(w, "invalid asset ID", http.StatusBadRequest)
        return
    }
    _ = id   // ← BUG: id parsed nhưng không dùng gì cả!
    jsonResponse(w, http.StatusOK, map[string]string{"id": id.String()})
}
```

## Root Cause

`_ = id` là dấu hiệu rõ ràng của placeholder: `id` được parse để không gây compile error nhưng không được truyền vào use case hay repo.

`AssetCRUDUseCase` không expose `Get(id)`. Tuy nhiên, `assetRepo.FindByID(ctx, id)` đã có trong infra layer. Cần thêm `Get` vào use case và gọi từ handler.

## Tác động

- `GET /api/v1/assets/{id}` không bao giờ trả về asset thực tế
- Frontend asset detail page hiển thị trống hoặc lỗi
- Toàn bộ asset drilldown flow bị broken

## References
- [handlers.go:L169](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/internal/delivery/http/handlers.go#L169-L177)
- [crud.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/internal/usecase/asset/crud.go)
- [asset_repo.go:L31](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/internal/infra/postgres/asset_repo.go#L31)
