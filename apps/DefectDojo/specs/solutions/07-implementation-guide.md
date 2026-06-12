# Implementation Guide — Step by Step

## Tổng quan

Hướng dẫn này mô tả cách implement từng bước để tạo ra monolithic Go application từ code base tại `services/`.

> **Nguyên tắc cốt lõi**: Không thay đổi bất kỳ dòng code nào trong `services/`. Chỉ thêm wrapper/runner code trong `apps/DefectDojo/`.

## Phase 1: Project Setup

### 1.1 Tạo cấu trúc thư mục

```bash
mkdir -p apps/DefectDojo/{cmd/defectdojo,internal/{app,config,gateway,runners,events,transport,migration}}
```

### 1.2 Tạo go.mod

```go
// apps/DefectDojo/go.mod
module github.com/defectdojo/apps/defectdojo

go 1.26.3

require (
    // Shared packages from services/
    github.com/osv/shared/pkg v0.0.0
    github.com/osv/shared/proto v0.0.0
    
    // Service packages (referenced as local modules via go.work)
    github.com/osv/auth-service v0.0.0
    github.com/defectdojo/finding-service v0.0.0
    github.com/defectdojo/product-service v0.0.0
    github.com/defectdojo/scan-service v0.0.0
    github.com/defectdojo/notification-service v0.0.0
    github.com/defectdojo/report-service v0.0.0
    github.com/defectdojo/integration-service v0.0.0
    github.com/defectdojo/vulnerability-service v0.0.0
    github.com/defectdojo/search-service v0.0.0
    github.com/defectdojo/ingestion-service v0.0.0
    github.com/defectdojo/ai-service v0.0.0
    github.com/defectdojo/impact-service v0.0.0
    github.com/defectdojo/unified-gateway v0.0.0
    
    // HTTP framework
    github.com/go-chi/chi/v5 v5.2.2
    github.com/go-chi/cors v1.2.1
    
    // gRPC
    google.golang.org/grpc v1.81.1
    google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af
    
    // Database
    github.com/jackc/pgx/v5 v5.10.0
    
    // NATS
    github.com/nats-io/nats.go v1.37.0
    
    // Redis
    github.com/redis/go-redis/v9 v9.7.3
    
    // Logging
    github.com/rs/zerolog v1.33.0
    
    // Config
    github.com/spf13/viper v1.19.0
    
    // Metrics
    github.com/prometheus/client_golang v1.21.1
)
```

### 1.3 Cập nhật go.work

```go
// services/go.work — THÊM vào use block
use (
    // ... existing entries ...
    ../apps/DefectDojo  // ADD THIS
)
```

## Phase 2: Config Layer

### 2.1 Unified Config

```go
// apps/DefectDojo/internal/config/config.go

package config

import (
    "github.com/spf13/viper"
)

type Config struct {
    // Database
    PostgresURL string `mapstructure:"POSTGRES_URL"`
    
    // NATS
    NatsURL string `mapstructure:"NATS_URL"`
    
    // Redis
    RedisURL string `mapstructure:"REDIS_URL"`
    
    // OpenSearch
    OpenSearchURL string `mapstructure:"OPENSEARCH_URL"`
    
    // JWT
    JWTSecret string `mapstructure:"JWT_SECRET"`
    
    // HTTP Ports
    HTTPPort string `mapstructure:"HTTP_PORT"`   // default: 8080
    GRPCPort string `mapstructure:"GRPC_PORT"`   // default: 9090
    
    // Service config
    Auth         AuthConfig         `mapstructure:",squash"`
    AI           AIConfig           `mapstructure:",squash"`
    Integration  IntegrationConfig  `mapstructure:",squash"`
    Notification NotificationConfig `mapstructure:",squash"`
}

type AuthConfig struct {
    JWTExpiry        string `mapstructure:"JWT_EXPIRY"`          // default: 24h
    RefreshExpiry    string `mapstructure:"REFRESH_EXPIRY"`      // default: 7d
    OAuthGoogleKey   string `mapstructure:"OAUTH_GOOGLE_KEY"`
    OAuthGoogleSecret string `mapstructure:"OAUTH_GOOGLE_SECRET"`
}

type AIConfig struct {
    Backend  string `mapstructure:"AI_BACKEND"`    // ollama, openai, azure
    Model    string `mapstructure:"AI_MODEL"`      // llama3, gpt-4, etc
    BaseURL  string `mapstructure:"AI_BASE_URL"`
    APIKey   string `mapstructure:"AI_API_KEY"`
}

type IntegrationConfig struct {
    JiraEncryptionKey string `mapstructure:"JIRA_ENCRYPTION_KEY"`
}

type NotificationConfig struct {
    SMTPHost     string `mapstructure:"SMTP_HOST"`
    SMTPPort     int    `mapstructure:"SMTP_PORT"`
    SMTPUser     string `mapstructure:"SMTP_USER"`
    SMTPPassword string `mapstructure:"SMTP_PASSWORD"`
    SlackToken   string `mapstructure:"SLACK_TOKEN"`
}

func Load() (*Config, error) {
    viper.AutomaticEnv()
    viper.SetDefault("HTTP_PORT", "8080")
    viper.SetDefault("GRPC_PORT", "9090")
    viper.SetDefault("NATS_URL", "nats://localhost:4222")
    viper.SetDefault("REDIS_URL", "redis://localhost:6379")
    viper.SetDefault("JWT_EXPIRY", "24h")
    
    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

## Phase 3: Service Runners

### 3.1 Base Runner Interface

```go
// apps/DefectDojo/internal/runners/base.go

