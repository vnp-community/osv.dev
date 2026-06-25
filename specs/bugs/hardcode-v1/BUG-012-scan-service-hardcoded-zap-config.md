# BUG-012 — Scan Service: Hardcoded ZAP Scanner URL và Timeout Values

## Metadata
- **ID**: BUG-012
- **Service**: `scan-service`
- **Files**:
  - [`internal/scanner/zap/scanner.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/scanner/zap/scanner.go)
  - [`internal/adapters/scanner/zap/zap_client.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/adapters/scanner/zap/zap_client.go)
  - [`internal/usecase/execute_scan/execute_scan.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/usecase/execute_scan/execute_scan.go)
- **Severity**: High
- **Category**: Hardcode / Configuration
- **Status**: Open

## Mô tả

### 1. ZAP Base URL hardcode

```go
// scanner/zap/scanner.go:17
const defaultZAPBase = "http://localhost:8080"
```

### 2. ZAP timeout defaults hardcode

```go
// adapters/scanner/zap/zap_client.go:171,174
if cfg.SpiderTimeout == 0 {
    cfg.SpiderTimeout = 5 * time.Minute     // MAGIC NUMBER
}
if cfg.ActiveScanTimeout == 0 {
    cfg.ActiveScanTimeout = 10 * time.Minute  // MAGIC NUMBER
}

// zap_client.go:234
ticker := time.NewTicker(5 * time.Second)   // polling interval hardcode
```

### 3. execute_scan usecase lặp lại constants

```go
// usecase/execute_scan/execute_scan.go:167-168
SpiderTimeout:     5 * time.Minute,    // DUPLICATE của zap_client.go
ActiveScanTimeout: 10 * time.Minute,   // DUPLICATE của zap_client.go
```

### 4. Scan scheduler intervals hardcode

```go
// scheduler/cron_worker.go:37
ticker := time.NewTicker(1 * time.Minute)

// scheduler/cron_worker.go:48
acquired, err := w.lock.TryAcquire(ctx, "schedule:leader", 90*time.Second)

// scheduler/scheduler.go:55
next := time.Now().UTC().Add(24 * time.Hour)  // default next scan: 24h hardcode
```

## Tác động

1. **ZAP localhost**: ZAP thường chạy như sidecar container hoặc separate service.
   `localhost:8080` sẽ không hoạt động trong K8s pod có ZAP ở container khác.

2. **Duplicate timeout values**: `5 * time.Minute` xuất hiện ở cả `zap_client.go` và
   `execute_scan.go`. Khi thay đổi timeout, engineer phải sửa cả hai nơi — dễ miss.

3. **Scheduler 24h default**: Scheduled scans mặc định chạy sau 24h — không có
   cách configure mà không sửa code.

4. **Leader lock 90s**: Distributed lock TTL 90s hardcode — không phù hợp với
   environments có network latency cao.

## Fix Proposal

### Tạo ScanConfig struct

```go
// scan-service/internal/config/config.go
type ScanConfig struct {
    ZAPBaseURL         string        // default: env ZAP_BASE_URL
    ZAPHTTPTimeout     time.Duration // default: 30s
    ZAPSpiderTimeout   time.Duration // default: 5m
    ZAPActiveScanTimeout time.Duration // default: 10m
    ZAPPollInterval    time.Duration // default: 5s

    SchedulerInterval  time.Duration // default: 1m
    SchedulerLeaderTTL time.Duration // default: 90s
    DefaultScanInterval time.Duration // default: 24h
}

func LoadScanConfig() ScanConfig {
    return ScanConfig{
        ZAPBaseURL:           envOr("ZAP_BASE_URL", "http://localhost:8080"),
        ZAPSpiderTimeout:     envDuration("ZAP_SPIDER_TIMEOUT", 5*time.Minute),
        ZAPActiveScanTimeout: envDuration("ZAP_ACTIVE_SCAN_TIMEOUT", 10*time.Minute),
        ZAPPollInterval:      envDuration("ZAP_POLL_INTERVAL", 5*time.Second),
        SchedulerInterval:    envDuration("SCHEDULER_INTERVAL", 1*time.Minute),
        SchedulerLeaderTTL:   envDuration("SCHEDULER_LEADER_TTL", 90*time.Second),
        DefaultScanInterval:  envDuration("DEFAULT_SCAN_INTERVAL", 24*time.Hour),
    }
}
```

### Loại bỏ duplicate trong execute_scan.go

```go
// execute_scan.go — dùng từ config thay vì hardcode
func NewExecuteScanUseCase(cfg config.ScanConfig, ...) {
    ...
    zapConfig := zap.Config{
        SpiderTimeout:     cfg.ZAPSpiderTimeout,     // from config, not magic number
        ActiveScanTimeout: cfg.ZAPActiveScanTimeout,
    }
}
```

## Files Affected

| File | Line | Issue |
|------|------|-------|
| [scanner/zap/scanner.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/scanner/zap/scanner.go) | 17 | `"http://localhost:8080"` |
| [zap_client.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/adapters/scanner/zap/zap_client.go) | 171, 174, 234 | 5m, 10m, 5s |
| [execute_scan.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/usecase/execute_scan/execute_scan.go) | 167-168 | 5m, 10m (duplicate) |
| [cron_worker.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/scheduler/cron_worker.go) | 37, 48 | 1m, 90s |
| [scheduler.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/scheduler/scheduler.go) | 55 | 24h |
