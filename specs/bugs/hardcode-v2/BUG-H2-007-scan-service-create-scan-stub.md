# BUG-H2-007 — scan-service: CreateScan trả 503 hardcode

## Metadata
- **ID**: BUG-H2-007
- **Service**: `scan-service`
- **File**: [`router.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/delivery/http/router.go)
- **Lines**: ~L120 (trước khi fix)
- **Severity**: 🔴 High
- **Category**: Stub Handler
- **Status**: ✅ Fixed (2026-06-24 — previous session)

## Mô tả

`CreateScan` handler trả cứng `{"error":"scan_service_not_ready","message":"..."}` với `503 Service Unavailable` ngay cả khi tất cả dependencies đã được inject đúng.

```go
// BUG: handler luôn trả 503 không phụ thuộc vào state
func (h *ScanAPIHandler) CreateScan(w http.ResponseWriter, r *http.Request) {
    writeScanJSON(w, http.StatusServiceUnavailable, map[string]string{
        "error":   "scan_service_not_ready",
        "message": "Scan service backend not fully initialized",
    })
}
```

## Fix Applied

- Implement `CreateScan` handler thực sự với JSON decode, user ID extraction, DB insert.
- Tạo `createScanUCAdapter` để bridge delivery layer với `ScanRepo`.
- Wire adapter vào `embedded.go`.
- Export `ParseScanType` để dùng trong adapter.

## References
- [create_scan_adapter.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/create_scan_adapter.go)
- [router.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/delivery/http/router.go)
