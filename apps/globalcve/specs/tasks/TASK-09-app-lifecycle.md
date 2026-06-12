# TASK-09 — App Lifecycle & main.go

## Mục Tiêu

Implement **main.go** và **app.go** — wire tất cả dependencies, khởi động các goroutine services song song với `errgroup`, xử lý signal và graceful shutdown.

## Phụ Thuộc

- TASK-01 đến TASK-08 (tất cả services phải được implement)

## Đầu Ra

- `cmd/main.go` — Entry point hoàn chỉnh
- `internal/app/app.go` — App lifecycle manager

---

## Checklist

- [x] Signal context (`SIGINT`, `SIGTERM`)
- [x] Khởi tạo shared infra theo đúng thứ tự và fail-fast/fail-open logic
- [x] errgroup để chạy tất cả services song song
- [x] Graceful shutdown flow đúng theo spec
- [x] Deferred cleanup (pool.Close, redis.Close, nats.Close)
- [x] Zerolog setup (structured logging)
- [x] Build info trong startup log

---

## 1. main.go

```go
package main

import (
    "os"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"

    "github.com/binhnt/globalcve/internal/app"
    "github.com/binhnt/globalcve/internal/config"
)

var (
    version   = "dev"
    buildTime = "unknown"
)

func main() {
    // Setup structured logger
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
    zerolog.SetGlobalLevel(zerolog.InfoLevel)

    log.Info().
        Str("version", version).
        Str("build_time", buildTime).
        Msg("GlobalCVE v3.0 starting")

    // Load config
    cfg, err := config.Load()
    if err != nil {
        log.Fatal().Err(err).Msg("failed to load config")
    }

    // Run application
    if err := app.Run(cfg); err != nil {
        log.Fatal().Err(err).Msg("application error")
    }

    log.Info().Msg("GlobalCVE v3.0 stopped")
}
```

---

## 2. App Lifecycle (`internal/app/app.go`)

```go
package app

import (
    "context"
    "os/signal"
    "syscall"

    "golang.org/x/sync/errgroup"
    "github.com/rs/zerolog/log"

    "github.com/binhnt/globalcve/internal/config"
    infraPostgres "github.com/binhnt/globalcve/internal/infra/postgres"
    infraRedis    "github.com/binhnt/globalcve/internal/infra/redis"
    infraNATS     "github.com/binhnt/globalcve/internal/infra/nats"
    infraOS       "github.com/binhnt/globalcve/internal/infra/opensearch"

    "github.com/binhnt/globalcve/internal/cvesearch"
    "github.com/binhnt/globalcve/internal/cvesync"
    "github.com/binhnt/globalcve/internal/kevservice"
    "github.com/binhnt/globalcve/internal/notification"
    "github.com/binhnt/globalcve/internal/gateway"
)

func Run(cfg *config.Config) error {
    // ─── Signal Context ───────────────────────────────────────────────────
    // §5.1: errgroup + signal context
    ctx, cancel := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    // ─── Shared Infrastructure ────────────────────────────────────────────
    // PostgreSQL — FAIL-FAST
    pool, err := infraPostgres.NewPool(ctx, cfg.Postgres)
    if err != nil {
        return fmt.Errorf("postgres init: %w", err)
    }
    defer pool.Close()
    log.Info().Msg("PostgreSQL connected")

    // Redis — FAIL-FAST
    redisClient, err := infraRedis.NewClient(ctx, cfg.Redis)
    if err != nil {
        return fmt.Errorf("redis init: %w", err)
    }
    defer redisClient.Close()
    log.Info().Msg("Redis connected")

    // NATS — FAIL-OPEN (§5.3)
    natsClient, err := infraNATS.NewClient(ctx, cfg.NATS)
    if err != nil {
        log.Warn().Err(err).Msg("NATS unavailable — events disabled")
        natsClient = nil
    } else {
        defer natsClient.Close()
        log.Info().Msg("NATS connected")
    }

    // OpenSearch — FAIL-OPEN
    osClient, err := infraOS.NewClient(cfg.OpenSearch)
    if err != nil {
        log.Warn().Err(err).Msg("OpenSearch unavailable")
        osClient = nil
    } else {
        log.Info().Msg("OpenSearch connected")
    }

    // ─── Services (Dependency Injection) ──────────────────────────────────
    cveSyncSvc := cvesync.New(cfg.Services, pool, natsClient)
    cveSearchSvc := cvesearch.New(cfg.Services, pool, redisClient)
    kevSvc := kevservice.New(cfg.Services, pool, natsClient)
    notifSvc := notification.New(cfg.Services, pool, natsClient)
    gatewaySvc := gateway.New(*cfg, redisClient, cveSearchSvc.Handler())

    // ─── Start All Services (errgroup) ─────────────────────────────────────
    // §5.1: nếu một service lỗi → cancel context → tất cả stop
    g, gctx := errgroup.WithContext(ctx)

    g.Go(func() error {
        log.Info().Int("port", cfg.Services.CVESync.Port).Msg("CVE Sync Service starting")
        return cveSyncSvc.Start(gctx)
    })

    g.Go(func() error {
        log.Info().Int("port", cfg.Services.CVESearch.Port).Msg("CVE Search Service starting")
        return cveSearchSvc.Start(gctx)
    })

    g.Go(func() error {
        log.Info().Int("port", cfg.Services.KEVService.Port).Msg("KEV Service starting")
        return kevSvc.Start(gctx)
    })

    g.Go(func() error {
        log.Info().Int("port", cfg.Services.Notification.Port).Msg("Notification Service starting")
        return notifSvc.Start(gctx)
    })

    g.Go(func() error {
        log.Info().Int("port", cfg.Server.Port).Msg("API Gateway starting")
        return gatewaySvc.Start(gctx)
    })

    // Wait for all services to stop
    if err := g.Wait(); err != nil && err != http.ErrServerClosed {
        return err
    }

    return nil
}
```

