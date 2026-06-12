# TASK-03: Service Registry & Transport Layer

**Phase**: 3 — Core Framework  
**Ước tính**: 6 giờ  
**Phụ thuộc**: TASK-02 hoàn thành  
**Output**: ServiceRunner interface, Registry, bufconn transport, app bootstrap

---

## Mục tiêu

Xây dựng framework core: `ServiceRunner` interface mà tất cả 13 service runners implement, `Registry` quản lý goroutine lifecycle, và `bufconn` transport layer cho in-process gRPC.

---

## T-03.1: ServiceRunner Interface

**File**: `apps/DefectDojo/internal/app/registry/runner.go`

```go
// Package registry manages the lifecycle of all service goroutines.
package registry

import "context"

// ServiceRunner is implemented by every service wrapper.
// Each implementation wraps a service from services/ directory.
type ServiceRunner interface {
    // Name returns the unique identifier (e.g., "auth-service").
    Name() string

    // Run starts the service and blocks until ctx is done or error occurs.
    // Must respect ctx cancellation for graceful shutdown.
    Run(ctx context.Context) error

    // Health returns nil if the service is operational.
    Health(ctx context.Context) error
}
```

---

## T-03.2: Registry Implementation

**File**: `apps/DefectDojo/internal/app/registry/registry.go`

```go
package registry

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/rs/zerolog/log"
)

// ServiceState enumerates goroutine lifecycle states.
type ServiceState int

const (
    StateIdle     ServiceState = iota // Registered, not yet started
    StateStarting                     // Run() called, waiting for ready
    StateRunning                      // Operational
    StateStopping                     // ctx cancelled, draining
    StateStopped                      // Clean stop
    StateFailed                       // Exited with error or panic
)

func (s ServiceState) String() string {
    switch s {
    case StateIdle:
        return "idle"
    case StateStarting:
        return "starting"
    case StateRunning:
        return "running"
    case StateStopping:
        return "stopping"
    case StateStopped:
        return "stopped"
    case StateFailed:
        return "failed"
    default:
        return "unknown"
    }
}

// serviceEntry holds a runner and its runtime metadata.
type serviceEntry struct {
    runner    ServiceRunner
    state     ServiceState
    startedAt time.Time
    err       error
    cancel    context.CancelFunc
    mu        sync.Mutex
}

func (e *serviceEntry) setState(s ServiceState) {
    e.mu.Lock()
    e.state = s
    e.mu.Unlock()
}

func (e *serviceEntry) setError(err error) {
    e.mu.Lock()
    e.err = err
    e.mu.Unlock()
}

// Registry manages all ServiceRunners as goroutines.
type Registry struct {
    mu      sync.RWMutex
    entries map[string]*serviceEntry
    wg      sync.WaitGroup

    // Prometheus gauge for service health
    healthGauge *prometheus.GaugeVec
}

// New creates an empty Registry.
func New() *Registry {
    return &Registry{
        entries: make(map[string]*serviceEntry),
    }
}

// Register adds a ServiceRunner. Must be called before Start.
func (r *Registry) Register(runner ServiceRunner) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.entries[runner.Name()] = &serviceEntry{
        runner: runner,
        state:  StateIdle,
    }
    log.Debug().Str("service", runner.Name()).Msg("service registered")
}

// Start launches all registered services as goroutines under parentCtx.
func (r *Registry) Start(parentCtx context.Context) {
    r.mu.RLock()
    entries := make(map[string]*serviceEntry, len(r.entries))
    for k, v := range r.entries {
        entries[k] = v
    }
    r.mu.RUnlock()

    for name, entry := range entries {
        svcCtx, cancel := context.WithCancel(parentCtx)
        entry.cancel = cancel
        entry.setState(StateStarting)
        entry.startedAt = time.Now()

        r.wg.Add(1)
        go r.runService(svcCtx, entry)

        log.Info().Str("service", name).Msg("starting service goroutine")
    }
}

// runService is the goroutine wrapper for a ServiceRunner.
func (r *Registry) runService(ctx context.Context, entry *serviceEntry) {
    defer r.wg.Done()
    name := entry.runner.Name()

    // Panic recovery
    defer func() {
        if rec := recover(); rec != nil {
            entry.setError(fmt.Errorf("panic: %v", rec))
            entry.setState(StateFailed)
            log.Error().
                Str("service", name).
                Interface("panic", rec).
                Msg("service goroutine panicked")
        }
    }()

    entry.setState(StateRunning)
    log.Info().Str("service", name).Msg("service goroutine running")

    if err := entry.runner.Run(ctx); err != nil && err != context.Canceled {
        entry.setError(err)
        entry.setState(StateFailed)
        log.Error().Str("service", name).Err(err).Msg("service exited with error")
        return
    }

    entry.setState(StateStopped)
    log.Info().Str("service", name).Msg("service goroutine stopped cleanly")
}

// Stop cancels a specific service's context.
func (r *Registry) Stop(name string) {
    r.mu.RLock()
    entry, ok := r.entries[name]
    r.mu.RUnlock()

    if !ok || entry.cancel == nil {
        return
    }

    entry.setState(StateStopping)
    entry.cancel()
    log.Info().Str("service", name).Msg("stop signal sent")
}

// StopAll cancels all services and waits for all goroutines to exit.
func (r *Registry) StopAll() {
    r.mu.RLock()
    for name, entry := range r.entries {
        if entry.cancel != nil {
            entry.setState(StateStopping)
            entry.cancel()
            log.Info().Str("service", name).Msg("stop signal sent")
        }
    }
    r.mu.RUnlock()
    r.wg.Wait()
}

// Wait blocks until all service goroutines have exited.
func (r *Registry) Wait() {
    r.wg.Wait()
}

// Status returns a snapshot of all service states.
func (r *Registry) Status() map[string]ServiceState {
    r.mu.RLock()
    defer r.mu.RUnlock()

    result := make(map[string]ServiceState, len(r.entries))
    for name, entry := range r.entries {
        entry.mu.Lock()
        result[name] = entry.state
        entry.mu.Unlock()
    }
    return result
}

// HealthAll runs health checks on all registered services.
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
```

