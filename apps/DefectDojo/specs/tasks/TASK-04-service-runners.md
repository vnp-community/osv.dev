# TASK-04: Service Runners (13 Goroutines)

**Phase**: 4 — Service Goroutines  
**Ước tính**: 32 giờ (2-3h/runner)  
**Phụ thuộc**: TASK-03 hoàn thành  
**Output**: 13 ServiceRunner implementations + app.registerServices()

---

## Nguyên tắc cho mỗi Runner

1. **Không thay đổi code** trong `services/` — chỉ import và wire
2. Mỗi runner implement `ServiceRunner` interface (Name, Run, Health)
3. `Run(ctx)` phải block cho đến khi `ctx` bị cancel
4. `Health(ctx)` phải return nil khi service sẵn sàng nhận traffic
5. Cleanup resources trong `defer` khi `ctx.Done()`

---

## Thứ tự Implementation

> Implement theo thứ tự dependency — service sau không phụ thuộc service trước chưa xong.

```
Tier 1 (No service deps):  auth → product → vuln
Tier 2 (Dep on Tier 1):    finding → search
Tier 3 (Dep on Tier 1+2):  scan → notification → report → ai → impact → integration → ingestion
Tier 4 (Last):             gateway (phụ thuộc tất cả)
```

---

## T-04.1: Auth Service Runner

**File**: `apps/DefectDojo/internal/runners/auth_runner.go`  
**Ước tính**: 3h  
**Import từ**: `github.com/osv/auth-service/internal/...`

### Tasks:
- [ ] **T-04.1a**: Nghiên cứu `auth-service/internal/` structure
  ```bash
  find /Users/binhnt/Lab/sec/cve/osv.dev/services/auth-service/internal -name "*.go" | head -40
  ```
- [ ] **T-04.1b**: Xác định exported constructors và interfaces
  - `domain/entity/user.go` — User type
  - `infra/repository/` — UserRepo, SessionRepo, APIKeyRepo
  - `usecase/` — AuthUseCase
  - `delivery/grpc/` hoặc `services/auth.go` — gRPC handler
- [ ] **T-04.1c**: Implement `AuthRunner.Run()`:
  - Init pgxpool từ config
  - Init Redis client
  - Wire repo → usecase → gRPC handler
  - Start gRPC server trên `a.authLis` (bufconn)
  - Start HTTP handlers (login, token refresh, OAuth)
  - Block trên `ctx.Done()`
- [ ] **T-04.1d**: Implement `AuthRunner.Health()`:
  - Ping gRPC server qua bufconn
  - Return nil nếu SERVING
- [ ] **T-04.1e**: Unit test `TestAuthRunnerLifecycle`

**Template**:
```go
package runners

import (
    "context"
    "fmt"

    // Import từ auth-service (không thay đổi)
    authInfra   "github.com/osv/auth-service/internal/infra"
    authUseCase "github.com/osv/auth-service/internal/usecase"
    authGRPC    "github.com/osv/auth-service/internal/delivery/grpc"
    
    "github.com/defectdojo/apps/defectdojo/internal/config"
    "github.com/defectdojo/apps/defectdojo/internal/transport"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
    "google.golang.org/grpc"
    "google.golang.org/grpc/test/bufconn"
)

type AuthRunner struct {
    cfg    *config.Config
    lis    *bufconn.Listener
    server *grpc.Server
}

func NewAuthRunner(cfg *config.Config, lis *bufconn.Listener) *AuthRunner {
    return &AuthRunner{cfg: cfg, lis: lis}
}

func (r *AuthRunner) Name() string { return "auth-service" }

func (r *AuthRunner) Run(ctx context.Context) error {
    // 1. Connect DB (auth-service has its own pool)
    db, err := pgxpool.New(ctx, r.cfg.PostgresURL)
    if err != nil {
        return fmt.Errorf("auth: db: %w", err)
    }
    defer db.Close()

    // 2. Connect Redis
    redisOpt, _ := redis.ParseURL(r.cfg.RedisURL)
    rdb := redis.NewClient(redisOpt)
    defer rdb.Close()

    // 3. Wire dependencies (using auth-service internal packages)
    // TODO: Adjust package paths based on T-00.2 audit
    userRepo    := authInfra.NewUserRepository(db)
    sessionRepo := authInfra.NewSessionRepository(rdb)
    apiKeyRepo  := authInfra.NewAPIKeyRepository(db)
    authUC      := authUseCase.New(userRepo, sessionRepo, apiKeyRepo, r.cfg.JWTSecret, r.cfg.JWTExpiry)

    // 4. Start gRPC server
    r.server = grpc.NewServer()
    authGRPC.Register(r.server, authUC)

    errCh := make(chan error, 1)
    go func() {
        if err := r.server.Serve(r.lis); err != nil {
            errCh <- err
        }
    }()

    // 5. Start HTTP auth endpoints (login, register, etc.)
    // TODO: Wire HTTP handlers for auth endpoints

    select {
    case <-ctx.Done():
        r.server.GracefulStop()
        return nil
    case err := <-errCh:
        return fmt.Errorf("auth: grpc: %w", err)
    }
}

func (r *AuthRunner) Health(ctx context.Context) error {
    return waitForGRPC(ctx, r.lis, "auth-service")
}
```

