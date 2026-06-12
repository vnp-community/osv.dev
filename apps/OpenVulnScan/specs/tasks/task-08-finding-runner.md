> **✅ COMPLETED** — Implemented via Bridge Pattern. `go build && go vet` passed.

# T08 — Finding Service Runner (Goroutine)

## Thông tin
| | |
|---|---|
| **Phase** | 2 — Tier 2 goroutines |
| **Ước tính** | 3–4 giờ |
| **Depends on** | T00, T02, T03, T06 (product) |
| **Blocks** | T09 (scan), T10 (report), T13 (gateway) |

## Mục tiêu

Tạo `internal/runners/finding_runner.go` — goroutine chạy **finding-service** với:
- gRPC server trên `findingLis` (bufconn)
- NATS subscriber: nhận `ovs.scan.completed` → batch create findings
- SLA computation goroutine (nếu có trong finding-service)

Giao tiếp:
- scan-service → finding-service: **gRPC bufconn** (batch create findings)
- scan-service → NATS → finding-service: async processing
- API Gateway → finding-service: **gRPC bufconn** (query findings)

---

## Step 1: Kiểm tra finding-service packages

```bash
grep "^module" services/finding-service/go.mod
ls services/finding-service/internal/
ls services/finding-service/internal/delivery/grpc/   # hoặc adapter/grpc/
ls services/finding-service/internal/usecase/
ls services/finding-service/internal/infra/repository/
ls services/finding-service/internal/infra/messaging/
```

**Ghi lại**:
- [x] Module name: github.com/osv/finding-service (Bridge Pattern — không import) ✓
- [x] gRPC handler package: findingBridge implements findingv1.FindingServiceServer ✓
- [x] BatchCreate usecase: findingBridge.BatchCreateFindings() với direct Postgres ✓
- [x] NATS subscriber: scan.completed → handleScanCompleted() → BatchCreateFindings ✓

---

## Step 2: Tạo `internal/runners/finding_runner.go`

```go
// internal/runners/finding_runner.go
package runners

import (
    "context"
    "encoding/json"
    "fmt"

    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/test/bufconn"

    // shared proto
    findingpb "github.com/osv/shared/proto/gen/go/finding/v1"

    // finding-service internals
    findinggrpc   "github.com/defectdojo/finding-service/internal/delivery/grpc"
    findingbatch  "github.com/defectdojo/finding-service/internal/usecase/batch_create"
    findingstatus "github.com/defectdojo/finding-service/internal/usecase/status_transition"
    findingrepo   "github.com/defectdojo/finding-service/internal/infra/repository/postgres"

    "github.com/osv/apps/openvulnscan/internal/transport"
    "github.com/osv/apps/openvulnscan/internal/events"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/nats-io/nats.go"
)

type FindingRunnerConfig struct {
    DBURL      string
    ProductLis *bufconn.Listener // để gọi product-service (nếu cần)
}

type FindingRunner struct {
    cfg    FindingRunnerConfig
    nc     *nats.Conn
    lis    *bufconn.Listener
    server *grpc.Server
}

func NewFindingRunner(cfg FindingRunnerConfig, nc *nats.Conn, lis *bufconn.Listener) *FindingRunner {
    return &FindingRunner{cfg: cfg, nc: nc, lis: lis}
}

func (r *FindingRunner) Name() string { return "finding-service" }

func (r *FindingRunner) Run(ctx context.Context) error {
    // 1. DB
    db, err := pgxpool.New(ctx, r.cfg.DBURL)
    if err != nil { return fmt.Errorf("finding: db: %w", err) }
    defer db.Close()

    // 2. Repos
    findingRepo := findingrepo.NewFindingRepository(db)
    slaRepo     := findingrepo.NewSLARepository(db) // nếu có

    // 3. Usecases
    batchUC  := findingbatch.New(findingRepo, slaRepo)
    statusUC := findingstatus.New(findingRepo)

    // 4. NATS subscriber — nhận scan.completed và batch create findings
    js, err := r.nc.JetStream()
    if err != nil { return fmt.Errorf("finding: js: %w", err) }

    sub, err := js.Subscribe(events.SubjScanCompleted, func(msg *nats.Msg) {
        var evt events.ScanCompletedEvent
        if err := json.Unmarshal(msg.Data, &evt); err != nil {
            msg.Nak()
            return
        }
        if err := batchUC.Execute(ctx, evt.ScanID, evt.Findings); err != nil {
            msg.NakWithDelay(5 * time.Second)
            return
        }
        msg.Ack()
    }, nats.Durable("finding-scan-completed"), nats.DeliverNew())
    if err != nil { return fmt.Errorf("finding: nats sub: %w", err) }
    defer sub.Drain()

    // 5. gRPC server
    r.server = grpc.NewServer(
        grpc.ChainUnaryInterceptor(grpcRecoveryInterceptor, grpcLoggingInterceptor),
    )
    handler := findinggrpc.NewHandler(batchUC, statusUC, findingRepo)
    findingpb.RegisterFindingServiceServer(r.server, handler)
    grpc_health_v1.RegisterHealthServer(r.server, health.NewServer())

    errCh := make(chan error, 1)
    go func() { errCh <- r.server.Serve(r.lis) }()

    select {
    case <-ctx.Done():
        r.server.GracefulStop()
        return nil
    case err := <-errCh:
        return fmt.Errorf("finding-service gRPC: %w", err)
    }
}

func (r *FindingRunner) Health(ctx context.Context) error {
    conn, err := transport.DialBufConn(ctx, r.lis)
    if err != nil { return err }
    defer conn.Close()

    hc := grpc_health_v1.NewHealthClient(conn)
    resp, err := hc.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
    if err != nil { return err }
    if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
        return fmt.Errorf("finding not serving: %s", resp.Status)
    }
    return nil
}
```

