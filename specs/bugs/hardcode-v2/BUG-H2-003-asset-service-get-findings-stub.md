# BUG-H2-003 — asset-service: GetFindings hardcode empty array

## Metadata
- **ID**: BUG-H2-003
- **Service**: `asset-service`
- **File**: [`handlers.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/internal/delivery/http/handlers.go)
- **Lines**: 279–282
- **Severity**: 🟡 Medium
- **Category**: Stub Handler
- **Status**: ✅ Fixed

## Mô tả

Handler `GetFindings` luôn trả về mảng rỗng.

```go
// handlers.go L279-282 — BUG
func (h *Handler) GetFindings(w http.ResponseWriter, r *http.Request) {
    jsonResponse(w, http.StatusOK, []interface{}{})
}
```

## Tác động

- `GET /api/v1/assets/{id}/findings` luôn trả `[]`
- UI Asset page không hiển thị vulnerability findings cho asset

## Fix

Gọi finding-service qua proxy HTTP hoặc gRPC client để lấy findings theo `asset_id`.
Nếu chưa có FindingClient, trả `[]` kèm header `X-Data-Source: not-implemented` để frontend biết.

## References
- [handlers.go:L279](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/internal/delivery/http/handlers.go#L279-L282)
