# BUG-H2-002 — asset-service: GetHistory hardcode empty array

## Metadata
- **ID**: BUG-H2-002
- **Service**: `asset-service`
- **File**: [`handlers.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/internal/delivery/http/handlers.go)
- **Lines**: 222–225
- **Severity**: 🟡 Medium
- **Category**: Stub Handler
- **Status**: ✅ Fixed

## Mô tả

Handler `GetHistory` luôn trả về mảng rỗng mà không có bất kỳ logic query nào.

```go
// handlers.go L222-225 — BUG
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
    jsonResponse(w, http.StatusOK, []interface{}{})
}
```

## Tác động

- `GET /api/v1/assets/{id}/history` không bao giờ trả lịch sử thay đổi
- Audit trail của asset bị mất hoàn toàn

## References
- [handlers.go:L222](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/internal/delivery/http/handlers.go#L222-L225)
