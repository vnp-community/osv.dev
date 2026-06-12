# Cấu trúc thư mục dự án (v3 — Goroutine-Per-Service)

## Nguyên tắc thiết kế v3

> **Mỗi service = 1 goroutine** với vòng đời độc lập.  
> **Giao tiếp qua gRPC bufconn** (in-process), **NATS** (async), hoặc **REST** (external).  
> Code mới chỉ là glue code — business logic từ `services/`.

## Vị trí: `osv.dev/apps/OpenVulnScan/`

```
apps/OpenVulnScan/
├── cmd/
│   └── server/
│       └── main.go                    # Entry point (~120 LOC)
│                                      # Bootstrap goroutines, signal handling
│
├── internal/
│   ├── app/
│   │   ├── app.go                     # App container (~200 LOC)
│   │   │                              # Khởi tạo infra, wire-up dependencies
│   │   ├── registry.go                # ServiceRegistry (~100 LOC)
│   │   │                              # Goroutine lifecycle: Start/Stop/Health/Wait
│   │   └── config.go                  # Config struct (~80 LOC)
│   │
│   ├── transport/
│   │   └── bufconn.go                 # bufconn helpers (~50 LOC)
│   │                                  # NewBufConnListener, DialBufConn, MakeBufConnDialer
│   │
│   ├── runners/                       # Một file per service goroutine
│   │   ├── auth_runner.go             # authService goroutine (~80 LOC)
│   │   ├── scan_runner.go             # scanService goroutine (~100 LOC)
│   │   ├── finding_runner.go          # findingService goroutine (~80 LOC)
│   │   ├── product_runner.go          # productService goroutine (~70 LOC)
│   │   ├── vuln_runner.go             # vulnService goroutine (~70 LOC)
│   │   ├── report_runner.go           # reportService goroutine (~70 LOC)
│   │   ├── query_runner.go            # queryService goroutine (~70 LOC)
│   │   ├── notify_runner.go           # notificationService goroutine (~80 LOC)
│   │   └── ingestion_runner.go        # ingestionService goroutine (~70 LOC)
│   │
│   ├── gateway/                       # HTTP API Gateway
│   │   ├── router.go                  # chi.Router mount all routes (~100 LOC)
│   │   ├── middleware.go              # JWT middleware via auth-service gRPC (~50 LOC)
│   │   └── handlers/
│   │       ├── scan.go                # HTTP → gRPC scan calls (~80 LOC)
│   │       ├── finding.go             # HTTP → gRPC finding calls (~70 LOC)
│   │       ├── product.go             # HTTP → gRPC product calls (~70 LOC)
│   │       ├── auth.go                # HTTP → gRPC auth calls (~60 LOC)
│   │       ├── vuln.go                # HTTP → gRPC vuln calls (~50 LOC)
│   │       ├── report.go              # HTTP → gRPC report calls (~50 LOC)
│   │       ├── dashboard.go           # HTTP → gRPC query calls (~60 LOC)
│   │       └── agent.go               # Agent REST endpoints (~60 LOC)
│   │
│   ├── events/
│   │   ├── setup.go                   # JetStream stream setup (~40 LOC)
│   │   └── subjects.go                # NATS subject constants (~20 LOC)
│   │
│   └── syslog/
│       └── channel.go                 # SIEM syslog channel adapter (~60 LOC)
│
├── migrations/
│   └── *.sql                          # Merge migrations từ tất cả services
│
├── configs/
│   ├── config.yaml
│   └── config.docker.yaml
│
├── docker-compose.yml
├── Dockerfile
├── go.mod
└── Makefile
```

### Tóm tắt code mới cần viết

| File | LOC | Mục đích |
|---|---|---|
| `cmd/server/main.go` | ~120 | Entry point, signal handling, graceful shutdown |
| `internal/app/app.go` | ~200 | Wire-up tất cả goroutines và dependencies |
| `internal/app/registry.go` | ~100 | Goroutine lifecycle manager |
| `internal/app/config.go` | ~80 | Config struct |
| `internal/transport/bufconn.go` | ~50 | bufconn factory helpers |
| `internal/runners/*.go` | ~9×80 = ~720 | 9 service goroutine runners |
| `internal/gateway/router.go` | ~100 | HTTP API gateway router |
| `internal/gateway/middleware.go` | ~50 | JWT middleware |
| `internal/gateway/handlers/*.go` | ~8×65 = ~520 | HTTP → gRPC handlers |
| `internal/events/setup.go` | ~40 | NATS JetStream setup |
| `internal/events/subjects.go` | ~20 | NATS subject constants |
| `internal/syslog/channel.go` | ~60 | Syslog adapter |
| `configs/config.yaml` | ~60 | Config |
| `docker-compose.yml` | ~80 | Infrastructure |
| `go.mod` | ~35 | Module + replace directives |
| **Tổng** | **~2,235 LOC** | **Toàn bộ business logic từ services** |

---

## go.mod