---

## T-04.2: Product Service Runner

**File**: `apps/DefectDojo/internal/runners/product_runner.go`  
**Ước tính**: 2.5h  
**Import từ**: `github.com/defectdojo/product-service/internal/...`

### Tasks:
- [ ] **T-04.2a**: Nghiên cứu `product-service/internal/domain/` structure
  - `domain/product/entity.go` ✅ đã biết
  - `domain/engagement/` — Engagement entity
  - `domain/test/` — Test entity
  - `domain/orchestrator/` — Orchestrator pattern
- [ ] **T-04.2b**: Xác định `delivery/grpc/` handler patterns
- [ ] **T-04.2c**: Implement `ProductRunner.Run()`:
  - Wire product/engagement/test repos
  - Wire Orchestrator
  - Start gRPC server trên `productLis`
- [ ] **T-04.2d**: Implement `ProductRunner.Health()`
- [ ] **T-04.2e**: Unit test

**Note**: Product service expose ProductService, EngagementService, TestService gRPC methods.

---

## T-04.3: Vulnerability Service Runner

**File**: `apps/DefectDojo/internal/runners/vuln_runner.go`  
**Ước tính**: 2h  
**Import từ**: `github.com/defectdojo/vulnerability-service/internal/...`

### Tasks:
- [ ] **T-04.3a**: Nghiên cứu vuln-service structure
  ```bash
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/vulnerability-service/internal/
  ```
- [ ] **T-04.3b**: Implement `VulnRunner.Run()`:
  - Wire CVE repository (PostgreSQL)
  - Wire NATS publisher (publish khi vuln mới ingested)
  - Start gRPC server
- [ ] **T-04.3c**: Implement health check
- [ ] **T-04.3d**: Unit test

---

## T-04.4: Search Service Runner

**File**: `apps/DefectDojo/internal/runners/search_runner.go`  
**Ước tính**: 2h  
**Import từ**: `github.com/defectdojo/search-service/internal/...`

### Tasks:
- [ ] **T-04.4a**: Nghiên cứu search-service:
  ```bash
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/
  ```
- [ ] **T-04.4b**: Implement `SearchRunner.Run()`:
  - Wire OpenSearch client
  - Wire index management
  - Start gRPC server trên `searchLis`
- [ ] **T-04.4c**: Health check: ping OpenSearch + gRPC
- [ ] **T-04.4d**: Unit test

---

## T-04.5: Finding Service Runner ⭐ Critical

**File**: `apps/DefectDojo/internal/runners/finding_runner.go`  
**Ước tính**: 4h  
**Import từ**: `github.com/defectdojo/finding-service/internal/...`  
**Phụ thuộc**: product-service.Dialer, auth-service.Dialer

### Tasks:
- [ ] **T-04.5a**: Nghiên cứu finding-service internal packages:
  ```bash
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/infra/
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/usecase/
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/
  ```
- [ ] **T-04.5b**: Xác định SLA checker implementation
  ```bash
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/usecase/sla/
  ```
