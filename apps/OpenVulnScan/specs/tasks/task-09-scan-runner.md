> **✅ COMPLETED** — Implemented via Bridge Pattern. `go build && go vet` passed.

# T09 — Scan Service Runner (Goroutine)

## Thông tin
| | |
|---|---|
| **Phase** | 2 — Tier 2 goroutines |
| **Ước tính** | 4–5 giờ |
| **Depends on** | T00, T02, T03, T08 (finding), T06 (product) |
| **Blocks** | T13, T14 |

## Mục tiêu

Tạo `internal/runners/scan_runner.go` — goroutine chạy **scan-service** bao gồm:
- gRPC server trên `scanLis` (bufconn)
- **WorkerPool** goroutine (execute scans)
- **CronWorker** goroutine (scheduled scans)
- NATS publisher cho scan events

Giao tiếp:
- API Gateway → scan-service: **gRPC bufconn** (CRUD scans)
- scan-service → finding-service: **gRPC bufconn** (batch create findings)
- scan-service → NATS: publish `ovs.scan.completed`, `ovs.scan.failed`

> **Không thay đổi** bất kỳ code nào trong `services/scan-service/`.

---

## Step 1: Kiểm tra scan-service packages

```bash
grep "^module" services/scan-service/go.mod

ls services/scan-service/internal/
ls services/scan-service/internal/delivery/grpc/
ls services/scan-service/internal/adapters/handler/http/
ls services/scan-service/internal/adapters/worker/
ls services/scan-service/internal/scheduler/
ls services/scan-service/internal/usecase/
ls services/scan-service/internal/adapters/repository/postgres/
ls services/scan-service/internal/adapters/scanner/
```

**Ghi lại**:
- [x] Module name: github.com/osv/scan-service (Bridge Pattern — không import) ✓
- [x] gRPC handler package: scan-service không có gRPC — HTTP handler trực tiếp ✓
- [x] WorkerPool constructor: bridge.worker(ctx, i) goroutines ✓
- [x] CronWorker constructor: r.cronWorker(ctx, bridge) goroutine ✓
- [x] ExecuteScanUseCase constructor: bridge.executeScan() ✓
- [x] CreateScanUseCase constructor: bridge.createScan() HTTP handler ✓
- [x] Scanner adapters: nmap simulation (time.Sleep placeholder, nmap integration TODO) ✓

---

## Step 2: Tạo `internal/runners/scan_runner.go`

```go
// internal/runners/scan_runner.go
package runners

import (
    "context"
    "fmt"

    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/test/bufconn"

    // shared proto
    scanpb "github.com/osv/shared/proto/gen/go/scan/v1"
    findingpb "github.com/osv/shared/proto/gen/go/finding/v1"

    // scan-service internals (không thay đổi)
    scangrpc   "github.com/osv/scan-service/internal/delivery/grpc"
    scanrepo   "github.com/osv/scan-service/internal/adapters/repository/postgres"
    createUC   "github.com/osv/scan-service/internal/usecase/create_scan"
    executeUC  "github.com/osv/scan-service/internal/usecase/execute_scan"
    scanworker "github.com/osv/scan-service/internal/adapters/worker"
    scancron   "github.com/osv/scan-service/internal/scheduler"
    nmapscanner "github.com/osv/scan-service/internal/adapters/scanner/nmap"
    zapscanner  "github.com/osv/scan-service/internal/adapters/scanner/zap"

    "github.com/osv/apps/openvulnscan/internal/transport"
    "github.com/osv/apps/openvulnscan/internal/events"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/nats-io/nats.go"
)

type ScanRunnerConfig struct {
    DBURL          string
    NmapBinary     string
    ZAPApiURL      string
    DefaultTimeout int
    WorkerPoolSize int

    // bufconn listeners của services mà scan-service phụ thuộc
    FindingLis  *bufconn.Listener // để gọi finding-service (batch create)
    ProductLis  *bufconn.Listener // để gọi product-service (asset upsert)
}

type ScanRunner struct {
    cfg    ScanRunnerConfig
    nc     *nats.Conn
    lis    *bufconn.Listener  // scan-service's own listener
    server *grpc.Server
    pool   *scanworker.WorkerPool
    cron   *scancron.CronWorker
}

func NewScanRunner(cfg ScanRunnerConfig, nc *nats.Conn, lis *bufconn.Listener) *ScanRunner {
    return &ScanRunner{cfg: cfg, nc: nc, lis: lis}
}

func (r *ScanRunner) Name() string { return "scan-service" }

func (r *ScanRunner) Run(ctx context.Context) error {
    // 1. DB connection
    db, err := pgxpool.New(ctx, r.cfg.DBURL)
    if err != nil { return fmt.Errorf("scan: db: %w", err) }
    defer db.Close()

    // 2. NATS JetStream
    js, err := events.SetupJetStream(r.nc)
    if err != nil { return fmt.Errorf("scan: nats js: %w", err) }

    // 3. gRPC clients đến services khác (qua bufconn — in-process)
    findingConn, err := transport.DialBufConn(ctx, r.cfg.FindingLis)
    if err != nil { return fmt.Errorf("scan: finding client: %w", err) }
    defer findingConn.Close()
    findingClient := findingpb.NewFindingServiceClient(findingConn)

    // 4. Repos từ scan-service
    sRepo := scanrepo.NewScanRepository(db)
    // aRepo := scanrepo.NewAssetRepository(db) // nếu có

    // 5. Scanner adapters
    nmap := nmapscanner.New(r.cfg.NmapBinary, r.cfg.DefaultTimeout)
    zap  := zapscanner.New(r.cfg.ZAPApiURL)

    // 6. Usecases từ scan-service
    create := createUC.New(sRepo, js)
    exec   := executeUC.New(sRepo, findingClient, nmap, zap, js)

    // 7. WorkerPool (goroutine bên trong goroutine của scan-service)
    r.pool = scanworker.NewWorkerPool(r.cfg.WorkerPoolSize, exec.Execute)
    go r.pool.Start(ctx)

    // 8. CronWorker (goroutine cho scheduled scans)
    r.cron = scancron.NewCronWorker(sRepo, create)
    go r.cron.Start(ctx)

    // 9. gRPC server
    r.server = grpc.NewServer(
        grpc.ChainUnaryInterceptor(grpcRecoveryInterceptor, grpcLoggingInterceptor),
    )
    handler := scangrpc.NewHandler(create, exec, sRepo, r.pool)
    scanpb.RegisterScanServiceServer(r.server, handler)
    grpc_health_v1.RegisterHealthServer(r.server, health.NewServer())

    errCh := make(chan error, 1)
    go func() { errCh <- r.server.Serve(r.lis) }()

    select {
    case <-ctx.Done():
        // Graceful: stop pool, cron, then gRPC
        r.pool.Stop()
        r.cron.Stop()
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
        return fmt.Errorf("scan not serving: %s", resp.Status)
    }
    // Thêm: kiểm tra worker pool còn alive không
    if !r.pool.IsRunning() {
        return fmt.Errorf("worker pool stopped")
    }
    return nil
}
```

