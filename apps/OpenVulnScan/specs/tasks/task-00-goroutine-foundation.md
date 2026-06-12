# T00 — Goroutine Foundation

## Thông tin
| | |
|---|---|
| **Phase** | 0 — Foundation |
| **Ước tính** | 3–4 giờ |
| **Depends on** | — |
| **Blocks** | T01, T04–T12 |

## Mục tiêu

Tạo toàn bộ **goroutine infrastructure** — phần core không phụ thuộc vào bất kỳ service nào:
- `ServiceRunner` interface
- `Registry` quản lý goroutine lifecycle
- `transport/bufconn.go` — in-process gRPC transport
- `events/` — NATS JetStream setup

---

## Các bước thực hiện

### 0.1 Tạo `internal/app/registry.go`

```go
// internal/app/registry.go
package app

import (
    "context"
    "fmt"
    "sync"
    "github.com/rs/zerolog"
)

// ServiceRunner là interface mà mỗi service goroutine phải implement.
type ServiceRunner interface {
    Name() string
    Run(ctx context.Context) error
    Health(ctx context.Context) error
}

type serviceState string

const (
    stateIdle    serviceState = "idle"
    stateRunning serviceState = "running"
    stateStopped serviceState = "stopped"
    stateFailed  serviceState = "failed"
)

type entry struct {
    runner ServiceRunner
    state  serviceState
    err    error
}

// Registry quản lý vòng đời của tất cả service goroutines.
type Registry struct {
    mu      sync.RWMutex
    entries map[string]*entry
    wg      sync.WaitGroup
    log     zerolog.Logger
}

func NewRegistry(l zerolog.Logger) *Registry {
    return &Registry{entries: make(map[string]*entry), log: l}
}

// Register đăng ký một ServiceRunner. Gọi trước Start.
func (r *Registry) Register(runner ServiceRunner) {
    r.mu.Lock()
    r.entries[runner.Name()] = &entry{runner: runner, state: stateIdle}
    r.mu.Unlock()
}

// Start khởi động tất cả goroutines.
func (r *Registry) Start(ctx context.Context) {
    r.mu.RLock()
    runners := make([]*entry, 0, len(r.entries))
    for _, e := range r.entries {
        runners = append(runners, e)
    }
    r.mu.RUnlock()

    for _, e := range runners {
        e := e
        r.wg.Add(1)
        go func() {
            defer r.wg.Done()
            defer r.recoverPanic(e)

            r.setEntry(e, stateRunning, nil)
            r.log.Info().Str("svc", e.runner.Name()).Msg("goroutine started")

            err := e.runner.Run(ctx)
            if err != nil && err != context.Canceled {
                r.setEntry(e, stateFailed, err)
                r.log.Error().Str("svc", e.runner.Name()).Err(err).Msg("goroutine failed")
            } else {
                r.setEntry(e, stateStopped, nil)
                r.log.Info().Str("svc", e.runner.Name()).Msg("goroutine stopped")
            }
        }()
    }
}

func (r *Registry) Wait() { r.wg.Wait() }

func (r *Registry) HealthAll(ctx context.Context) map[string]error {
    r.mu.RLock()
    snap := make(map[string]*entry, len(r.entries))
    for k, v := range r.entries { snap[k] = v }
    r.mu.RUnlock()

    out := make(map[string]error, len(snap))
    var mu sync.Mutex
    var wg sync.WaitGroup
    for name, e := range snap {
        wg.Add(1)
        go func(n string, en *entry) {
            defer wg.Done()
            err := en.runner.Health(ctx)
            mu.Lock(); out[n] = err; mu.Unlock()
        }(name, e)
    }
    wg.Wait()
    return out
}

func (r *Registry) Status() map[string]string {
    r.mu.RLock(); defer r.mu.RUnlock()
    s := make(map[string]string, len(r.entries))
    for k, v := range r.entries { s[k] = string(v.state) }
    return s
}

func (r *Registry) setEntry(e *entry, state serviceState, err error) {
    r.mu.Lock(); e.state = state; e.err = err; r.mu.Unlock()
}

func (r *Registry) recoverPanic(e *entry) {
    if rec := recover(); rec != nil {
        err := fmt.Errorf("panic: %v", rec)
        r.setEntry(e, stateFailed, err)
        r.log.Error().Str("svc", e.runner.Name()).Interface("panic", rec).Msg("goroutine panicked")
    }
}
```

