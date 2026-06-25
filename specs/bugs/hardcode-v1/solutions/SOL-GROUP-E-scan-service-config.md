# SOL-GROUP-E — Scan Service ZAP Config Externalization

> **Fixes**: BUG-012  
> **Service**: `scan-service`  
> **Priority**: 🔴 High — ZAP là core active scanning feature; fails in container environments

---

## BUG-012 — Scan Service: Hardcoded ZAP Scanner URL & Timeout Values

### Root Cause

1. `scanner/zap/scanner.go` hardcode `const defaultZAPBase = "http://localhost:8080"`
2. Timeout values (`5m`, `10m`, `5s`) bị duplicate ở 2 files: `zap_client.go` và `execute_scan.go`
3. Scheduler intervals (`1m`, `90s`, `24h`) không configurable qua env vars

### Files Cần Sửa

- `services/scan-service/internal/scanner/zap/scanner.go`
- `services/scan-service/internal/adapters/scanner/zap/zap_client.go`
- `services/scan-service/internal/usecase/execute_scan/execute_scan.go`
- `services/scan-service/internal/scheduler/cron_worker.go`
- `services/scan-service/internal/scheduler/scheduler.go`
- **[NEW]** `services/scan-service/internal/config/config.go`

---

### Solution

**Bước 1**: Tạo `ScanConfig` struct tập trung tất cả scan configuration:

```go
// [NEW] services/scan-service/internal/config/config.go

// Package config định nghĩa tất cả configuration cho scan-service.
package config

import (
    "time"
    "github.com/osv/shared/pkg/config"
)

// ScanConfig chứa toàn bộ cấu hình cho scan-service.
// Tất cả giá trị đều có thể override qua environment variables.
type ScanConfig struct {
    // ZAP Scanner config
    ZAPBaseURL           string        // env: ZAP_BASE_URL          default: http://localhost:8080 (warn)
    ZAPAPIKey            string        // env: ZAP_API_KEY           default: "" (optional)
    ZAPHTTPTimeout       time.Duration // env: ZAP_HTTP_TIMEOUT      default: 30s
    ZAPSpiderTimeout     time.Duration // env: ZAP_SPIDER_TIMEOUT    default: 5m
    ZAPActiveScanTimeout time.Duration // env: ZAP_ACTIVE_SCAN_TIMEOUT default: 10m
    ZAPPollInterval      time.Duration // env: ZAP_POLL_INTERVAL     default: 5s

    // Scheduler config
    SchedulerTickInterval time.Duration // env: SCHEDULER_TICK_INTERVAL  default: 1m
    SchedulerLeaderTTL    time.Duration // env: SCHEDULER_LEADER_TTL     default: 90s
    DefaultScanInterval   time.Duration // env: DEFAULT_SCAN_INTERVAL    default: 24h
}

// LoadScanConfig load config từ env vars.
// Tất cả config ZAP đều phải đến từ env vars — không hardcode trong source.
func LoadScanConfig() ScanConfig {
    zapURL := config.HTTPServiceAddr("ZAP_BASE_URL", "localhost", 8080)
    // ↑ Tự động log WARN: "ZAP_BASE_URL not set, using localhost — configure in production"

    return ScanConfig{
        // ZAP config
        ZAPBaseURL:           zapURL,
        ZAPAPIKey:            config.Str("ZAP_API_KEY", ""),  // optional
        ZAPHTTPTimeout:       config.Duration("ZAP_HTTP_TIMEOUT", 30*time.Second),
        ZAPSpiderTimeout:     config.Duration("ZAP_SPIDER_TIMEOUT", 5*time.Minute),
        ZAPActiveScanTimeout: config.Duration("ZAP_ACTIVE_SCAN_TIMEOUT", 10*time.Minute),
        ZAPPollInterval:      config.Duration("ZAP_POLL_INTERVAL", 5*time.Second),

        // Scheduler config
        SchedulerTickInterval: config.Duration("SCHEDULER_TICK_INTERVAL", 1*time.Minute),
        SchedulerLeaderTTL:    config.Duration("SCHEDULER_LEADER_TTL", 90*time.Second),
        DefaultScanInterval:   config.Duration("DEFAULT_SCAN_INTERVAL", 24*time.Hour),
    }
}

// ZAPClientConfig chuyển ScanConfig sang format cho zap_client.
func (c ScanConfig) ZAPClientConfig() ZAPConfig {
    return ZAPConfig{
        BaseURL:           c.ZAPBaseURL,
        APIKey:            c.ZAPAPIKey,
        HTTPTimeout:       c.ZAPHTTPTimeout,
        SpiderTimeout:     c.ZAPSpiderTimeout,
        ActiveScanTimeout: c.ZAPActiveScanTimeout,
        PollInterval:      c.ZAPPollInterval,
    }
}

// ZAPConfig là config struct cho ZAP client.
type ZAPConfig struct {
    BaseURL           string
    APIKey            string
    HTTPTimeout       time.Duration
    SpiderTimeout     time.Duration
    ActiveScanTimeout time.Duration
    PollInterval      time.Duration
}
```

