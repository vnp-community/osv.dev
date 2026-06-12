# Goroutine Registry & Service Runner Pattern (v3)

## Mục tiêu

Định nghĩa pattern chuẩn để mỗi service chạy như một **goroutine độc lập** với:
- Vòng đời rõ ràng: Start → Running → Stop
- Health check tích hợp
- Panic recovery
- Graceful shutdown qua `context.Context`

---

## ServiceRunner Interface

```go
// internal/app/registry.go
package app

import "context"

// ServiceRunner là interface bắt buộc cho mỗi service goroutine.
type ServiceRunner interface {
    // Name trả về tên duy nhất của service (dùng cho logging và health check)
    Name() string

    // Run khởi động service và block cho đến khi ctx bị cancel hoặc có lỗi.
    // Trả về nil khi shutdown gracefully, error khi có sự cố.
    Run(ctx context.Context) error

    // Health kiểm tra xem service có đang hoạt động đúng không.
    Health(ctx context.Context) error
}
```

---

## Registry Implementation

```go
// internal/app/registry.go

package app

import (
    "context"
    "fmt"
    "sync"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

type serviceEntry struct {
    runner ServiceRunner
    state  string // "idle" | "running" | "stopped" | "failed"
    err    error
}

// Registry quản lý vòng đời của tất cả service goroutines.
type Registry struct {
    mu      sync.RWMutex
    entries map[string]*serviceEntry
    wg      sync.WaitGroup
    log     zerolog.Logger
}

func NewRegistry(l zerolog.Logger) *Registry {
    return &Registry{
        entries: make(map[string]*serviceEntry),
        log:     l,
    }
}

// Register đăng ký một ServiceRunner. Phải gọi trước Start.
func (r *Registry) Register(runner ServiceRunner) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.entries[runner.Name()] = &serviceEntry{runner: runner, state: "idle"}
    r.log.Debug().Str("service", runner.Name()).Msg("registered")
}

// Start khởi động tất cả registered services như goroutines độc lập.
func (r *Registry) Start(ctx context.Context) {
    r.mu.RLock()
    entries := make([]*serviceEntry, 0, len(r.entries))
    for _, e := range r.entries {
        entries = append(entries, e)
    }
    r.mu.RUnlock()

    for _, entry := range entries {
        e := entry
        r.wg.Add(1)
        go func() {
            defer r.wg.Done()
            defer r.recoverFromPanic(e)

            r.setState(e, "running")
            r.log.Info().Str("service", e.runner.Name()).Msg("goroutine started")

            if err := e.runner.Run(ctx); err != nil && err != context.Canceled {
                r.setError(e, err)
                r.setState(e, "failed")
                r.log.Error().Str("service", e.runner.Name()).Err(err).Msg("goroutine failed")
            } else {
                r.setState(e, "stopped")
                r.log.Info().Str("service", e.runner.Name()).Msg("goroutine stopped cleanly")
            }
        }()
    }
}

// Wait block cho đến khi tất cả goroutines kết thúc.
func (r *Registry) Wait() { r.wg.Wait() }

// HealthAll chạy health check cho tất cả services đồng thời.
func (r *Registry) HealthAll(ctx context.Context) map[string]error {
    r.mu.RLock()
    entries := make(map[string]*serviceEntry, len(r.entries))
    for k, v := range r.entries {
        entries[k] = v
    }
    r.mu.RUnlock()

    results := make(map[string]error, len(entries))
    var mu sync.Mutex
    var wg sync.WaitGroup

    for name, entry := range entries {
        wg.Add(1)
        go func(n string, e *serviceEntry) {
            defer wg.Done()
            err := e.runner.Health(ctx)
            mu.Lock()
            results[n] = err
            mu.Unlock()
        }(name, entry)
    }
    wg.Wait()
    return results
}

// Status trả về trạng thái của tất cả services.
func (r *Registry) Status() map[string]string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    status := make(map[string]string, len(r.entries))
    for k, v := range r.entries {
        status[k] = v.state
    }
    return status
}

func (r *Registry) setState(e *serviceEntry, state string) {
    r.mu.Lock()
    e.state = state
    r.mu.Unlock()
}

func (r *Registry) setError(e *serviceEntry, err error) {
    r.mu.Lock()
    e.err = err
    r.mu.Unlock()
}

func (r *Registry) recoverFromPanic(e *serviceEntry) {
    if rec := recover(); rec != nil {
        r.setError(e, fmt.Errorf("panic: %v", rec))
        r.setState(e, "failed")
        r.log.Error().Str("service", e.runner.Name()).Interface("panic", rec).Msg("goroutine panicked")
    }
}
```

---

## bufconn Transport Helpers

```go
// internal/transport/bufconn.go
package transport

import (
    "context"
    "net"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/test/bufconn"
)

const DefaultBufSize = 1 << 20 // 1MB

// NewBufConnListener tạo in-process gRPC listener.
func NewBufConnListener() *bufconn.Listener {
    return bufconn.Listen(DefaultBufSize)
}

// DialBufConn tạo gRPC client connection đến một bufconn listener.
// Dùng để connect từ API gateway hoặc service này đến service khác.
func DialBufConn(ctx context.Context, lis *bufconn.Listener) (*grpc.ClientConn, error) {
    return grpc.DialContext(ctx,
        "passthrough://bufnet",
        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
            return lis.DialContext(ctx)
        }),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithBlock(),
        grpc.WithTimeout(5*time.Second),
    )
}

// MustDialBufConn là DialBufConn nhưng panic nếu lỗi (dùng trong startup).
func MustDialBufConn(ctx context.Context, lis *bufconn.Listener) *grpc.ClientConn {
    conn, err := DialBufConn(ctx, lis)
    if err != nil {
        panic("transport: failed to dial bufconn: " + err.Error())
    }
    return conn
}
```

