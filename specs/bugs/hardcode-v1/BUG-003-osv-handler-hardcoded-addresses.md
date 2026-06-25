# BUG-003 — OSV Handler: Hardcoded localhost gRPC/HTTP fallback addresses

## Metadata
- **ID**: BUG-003
- **Service**: `gateway-service`
- **File**: [`internal/proxy/osv_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/proxy/osv_handler.go)
- **Lines**: 44–51
- **Severity**: High
- **Category**: Hardcode / Network
- **Status**: Open

## Mô tả

Hàm `OSVHandlerConfigFromEnv()` sử dụng fallback hardcode khi env var không được set:

```go
func OSVHandlerConfigFromEnv() OSVHandlerConfig {
    dataAddr := os.Getenv("DATA_SERVICE_ADDR")
    if dataAddr == "" {
        dataAddr = "localhost:50053"          // HARDCODE gRPC address
    }
    searchHTTP := os.Getenv("SEARCH_SERVICE_HTTP")
    if searchHTTP == "" {
        searchHTTP = "http://localhost:8083"  // HARDCODE HTTP address
    }
    ...
}
```

Đây là pattern đúng nhưng:
1. Không có validation rằng địa chỉ có format hợp lệ
2. Không có logging/warning khi dùng fallback (khó debug trong production)
3. `OSVHandlerConfig` comment nói "Default: localhost:50053" — document hardcode thay
   vì fix nó

Ngoài ra, trong `OSVV1Router`:
```go
// Line 77: hardcoded timeout
httpClient: &http.Client{Timeout: 15 * time.Second},
```

Timeout 15s không thể configure qua env var.

## Tác động

- Trong môi trường Docker/K8s, services không chạy trên `localhost` mà dùng service
  discovery names (`data-service`, `search-service`). Nếu env vars không được inject,
  routing sẽ fail silently (logs chỉ show Warn, không Fatal).
- HTTP client timeout cố định 15s không phù hợp với tất cả environments.

## Fix Proposal

```go
func OSVHandlerConfigFromEnv() OSVHandlerConfig {
    dataAddr := os.Getenv("DATA_SERVICE_ADDR")
    if dataAddr == "" {
        dataAddr = "localhost:50053"
        log.Warn().Str("default", dataAddr).
            Msg("DATA_SERVICE_ADDR not set, using default — set env var in production")
    }

    searchHTTP := os.Getenv("SEARCH_SERVICE_HTTP")
    if searchHTTP == "" {
        searchHTTP = "http://localhost:8083"
        log.Warn().Str("default", searchHTTP).
            Msg("SEARCH_SERVICE_HTTP not set, using default — set env var in production")
    }

    // Configurable timeout
    timeoutSec := 15
    if v := os.Getenv("OSV_HANDLER_TIMEOUT_SEC"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            timeoutSec = n
        }
    }

    return OSVHandlerConfig{
        DataServiceAddr:   dataAddr,
        SearchServiceHTTP: searchHTTP,
        HTTPTimeoutSec:    timeoutSec, // new field
    }
}
```

## References

- [osv_handler.go L44-56](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/proxy/osv_handler.go#L44-L56)
- [osv_handler.go L77](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/proxy/osv_handler.go#L77)