package runners

import (
    "context"
    "net"
    
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/test/bufconn"
)

// Dialer is a function that creates a gRPC client connection.
type Dialer func(ctx context.Context) (*grpc.ClientConn, error)

// MakeBufConnDialer creates a Dialer for the given bufconn.Listener.
func MakeBufConnDialer(lis *bufconn.Listener) Dialer {
    return func(ctx context.Context) (*grpc.ClientConn, error) {
        return grpc.DialContext(ctx, "bufnet",
            grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
                return lis.DialContext(ctx)
            }),
            grpc.WithTransportCredentials(insecure.NewCredentials()),
        )
    }
}
```

### 3.2 Triển khai tất cả runners

**Thứ tự implement** (theo dependency):

```
Step 1: runners/auth_runner.go
    - Import: github.com/osv/auth-service/internal/{domain,infra,usecase,delivery}
    - Expose: bufconn.Listener (cho gateway và finding-service)
    - HTTP: POST /api/v2/api-token-auth/, GET/POST /api/v2/users/

Step 2: runners/product_runner.go
    - Import: github.com/defectdojo/product-service/internal/{domain,infra,usecase,delivery}
    - Expose: bufconn.Listener (cho finding, scan, report)
    - No HTTP direct (all via gateway)

Step 3: runners/finding_runner.go
    - Import: github.com/defectdojo/finding-service/internal/{domain,infra,usecase,delivery}
    - Deps: product-service.Dialer, auth-service.Dialer
    - Expose: bufconn.Listener (cho scan, report, notification, ai)
    - SLA: go slaChecker.Run(ctx, 1*time.Hour)

Step 4: runners/scan_runner.go
    - Import: github.com/defectdojo/scan-service/internal/{domain,infra,usecase,delivery,parsers}
    - Deps: finding-service.Dialer, product-service.Dialer
    - Expose: bufconn.Listener (cho ingestion, gateway)
    - Workers: worker pool for file parsing

Step 5: runners/vuln_runner.go
    - Import: github.com/defectdojo/vulnerability-service/internal/{domain,infra,usecase,delivery}
    - No service deps
    - Expose: bufconn.Listener

Step 6: runners/notification_runner.go
    - Import: github.com/defectdojo/notification-service/internal/{domain,infra,usecase,delivery}
    - Deps: NATS consumer (dd.finding.created, dd.scan.completed, dd.finding.sla.breach)
    - Expose: bufconn.Listener (cho gateway)

Step 7: runners/report_runner.go
    - Import: github.com/defectdojo/report-service/internal/{domain,infra,usecase,adapter}
    - Deps: finding-service.Dialer, product-service.Dialer
    - Expose: bufconn.Listener (cho gateway)

Step 8: runners/ai_runner.go
    - Import: github.com/defectdojo/ai-service/internal/{domain,infra,usecase}
    - Deps: NATS consumer (dd.finding.severity.critical)
    - No gRPC server (NATS-driven)

