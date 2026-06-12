# Chiến lược giao tiếp giữa các Service Goroutines (v3)

## Tổng quan

Trong kiến trúc **goroutine-per-service**, mỗi service chạy trong goroutine riêng biệt, giao tiếp qua 3 cơ chế:

| Cơ chế | Khi nào dùng | Ưu điểm |
|---|---|---|
| **gRPC bufconn** | Service-to-service calls (in-process) | Zero network, type-safe, streaming |
| **NATS JetStream** | Async events, fan-out, pub/sub | Durable, at-least-once, decoupled |
| **REST HTTP** | External clients, agent endpoints | Standard, tooling-friendly |

---

## 1. gRPC bufconn — In-Process gRPC

### Nguyên lý

`google.golang.org/grpc/test/bufconn` cho phép chạy gRPC **không dùng network socket**. Mỗi service goroutine tạo một `bufconn.Listener`, các clients kết nối qua `bufconn.DialContext`.

```go
// transport/bufconn.go
package transport

import (
    "context"
    "net"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/test/bufconn"
)

const bufSize = 1 << 20 // 1MB

func NewBufConnListener() *bufconn.Listener {
    return bufconn.Listen(bufSize)
}

// MakeBufConnDialer tạo gRPC dialer cho bufconn listener
func MakeBufConnDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
    return func(ctx context.Context, _ string) (net.Conn, error) {
        return lis.DialContext(ctx)
    }
}

// DialBufConn tạo gRPC client connection qua bufconn
func DialBufConn(ctx context.Context, lis *bufconn.Listener) (*grpc.ClientConn, error) {
    return grpc.DialContext(ctx, "passthrough://bufnet",
        grpc.WithContextDialer(MakeBufConnDialer(lis)),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
}
```

### Triển khai cho từng service goroutine

```go
// internal/app/app.go — khởi tạo listeners
type App struct {
    // bufconn listeners — mỗi service goroutine tạo 1 listener
    authLis    *bufconn.Listener
    scanLis    *bufconn.Listener
    findingLis *bufconn.Listener
    productLis *bufconn.Listener
    vulnLis    *bufconn.Listener
    reportLis  *bufconn.Listener
    queryLis   *bufconn.Listener
}

func (a *App) createListeners() {
    a.authLis    = transport.NewBufConnListener()
    a.scanLis    = transport.NewBufConnListener()
    a.findingLis = transport.NewBufConnListener()
    a.productLis = transport.NewBufConnListener()
    a.vulnLis    = transport.NewBufConnListener()
    a.reportLis  = transport.NewBufConnListener()
    a.queryLis   = transport.NewBufConnListener()
}
```

### Service goroutine pattern

```go
// Mỗi service goroutine = gRPC server + listener + business logic

// auth-service goroutine
func (a *App) runAuthService(ctx context.Context) error {
    grpcSrv := grpc.NewServer(
        grpc.ChainUnaryInterceptor(loggingInterceptor, recoveryInterceptor),
    )

    // Import và register handler từ auth-service
    authHandler := authgrpc.NewHandler(
        a.loginUC, a.registerUC, a.tokenUC, a.oauthUC,
    )
    authpb.RegisterAuthServiceServer(grpcSrv, authHandler)

    // Health check
    grpc_health_v1.RegisterHealthServer(grpcSrv, health.NewServer())

    errCh := make(chan error, 1)
    go func() { errCh <- grpcSrv.Serve(a.authLis) }()

    select {
    case <-ctx.Done():
        grpcSrv.GracefulStop()
        return nil
    case err := <-errCh:
        return fmt.Errorf("auth-service grpc: %w", err)
    }
}
```

### Kết nối client qua bufconn

```go
// API Gateway kết nối đến service goroutines
func (a *App) connectClients(ctx context.Context) error {
    var err error

    // Auth client
    authConn, err := transport.DialBufConn(ctx, a.authLis)
    if err != nil { return err }
    a.authClient = authpb.NewAuthServiceClient(authConn)

    // Scan client
    scanConn, err := transport.DialBufConn(ctx, a.scanLis)
    if err != nil { return err }
    a.scanClient = scanpb.NewScanServiceClient(scanConn)

    // Finding client
    findingConn, err := transport.DialBufConn(ctx, a.findingLis)
    if err != nil { return err }
    a.findingClient = findingpb.NewFindingServiceClient(findingConn)

    // ... tương tự cho product, vuln, report, query
    return nil
}
```

---

## 2. NATS JetStream — Async Event Bus

### Stream setup

```go
// internal/events/setup.go
package events

const (
    StreamName    = "OPENVULNSCAN"
    StreamSubject = "ovs.>"
)

// SetupJetStream tạo stream nếu chưa tồn tại
func SetupJetStream(nc *nats.Conn) (nats.JetStreamContext, error) {
    js, err := nc.JetStream()
    if err != nil { return nil, err }

    _, err = js.AddStream(&nats.StreamConfig{
        Name:      StreamName,
        Subjects:  []string{StreamSubject},
        Storage:   nats.FileStorage,
        Replicas:  1,
        MaxAge:    7 * 24 * time.Hour,
        Retention: nats.LimitsPolicy,
    })
    if err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
        return nil, err
    }
    return js, nil
}
```

### Event subjects