- [ ] **T-04.5c**: Implement `FindingRunner.Run()`:
  - Wire FindingRepository (postgres)
  - Wire SLARepository
  - Wire AuditRepository
  - Wire FindingUseCase (với NATS publisher)
  - Wire SLAUseCase
  - Start gRPC server (FindingService) trên `findingLis`
  - Launch SLA checker goroutine: `go slaUC.RunSLAChecker(ctx, 1*time.Hour)`
  - Launch NATS consumer goroutine (nếu cần)
- [ ] **T-04.5d**: Implement `FindingRunner.Health()`
- [ ] **T-04.5e**: Unit tests:
  - `TestFindingRunnerStart`
  - `TestFindingRunnerSLAChecker`
  - `TestBatchCreateFindings`

---

## T-04.6: Scan Service Runner ⭐ Critical

**File**: `apps/DefectDojo/internal/runners/scan_runner.go`  
**Ước tính**: 4h  
**Import từ**: `github.com/defectdojo/scan-service/internal/...`  
**Phụ thuộc**: finding-service.Dialer, product-service.Dialer

### Tasks:
- [ ] **T-04.6a**: Nghiên cứu scan-service parsers:
  ```bash
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/parsers/
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/internal/domain/
  ```
- [ ] **T-04.6b**: Implement `ScanRunner.Run()`:
  - Wire ScanRepository
  - Wire FindingServiceClient (via finding-service.Dialer)
  - Wire ProductServiceClient (via product-service.Dialer)
  - Wire parser pool (worker goroutines)
  - Start gRPC server (ScanService) trên `scanLis`
  - Launch worker pool: `go r.runParserWorkers(ctx, numWorkers)`
- [ ] **T-04.6c**: Worker pool implementation:
  ```go
  func (r *ScanRunner) runParserWorkers(ctx context.Context, n int) {
      for i := 0; i < n; i++ {
          go r.parserWorker(ctx)
      }
  }
  ```
- [ ] **T-04.6d**: Health check
- [ ] **T-04.6e**: Unit tests cho SBOM parsing, SAST parsing

---

## T-04.7: Notification Service Runner

**File**: `apps/DefectDojo/internal/runners/notification_runner.go`  
**Ước tính**: 3h  
**Import từ**: `github.com/defectdojo/notification-service/internal/...`

### Tasks:
- [ ] **T-04.7a**: Nghiên cứu notification-service:
  ```bash
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/domain/
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/adapter/
  ```
- [ ] **T-04.7b**: Implement `NotificationRunner.Run()`:
  - Wire AlertRepository
  - Wire RuleRepository
  - Wire channel adapters: Email, Slack, Webhook, Teams
  - Wire NotificationUseCase
  - Start gRPC server trên `notifLis`
  - Start NATS consumers:
    - `dd.finding.created` → check rules → send alerts
    - `dd.scan.completed` → send summary
    - `dd.finding.sla.breach` → send urgent alert
- [ ] **T-04.7c**: Health check
- [ ] **T-04.7d**: Unit tests cho alert rules evaluation

---

## T-04.8: Report Service Runner

**File**: `apps/DefectDojo/internal/runners/report_runner.go`  
**Ước tính**: 3h  
**Import từ**: `github.com/defectdojo/report-service/internal/...`  
**Phụ thuộc**: finding-service.Dialer, product-service.Dialer

### Tasks:
- [ ] **T-04.8a**: Nghiên cứu report-service:
  ```bash
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/report-service/internal/
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/report-service/internal/formatters/
  ```
- [ ] **T-04.8b**: Implement `ReportRunner.Run()`:
  - Wire FindingServiceClient (stream findings)
  - Wire ProductServiceClient
  - Wire formatters (PDF, HTML, CSV, JSON)
  - Wire ReportUseCase
  - Start gRPC server trên `reportLis`
- [ ] **T-04.8c**: Health check
- [ ] **T-04.8d**: Unit test: generate findings report, check pagination

---

## T-04.9: AI Service Runner

**File**: `apps/DefectDojo/internal/runners/ai_runner.go`  
**Ước tính**: 2h  
**Import từ**: `github.com/defectdojo/ai-service/internal/...`