---

## T-03.3: Transport — bufconn Helper

**File**: `apps/DefectDojo/internal/transport/bufconn.go`

```go
// Package transport provides in-process gRPC transport using bufconn.
package transport

import (
    "context"
    "net"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/test/bufconn"
)

const defaultBufSize = 1 * 1024 * 1024 // 1MB

// Dialer is a factory function that creates a gRPC client connection.
// Used to pass connectivity from server (runner) to client (other runners).
type Dialer func(ctx context.Context) (*grpc.ClientConn, error)

// NewBufConnListener creates a buffered in-process listener.
func NewBufConnListener() *bufconn.Listener {
    return bufconn.Listen(defaultBufSize)
}

// MakeBufConnDialer wraps a bufconn.Listener in a Dialer.
// The returned Dialer creates new client connections to the given listener.
func MakeBufConnDialer(lis *bufconn.Listener) Dialer {
    return func(ctx context.Context) (*grpc.ClientConn, error) {
        return grpc.NewClient("passthrough://bufnet",
            grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
                return lis.DialContext(ctx)
            }),
            grpc.WithTransportCredentials(insecure.NewCredentials()),
        )
    }
}

// MustDial calls d(ctx) and panics on error (use for startup wiring).
func MustDial(ctx context.Context, d Dialer) *grpc.ClientConn {
    conn, err := d(ctx)
    if err != nil {
        panic("transport.MustDial: " + err.Error())
    }
    return conn
}
```

---

## T-03.4: Base Runner Helper

**File**: `apps/DefectDojo/internal/runners/base.go`

```go
// Package runners implements ServiceRunner for each service in services/.
package runners

import (
    "context"
    "fmt"
    "net"
    "time"

    "github.com/defectdojo/apps/defectdojo/internal/transport"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/test/bufconn"
)

// waitForGRPC polls the gRPC server on lis until it responds or ctx expires.
func waitForGRPC(ctx context.Context, lis *bufconn.Listener, name string) error {
    dialer := transport.MakeBufConnDialer(lis)
    ticker := time.NewTicker(50 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return fmt.Errorf("%s: timed out waiting for gRPC ready", name)
        case <-ticker.C:
            conn, err := dialer(ctx)
            if err != nil {
                continue
            }
            conn.Close()
            log.Debug().Str("service", name).Msg("gRPC server ready")
            return nil
        }
    }
}

// newPgxPool opens a pgxpool connection. Helper for runners.
func newPgxPool(ctx context.Context, url, service string) (*pgxpool.Pool, error) {
    pool, err := pgxpool.New(ctx, url)
    if err != nil {
        return nil, fmt.Errorf("%s: db connect: %w", service, err)
    }
    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("%s: db ping: %w", service, err)
    }
    return pool, nil
}

// grpcHealthCheck performs a gRPC health check via bufconn.
func grpcHealthCheck(ctx context.Context, lis *bufconn.Listener) error {
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    dialer := transport.MakeBufConnDialer(lis)
    conn, err := dialer(ctx)
    if err != nil {
        return err
    }
    defer conn.Close()

    client := grpc_health_v1.NewHealthClient(conn)
    resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
    if err != nil {
        return err
    }
    if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
        return fmt.Errorf("not serving: %s", resp.Status)
    }
    return nil
}

// mustListen panics if bufconn.Listen fails (should not happen).
func mustListen() *bufconn.Listener {
    return bufconn.Listen(1 << 20) // 1MB
}
```

---

## T-03.5: App Bootstrap

**File**: `apps/DefectDojo/internal/app/app.go`