Step 9: runners/impact_runner.go
    - Import: github.com/defectdojo/impact-service/internal/{domain,infra,usecase}
    - Deps: NATS consumer (dd.finding.created), vuln-service.Dialer

Step 10: runners/integration_runner.go
    - Import: github.com/defectdojo/integration-service/internal/{domain,infra,usecase}
    - Deps: NATS consumer (dd.finding.severity.critical, dd.finding.sla.breach)
    - Expose: bufconn.Listener (cho gateway - JIRA config management)

Step 11: runners/search_runner.go
    - Import: github.com/defectdojo/search-service/internal/{domain,infra,usecase,delivery}
    - Deps: OpenSearch client
    - Expose: bufconn.Listener (cho gateway)

Step 12: runners/ingestion_runner.go
    - Import: github.com/defectdojo/ingestion-service/internal/{domain,infra,usecase}
    - Deps: NATS consumer, search-service.Dialer, vuln-service.Dialer

Step 13: runners/gateway_runner.go
    - Import: unified-gateway internal packages (or reimplement thin layer)
    - Deps: ALL service Dialers
    - HTTP :8080, gRPC :9090
```

## Phase 4: Application Bootstrap

```go
// apps/DefectDojo/internal/app/app.go

package app

import (
    "context"
    "fmt"
    
    "github.com/defectdojo/apps/defectdojo/internal/config"
    "github.com/defectdojo/apps/defectdojo/internal/events"
    "github.com/defectdojo/apps/defectdojo/internal/migration"
    "github.com/defectdojo/apps/defectdojo/internal/runners"
    "github.com/defectdojo/apps/defectdojo/internal/app/registry"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/nats-io/nats.go"
    "github.com/redis/go-redis/v9"
    "google.golang.org/grpc/test/bufconn"
)