---

## 3. Graceful Shutdown Flow

Theo §5.2 của architecture-solutions.md:

```
SIGTERM/SIGINT →
  1. signal.NotifyContext cancels ctx
  2. gctx propagates cancellation to all goroutines
  3. Each service's goroutine receives <-ctx.Done()
  4. Each service:
     a. HTTP server: server.Shutdown(15s timeout)
     b. Cron schedulers: cron.Stop().Done() (wait for running jobs)
     c. NATS consumers: msgCtx.Stop()
  5. errgroup.Wait() → all goroutines return
  6. Deferred cleanup runs (LIFO):
     - natsClient.Close() (Drain + Close)
     - redisClient.Close()
     - pool.Close()
  7. main() returns nil
```

### Shutdown Timeout Budget

| Step | Timeout |
|------|---------|
| HTTP Shutdown | 15 giây |
| Cron Stop | chờ jobs đang chạy |
| NATS Drain | 5 giây |
| Total | ~30 giây max |

---

## 4. Error Handling Strategy (§5.3)

| Infra | Strategy | Hành Động |
|-------|----------|-----------|
| PostgreSQL | fail-fast | `log.Fatal` — app không start |
| Redis | fail-fast | `log.Fatal` — app không start |
| NATS | fail-open | `log.Warn` — tiếp tục, events disabled |
| OpenSearch | fail-open | `log.Warn` — tiếp tục, vector search disabled |
| Service goroutine lỗi | fail-all | errgroup cancel context → all services stop |

---

## 5. Build Info

Sử dụng `-ldflags` để inject version và build time:

```makefile
# Trong Makefile
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

build:
    go build \
        -ldflags="-X main.version=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)" \
        -o bin/$(BINARY) $(MAIN)
```

### Startup Log Output:
```json
{"level":"info","version":"abc1234","build_time":"2026-06-09T08:00:00Z","message":"GlobalCVE v3.0 starting"}
{"level":"info","message":"PostgreSQL connected"}
{"level":"info","message":"Redis connected"}
{"level":"info","message":"NATS connected"}
{"level":"info","port":8082,"message":"CVE Sync Service starting"}
{"level":"info","port":8081,"message":"CVE Search Service starting"}
{"level":"info","port":8083,"message":"KEV Service starting"}
{"level":"info","port":8084,"message":"Notification Service starting"}
{"level":"info","port":8080,"message":"API Gateway starting"}
```

---

## 6. Config bổ sung cho TASK-01

Thêm `Auth` section vào config cho API Gateway auth:

```yaml
# config/config.yaml
auth:
  admin_api_key: "${ADMIN_API_KEY}"
```

```go
// internal/config/config.go
type Config struct {
    // ... existing fields ...
    Auth AuthConfig
}

type AuthConfig struct {
    AdminAPIKey string `mapstructure:"admin_api_key"`
}
```

---

## Định Nghĩa Hoàn Thành

- [x] `go run ./cmd/main.go` khởi động thành công, tất cả services up
- [x] `curl localhost:8080/health` trả về status của 4 services
- [x] `Ctrl+C` → graceful shutdown, log "GlobalCVE shutdown complete"
- [x] `kill -SIGTERM <pid>` → graceful shutdown
- [x] Nếu PostgreSQL down → app exit ngay với error message rõ ràng
- [x] Nếu NATS down → app chạy bình thường, log warning
- [x] Build info in đúng trong startup log

---

*TASK-09 | App Lifecycle & main.go | GlobalCVE v3.0*