### Tasks:
- [ ] **T-04.9a**: Nghiên cứu ai-service:
  ```bash
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/ai-service/internal/
  ```
- [ ] **T-04.9b**: Implement `AIRunner.Run()`:
  - Wire LLM adapter (Ollama/OpenAI/Azure)
  - Wire AIUseCase
  - Start NATS consumer: `dd.finding.severity.critical`
  - On message: call LLM → publish `dd.ai.triage.completed`
- [ ] **T-04.9c**: Health check: ping LLM endpoint
- [ ] **T-04.9d**: Unit test với mock LLM

---

## T-04.10: Impact Service Runner

**File**: `apps/DefectDojo/internal/runners/impact_runner.go`  
**Ước tính**: 2h  
**Import từ**: `github.com/defectdojo/impact-service/internal/...`  
**Phụ thuộc**: vuln-service.Dialer

### Tasks:
- [ ] **T-04.10a**: Nghiên cứu impact-service
- [ ] **T-04.10b**: Implement `ImpactRunner.Run()`:
  - Wire VulnServiceClient (via vuln-service.Dialer)
  - Wire ImpactUseCase
  - Start NATS consumer: `dd.finding.created`
  - On message: lookup CVE → calculate impact → publish `dd.impact.assessed`
- [ ] **T-04.10c**: Health check
- [ ] **T-04.10d**: Unit test

---

## T-04.11: Integration Service Runner

**File**: `apps/DefectDojo/internal/runners/integration_runner.go`  
**Ước tính**: 3h  
**Import từ**: `github.com/defectdojo/integration-service/internal/...`

### Tasks:
- [ ] **T-04.11a**: Nghiên cứu integration-service:
  ```bash
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/integration-service/internal/
  ```
- [ ] **T-04.11b**: Implement `IntegrationRunner.Run()`:
  - Wire JIRA adapter (với encryption key)
  - Wire GitHub adapter
  - Wire IntegrationUseCase
  - Start gRPC server trên `integLis` (JIRA config CRUD)
  - Start NATS consumers:
    - `dd.finding.severity.critical` → create JIRA/GitHub issue
    - `dd.finding.sla.breach` → update JIRA priority
    - `dd.finding.closed` → close JIRA issue
- [ ] **T-04.11c**: Health check
- [ ] **T-04.11d**: Unit test với mock JIRA client

---

## T-04.12: Ingestion Service Runner

**File**: `apps/DefectDojo/internal/runners/ingestion_runner.go`  
**Ước tính**: 2h  
**Import từ**: `github.com/defectdojo/ingestion-service/internal/...`  
**Phụ thuộc**: vuln-service.Dialer, search-service.Dialer

### Tasks:
- [ ] **T-04.12a**: Nghiên cứu ingestion-service:
  ```bash
  ls /Users/binhnt/Lab/sec/cve/osv.dev/services/ingestion-service/internal/
  ```
- [ ] **T-04.12b**: Implement `IngestionRunner.Run()`:
  - Wire VulnServiceClient (write new CVEs)
  - Wire SearchServiceClient (index new CVEs)
  - Wire IngestionUseCase
  - Start NATS consumer (trigger ingestion from external sources)
  - Run periodic sync (NVD, OSV.dev) via ticker
- [ ] **T-04.12c**: Health check
- [ ] **T-04.12d**: Unit test

---

## T-04.13: Gateway Runner ⭐ Critical

**File**: `apps/DefectDojo/internal/runners/gateway_runner.go`  
**Ước tính**: 4h  
**Phụ thuộc**: TẤT CẢ service runners (dùng dialers để connect)

### Tasks:
- [ ] **T-04.13a**: Định nghĩa `GatewayDialers` struct (tất cả 9 dialers)
- [ ] **T-04.13b**: Implement `GatewayRunner.Run()`:
  - Connect tới tất cả services qua dialers
  - Tạo typed gRPC clients (AuthServiceClient, FindingServiceClient, ...)
  - Build HTTP router (`gateway.NewRouter(clients)`) — từ TASK-05
  - Build gRPC server (external gRPC)
  - Start `http.Server` trên `:8080`
  - Start gRPC server trên `:9090`