**Bước 2**: Sửa `scanner/zap/scanner.go` — xóa hardcoded const:

```go
// services/scan-service/internal/scanner/zap/scanner.go

// [REMOVE] Xóa constant này — không nên có default URL trong scanner layer
// const defaultZAPBase = "http://localhost:8080"

// ZAPScanner implement Scanner interface cho OWASP ZAP.
type ZAPScanner struct {
    client  *ZAPClient
    baseURL string
    apiKey  string
}

// NewZAPScanner tạo ZAPScanner với config được inject từ ngoài.
// [FIX] baseURL là bắt buộc — fail fast nếu rỗng.
func NewZAPScanner(cfg config.ZAPConfig) (*ZAPScanner, error) {
    if cfg.BaseURL == "" {
        return nil, fmt.Errorf("ZAP scanner: BaseURL is required — set ZAP_BASE_URL env var")
    }

    client := NewZAPClient(cfg)
    return &ZAPScanner{
        client:  client,
        baseURL: cfg.BaseURL,
        apiKey:  cfg.APIKey,
    }, nil
}
```

**Bước 3**: Sửa `zap_client.go` — xóa duplicate timeout defaults:

```go
// services/scan-service/internal/adapters/scanner/zap/zap_client.go

// ZAPClient là HTTP client wrapper cho ZAP REST API.
type ZAPClient struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
    cfg        config.ZAPConfig
}

// NewZAPClient tạo ZAPClient với config được inject đầy đủ.
// [FIX] Tất cả timeouts đến từ cfg — không tự set default trong client.
func NewZAPClient(cfg config.ZAPConfig) *ZAPClient {
    if cfg.HTTPTimeout <= 0 {
        cfg.HTTPTimeout = 30 * time.Second  // safety fallback
    }

    return &ZAPClient{
        baseURL: cfg.BaseURL,
        apiKey:  cfg.APIKey,
        httpClient: &http.Client{
            Timeout: cfg.HTTPTimeout,
        },
        cfg: cfg,
    }
}

// StartSpider bắt đầu ZAP spider scan với configurable timeout.
func (c *ZAPClient) StartSpider(ctx context.Context, target string) error {
    // [FIX] Dùng c.cfg.SpiderTimeout thay vì hardcode 5 * time.Minute
    ctx, cancel := context.WithTimeout(ctx, c.cfg.SpiderTimeout)
    defer cancel()

    // polling với configurable interval
    ticker := time.NewTicker(c.cfg.PollInterval) // [FIX] was: 5 * time.Second hardcoded
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            progress, err := c.getSpiderProgress(ctx)
            if err != nil {
                return err
            }
            if progress >= 100 {
                return nil
            }
        case <-ctx.Done():
            return fmt.Errorf("spider scan timed out after %s", c.cfg.SpiderTimeout)
        }
    }
}

// StartActiveScan bắt đầu ZAP active scan với configurable timeout.
func (c *ZAPClient) StartActiveScan(ctx context.Context, target string) error {
    // [FIX] Dùng c.cfg.ActiveScanTimeout thay vì hardcode 10 * time.Minute
    ctx, cancel := context.WithTimeout(ctx, c.cfg.ActiveScanTimeout)
    defer cancel()

    ticker := time.NewTicker(c.cfg.PollInterval) // [FIX] configurable
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            progress, err := c.getActiveScanProgress(ctx)
            if err != nil {
                return err
            }
            if progress >= 100 {
                return nil
            }
        case <-ctx.Done():
            return fmt.Errorf("active scan timed out after %s", c.cfg.ActiveScanTimeout)
        }
    }
}
```

