# TASK-005 — Fix BUG-012: Externalize ZAP Config & Tạo ScanConfig Struct

> **Bug**: BUG-012  
> **Priority**: 🔴 High — ZAP web scanning fails hoàn toàn trong container  
> **Depends on**: TASK-000  
> **Solution ref**: [SOL-GROUP-E](../solutions/SOL-GROUP-E-scan-service-config.md)  
> **Trạng thái**: ✅ DONE — 2026-06-22  
> **Ghi chú**: Tạo `internal/config/scan_config.go` với `ScanConfig` struct và `Load()`. Xóa `const defaultZAPBase`, `defaultSpiderTime`, `defaultActiveScan`. Thêm `SpiderTimeout`, `ActiveScanTimeout`, `PollInterval` vào `ScannerConfig`. `waitForSpider`/`waitForActiveScan` dùng config fields. `CronWorker` có `tickInterval` và `leaderTTL` configurable qua `SCHEDULER_TICK_INTERVAL`/`SCHEDULER_LEADER_TTL` env vars. Build pass.

## Files Cần Đọc Trước

```
services/scan-service/internal/scanner/zap/scanner.go
services/scan-service/internal/adapters/scanner/zap/zap_client.go
services/scan-service/internal/usecase/execute_scan/execute_scan.go
services/scan-service/internal/scheduler/cron_worker.go
services/scan-service/internal/scheduler/scheduler.go
services/scan-service/cmd/server/main.go
services/scan-service/go.mod                           (lấy module name)
```

## Files Sẽ Được Tạo / Sửa

```
services/scan-service/internal/config/scan_config.go  [NEW]
services/scan-service/internal/scanner/zap/scanner.go [MODIFY]
services/scan-service/internal/adapters/scanner/zap/zap_client.go [MODIFY]
services/scan-service/internal/usecase/execute_scan/execute_scan.go [MODIFY]
services/scan-service/internal/scheduler/cron_worker.go [MODIFY]
services/scan-service/internal/scheduler/scheduler.go  [MODIFY]
services/scan-service/cmd/server/main.go               [MODIFY — wire ScanConfig]
```

## Thay Đổi Chi Tiết

### Bước 1: Tạo `internal/config/scan_config.go` [NEW]

```go
// Package config định nghĩa tất cả runtime configuration cho scan-service.
// Tất cả giá trị đều override được qua environment variables.
package config

import (
    "fmt"
    "log/slog"
    "os"
    "strconv"
    "time"
)

// ScanConfig là single source of truth cho mọi config của scan-service.
type ScanConfig struct {
    // ZAP Scanner
    ZAPBaseURL           string
    ZAPAPIKey            string
    ZAPHTTPTimeout       time.Duration
    ZAPSpiderTimeout     time.Duration
    ZAPActiveScanTimeout time.Duration
    ZAPPollInterval      time.Duration

    // Scheduler
    SchedulerTickInterval time.Duration
    SchedulerLeaderTTL    time.Duration
    DefaultScanInterval   time.Duration
}

// Load đọc ScanConfig từ environment variables với sensible defaults.
func Load() ScanConfig {
    zapURL := envHTTPAddr("ZAP_BASE_URL", "localhost", 8080)

    return ScanConfig{
        ZAPBaseURL:           zapURL,
        ZAPAPIKey:            os.Getenv("ZAP_API_KEY"),
        ZAPHTTPTimeout:       envDuration("ZAP_HTTP_TIMEOUT", 30*time.Second),
        ZAPSpiderTimeout:     envDuration("ZAP_SPIDER_TIMEOUT", 5*time.Minute),
        ZAPActiveScanTimeout: envDuration("ZAP_ACTIVE_SCAN_TIMEOUT", 10*time.Minute),
        ZAPPollInterval:      envDuration("ZAP_POLL_INTERVAL", 5*time.Second),
        SchedulerTickInterval: envDuration("SCHEDULER_TICK_INTERVAL", 1*time.Minute),
        SchedulerLeaderTTL:    envDuration("SCHEDULER_LEADER_TTL", 90*time.Second),
        DefaultScanInterval:   envDuration("DEFAULT_SCAN_INTERVAL", 24*time.Hour),
    }
}

func envHTTPAddr(key, defaultHost string, defaultPort int) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    fallback := fmt.Sprintf("http://%s:%d", defaultHost, defaultPort)
    slog.Warn("env var not set, using localhost fallback — configure in production",
        "env_key", key, "fallback", fallback)
    return fallback
}

func envDuration(key string, def time.Duration) time.Duration {
    v := os.Getenv(key)
    if v == "" {
        return def
    }
    d, err := time.ParseDuration(v)
    if err != nil || d <= 0 {
        slog.Warn("invalid duration, using default", "env_key", key, "value", v)
        return def
    }
    return d
}

func envInt(key string, def int) int {
    v := os.Getenv(key)
    if v == "" {
        return def
    }
    n, err := strconv.Atoi(v)
    if err != nil || n <= 0 {
        return def
    }
    return n
}
```

