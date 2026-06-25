# 03 — OSV Server Upgrade (`apps/osv/cmd/server`)

> **Mục tiêu**: `apps/osv/cmd/server/main.go` khởi động tất cả microservices
> trong cùng một process (goroutines độc lập), trao đổi qua gRPC nội bộ.

---

## 1. Cấu trúc hiện tại

```
apps/osv/cmd/server/main.go   ← Stub (chỉ có TODO comments)
apps/osv/internal/api/        ← Legacy gRPC server (VulnStore interface)
```

Hiện tại `run()` function chỉ là stub với `<-ctx.Done()`.

---

## 2. Mục tiêu

```
apps/osv/cmd/server/main.go:
  1. Khởi động NATS (embedded hoặc external)
  2. Khởi động từng service trong goroutine riêng
  3. Health check tổng hợp
  4. Graceful shutdown theo thứ tự đúng
```

---

## 3. Giải pháp — Service Orchestrator

### 3.1 Cấu trúc mới

```
apps/osv/
├── cmd/
│   ├── server/
│   │   ├── main.go          ← UPDATE (thêm wire-up logic)
│   │   ├── wire.go          ← NEW: dependency injection
│   │   └── config.go        ← NEW: config loading
│   ├── api/                 ← GIỮ NGUYÊN (legacy)
│   └── api-devserver/       ← GIỮ NGUYÊN (legacy)
├── internal/
│   ├── api/                 ← GIỮ NGUYÊN (legacy)
│   └── [NEW] orchestrator/
│       ├── orchestrator.go  ← Service lifecycle manager
│       ├── service.go       ← Service interface
│       ├── nats_runner.go   ← NATS embedded runner
│       └── health.go        ← Aggregated health check
└── go.mod                   ← UPDATE: thêm service deps
```

### 3.2 `internal/orchestrator/service.go` — Service Interface

```go
// Package orchestrator manages the lifecycle of all microservices.
package orchestrator

import "context"

// Service is the common interface for all embedded microservices.
type Service interface {
    // Name returns a human-readable service identifier.
    Name() string
    // Start begins serving. Blocks until ctx is cancelled or error occurs.
    Start(ctx context.Context) error
}
```

### 3.3 `internal/orchestrator/orchestrator.go` — Lifecycle Manager

```go
// Orchestrator starts all services concurrently and coordinates shutdown.
type Orchestrator struct {
    services []Service
    log      zerolog.Logger
}

func New(log zerolog.Logger) *Orchestrator {
    return &Orchestrator{log: log}
}

func (o *Orchestrator) Register(svcs ...Service) {
    o.services = append(o.services, svcs...)
}

// Run starts all services in separate goroutines.
// Returns when all services have stopped (either via ctx cancellation or error).
func (o *Orchestrator) Run(ctx context.Context) error {
    g, gCtx := errgroup.WithContext(ctx)
    
    for _, svc := range o.services {
        svc := svc  // capture
        g.Go(func() error {
            o.log.Info().Str("service", svc.Name()).Msg("starting")
            if err := svc.Start(gCtx); err != nil && !errors.Is(err, context.Canceled) {
                o.log.Error().Err(err).Str("service", svc.Name()).Msg("service error")
                return err
            }
            return nil
        })
    }
    
    return g.Wait()
}
```

### 3.4 `cmd/server/wire.go` — Service Wiring

```go
// wire.go builds and registers all services.
package main

import (
    "github.com/osv/apps/osv/internal/orchestrator"
    
    // Microservice packages
    dataservice  "github.com/osv/data-service/cmd/server"
    searchsvc    "github.com/osv/search-service/cmd/server"
    aisvc        "github.com/osv/ai-service/cmd/server"
    findingsvc   "github.com/osv/finding-service/cmd/server"
    identitysvc  "github.com/osv/identity-service/cmd/server"
    notifsvc     "github.com/osv/notification-service/cmd/server"
    scansvc      "github.com/osv/scan-service/cmd/server"
    gatewaysvc   "github.com/osv/gateway-service/cmd/server"
)

func buildOrchestrator(cfg *Config, log zerolog.Logger) (*orchestrator.Orchestrator, error) {
    o := orchestrator.New(log)
    
    // 1. NATS (must start first — services depend on it)
    if cfg.NATSEmbedded {
        o.Register(orchestrator.NewNATSRunner(cfg.NATSPort, log))
    }
    
    // 2. Core data services (no inter-service deps)
    o.Register(
        NewServiceWrapper("identity-service", identitysvc.NewServer(cfg.Identity), log),
        NewServiceWrapper("data-service",     dataservice.NewServer(cfg.Data), log),
    )
    
    // 3. Search + AI (depend on data-service for queries)
    o.Register(
        NewServiceWrapper("search-service",   searchsvc.NewServer(cfg.Search), log),
        NewServiceWrapper("ai-service",       aisvc.NewServer(cfg.AI), log),
    )
    
    // 4. Application services (depend on identity + data)
    o.Register(
        NewServiceWrapper("finding-service",  findingsvc.NewServer(cfg.Finding), log),
        NewServiceWrapper("scan-service",     scansvc.NewServer(cfg.Scan), log),
        NewServiceWrapper("notification-svc", notifsvc.NewServer(cfg.Notification), log),
    )
    
    // 5. Gateway (last — depends on all upstream services)
    o.Register(
        NewServiceWrapper("gateway-service",  gatewaysvc.NewServer(cfg.Gateway), log),
    )
    
    return o, nil
}
```