```go
module github.com/osv/apps/openvulnscan

go 1.22

require (
    // Shared foundations
    github.com/osv/shared/pkg   v0.0.0
    github.com/osv/shared/proto v0.0.0

    // Business services
    github.com/osv/auth-service            v0.0.0
    github.com/osv/scan-service            v0.0.0
    github.com/defectdojo/finding-service  v0.0.0
    github.com/osv/product-service         v0.0.0
    github.com/osv/vulnerability-service   v0.0.0
    github.com/osv/report-service          v0.0.0
    github.com/osv/ingestion-service       v0.0.0
    github.com/osv/notification-service    v0.0.0
    github.com/osv/query-service           v0.0.0

    // HTTP & gRPC
    github.com/go-chi/chi/v5               v5.2.2
    github.com/go-chi/cors                 v1.2.1
    google.golang.org/grpc                 v1.81.1
    google.golang.org/protobuf             v1.36.0

    // NATS
    github.com/nats-io/nats.go             v1.37.0

    // Database
    github.com/jackc/pgx/v5               v5.10.0
    github.com/redis/go-redis/v9           v9.7.3

    // Misc
    github.com/rs/zerolog                  v1.33.0
    github.com/google/uuid                 v1.6.0
    github.com/spf13/viper                 v1.19.0
)

replace (
    github.com/osv/shared/pkg             => ../../services/shared/pkg
    github.com/osv/shared/proto           => ../../services/shared/proto
    github.com/osv/auth-service           => ../../services/auth-service
    github.com/osv/scan-service           => ../../services/scan-service
    github.com/defectdojo/finding-service => ../../services/finding-service
    github.com/osv/product-service        => ../../services/product-service
    github.com/osv/vulnerability-service  => ../../services/vulnerability-service
    github.com/osv/report-service         => ../../services/report-service
    github.com/osv/ingestion-service      => ../../services/ingestion-service
    github.com/osv/notification-service   => ../../services/notification-service
    github.com/osv/query-service          => ../../services/query-service
)
```

---

## go.work

Cập nhật `osv.dev/services/go.work`:

```go
use (
    ...
    ../apps/OpenVulnScan   // ← thêm dòng này
)
```

---

## internal/app/registry.go

```go
package app

import (
    "context"
    "sync"
    "github.com/rs/zerolog"
)

// ServiceRunner là interface mà mỗi goroutine service implement.
type ServiceRunner interface {
    Name() string
    Run(ctx context.Context) error
    Health(ctx context.Context) error
}

// Registry quản lý vòng đời của tất cả service goroutines.
type Registry struct {
    runners []ServiceRunner
    wg      sync.WaitGroup
    log     zerolog.Logger
}

func NewRegistry(log zerolog.Logger) *Registry {
    return &Registry{log: log}
}

func (r *Registry) Register(runner ServiceRunner) {
    r.runners = append(r.runners, runner)
}

// Start khởi động tất cả runners như goroutines độc lập.
func (r *Registry) Start(ctx context.Context) {
    for _, svc := range r.runners {
        r.wg.Add(1)
        go func(s ServiceRunner) {
            defer r.wg.Done()
            r.log.Info().Str("service", s.Name()).Msg("starting service goroutine")
            if err := s.Run(ctx); err != nil && err != context.Canceled {
                r.log.Error().Str("service", s.Name()).Err(err).Msg("service goroutine exited with error")
            } else {
                r.log.Info().Str("service", s.Name()).Msg("service goroutine stopped")
            }
        }(svc)
    }
}

// Wait blocks until tất cả goroutines kết thúc.
func (r *Registry) Wait() { r.wg.Wait() }

// HealthAll kiểm tra health của tất cả services.
func (r *Registry) HealthAll(ctx context.Context) map[string]error {
    results := make(map[string]error)
    for _, svc := range r.runners {
        results[svc.Name()] = svc.Health(ctx)
    }
    return results
}
```

---

## internal/runners/auth_runner.go (template)

