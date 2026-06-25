# TASK-002 — Fix BUG-001: Thêm SearchAddr vào Gateway EmbeddedConfig

> **Bug**: BUG-001  
> **Priority**: 🔴 High — CVE search breaks hoàn toàn trong container  
> **Depends on**: TASK-000  
> **Solution ref**: [SOL-GROUP-A](../solutions/SOL-GROUP-A-config-externalization.md#bug-001)  
> **Trạng thái**: ✅ DONE — 2026-06-22  
> **Ghi chú**: Thêm `searchHTTP` variable resolve từ `cfg.SearchAddr` → `SEARCH_SERVICE_HTTP` env → localhost fallback. Thêm `log.Warn()` khi dùng localhost. Xóa 3 chỗ hardcode `"http://localhost:8083"` trong `embedded.go`. Build pass.

## Files Cần Đọc Trước

```
services/gateway-service/embedded.go          (toàn bộ file)
services/gateway-service/internal/bff/handlers/handler_ui_api.go   (lines 1-50, xem constructor)
```

## Files Sẽ Bị Sửa

```
services/gateway-service/embedded.go          [MODIFY]
```

## Thay Đổi Chi Tiết

### Bước 1: Tìm struct `EmbeddedConfig`

Trong `gateway-service/embedded.go`, tìm struct:
```go
type EmbeddedConfig struct {
    ...
}
```

Thêm field `SearchAddr` (và các field còn thiếu) vào struct. Đọc file thực tế để biết
chính xác các fields hiện có, sau đó thêm các fields còn thiếu:

```go
type EmbeddedConfig struct {
    // ... giữ nguyên các fields hiện có ...
    SearchAddr  string  // [ADD] search-service HTTP address — was missing, caused BUG-001
    JiraAddr    string  // [ADD] jira-service HTTP address (nếu chưa có)
    AuditAddr   string  // [ADD] audit-service HTTP address (nếu chưa có)
}
```

**Chỉ thêm field nếu chưa có** — đọc file thực tế trước.

### Bước 2: Tìm hàm `WireEmbedded` hoặc tương đương

Trong hàm wire, tìm đoạn resolve service addresses. Hiện tại search-service được hardcode:

```go
// Dạng có thể xuất hiện (line ~81, ~97, ~145):
"search-service": "http://localhost:8083",
"search":         "http://localhost:8083",
```

Thay đổi:

**Thêm** biến `searchHTTP` trước map definition (tương tự các service khác):
```go
// Tìm đoạn resolve các service addresses, thêm dòng này cùng nhóm:
searchHTTP := coalesce(cfg.SearchAddr, os.Getenv("SEARCH_SERVICE_HTTP"), "http://localhost:8083")
if searchHTTP == "http://localhost:8083" {
    log.Warn().Str("fallback", searchHTTP).
        Msg("SEARCH_SERVICE_HTTP not set, using localhost — CVE search will fail in container")
}
```

**Tìm** hàm `coalesce` trong file — nếu chưa có, thêm:
```go
// coalesce trả về string đầu tiên không rỗng trong danh sách.
func coalesce(vals ...string) string {
    for _, v := range vals {
        if v != "" {
            return v
        }
    }
    return ""
}
```

**Thay thế** hardcoded search-service values trong map:
```go
// Tìm tất cả chỗ có "http://localhost:8083" liên quan đến search, thay bằng searchHTTP:
upstreamURLs := map[string]string{
    // ...
    "search-service": searchHTTP,  // [FIX] was: "http://localhost:8083"
    "search":         searchHTTP,  // [FIX] was: "http://localhost:8083"
    // ...
}
```

### Bước 3: Tìm call đến `NewUIAPIHandler` (khoảng line 145)

Nếu `NewUIAPIHandler` nhận search URL như 1 tham số riêng:
```go
// Tìm pattern này và thay giá trị hardcode bằng searchHTTP:
NewUIAPIHandler(ctx, ..., "http://localhost:8083", ...)
// Thay bằng:
NewUIAPIHandler(ctx, ..., searchHTTP, ...)
```

## Quy Tắc Quan Trọng

1. **Đọc `embedded.go` thực tế** trước khi sửa — xác định chính xác tên hàm `coalesce` trong file (có thể khác tên)
2. **Grep để tìm tất cả chỗ hardcode**: `grep -n "localhost:8083" services/gateway-service/embedded.go`
3. **Không thay đổi logic khác** trong file

## Verification

```bash
# Tìm các chỗ còn hardcode localhost:8083 trong gateway-service
grep -rn "localhost:8083" services/gateway-service/
# → Kết quả phải rỗng (không còn hardcode nào)

# Build gateway-service
go build ./services/gateway-service/cmd/server/

# Test: SEARCH_SERVICE_HTTP được đọc
SEARCH_SERVICE_HTTP=http://search-service:8083 go run ./services/gateway-service/cmd/server/ &
# → logs không có WARN về SEARCH_SERVICE_HTTP

# Test: warning xuất hiện khi không set
go run ./services/gateway-service/cmd/server/ 2>&1 | grep -i "search"
# → phải thấy WARN: "SEARCH_SERVICE_HTTP not set..."
```

## Acceptance Criteria

- [ ] `EmbeddedConfig` có field `SearchAddr`
- [ ] Không còn `"http://localhost:8083"` hardcode trong `embedded.go`
- [ ] `searchHTTP` được resolve từ: `cfg.SearchAddr` → `SEARCH_SERVICE_HTTP` env → localhost fallback
- [ ] Localhost fallback có warning log rõ ràng
- [ ] `go build ./services/gateway-service/...` thành công