### 3.5 `cmd/server/config.go` — Configuration

```go
type Config struct {
    // NATS
    NATSEmbedded bool   // NATS_EMBEDDED=true → embed NATS server
    NATSPort     int    // NATS_PORT=4222
    NATSExternal string // NATS_URL (when not embedded)
    
    // Service configs
    Identity     identitysvc.Config
    Data         dataservice.Config
    Search       searchsvc.Config
    AI           aisvc.Config
    Finding      findingsvc.Config
    Scan         scansvc.Config
    Notification notifsvc.Config
    Gateway      gatewaysvc.Config
    
    // Shared infra
    PostgresDSN  string // POSTGRES_DSN
    MongoURI     string // MONGO_URI
    RedisURL     string // REDIS_URL
}

func LoadConfig() (*Config, error) {
    return &Config{
        NATSEmbedded: os.Getenv("NATS_EMBEDDED") == "true",
        NATSPort:     envInt("NATS_PORT", 4222),
        NATSExternal: os.Getenv("NATS_URL"),
        PostgresDSN:  os.Getenv("POSTGRES_DSN"),
        MongoURI:     os.Getenv("MONGO_URI"),
        RedisURL:     os.Getenv("REDIS_URL"),
        
        Identity: identitysvc.Config{
            HTTPPort: envInt("IDENTITY_HTTP_PORT", 8081),
            GRPCPort: envInt("IDENTITY_GRPC_PORT", 50051),
        },
        Data: dataservice.Config{
            HTTPPort: envInt("DATA_HTTP_PORT", 8082),
            GRPCPort: envInt("DATA_GRPC_PORT", 50053),
            NATSSubjects: []string{"osv.vuln.imported", "osv.ai.enrichment.completed"},
        },
        Search: searchsvc.Config{
            HTTPPort: envInt("SEARCH_HTTP_PORT", 8083),
            GRPCPort: envInt("SEARCH_GRPC_PORT", 50056),
        },
        AI: aisvc.Config{
            HTTPPort: envInt("AI_HTTP_PORT", 8086),
            GRPCPort: envInt("AI_GRPC_PORT", 50052),
        },
        Finding: findingsvc.Config{
            HTTPPort: envInt("FINDING_HTTP_PORT", 8085),
            GRPCPort: envInt("FINDING_GRPC_PORT", 50060),
        },
        Scan: scansvc.Config{
            HTTPPort: envInt("SCAN_HTTP_PORT", 8087),
        },
        Notification: notifsvc.Config{
            HTTPPort: envInt("NOTIF_HTTP_PORT", 8084),
        },
        Gateway: gatewaysvc.Config{
            HTTPPort: envInt("GATEWAY_HTTP_PORT", 8080),
            GRPCPort: envInt("GATEWAY_GRPC_PORT", 9090),
            // Internal service addresses (when all embedded, use localhost)
            DataAddr:     envOrDefault("DATA_SERVICE_ADDR",     "localhost:50053"),
            SearchAddr:   envOrDefault("SEARCH_SERVICE_ADDR",   "localhost:50056"),
            AIAddr:       envOrDefault("AI_SERVICE_ADDR",       "localhost:50052"),
            FindingAddr:  envOrDefault("FINDING_SERVICE_ADDR",  "localhost:50060"),
            IdentityAddr: envOrDefault("IDENTITY_SERVICE_ADDR", "localhost:50051"),
        },
    }, nil
}
```

### 3.6 `cmd/server/main.go` — Updated entrypoint

```go
// Command server is the main entrypoint for the OSV.dev web server.
// Composes all microservices into a single process for development/staging.
// For production, use individual service deployments.
package main

func main() {
    logger.InitGlobalLogger()
    defer logger.Close()

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    log := zerolog.New(os.Stdout).With().Timestamp().Logger()
    
    cfg, err := LoadConfig()
    if err != nil {
        log.Fatal().Err(err).Msg("failed to load config")
    }

    o, err := buildOrchestrator(cfg, log)
    if err != nil {
        log.Fatal().Err(err).Msg("failed to build orchestrator")
    }

    log.Info().Msg("OSV server starting — all services in goroutines")
    
    if err := o.Run(ctx); err != nil {
        log.Error().Err(err).Msg("server exited with error")
        os.Exit(1)
    }
    
    log.Info().Msg("OSV server shutdown complete")
}
```