**Acceptance**: Có thể register 3 dummy runners, Start(), và tất cả run đồng thời.

---

### 0.2 Tạo `internal/transport/bufconn.go`

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

// DialBufConn tạo gRPC ClientConn qua bufconn (không qua network).
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

// MustDialBufConn panic nếu không kết nối được (dùng trong startup).
func MustDialBufConn(ctx context.Context, lis *bufconn.Listener) *grpc.ClientConn {
    conn, err := DialBufConn(ctx, lis)
    if err != nil { panic("transport.MustDialBufConn: " + err.Error()) }
    return conn
}
```

---

### 0.3 Tạo `internal/events/setup.go`

```go
// internal/events/setup.go
package events

import (
    "errors"
    "time"
    "github.com/nats-io/nats.go"
)

const StreamName = "OPENVULNSCAN"

func SetupJetStream(nc *nats.Conn) (nats.JetStreamContext, error) {
    js, err := nc.JetStream()
    if err != nil { return nil, err }

    _, err = js.AddStream(&nats.StreamConfig{
        Name:      StreamName,
        Subjects:  []string{"ovs.>"},
        Storage:   nats.FileStorage,
        Replicas:  1,
        MaxAge:    7 * 24 * time.Hour,
        Retention: nats.LimitsPolicy,
        Duplicates: 5 * time.Minute,
    })
    if err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
        return nil, err
    }
    return js, nil
}
```

---

### 0.4 Tạo `internal/events/subjects.go`

```go
// internal/events/subjects.go
package events

const (
    // Scan
    SubjScanCreated   = "ovs.scan.created"
    SubjScanCompleted = "ovs.scan.completed"
    SubjScanFailed    = "ovs.scan.failed"

    // Finding
    SubjFindingBatchCreated  = "ovs.finding.batch_created"
    SubjFindingStatusChanged = "ovs.finding.status_changed"

    // Agent
    SubjAgentReportSubmitted = "ovs.agent.report.submitted"

    // Notification
    SubjNotificationDispatch = "ovs.notification.dispatch"
)
```

---

## Output

- [x] `internal/app/registry.go` — ServiceRunner interface + Registry
- [x] `internal/transport/bufconn.go` — bufconn helpers
- [x] `internal/events/setup.go` — JetStream stream setup
- [x] `internal/events/subjects.go` — NATS subject constants

## Trạng thái: ✅ HOÀN THÀNH
> Thực thi: 2026-06-09
> File thực tế: [`internal/app/registry.go`](../../internal/app/registry.go) — thread-safe với RWMutex, panic recovery
> File thực tế: [`internal/transport/bufconn.go`](../../internal/transport/bufconn.go) — bufconn + DialBufConn
> File thực tế: [`internal/events/setup.go`](../../internal/events/setup.go) — NATS JetStream stream OPENVULNSCAN
> File thực tế: [`internal/events/subjects.go`](../../internal/events/subjects.go) — 8 NATS subjects

## Acceptance Criteria

```go
// Test goroutine registry
reg := app.NewRegistry(log)
reg.Register(&mockRunner{name: "svc-a"})
reg.Register(&mockRunner{name: "svc-b"})

ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

reg.Start(ctx)
// Tất cả 2 goroutines đang "running"
assert.Equal(t, map[string]string{"svc-a": "running", "svc-b": "running"}, reg.Status())

cancel()
reg.Wait()
// Tất cả stopped
assert.Equal(t, "stopped", reg.Status()["svc-a"])
```

```go
// Test bufconn round-trip
lis := transport.NewBufConnListener()
// Start dummy gRPC server trên lis
// Dial và gọi RPC
conn, err := transport.DialBufConn(ctx, lis)
assert.NoError(t, err)
```
