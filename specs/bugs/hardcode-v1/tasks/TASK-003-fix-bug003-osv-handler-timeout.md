# TASK-003 — Fix BUG-003: OSV Handler Warning Logs & Configurable Timeout

> **Bug**: BUG-003  
> **Priority**: 🔴 High — OSV data lookup fails silently trong container  
> **Depends on**: TASK-000  
> **Solution ref**: [SOL-GROUP-A](../solutions/SOL-GROUP-A-config-externalization.md#bug-003)  
> **Trạng thái**: ✅ DONE — 2026-06-22  
> **Ghi chú**: Thêm `log.Warn()` cho DATA_SERVICE_ADDR và SEARCH_SERVICE_HTTP fallback. Thêm `HTTPTimeoutSec` field + `OSV_HANDLER_TIMEOUT_SEC` env var. HTTP client dùng `time.Duration(cfg.HTTPTimeoutSec)` thay vì hardcode 15s. Build pass.

## Files Cần Đọc Trước

```
services/gateway-service/internal/proxy/osv_handler.go   (toàn bộ file)
```

## Files Sẽ Bị Sửa

```
services/gateway-service/internal/proxy/osv_handler.go   [MODIFY]
```

## Thay Đổi Chi Tiết

### Bước 1: Đọc file thực tế

Đọc `osv_handler.go` để xác định:
- Tên chính xác của `OSVHandlerConfig` struct
- Tên chính xác của hàm `OSVHandlerConfigFromEnv()`
- Line numbers của các hardcode (khoảng L44-51 và L77)

### Bước 2: Thêm warning log cho DATA_SERVICE_ADDR fallback

Tìm:
```go
dataAddr := os.Getenv("DATA_SERVICE_ADDR")
if dataAddr == "" {
    dataAddr = "localhost:50053"
}
```

Thay bằng:
```go
dataAddr := os.Getenv("DATA_SERVICE_ADDR")
if dataAddr == "" {
    dataAddr = "localhost:50053"
    log.Warn().Str("fallback", dataAddr).
        Msg("DATA_SERVICE_ADDR not set, using default — set env var in production")
}
```

### Bước 3: Thêm warning log cho SEARCH_SERVICE_HTTP fallback

Tìm:
```go
searchHTTP := os.Getenv("SEARCH_SERVICE_HTTP")
if searchHTTP == "" {
    searchHTTP = "http://localhost:8083"
}
```

Thay bằng:
```go
searchHTTP := os.Getenv("SEARCH_SERVICE_HTTP")
if searchHTTP == "" {
    searchHTTP = "http://localhost:8083"
    log.Warn().Str("fallback", searchHTTP).
        Msg("SEARCH_SERVICE_HTTP not set, using default — set env var in production")
}
```

### Bước 4: Thêm configurable timeout

Tìm field `HTTPTimeoutSec` trong `OSVHandlerConfig`. Nếu chưa có, thêm:
```go
type OSVHandlerConfig struct {
    DataServiceAddr   string
    SearchServiceHTTP string
    HTTPTimeoutSec    int    // [ADD] default: 15, override via OSV_HANDLER_TIMEOUT_SEC
}
```

Trong `OSVHandlerConfigFromEnv()`, thêm đọc timeout:
```go
timeoutSec := 15
if v := os.Getenv("OSV_HANDLER_TIMEOUT_SEC"); v != "" {
    if n, err := strconv.Atoi(v); err == nil && n > 0 {
        timeoutSec = n
    }
}
// Return config với timeoutSec
return OSVHandlerConfig{
    DataServiceAddr:   dataAddr,
    SearchServiceHTTP: searchHTTP,
    HTTPTimeoutSec:    timeoutSec,  // [ADD]
}
```

### Bước 5: Dùng timeout từ config (khoảng line 77)

Tìm chỗ khởi tạo HTTP client với timeout hardcode:
```go
httpClient: &http.Client{Timeout: 15 * time.Second},
```

Nếu `OSVV1Router` hoặc tương đương nhận config, sửa để dùng config timeout:
```go
httpClient: &http.Client{
    Timeout: time.Duration(cfg.HTTPTimeoutSec) * time.Second,
},
```

**Nếu** struct hiện không nhận config, thêm field và update constructor.

### Bước 6: Thêm import `strconv` nếu chưa có

```go
import (
    // ... existing imports ...
    "strconv"  // [ADD] nếu chưa có
)
```

## Quy Tắc Quan Trọng

1. **Grep chính xác**: `grep -n "localhost:50053\|localhost:8083\|15 \* time.Second" services/gateway-service/internal/proxy/osv_handler.go`
2. **Kiểm tra logger type** trong file — dùng `zerolog` (log.Warn()) hay `slog` hay fmt
3. **Không thay đổi logic routing** — chỉ thêm log và timeout config

## Verification

```bash
# Build
go build ./services/gateway-service/...

# Kiểm tra không còn hardcode timeout 15s (đã dùng config)
grep -n "15 \* time.Second" services/gateway-service/internal/proxy/osv_handler.go
# → phải rỗng (đã thay bằng cfg.HTTPTimeoutSec)

# Test: timeout có thể thay đổi
OSV_HANDLER_TIMEOUT_SEC=30 go run ./services/gateway-service/cmd/server/ &
# → timeout 30s được dùng

# Test: warning logs xuất hiện
go run ./services/gateway-service/cmd/server/ 2>&1 | grep -i "DATA_SERVICE_ADDR\|SEARCH_SERVICE_HTTP"
# → WARN logs
```

## Acceptance Criteria

- [ ] `DATA_SERVICE_ADDR` fallback có warning log với env key name
- [ ] `SEARCH_SERVICE_HTTP` fallback có warning log với env key name
- [ ] `OSVHandlerConfig` có field `HTTPTimeoutSec`
- [ ] `OSVHandlerConfigFromEnv()` đọc `OSV_HANDLER_TIMEOUT_SEC` từ env
- [ ] HTTP client dùng configurable timeout (không còn hardcode `15 * time.Second`)
- [ ] `go build ./services/gateway-service/...` thành công