---

## Goroutine Startup Pattern

Mỗi runner implement theo pattern sau:

```go
// Ví dụ: runners/scan_runner.go
package runners

import (
    "context"
    "fmt"

    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/test/bufconn"

    // Import từ scan-service (không thay đổi)
    scanpb     "github.com/osv/shared/proto/gen/go/scan/v1"
    scangrpc   "github.com/osv/scan-service/internal/delivery/grpc"
    scanrepo   "github.com/osv/scan-service/internal/adapters/repository/postgres"
    createUC   "github.com/osv/scan-service/internal/usecase/create_scan"
    executeUC  "github.com/osv/scan-service/internal/usecase/execute_scan"
    scanworker "github.com/osv/scan-service/internal/adapters/worker"
    scancron   "github.com/osv/scan-service/internal/scheduler"

    "github.com/osv/apps/openvulnscan/internal/transport"
    "github.com/nats-io/nats.go"
)

type ScanRunner struct {
    cfg    ScanRunnerConfig
    nc     *nats.Conn
    lis    *bufconn.Listener
    server *grpc.Server
    pool   *scanworker.WorkerPool
    cron   *scancron.CronWorker
}

type ScanRunnerConfig struct {
    DBURL          string
    FindingLis     *bufconn.Listener // để tạo finding-service client
    ProductLis     *bufconn.Listener // để tạo product-service client
    WorkerPoolSize int
    NmapBinary     string
    ZAPApiURL      string
}

func NewScanRunner(cfg ScanRunnerConfig, nc *nats.Conn, lis *bufconn.Listener) *ScanRunner {
    return &ScanRunner{cfg: cfg, nc: nc, lis: lis}
}

func (r *ScanRunner) Name() string { return "scan-service" }

func (r *ScanRunner) Run(ctx context.Context) error {
    // 1. Connect DB
    db, err := scanrepo.ConnectDB(r.cfg.DBURL)
    if err != nil { return fmt.Errorf("scan: db: %w", err) }
    defer db.Close()

    // 2. Tạo gRPC clients đến services khác (qua bufconn)
    findingConn, err := transport.DialBufConn(ctx, r.cfg.FindingLis)
    if err != nil { return fmt.Errorf("scan: finding client: %w", err) }
    defer findingConn.Close()

    // 3. Init repos, usecases từ scan-service
    sRepo  := scanrepo.NewScanRepo(db)
    create := createUC.New(sRepo, r.nc)
    exec   := executeUC.New(sRepo, findingConn, r.nc)

    // 4. Worker pool goroutine (bên trong goroutine của scan-service)
    r.pool = scanworker.NewWorkerPool(r.cfg.WorkerPoolSize, exec.Execute)
    go r.pool.Start(ctx)

    // 5. CronWorker goroutine
    r.cron = scancron.NewCronWorker(sRepo, create)
    go r.cron.Start(ctx)

    // 6. Khởi gRPC server
    r.server = grpc.NewServer()
    scanHandler := scangrpc.NewHandler(create, exec, sRepo)
    scanpb.RegisterScanServiceServer(r.server, scanHandler)
    grpc_health_v1.RegisterHealthServer(r.server, health.NewServer())

    errCh := make(chan error, 1)
    go func() { errCh <- r.server.Serve(r.lis) }()

    select {
    case <-ctx.Done():
        r.server.GracefulStop()
        return nil
    case err := <-errCh:
        return fmt.Errorf("scan-service gRPC: %w", err)
    }
}

func (r *ScanRunner) Health(ctx context.Context) error {
    conn, err := transport.DialBufConn(ctx, r.lis)
    if err != nil { return err }
    defer conn.Close()
    hc := grpc_health_v1.NewHealthClient(conn)
    resp, err := hc.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
    if err != nil { return err }
    if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
        return fmt.Errorf("not serving")
    }
    return nil
}
```

---

## Startup Sequence trong main.go

```go
// cmd/server/main.go
func main() {
    cfg := app.LoadConfig("configs/config.yaml")
    a, err := app.New(cfg)
    if err != nil { log.Fatal().Err(err).Msg("init failed") }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start tất cả service goroutines
    if err := a.Start(ctx); err != nil {
        log.Fatal().Err(err).Msg("start failed")
    }

    // Start API Gateway (HTTP server)
    go a.ServeHTTP(ctx)

    log.Info().Str("addr", cfg.Server.HTTPAddr).Msg("OpenVulnScan started")

    // Wait for signal
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
    <-sig

    log.Info().Msg("shutdown initiated")
    cancel() // Trigger context cancellation → tất cả goroutines stop

    // Wait với timeout
    done := make(chan struct{})
    go func() { a.Wait(); close(done) }()

    select {
    case <-done:
        log.Info().Msg("shutdown complete")
    case <-time.After(30 * time.Second):
        log.Warn().Msg("shutdown timeout — forcing exit")
    }
}
```

---

## Dependency Order Diagram

```
authLis ────────────────────────────────────────────────┐
vulnLis ───────────────────────────────────────────┐    │
productLis ────────────────────────────────────┐   │    │
queryLis ──────────────────────────────────┐   │   │    │
findingLis ────────────────────────────┐   │   │   │    │
                                       ↓   ↓   ↓   ↓    ↓
scanLis ←── ScanRunner deps: [findingLis, productLis]
reportLis ←── ReportRunner deps: [findingLis, productLis]
notify ←── NotifyRunner deps: [nc (NATS)]
ingestion ←── IngestionRunner deps: [nc (NATS), vulnLis]

API Gateway ←── deps: [authLis, scanLis, findingLis, productLis,
                        vulnLis, reportLis, queryLis, nc]
```
