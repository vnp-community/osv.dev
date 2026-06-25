# BUG-005 — Metrics Server: Hardcoded Prometheus Port per Service

## Metadata
- **ID**: BUG-005
- **Services**: `gateway-service`, `search-service`, `data-service`, `notification-service`
- **Severity**: Low
- **Category**: Hardcode / Port
- **Status**: Open

## Mô tả

Mỗi service gọi `observability.StartMetricsServer(PORT)` với port integer được hardcode:

```go
// gateway-service/cmd/server/main.go:29
observability.StartMetricsServer(9090)

// search-service/cmd/server/main.go:44
observability.StartMetricsServer(9091)

// data-service/cmd/server/main.go:55
observability.StartMetricsServer(9092)

// notification-service/cmd/server/main.go:38
observability.StartMetricsServer(9094)
```

## Tác động

1. Khi deploy nhiều instances của cùng một service trên cùng host, port conflict xảy ra.
2. Không thể thay đổi Prometheus scrape port mà không rebuild service.
3. Không có pattern nhất quán — một số service không có metrics server (ranking-service
   chỉ có HTTP thông thường, scan-service không rõ).

## Fix Proposal

```go
// Đọc từ env var với fallback
metricsPort := envOrInt("METRICS_PORT", 9090)
observability.StartMetricsServer(metricsPort)

// Helper function:
func envOrInt(key string, def int) int {
    if v := os.Getenv(key); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            return n
        }
    }
    return def
}
```

Hoặc cập nhật signature của `StartMetricsServer`:

```go
// shared/pkg/observability
func StartMetricsServer(portOrEnv interface{}) {
    // accept int (direct) or string (env var name)
}
```

## Files Affected

| File | Line | Current Port |
|------|------|-------------|
| [gateway-service/cmd/server/main.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/cmd/server/main.go) | 29 | 9090 |
| [search-service/cmd/server/main.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/cmd/server/main.go) | 44 | 9091 |
| [data-service/cmd/server/main.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/cmd/server/main.go) | 55 | 9092 |
| [notification-service/cmd/server/main.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/cmd/server/main.go) | 38 | 9094 |
