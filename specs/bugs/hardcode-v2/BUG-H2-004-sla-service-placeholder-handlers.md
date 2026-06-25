# BUG-H2-004 — sla-service: Placeholder handlers hardcode 0/501 trong main.go

## Metadata
- **ID**: BUG-H2-004
- **Service**: `sla-service`
- **File**: [`main.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/sla-service/cmd/server/main.go)
- **Lines**: 128–180
- **Severity**: 🔴 High
- **Category**: Placeholder Handler
- **Status**: ✅ Fixed

## Mô tả

5 handler functions trong `main.go` được mount vào `/api/v2/sla-configurations/*` nhưng tất cả trả về hardcode 0 hoặc 501. `pool *pgxpool.Pool` được truyền vào nhưng không dùng gì cả.

```go
// main.go L131-180 — BUG
func listSLAConfigsHandler(pool *pgxpool.Pool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, `{"count":0,"results":[]}`)  // ← hardcode 0, không query!
    }
}
func createSLAConfigHandler(pool *pgxpool.Pool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusNotImplemented)     // ← 501
        fmt.Fprintf(w, `{"error":"not implemented yet"}`)
    }
}
func getSLAConfigHandler(pool *pgxpool.Pool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, `{"error":"not implemented yet"}`)  // ← 200 nhưng error body!
    }
}
func updateSLAConfigHandler(pool *pgxpool.Pool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusNotImplemented)     // ← 501
    }
}
func bulkCreateSLAConfigsHandler(pool *pgxpool.Pool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, `{"created_count":0,"failed_count":0,"results":[]}`) // hardcode 0
    }
}
```

## Root Cause

`httpdelivery.SLAConfigHandler` đã được implement đầy đủ (có `List`, `Create`, `Get`, `Update`, `Delete`, `BulkCreate`, `BulkAssign`) và đã được wire vào `/api/v1/sla/config`. Nhưng `/api/v2/sla-configurations/*` vẫn dùng placeholder functions từ thời TASK-DD-018 chưa complete.

## Tác động

- SLA configuration page không load được data
- Tạo/sửa SLA config hoàn toàn không hoạt động
- BulkCreate/BulkAssign silent fail (trả 207 với `created_count:0`)

## References
- [main.go:L128-180](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/sla-service/cmd/server/main.go#L128-L180)
- [httpdelivery.SLAConfigHandler](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/sla-service/internal/delivery/http/)