**Bước 4**: Sửa `execute_scan.go` — xóa duplicate timeout values:

```go
// services/scan-service/internal/usecase/execute_scan/execute_scan.go

import "github.com/osv/scan-service/internal/config"

// ExecuteScanUseCase điều phối việc chạy 1 scan.
type ExecuteScanUseCase struct {
    zapScanner *zap.ZAPScanner
    nmapScanner *nmap.NmapScanner
    scanCfg    config.ScanConfig  // [ADD] inject config
    // ...
}

// NewExecuteScanUseCase tạo use case với injected config.
func NewExecuteScanUseCase(scanCfg config.ScanConfig, ...) *ExecuteScanUseCase {
    return &ExecuteScanUseCase{
        scanCfg: scanCfg,
        // ...
    }
}

func (uc *ExecuteScanUseCase) executeZAPScan(ctx context.Context, scan *domain.Scan) error {
    zapCfg := uc.scanCfg.ZAPClientConfig()
    scanner, err := zap.NewZAPScanner(zapCfg)
    if err != nil {
        return fmt.Errorf("init ZAP scanner: %w", err)
    }

    // [FIX] REMOVE duplicate constants:
    // Trước:
    //   SpiderTimeout:     5 * time.Minute,   ← duplicate với zap_client.go
    //   ActiveScanTimeout: 10 * time.Minute,  ← duplicate với zap_client.go
    //
    // Sau: ZAPScanner đã được config với đúng timeouts từ scanCfg — không cần pass lại
    return scanner.Scan(ctx, scan.Target)
}
```

**Bước 5**: Sửa `cron_worker.go` — scheduler intervals configurable:

```go
// services/scan-service/internal/scheduler/cron_worker.go

import "github.com/osv/scan-service/internal/config"

// CronWorker là distributed scheduler worker.
type CronWorker struct {
    cfg     config.ScanConfig
    lock    DistributedLock
    scanner Scanner
    // ...
}

func NewCronWorker(cfg config.ScanConfig, ...) *CronWorker {
    return &CronWorker{cfg: cfg, ...}
}

func (w *CronWorker) Run(ctx context.Context) {
    // [FIX] Dùng cfg.SchedulerTickInterval thay vì hardcode 1 * time.Minute
    ticker := time.NewTicker(w.cfg.SchedulerTickInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            // [FIX] Dùng cfg.SchedulerLeaderTTL thay vì hardcode 90*time.Second
            acquired, err := w.lock.TryAcquire(ctx, "schedule:leader", w.cfg.SchedulerLeaderTTL)
            if err != nil || !acquired {
                continue
            }
            w.processDueScans(ctx)
            w.lock.Release(ctx, "schedule:leader")

        case <-ctx.Done():
            return
        }
    }
}
```

**Bước 6**: Sửa `scheduler.go` — default scan interval configurable:

```go
// services/scan-service/internal/scheduler/scheduler.go

import "github.com/osv/scan-service/internal/config"

type Scheduler struct {
    cfg config.ScanConfig
    // ...
}

func NewScheduler(cfg config.ScanConfig, ...) *Scheduler {
    return &Scheduler{cfg: cfg, ...}
}

func (s *Scheduler) computeNextScanTime(scan *domain.ScheduledScan) time.Time {
    if scan.Interval != nil && *scan.Interval > 0 {
        return time.Now().UTC().Add(*scan.Interval)
    }
    // [FIX] Dùng cfg.DefaultScanInterval thay vì hardcode 24 * time.Hour
    return time.Now().UTC().Add(s.cfg.DefaultScanInterval)
}
```

**Bước 7**: Wire trong `main.go` hoặc `embedded.go`:

```go
// services/scan-service/cmd/server/main.go

import scanconfig "github.com/osv/scan-service/internal/config"

func main() {
    ctx := context.Background()

    // [FIX] Load tất cả config từ env vars — tập trung tại đây
    scanCfg := scanconfig.LoadScanConfig()

    // Wire ZAP scanner với config
    zapScanner, err := zap.NewZAPScanner(scanCfg.ZAPClientConfig())
    if err != nil {
        log.Fatal().Err(err).Msg("failed to initialize ZAP scanner")
    }

    // Wire use cases với config
    executeScanUC := executescan.NewExecuteScanUseCase(scanCfg, zapScanner, ...)

    // Wire scheduler với config
    cronWorker := scheduler.NewCronWorker(scanCfg, lock, executeScanUC)
    
    go cronWorker.Run(ctx)
    // ...
}
```

---

## Env Vars Reference

```yaml
# docker-compose.yml — scan-service

scan-service:
  environment:
    # ZAP Scanner
    ZAP_BASE_URL:            http://zap:8080      # [FIX] không còn localhost hardcode
    ZAP_API_KEY:             ${ZAP_API_KEY}       # optional
    ZAP_HTTP_TIMEOUT:        30s
    ZAP_SPIDER_TIMEOUT:      5m
    ZAP_ACTIVE_SCAN_TIMEOUT: 15m                  # có thể tăng cho large targets
    ZAP_POLL_INTERVAL:       5s

    # Scheduler
    SCHEDULER_TICK_INTERVAL: 1m
    SCHEDULER_LEADER_TTL:    90s
    DEFAULT_SCAN_INTERVAL:   24h

  depends_on:
    - zap

# ZAP sidecar service
zap:
  image: ghcr.io/zaproxy/zaproxy:stable
  command: zap.sh -daemon -port 8080 -host 0.0.0.0
    -config api.addrs.addr.name=.* -config api.addrs.addr.regex=true
    -config api.key=${ZAP_API_KEY}
  ports:
    - "8080:8080"
```

---

## Tóm Tắt Thay Đổi

| File | Thay Đổi Chính |
|------|----------------|
| `config/config.go` **[NEW]** | `ScanConfig` struct; `LoadScanConfig()` từ env vars |
| `scanner/zap/scanner.go` | Xóa `const defaultZAPBase`; require config inject |
| `adapters/scanner/zap/zap_client.go` | Xóa hardcoded timeouts; dùng `cfg.SpiderTimeout`, `cfg.PollInterval` |
| `usecase/execute_scan/execute_scan.go` | Xóa duplicate `5*time.Minute`, `10*time.Minute` |
| `scheduler/cron_worker.go` | Dùng `cfg.SchedulerTickInterval`, `cfg.SchedulerLeaderTTL` |
| `scheduler/scheduler.go` | Dùng `cfg.DefaultScanInterval` thay vì `24*time.Hour` |
| `cmd/server/main.go` | Load `ScanConfig` và inject vào tất cả components |

## Test Verification

```bash
# Verify ZAP URL configurable
ZAP_BASE_URL=http://zap-service:8080 go run ./services/scan-service/cmd/server/
# → logs không có WARN về ZAP_BASE_URL
# → ZAP client kết nối đến zap-service:8080

# Verify fail fast khi ZAP_BASE_URL rỗng (nếu muốn strict mode)
# Hoặc verify WARN log khi dùng localhost fallback:
unset ZAP_BASE_URL
go run ./services/scan-service/cmd/server/
# → WARN: "ZAP_BASE_URL not set, using localhost — configure in production"

# Verify timeout configurable
ZAP_BASE_URL=http://zap:8080 ZAP_SPIDER_TIMEOUT=10m ZAP_ACTIVE_SCAN_TIMEOUT=30m \
go run ./services/scan-service/cmd/server/
# → no duplicate timeout logs; config from env vars

# Verify scheduler interval configurable
SCHEDULER_TICK_INTERVAL=30s DEFAULT_SCAN_INTERVAL=12h \
go run ./services/scan-service/cmd/server/
# → scheduler ticks mỗi 30s; default scan interval là 12h
```