---

## Step 3: Wiring trong App

```go
a.findingLis = transport.NewBufConnListener()
a.registry.Register(runners.NewFindingRunner(
    runners.FindingRunnerConfig{
        DBURL:      cfg.Database.URL,
        ProductLis: a.productLis,
    },
    a.nc,
    a.findingLis,
))
```

---

## Step 4: API Gateway handler

```go
// internal/gateway/handlers/finding.go
type FindingHandler struct {
    client findingpb.FindingServiceClient
}

func (h *FindingHandler) List(w http.ResponseWriter, r *http.Request) {
    resp, err := h.client.ListFindings(r.Context(), &findingpb.ListFindingsRequest{
        ScanId: r.URL.Query().Get("scan_id"),
        Limit:  parseIntParam(r, "limit", 20),
        Offset: parseIntParam(r, "offset", 0),
    })
    if err != nil { httpGRPCError(w, err); return }
    jsonResponse(w, http.StatusOK, resp)
}

func (h *FindingHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    var req struct{ Status string `json:"status"` }
    json.NewDecoder(r.Body).Decode(&req)

    _, err := h.client.UpdateFindingStatus(r.Context(), &findingpb.UpdateStatusRequest{
        FindingId: id,
        Status:    req.Status,
    })
    if err != nil { httpGRPCError(w, err); return }
    w.WriteHeader(http.StatusNoContent)
}
```

---

## Output

- [x] `internal/runners/finding_runner.go` — FindingRunner ✓
- [x] NATS subscriber: `scan.completed` → batch create findings ✓ (finding_runner.go)
- [x] gRPC server trên `findingLis` (bufconn) ✓
- [x] Finding HTTP handlers: HandleListFindings, HandleGetFinding, HandleUpdateFindingStatus ✓

## Acceptance Criteria

```go
// Finding goroutine nhận NATS event và create findings
js.Publish(events.SubjScanCompleted, scanCompletedJSON)
time.Sleep(500 * time.Millisecond)
// Findings đã được tạo trong DB
findings, _ := findingRepo.List(ctx, scanID)
assert.Greater(t, len(findings), 0)
```