```go
// internal/events/subjects.go
const (
    // Scan lifecycle
    SubjScanCreated   = "ovs.scan.created"
    SubjScanCompleted = "ovs.scan.completed"
    SubjScanFailed    = "ovs.scan.failed"

    // Finding lifecycle
    SubjFindingBatchCreated = "ovs.finding.batch_created"
    SubjFindingStatusChanged = "ovs.finding.status_changed"

    // Agent
    SubjAgentReportSubmitted = "ovs.agent.report.submitted"

    // Notification
    SubjNotificationDispatch = "ovs.notification.dispatch"
)
```

### Fan-out pattern

```go
// scan.completed → finding-service + notification-service
// Scan goroutine publish:
js.Publish(SubjScanCompleted, scanEventJSON)

// Finding goroutine subscribe:
js.Subscribe(SubjScanCompleted, func(msg *nats.Msg) {
    // batch create findings
    findingUC.BatchCreate(ctx, scanResult)
    msg.Ack()
}, nats.Durable("finding-scan-completed"))

// Notification goroutine subscribe:
js.Subscribe(SubjScanCompleted, func(msg *nats.Msg) {
    // check rules, dispatch alerts
    notifyUC.HandleScanEvent(ctx, scanEvent)
    msg.Ack()
}, nats.Durable("notify-scan-completed"))
```

---

## 3. REST HTTP — API Gateway & External

### API Gateway (HTTP → gRPC)

API Gateway nhận HTTP request từ client, chuyển đổi sang gRPC call đến service goroutine phù hợp:

```go
// internal/gateway/handlers/scan.go
func (h *ScanHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateScanRequest
    json.NewDecoder(r.Body).Decode(&req)

    // Gọi scan-service goroutine qua gRPC bufconn
    resp, err := h.scanClient.CreateScan(r.Context(), &scanpb.CreateScanRequest{
        Target:     req.Target,
        ScanType:   req.ScanType,
        ScheduledAt: req.ScheduledAt,
    })
    if err != nil {
        httpError(w, err)
        return
    }
    jsonResponse(w, resp)
}
```

### Internal REST (service → service, fallback)

Khi service chưa có gRPC proto, dùng HTTP nội bộ với localhost port:

```go
// Chỉ dùng khi không có gRPC option
// report-service REST (nếu chưa có proto)
reportURL := "http://localhost:" + cfg.Report.InternalPort
resp, err := http.Post(reportURL+"/generate", "application/json", body)
```

---

## 4. Chiến lược lựa chọn protocol

| Tình huống | Protocol | Lý do |
|---|---|---|
| API Gateway → auth-service | **gRPC bufconn** | Sync, low latency, JWT validation trên mỗi request |
| API Gateway → scan CRUD | **gRPC bufconn** | Type-safe, Scan proto đã có |
| API Gateway → finding query | **gRPC bufconn** | Finding proto đã có |
| API Gateway → product/asset | **gRPC bufconn** | Product proto đã có |
| API Gateway → CVE lookup | **gRPC bufconn** | Vuln proto đã có |
| API Gateway → dashboard | **gRPC bufconn** | Query proto hoặc direct usecase |
| API Gateway → PDF report | **gRPC bufconn** | Report proto hoặc streaming |
| Scan execute (async) | **NATS** + WorkerPool | Fire-and-forget, không cần wait |
| Scan cron scheduler | ticker goroutine | Internal, không cần IPC |
| Scan complete → findings | **NATS** JetStream | Fan-out, durable |
| Findings → notification | **NATS** JetStream | Fan-out, durable |
| Agent report → OSV enrich | **NATS** JetStream | Heavy IO, không block HTTP |
| SIEM / syslog forward | NATS → syslog channel | Trigger từ notification rules |
| External agent ↔ server | **REST** HTTP | Standard, agent-friendly |

---

## 5. Flow hoàn chỉnh: Tạo Scan (v3)

```
POST /api/v1/scans (HTTP)
    │
    ▼ JWT middleware → gRPC call đến auth-service goroutine (bufconn)
    │
    ▼ API Gateway handler
    │
    ├── gRPC call → scan-service goroutine (bufconn)
    │   └── createScanUC.Execute()
    │       ├── INSERT scan (PostgreSQL)
    │       └── NATS publish: ovs.scan.created
    │
    └── Return HTTP 201 {scan_id}

[Goroutine: scan-worker-pool]
    ├── Subscribe NATS: ovs.scan.created
    ├── executeScanUC.Execute() → nmap/zap
    ├── UPDATE scan status=completed (PostgreSQL)
    └── NATS publish: ovs.scan.completed

[Goroutine: finding-svc — NATS subscriber]
    ├── Subscribe: ovs.scan.completed
    └── findingUC.BatchCreate() → INSERT findings

[Goroutine: notify-svc — NATS subscriber]
    ├── Subscribe: ovs.scan.completed + ovs.finding.batch_created
    └── dispatch → email/slack/syslog channels
```

---

## 6. Registry & Lifecycle

```go
// internal/app/registry.go
type ServiceRunner interface {
    Name() string
    Run(ctx context.Context) error
    Health(ctx context.Context) error
}

type Registry struct {
    runners []ServiceRunner
    wg      sync.WaitGroup
}

func (r *Registry) Start(ctx context.Context) {
    for _, svc := range r.runners {
        r.wg.Add(1)
        go func(s ServiceRunner) {
            defer r.wg.Done()
            if err := s.Run(ctx); err != nil && err != context.Canceled {
                log.Error().Str("service", s.Name()).Err(err).Msg("service exited")
            }
        }(svc)
    }
}

func (r *Registry) Wait() { r.wg.Wait() }
```