```go
package runners

import (
    "context"
    "fmt"
    "net"

    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/test/bufconn"

    // Import từ auth-service (không thay đổi)
    authpb      "github.com/osv/shared/proto/gen/go/auth/v1"
    authgrpc    "github.com/osv/auth-service/internal/delivery/grpc"
    authlogin   "github.com/osv/auth-service/internal/usecase/login"
    authregister "github.com/osv/auth-service/internal/usecase/register"
    authjwt     "github.com/osv/auth-service/internal/infra/auth"
    authrepo    "github.com/osv/auth-service/internal/infra/repository"
    "github.com/osv/apps/openvulnscan/internal/transport"
)

type AuthRunner struct {
    cfg     AuthRunnerConfig
    lis     *bufconn.Listener
    server  *grpc.Server
}

type AuthRunnerConfig struct {
    DBURL      string
    RedisURL   string
    JWTSecret  string
    JWTExpiry  time.Duration
}

func NewAuthRunner(cfg AuthRunnerConfig, lis *bufconn.Listener) *AuthRunner {
    return &AuthRunner{cfg: cfg, lis: lis}
}

func (r *AuthRunner) Name() string { return "auth-service" }

func (r *AuthRunner) Run(ctx context.Context) error {
    // Khởi tạo dependencies từ auth-service
    db, err := authrepo.ConnectDB(r.cfg.DBURL)
    if err != nil { return fmt.Errorf("auth: db: %w", err) }
    defer db.Close()

    userRepo    := authrepo.NewUserRepo(db)
    sessionRepo := authrepo.NewSessionRepo(db)
    jwtSigner   := authjwt.NewJWTSigner(r.cfg.JWTSecret, r.cfg.JWTExpiry)

    loginUC    := authlogin.New(userRepo, sessionRepo, jwtSigner)
    registerUC := authregister.New(userRepo, jwtSigner)

    // Tạo gRPC server và register handler
    r.server = grpc.NewServer()
    authHandler := authgrpc.NewHandler(loginUC, registerUC, /* ... */)
    authpb.RegisterAuthServiceServer(r.server, authHandler)
    grpc_health_v1.RegisterHealthServer(r.server, health.NewServer())

    errCh := make(chan error, 1)
    go func() { errCh <- r.server.Serve(r.lis) }()

    select {
    case <-ctx.Done():
        r.server.GracefulStop()
        return nil
    case err := <-errCh:
        return fmt.Errorf("auth-service: %w", err)
    }
}

func (r *AuthRunner) Health(ctx context.Context) error {
    conn, err := transport.DialBufConn(ctx, r.lis)
    if err != nil { return err }
    defer conn.Close()

    client := grpc_health_v1.NewHealthClient(conn)
    resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
    if err != nil { return err }
    if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
        return fmt.Errorf("not serving")
    }
    return nil
}
```

---

## internal/app/app.go (skeleton)

```go
package app

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/nats-io/nats.go"
    "github.com/redis/go-redis/v9"
    "google.golang.org/grpc/test/bufconn"

    "github.com/osv/apps/openvulnscan/internal/events"
    "github.com/osv/apps/openvulnscan/internal/runners"
    "github.com/osv/apps/openvulnscan/internal/transport"

    // Proto clients
    authpb    "github.com/osv/shared/proto/gen/go/auth/v1"
    scanpb    "github.com/osv/shared/proto/gen/go/scan/v1"
    findingpb "github.com/osv/shared/proto/gen/go/finding/v1"
)

type App struct {
    cfg      *Config
    registry *Registry

    // Infra
    db    *pgxpool.Pool
    nc    *nats.Conn
    redis *redis.Client

    // bufconn listeners
    authLis    *bufconn.Listener
    scanLis    *bufconn.Listener
    findingLis *bufconn.Listener
    productLis *bufconn.Listener
    vulnLis    *bufconn.Listener
    reportLis  *bufconn.Listener
    queryLis   *bufconn.Listener

    // gRPC clients (kết nối đến service goroutines)
    authClient    authpb.AuthServiceClient
    scanClient    scanpb.ScanServiceClient
    findingClient findingpb.FindingServiceClient
    // ...
}

func New(cfg *Config) (*App, error) {
    a := &App{cfg: cfg, registry: NewRegistry(log)}

    // 1. Connect infra
    if err := a.connectInfra(); err != nil {
        return nil, err
    }

    // 2. Setup NATS JetStream streams
    if _, err := events.SetupJetStream(a.nc); err != nil {
        return nil, fmt.Errorf("setup jetstream: %w", err)
    }

    // 3. Create bufconn listeners
    a.authLis    = transport.NewBufConnListener()
    a.scanLis    = transport.NewBufConnListener()
    a.findingLis = transport.NewBufConnListener()
    a.productLis = transport.NewBufConnListener()
    a.vulnLis    = transport.NewBufConnListener()
    a.reportLis  = transport.NewBufConnListener()
    a.queryLis   = transport.NewBufConnListener()

    // 4. Register service goroutines
    a.registry.Register(runners.NewAuthRunner(cfg.Auth, a.authLis))
    a.registry.Register(runners.NewScanRunner(cfg.Scan, a.nc, a.scanLis))
    a.registry.Register(runners.NewFindingRunner(cfg.Finding, a.nc, a.findingLis))
    a.registry.Register(runners.NewProductRunner(cfg.Product, a.productLis))
    a.registry.Register(runners.NewVulnRunner(cfg.Vuln, a.vulnLis))
    a.registry.Register(runners.NewReportRunner(cfg.Report, a.reportLis))
    a.registry.Register(runners.NewQueryRunner(cfg.Query, a.queryLis))
    a.registry.Register(runners.NewNotifyRunner(cfg.Notification, a.nc))
    a.registry.Register(runners.NewIngestionRunner(cfg.Ingestion, a.nc))

    return a, nil
}

func (a *App) Start(ctx context.Context) error {
    // Start tất cả service goroutines
    a.registry.Start(ctx)

    // Connect gRPC clients đến service goroutines
    return a.connectClients(ctx)
}
```