### Bước 2: Sửa `scanner/zap/scanner.go`

Tìm:
```go
const defaultZAPBase = "http://localhost:8080"
```
→ **Xóa constant này hoàn toàn**.

Tìm constructor của ZAP scanner, thêm validation:
```go
// Nếu constructor nhận URL string, thêm validation:
func NewZAPScanner(baseURL string, ...) (*ZAPScanner, error) {
    if baseURL == "" {
        return nil, fmt.Errorf("ZAP scanner: baseURL is required (ZAP_BASE_URL env var)")
    }
    // ...
}
```

Đọc file thực tế để xác định constructor signature hiện tại.

### Bước 3: Sửa `adapters/scanner/zap/zap_client.go`

Tìm các magic numbers:
```bash
grep -n "5 \* time.Minute\|10 \* time.Minute\|5 \* time.Second\|90\*time" \
    services/scan-service/internal/adapters/scanner/zap/zap_client.go
```

Thay mỗi hardcode bằng struct field:
- `5 * time.Minute` → `cfg.SpiderTimeout`
- `10 * time.Minute` → `cfg.ActiveScanTimeout`  
- `5 * time.Second` (ticker/polling) → `cfg.PollInterval`

ZAPClient cần nhận config (hoặc các timeout values) từ ngoài thay vì dùng magic numbers.

### Bước 4: Sửa `usecase/execute_scan/execute_scan.go`

Tìm duplicate constants:
```bash
grep -n "5 \* time.Minute\|10 \* time.Minute" \
    services/scan-service/internal/usecase/execute_scan/execute_scan.go
```

Xóa các hardcode này. Thay bằng giá trị đến từ `ScanConfig` được inject vào use case.

Nếu use case tạo ZAP config inline:
```go
// Trước (BUG):
zapConfig := zap.Config{
    SpiderTimeout:     5 * time.Minute,   // duplicate!
    ActiveScanTimeout: 10 * time.Minute,  // duplicate!
}

// Sau (FIX) — inject từ ScanConfig:
zapConfig := zap.Config{
    SpiderTimeout:     uc.cfg.ZAPSpiderTimeout,
    ActiveScanTimeout: uc.cfg.ZAPActiveScanTimeout,
}
```

### Bước 5: Sửa `scheduler/cron_worker.go`

Tìm:
```go
ticker := time.NewTicker(1 * time.Minute)         // hardcode
acquired, err := w.lock.TryAcquire(ctx, "schedule:leader", 90*time.Second) // hardcode
```

Thêm config field vào `CronWorker` struct và dùng:
```go
ticker := time.NewTicker(w.cfg.SchedulerTickInterval)
acquired, err := w.lock.TryAcquire(ctx, "schedule:leader", w.cfg.SchedulerLeaderTTL)
```

### Bước 6: Sửa `scheduler/scheduler.go`

Tìm:
```go
next := time.Now().UTC().Add(24 * time.Hour)
```

Thay bằng:
```go
next := time.Now().UTC().Add(s.cfg.DefaultScanInterval)
```

### Bước 7: Wire ScanConfig trong `cmd/server/main.go`

```go
func main() {
    // Load config một lần — inject vào tất cả components
    scanCfg := config.Load()

    // Wire ZAP scanner với config
    // (tuỳ constructor signature thực tế)
    // ...

    // Wire scheduler
    // ...
}
```

## Verification

```bash
# Build
go build ./services/scan-service/...

# Kiểm tra không còn magic numbers
grep -rn "localhost:8080\|5 \* time\.Minute\|10 \* time\.Minute\|90\*time\|24 \* time\.Hour" \
    services/scan-service/internal/
# → phải rỗng (ngoại trừ trong config/scan_config.go là default values)

# Test: ZAP URL configurable
ZAP_BASE_URL=http://zap:8080 go run ./services/scan-service/cmd/server/ &
# → không có WARN về ZAP_BASE_URL

# Test: Timeout configurable
ZAP_SPIDER_TIMEOUT=10m ZAP_ACTIVE_SCAN_TIMEOUT=30m \
    go run ./services/scan-service/cmd/server/ 2>&1 | grep -i "spider\|active"
```

## Acceptance Criteria

- [ ] `internal/config/scan_config.go` được tạo với `Load()` function
- [ ] `const defaultZAPBase` bị xóa khỏi `scanner.go`
- [ ] Không còn `5 * time.Minute` hay `10 * time.Minute` ngoài `scan_config.go`
- [ ] Không còn `90*time.Second` hardcode trong `cron_worker.go`
- [ ] Không còn `24 * time.Hour` hardcode trong `scheduler.go`
- [ ] `go build ./services/scan-service/...` thành công