- [ ] **T-04.13c**: Implement `GatewayRunner.Health()`:
  - `/health/ready` endpoint — true khi tất cả services healthy
- [ ] **T-04.13d**: Unit test

---

## T-04.14: registerServices() Method

**File**: `apps/DefectDojo/internal/app/app.go` (update phần registerServices)

```go
func (a *App) registerServices(ctx context.Context, js nats.JetStreamContext) {
    // Create dialers từ listeners
    dialAuth    := transport.MakeBufConnDialer(a.authLis)
    dialProduct := transport.MakeBufConnDialer(a.productLis)
    dialFinding := transport.MakeBufConnDialer(a.findingLis)
    dialScan    := transport.MakeBufConnDialer(a.scanLis)
    dialVuln    := transport.MakeBufConnDialer(a.vulnLis)
    dialSearch  := transport.MakeBufConnDialer(a.searchLis)
    dialNotif   := transport.MakeBufConnDialer(a.notifLis)
    dialReport  := transport.MakeBufConnDialer(a.reportLis)
    dialInteg   := transport.MakeBufConnDialer(a.integLis)

    // Tier 1: No service deps
    a.registry.Register(runners.NewAuthRunner(a.cfg, a.authLis))
    a.registry.Register(runners.NewProductRunner(a.cfg, a.productLis))
    a.registry.Register(runners.NewVulnRunner(a.cfg, a.nc, a.vulnLis))
    a.registry.Register(runners.NewSearchRunner(a.cfg, a.searchLis))

    // Tier 2
    a.registry.Register(runners.NewFindingRunner(a.cfg, a.nc, dialProduct, dialAuth, a.findingLis))

    // Tier 3
    a.registry.Register(runners.NewScanRunner(a.cfg, a.nc, dialFinding, dialProduct, a.scanLis))
    a.registry.Register(runners.NewNotificationRunner(a.cfg, a.nc, a.notifLis))
    a.registry.Register(runners.NewReportRunner(a.cfg, dialFinding, dialProduct, a.reportLis))
    a.registry.Register(runners.NewAIRunner(a.cfg, a.nc, dialFinding))
    a.registry.Register(runners.NewImpactRunner(a.cfg, a.nc, dialVuln))
    a.registry.Register(runners.NewIntegrationRunner(a.cfg, a.nc, a.integLis))
    a.registry.Register(runners.NewIngestionRunner(a.cfg, a.nc, dialVuln, dialSearch))

    // Tier 4: Gateway last
    a.registry.Register(runners.NewGatewayRunner(a.cfg, runners.GatewayDialers{
        Auth:        dialAuth,
        Product:     dialProduct,
        Finding:     dialFinding,
        Scan:        dialScan,
        Vuln:        dialVuln,
        Search:      dialSearch,
        Notification: dialNotif,
        Report:      dialReport,
        Integration: dialInteg,
    }))
}
```

---

## Definition of Done — TASK-04

- [ ] T-04.1 Auth runner hoàn chỉnh và test pass
- [ ] T-04.2 Product runner hoàn chỉnh và test pass
- [ ] T-04.3 Vuln runner hoàn chỉnh và test pass
- [ ] T-04.4 Search runner hoàn chỉnh và test pass
- [ ] T-04.5 Finding runner hoàn chỉnh (incl. SLA ticker) và test pass
- [ ] T-04.6 Scan runner hoàn chỉnh (incl. worker pool) và test pass
- [ ] T-04.7 Notification runner hoàn chỉnh và test pass
- [ ] T-04.8 Report runner hoàn chỉnh và test pass
- [ ] T-04.9 AI runner hoàn chỉnh và test pass
- [ ] T-04.10 Impact runner hoàn chỉnh và test pass
- [ ] T-04.11 Integration runner hoàn chỉnh và test pass
- [ ] T-04.12 Ingestion runner hoàn chỉnh và test pass
- [ ] T-04.13 Gateway runner skeleton hoàn chỉnh
- [ ] T-04.14 `registerServices()` wires tất cả runners
- [ ] `go build ./...` pass
- [ ] `go test ./internal/runners/...` pass