type App struct {
    cfg      *config.Config
    registry *registry.Registry
    db       *pgxpool.Pool
    nats     *nats.Conn
    redis    *redis.Client
    
    // bufconn listeners (for in-process gRPC)
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

func New(cfg *config.Config) (*App, error) {
    return &App{cfg: cfg, registry: registry.New()}, nil
}

func (a *App) Start(ctx context.Context) error {
    // 1. Connect infrastructure
    if err := a.connectInfra(ctx); err != nil {
        return fmt.Errorf("infra: %w", err)
    }
    
    // 2. Run migrations
    if err := migration.RunAll(ctx, a.db); err != nil {
        return fmt.Errorf("migrations: %w", err)
    }
    
    // 3. Setup NATS JetStream streams
    js, _ := a.nats.JetStream()
    if err := events.SetupJetStream(js); err != nil {
        return fmt.Errorf("nats setup: %w", err)
    }
    
    // 4. Create bufconn listeners
    a.createListeners()
    
    // 5. Register & start services
    a.registerServices(ctx, js)
    a.registry.Start(ctx)
    
    return nil
}

func (a *App) createListeners() {
    bufSize := 1 << 20 // 1MB
    a.authLis    = bufconn.Listen(bufSize)
    a.productLis = bufconn.Listen(bufSize)
    a.findingLis = bufconn.Listen(bufSize)
    a.scanLis    = bufconn.Listen(bufSize)
    a.vulnLis    = bufconn.Listen(bufSize)
    a.notifLis   = bufconn.Listen(bufSize)
    a.reportLis  = bufconn.Listen(bufSize)
    a.searchLis  = bufconn.Listen(bufSize)
    a.integLis   = bufconn.Listen(bufSize)
}

func (a *App) registerServices(ctx context.Context, js nats.JetStreamContext) {
    // Dialers
    dialAuth    := runners.MakeBufConnDialer(a.authLis)
    dialProduct := runners.MakeBufConnDialer(a.productLis)
    dialFinding := runners.MakeBufConnDialer(a.findingLis)
    dialVuln    := runners.MakeBufConnDialer(a.vulnLis)
    dialSearch  := runners.MakeBufConnDialer(a.searchLis)
    
    // Register services
    a.registry.Register(runners.NewAuthRunner(a.cfg, a.db, a.redis, a.authLis))
    a.registry.Register(runners.NewProductRunner(a.cfg, a.db, a.productLis))
    a.registry.Register(runners.NewVulnRunner(a.cfg, a.db, a.nats, a.vulnLis))
    a.registry.Register(runners.NewSearchRunner(a.cfg, a.searchLis))
    a.registry.Register(runners.NewFindingRunner(a.cfg, a.db, a.nats, dialProduct, dialAuth, a.findingLis))
    a.registry.Register(runners.NewScanRunner(a.cfg, a.db, a.nats, dialFinding, dialProduct, a.scanLis))
    a.registry.Register(runners.NewNotificationRunner(a.cfg, a.db, a.nats, a.notifLis))
    a.registry.Register(runners.NewReportRunner(a.cfg, a.db, dialFinding, dialProduct, a.reportLis))
    a.registry.Register(runners.NewAIRunner(a.cfg, a.nats, dialFinding))
    a.registry.Register(runners.NewImpactRunner(a.cfg, a.nats, dialVuln, dialFinding))
    a.registry.Register(runners.NewIntegrationRunner(a.cfg, a.db, a.nats, a.integLis))
    a.registry.Register(runners.NewIngestionRunner(a.cfg, a.db, a.nats, dialVuln, dialSearch))
    a.registry.Register(runners.NewGatewayRunner(a.cfg,
        dialAuth, dialProduct, dialFinding, dialVuln,
        dialSearch, a.notifLis, a.reportLis, a.integLis,
        runners.MakeBufConnDialer(a.scanLis),
    ))
}

func (a *App) Shutdown(ctx context.Context) error {
    // 1. Stop gateway first
    a.registry.Stop("unified-gateway")
    
    // 2. Drain NATS
    a.nats.Drain()
    
    // 3. Stop all remaining services
    a.registry.StopAll()
    
    // 4. Close infrastructure
    a.db.Close()
    a.redis.Close()
    
    return nil
}
```

## Phase 5: Gateway Implementation

```go
// apps/DefectDojo/internal/runners/gateway_runner.go

package runners

import (
    "context"
    "net/http"
    
    "github.com/defectdojo/apps/defectdojo/internal/gateway"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "google.golang.org/grpc/test/bufconn"
    
    authpb    "github.com/osv/shared/proto/gen/go/auth/v1"
    findingpb "github.com/osv/shared/proto/gen/go/finding/dd/v1"
    productpb "github.com/osv/shared/proto/gen/go/product/dd/v1"
)

type GatewayRunner struct {
    cfg         *config.Config
    dialers     GatewayDialers
}

type GatewayDialers struct {
    Auth        Dialer
    Product     Dialer
    Finding     Dialer
    Scan        Dialer
    Vuln        Dialer
    Search      Dialer
    Notification Dialer
    Report      Dialer
    Integration Dialer
}

func (r *GatewayRunner) Name() string { return "unified-gateway" }

func (r *GatewayRunner) Run(ctx context.Context) error {
    // Connect to all in-process services
    authConn, _ := r.dialers.Auth(ctx)
    productConn, _ := r.dialers.Product(ctx)
    findingConn, _ := r.dialers.Finding(ctx)
    // ... more connections
    
    // Create typed clients
    clients := &gateway.ServiceClients{
        Auth:         authpb.NewAuthServiceClient(authConn),
        Product:      productpb.NewProductServiceClient(productConn),
        Finding:      findingpb.NewFindingServiceClient(findingConn),
        // ...
    }
    
    // Build router
    router := gateway.NewRouter(clients)
    
    srv := &http.Server{
        Addr:    ":" + r.cfg.HTTPPort,
        Handler: router,
    }
    
    log.Info().Str("port", r.cfg.HTTPPort).Msg("gateway HTTP listening")
    
    errCh := make(chan error, 1)
    go func() {
        errCh <- srv.ListenAndServe()
    }()
    
    select {
    case <-ctx.Done():
        return srv.Shutdown(context.Background())
    case err := <-errCh:
        return err
    }
}
```

## Phase 6: Testing

### Unit Tests

```go
// Mỗi runner có unit test riêng sử dụng mock hoặc in-memory implementations
// apps/DefectDojo/internal/runners/finding_runner_test.go

func TestFindingRunnerLifecycle(t *testing.T) {
    // Start runner
    runner := NewFindingRunner(testConfig, testDB, testNATS, testProductDialer, testAuthDialer, bufconn.Listen(1<<20))
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    go runner.Run(ctx)
    
    // Wait for startup
    time.Sleep(100 * time.Millisecond)
    
    // Check health
    assert.NoError(t, runner.Health(ctx))
    
    // Make gRPC call
    conn, _ := runner.Listener().DialContext(ctx)
    client := findingpb.NewFindingServiceClient(conn)
    
    resp, err := client.BatchCreateFindings(ctx, &findingpb.BatchCreateFindingsRequest{
        TestId: "test-uuid",
        // ...
    })
    assert.NoError(t, err)
    assert.Greater(t, len(resp.FindingIds), 0)
}
```

### Integration Tests

```bash
# Test full flow: import scan → findings created → notification sent
# apps/DefectDojo/tests/integration/scan_import_test.go

go test ./... -tags=integration -timeout 120s
```

## Phase 7: Deployment

### Docker

```dockerfile
# apps/DefectDojo/Dockerfile
FROM golang:1.26-alpine AS builder

WORKDIR /workspace
COPY . .
RUN go build -o /bin/defectdojo ./apps/DefectDojo/cmd/defectdojo/

FROM alpine:3.19
COPY --from=builder /bin/defectdojo /bin/defectdojo
EXPOSE 8080 9090
ENTRYPOINT ["/bin/defectdojo"]
```

### Docker Compose

```yaml
# apps/DefectDojo/docker-compose.yml
version: "3.9"
services:
  postgres:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_USER: defectdojo
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: defectdojo
    volumes: [pgdata:/var/lib/postgresql/data]
    
  redis:
    image: redis:7-alpine
    
  nats:
    image: nats:2.10-alpine
    command: ["-js", "-m", "8222"]
    
  opensearch:
    image: opensearchproject/opensearch:2
    environment:
      - discovery.type=single-node
      - DISABLE_SECURITY_PLUGIN=true
      
  defectdojo:
    build: ../../   # root of monorepo
    dockerfile: apps/DefectDojo/Dockerfile
    ports:
      - "8080:8080"
      - "9090:9090"
    environment:
      POSTGRES_URL: postgres://defectdojo:${POSTGRES_PASSWORD}@postgres/defectdojo?sslmode=disable
      NATS_URL: nats://nats:4222
      REDIS_URL: redis://redis:6379
      OPENSEARCH_URL: http://opensearch:9200
      JWT_SECRET: ${JWT_SECRET}
      AI_BACKEND: ${AI_BACKEND:-ollama}
      JIRA_ENCRYPTION_KEY: ${JIRA_ENCRYPTION_KEY}
    depends_on: [postgres, redis, nats, opensearch]
    
volumes:
  pgdata:
```

## Checklist Implementation

- [ ] **Phase 1**: Project setup (go.mod, go.work, directory structure)
- [ ] **Phase 2**: Config layer
- [ ] **Phase 3.1**: registry.go — ServiceRunner interface & Registry
- [ ] **Phase 3.2**: auth_runner.go
- [ ] **Phase 3.3**: product_runner.go
- [ ] **Phase 3.4**: vuln_runner.go
- [ ] **Phase 3.5**: finding_runner.go (với SLA ticker)
- [ ] **Phase 3.6**: scan_runner.go (với worker pool)
- [ ] **Phase 3.7**: notification_runner.go (NATS consumer)
- [ ] **Phase 3.8**: report_runner.go (với gRPC stream)
- [ ] **Phase 3.9**: ai_runner.go (NATS-driven)
- [ ] **Phase 3.10**: impact_runner.go
- [ ] **Phase 3.11**: integration_runner.go (JIRA/GitHub)
- [ ] **Phase 3.12**: search_runner.go
- [ ] **Phase 3.13**: ingestion_runner.go
- [ ] **Phase 3.14**: gateway_runner.go
- [ ] **Phase 4**: app.go bootstrap
- [ ] **Phase 5**: gateway/router.go + all handlers
- [ ] **Phase 6**: Tests
- [ ] **Phase 7**: Docker + Docker Compose