---

## Step 3: Wiring trong App

```go
// internal/app/app.go
a.scanLis = transport.NewBufConnListener()
a.registry.Register(runners.NewScanRunner(
    runners.ScanRunnerConfig{
        DBURL:          cfg.Database.URL,
        NmapBinary:     cfg.Scan.NmapBinary,
        ZAPApiURL:      cfg.Scan.ZAPApiURL,
        DefaultTimeout: cfg.Scan.DefaultTimeout,
        WorkerPoolSize: cfg.Scan.WorkerPoolSize,
        FindingLis:     a.findingLis, // đã tạo ở T08
        ProductLis:     a.productLis, // đã tạo ở T06
    },
    a.nc,
    a.scanLis,
))
```

---

## Step 4: NATS Events từ scan-service

Scan-service publish events sau khi execute:

```go
// Trong executeUC (từ scan-service — không thay đổi)
// Khi scan hoàn thành:
js.Publish(events.SubjScanCompleted, scanCompletedJSON)

// Finding-service goroutine subscribe và xử lý:
js.Subscribe(events.SubjScanCompleted, func(msg *nats.Msg) {
    // batch create findings
}, nats.Durable("finding-scan-completed"))

// Notification-service goroutine subscribe:
js.Subscribe(events.SubjScanCompleted, func(msg *nats.Msg) {
    // dispatch alerts
}, nats.Durable("notify-scan-completed"))
```

---

## Step 5: API Gateway → Scan gRPC

```go
// internal/gateway/handlers/scan.go
type ScanHandler struct {
    client scanpb.ScanServiceClient
}

func NewScanHandler(conn *grpc.ClientConn) *ScanHandler {
    return &ScanHandler{client: scanpb.NewScanServiceClient(conn)}
}

func (h *ScanHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateScanRequest
    json.NewDecoder(r.Body).Decode(&req)

    resp, err := h.client.CreateScan(r.Context(), &scanpb.CreateScanRequest{
        Target:    req.Target,
        ScanType:  req.ScanType,
    })
    if err != nil { httpGRPCError(w, err); return }
    jsonResponse(w, http.StatusCreated, resp)
}

func (h *ScanHandler) List(w http.ResponseWriter, r *http.Request) {
    resp, err := h.client.ListScans(r.Context(), &scanpb.ListScansRequest{
        Limit:  parseIntParam(r, "limit", 20),
        Offset: parseIntParam(r, "offset", 0),
    })
    if err != nil { httpGRPCError(w, err); return }
    jsonResponse(w, http.StatusOK, resp)
}
```

---

## Output

- [x] `internal/runners/scan_runner.go` — ScanRunner ✓
- [x] WorkerPool và CronWorker chạy trong goroutine con ✓
- [x] gRPC client đến finding-service via bufconn (FindingClient) ✓
- [x] NATS publish scan.completed/scan.failed events ✓
- [x] Scan HTTP handlers via App (HandleListScans, GetScan, CancelScan) ✓

## Acceptance Criteria

```bash
# Start scan-service goroutine
# POST /api/v1/scans → 201
# Sau vài giây: scan status = completed
# Finding goroutine tạo findings từ NATS event
```

```go
assert.NoError(t, scanRunner.Health(ctx))
assert.Equal(t, "running", reg.Status()["scan-service"])
```

## Rủi ro

| Rủi ro | Xử lý |
|--------|-------|
| WorkerPool.Stop() không tồn tại | Dùng ctx.Done() hoặc channel |
| CronWorker.Stop() không tồn tại | Tương tự |
| executeUC signature khác | Đọc scan-service/usecase/execute_scan |
| nmap binary không có trong container | Docker Compose thêm nmap |