```go
// Package app bootstraps the DefectDojo monolith.
package app

import (
    "context"
    "fmt"

    "github.com/defectdojo/apps/defectdojo/internal/app/registry"
    "github.com/defectdojo/apps/defectdojo/internal/config"
    "github.com/defectdojo/apps/defectdojo/internal/events"
    "github.com/defectdojo/apps/defectdojo/internal/migration"
    "github.com/defectdojo/apps/defectdojo/internal/transport"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/nats-io/nats.go"
    "github.com/redis/go-redis/v9"
    "google.golang.org/grpc/test/bufconn"
)

// App is the monolith application.
type App struct {
    cfg      *config.Config
    registry *registry.Registry

    // Shared infrastructure
    db    *pgxpool.Pool
    nc    *nats.Conn
    redis *redis.Client

    // In-process gRPC listeners (one per service)
    // These are initialized in createListeners().
    authLis    *bufconn.Listener
    productLis *bufconn.Listener
    findingLis *bufconn.Listener
    scanLis    *bufconn.Listener
    vulnLis    *bufconn.Listener
    notifLis   *bufconn.Listener
    reportLis  *bufconn.Listener
    searchLis  *bufconn.Listener
    integLis   *bufconn.Listener
}

// New creates a new App (does NOT connect or start).
func New(cfg *config.Config) (*App, error) {
    return &App{
        cfg:      cfg,
        registry: registry.New(),
    }, nil
}

// Start connects to infrastructure, runs migrations, and launches all services.
func (a *App) Start(ctx context.Context) error {
    // Step 1: Connect infrastructure
    if err := a.connectInfra(ctx); err != nil {
        return fmt.Errorf("connect infra: %w", err)
    }

    // Step 2: Run DB migrations
    if err := migration.RunAll(ctx, a.db, "../../services"); err != nil {
        return fmt.Errorf("run migrations: %w", err)
    }

    // Step 3: Setup NATS JetStream
    js, err := a.nc.JetStream()
    if err != nil {
        return fmt.Errorf("jetstream: %w", err)
    }
    if err := events.SetupJetStream(js); err != nil {
        return fmt.Errorf("setup jetstream: %w", err)
    }

    // Step 4: Create in-process gRPC listeners
    a.createListeners()

    // Step 5: Register & start all service runners
    a.registerServices(ctx, js)
    a.registry.Start(ctx)

    return nil
}

// createListeners initializes one bufconn.Listener per service.
func (a *App) createListeners() {
    a.authLis    = transport.NewBufConnListener()
    a.productLis = transport.NewBufConnListener()
    a.findingLis = transport.NewBufConnListener()
    a.scanLis    = transport.NewBufConnListener()
    a.vulnLis    = transport.NewBufConnListener()
    a.notifLis   = transport.NewBufConnListener()
    a.reportLis  = transport.NewBufConnListener()
    a.searchLis  = transport.NewBufConnListener()
    a.integLis   = transport.NewBufConnListener()
}

// Shutdown gracefully stops all services.
func (a *App) Shutdown(ctx context.Context) error {
    // 1. Stop gateway first (reject new HTTP/gRPC requests)
    a.registry.Stop("unified-gateway")

    // 2. Drain NATS (wait for in-flight messages)
    if err := a.nc.Drain(); err != nil {
        // Log but don't fail — best effort
        _ = err
    }

    // 3. Stop all remaining services
    a.registry.StopAll()

    // 4. Close infrastructure
    a.db.Close()
    a.redis.Close()
    a.nc.Close()

    return nil
}
```

> **Lưu ý**: `registerServices()` được implement trong TASK-04 sau khi tất cả runners được tạo.

---

## T-03.6: Update main.go

**File**: `apps/DefectDojo/cmd/defectdojo/main.go` (update từ TASK-01)

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/defectdojo/apps/defectdojo/internal/app"
    "github.com/defectdojo/apps/defectdojo/internal/config"
    "github.com/rs/zerolog/log"
)

func main() {
    // Load config
    cfg, err := config.Load()
    if err != nil {
        log.Fatal().Err(err).Msg("failed to load config")
    }

    // Setup logging
    config.SetupLogging(&cfg.Log)
    log.Info().Msg("DefectDojo Go starting...")

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Create application
    application, err := app.New(cfg)
    if err != nil {
        log.Fatal().Err(err).Msg("failed to create application")
    }

    // Start all services
    if err := application.Start(ctx); err != nil {
        log.Fatal().Err(err).Msg("failed to start application")
    }

    log.Info().Str("http_port", cfg.HTTPPort).Str("grpc_port", cfg.GRPCPort).
        Msg("DefectDojo Go is running")

    // Wait for shutdown signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Info().Msg("shutdown signal received")

    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer shutdownCancel()

    if err := application.Shutdown(shutdownCtx); err != nil {
        log.Error().Err(err).Msg("shutdown error")
    }

    log.Info().Msg("DefectDojo Go stopped")
}
```

---

## Definition of Done

- [ ] `ServiceRunner` interface được định nghĩa
- [ ] `Registry`: Register, Start, Stop, StopAll, HealthAll hoạt động
- [ ] `Registry` có panic recovery cho mỗi goroutine
- [ ] `transport.BufConnDialer` factory hoạt động
- [ ] `app.App` struct với `Start` và `Shutdown` methods
- [ ] `main.go` sử dụng `app.New()` và `app.Start()`
- [ ] `go build ./...` pass
- [ ] Unit test cho Registry (start/stop/status)