---

## 4. NATS Embedded Runner

### `internal/orchestrator/nats_runner.go`

```go
// NATSRunner embeds a NATS server in-process.
// This avoids needing an external NATS deployment for development/testing.
type NATSRunner struct {
    port int
    srv  *server.Server
    log  zerolog.Logger
}

func NewNATSRunner(port int, log zerolog.Logger) *NATSRunner {
    return &NATSRunner{port: port, log: log}
}

func (r *NATSRunner) Name() string { return "nats-embedded" }

func (r *NATSRunner) Start(ctx context.Context) error {
    opts := &server.Options{
        Port:      r.port,
        JetStream: true,
        StoreDir:  os.TempDir(),
    }
    srv, err := server.NewServer(opts)
    if err != nil {
        return fmt.Errorf("nats server init: %w", err)
    }
    r.srv = srv
    
    go srv.Start()
    if !srv.ReadyForConnections(10 * time.Second) {
        return errors.New("nats embedded: not ready after 10s")
    }
    r.log.Info().Int("port", r.port).Msg("NATS embedded server started")
    
    <-ctx.Done()
    srv.Shutdown()
    return nil
}
```

**Dependency**: `github.com/nats-io/nats-server/v2 v2.10.x`

---

## 5. Aggregated Health Check

### `internal/orchestrator/health.go`

```go
// AggregatedHealth exposes /health that checks all embedded services.
type AggregatedHealth struct {
    services []HealthChecker
}

type HealthChecker interface {
    HealthCheck(ctx context.Context) error
}

// ServeHTTP returns 200 if all services healthy, 503 otherwise.
func (h *AggregatedHealth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    var failures []string
    for _, svc := range h.services {
        if err := svc.HealthCheck(r.Context()); err != nil {
            failures = append(failures, err.Error())
        }
    }
    if len(failures) > 0 {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]interface{}{"unhealthy": failures})
        return
    }
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

---

## 6. go.mod thay đổi cho `apps/osv`

```go
// apps/osv/go.mod — Thêm (không xóa existing):

require (
    // ... existing deps ...
    
    // NEW — embedded services
    github.com/osv/data-service v0.0.0
    github.com/osv/search-service v0.0.0
    github.com/osv/ai-service v0.0.0
    github.com/osv/finding-service v0.0.0
    github.com/osv/identity-service v0.0.0
    github.com/osv/notification-service v0.0.0
    github.com/osv/scan-service v0.0.0
    github.com/osv/gateway-service v0.0.0
    github.com/osv/shared/proto v0.0.0
    
    // NEW — NATS embedded
    github.com/nats-io/nats-server/v2 v2.10.24
    github.com/nats-io/nats.go v1.42.0
    
    // NEW — service dependencies
    golang.org/x/sync v0.14.0  // errgroup
)

replace (
    // ... existing replaces ...
    github.com/osv/data-service => ../../services/data-service            // NEW
    github.com/osv/search-service => ../../services/search-service        // NEW
    github.com/osv/ai-service => ../../services/ai-service                // NEW
    github.com/osv/finding-service => ../../services/finding-service      // NEW
    github.com/osv/identity-service => ../../services/identity-service    // NEW
    github.com/osv/notification-service => ../../services/notification-service  // NEW
    github.com/osv/scan-service => ../../services/scan-service            // NEW
    github.com/osv/gateway-service => ../../services/gateway-service      // NEW
    github.com/osv/shared/proto => ../../services/shared/proto            // NEW
)
```

---

## 7. Startup/Shutdown Sequence

```
STARTUP (theo thứ tự dependency):
  1. Load config
  2. NATS embedded (nếu NATS_EMBEDDED=true)
  3. identity-service goroutine (gRPC :50051 + HTTP :8081)
  4. data-service goroutine (gRPC :50053 + HTTP :8082)
     └─ waits for NATS ready
  5. search-service goroutine (gRPC :50056 + HTTP :8083)
  6. ai-service goroutine (gRPC :50052 + HTTP :8086)
  7. finding-service goroutine (gRPC :50060 + HTTP :8085)
  8. scan-service goroutine (HTTP :8087)
  9. notification-service goroutine (HTTP :8084)
  10. gateway-service goroutine (HTTP :8080 + gRPC :9090)
      └─ waits for upstream services ready

SHUTDOWN (SIGTERM → reverse order):
  10. gateway-service (stop accepting requests)
  9-7. Application services (drain in-flight)
  6-5. Search + AI (finish batch)
  4-3. Core services (flush writes)
  2. NATS (flush JetStream)
```
