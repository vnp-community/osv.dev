# Kiến trúc Monolithic Go Application

## Tổng quan Kiến trúc

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        DefectDojo-Go  (Single Binary)                           │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                      main.go — Application Bootstrap                    │   │
│  │  • Load config (env/yaml)                                               │   │
│  │  • Init shared infra (DB pool, NATS conn, Redis conn)                   │   │
│  │  • Init ServiceRegistry                                                 │   │
│  │  • Start all services as goroutines                                     │   │
│  │  • Block on os.Signal / context.Done()                                  │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                        │                                        │
│  ┌─────────────────────────────────────▼───────────────────────────────────┐   │
│  │                     ServiceRegistry (central hub)                        │   │
│  │  • Goroutine lifecycle management (Start/Stop/Health)                   │   │
│  │  • In-process gRPC connections (bufconn)                                │   │
│  │  • NATS JetStream event bus                                             │   │
│  │  • Shared dependency injection                                          │   │
│  └───────────────────────────────────────────────────────────────────────────  │
│                                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ auth-service │  │product-service│  │finding-service│  │   scan-service  │   │
│  │ goroutine    │  │ goroutine    │  │ goroutine    │  │  goroutine       │   │
│  │              │  │              │  │              │  │                  │   │
│  │ • gRPC srv   │  │ • gRPC srv   │  │ • gRPC srv   │  │ • gRPC srv       │   │
│  │ • REST hdlr  │  │ • REST hdlr  │  │ • REST hdlr  │  │ • NATS consumer  │   │
│  │ • JWT/OAuth  │  │ • Product DB │  │ • Finding DB │  │ • Parser pool    │   │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────────┘   │
│                                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │vuln-service  │  │notification- │  │ report-serv  │  │integration-serv  │   │
│  │ goroutine    │  │ service      │  │ goroutine    │  │ goroutine        │   │
│  │              │  │ goroutine    │  │              │  │                  │   │
│  │ • gRPC srv   │  │ • NATS sub   │  │ • gRPC srv   │  │ • JIRA client    │   │
│  │ • CVE lookup │  │ • Email/Slack│  │ • PDF/HTML   │  │ • GitHub client  │   │
│  │ • NATS pub   │  │ • Webhook    │  │ • Stream     │  │ • NATS consumer  │   │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────────┘   │
│                                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │  ai-service  │  │impact-service│  │search-service│  │ ingestion-serv   │   │
│  │ goroutine    │  │ goroutine    │  │ goroutine    │  │ goroutine        │   │
│  │              │  │              │  │              │  │                  │   │
│  │ • NATS sub   │  │ • NATS sub   │  │ • gRPC srv   │  │ • NVD/OSV fetch  │   │
│  │ • LLM client │  │ • CVE impact │  │ • OpenSearch │  │ • NATS pub       │   │
│  │ • NATS pub   │  │ • NATS pub   │  │ • Index sync │  │ • DB writer      │   │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                   unified-gateway goroutine                              │   │
│  │  • HTTP/REST :8080 — DefectDojo v2 API compatible                      │   │
│  │  • gRPC :9090 — External gRPC access                                   │   │
│  │  • JWT validation → auth-service                                        │   │
│  │  • Rate limiting, CORS, request routing                                 │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Goroutine Topology

### Goroutine Hierarchy

```
main goroutine
│
├── serviceRegistry.Start()
│   ├── go authService.Run(ctx)        [goroutine-1]
│   │   ├── go grpcServer.Serve()     [goroutine-1a]
│   │   └── go httpHandler.ListenAndServe() [goroutine-1b]
│   │
│   ├── go productService.Run(ctx)     [goroutine-2]
│   │   ├── go grpcServer.Serve()     [goroutine-2a]
│   │   └── go eventSubscriber.Run()  [goroutine-2b]
│   │
│   ├── go findingService.Run(ctx)     [goroutine-3]
│   │   ├── go grpcServer.Serve()     [goroutine-3a]
│   │   ├── go slaChecker.Run()       [goroutine-3b]  (ticker)
│   │   └── go natsConsumer.Run()     [goroutine-3c]
│   │
│   ├── go scanService.Run(ctx)        [goroutine-4]
│   │   ├── go grpcServer.Serve()     [goroutine-4a]
│   │   ├── go natsConsumer.Run()     [goroutine-4b]
│   │   └── go parserWorkerPool()     [goroutine-4c..4n] (worker pool)
│   │
│   ├── go vulnService.Run(ctx)        [goroutine-5]
│   ├── go notificationService.Run(ctx)[goroutine-6]
│   ├── go reportService.Run(ctx)      [goroutine-7]
│   ├── go aiService.Run(ctx)          [goroutine-8]
│   ├── go impactService.Run(ctx)      [goroutine-9]
│   ├── go integrationService.Run(ctx) [goroutine-10]
│   ├── go searchService.Run(ctx)      [goroutine-11]
│   ├── go ingestionService.Run(ctx)   [goroutine-12]
│   └── go unifiedGateway.Run(ctx)     [goroutine-13]
│       ├── go httpServer.ListenAndServe()  [goroutine-13a]
│       └── go grpcServer.Serve()          [goroutine-13b]
│
└── signal.Notify() → gracefulShutdown()
```

## Phương thức Giao tiếp

### 1. In-Process gRPC (bufconn) — Ưu tiên chính

```go
// Ưu điểm: zero network overhead, type-safe, streaming support
// Sử dụng khi: service-to-service calls trong cùng process

import "google.golang.org/grpc/test/bufconn"

listener := bufconn.Listen(1024 * 1024)
// finding-service expose gRPC server trên listener này
// report-service connect tới listener này để stream findings
```

### 2. NATS JetStream — Async Event Bus

```go
// Sử dụng khi: async workflows, fan-out events, durability cần thiết

// Scan hoàn thành → finding-service xử lý → notification-service alert
// Subject naming: dd.{service}.{event}
// dd.scan.completed
// dd.finding.created
// dd.finding.severity.critical
// dd.sla.breach
// dd.report.requested
```

### 3. Direct Go Interface — Same-Process Optimization

```go
// Sử dụng khi: hot paths không cần network boundary
// Implement khi 2 service trực tiếp có dependency

type FindingQuerier interface {
    ListForReport(ctx context.Context, filter FindingFilter) (<-chan *Finding, error)
}
// report-service nhận interface, finding-service implement
```

## Module Structure (Go Workspace)

```
apps/DefectDojo/
├── go.work                         # Workspace file
├── cmd/
│   └── defectdojo/
│       └── main.go                 # Entry point
├── internal/
│   ├── app/
│   │   ├── app.go                  # Application bootstrap
│   │   └── registry/
│   │       └── registry.go         # Service registry
│   ├── config/
│   │   └── config.go               # Unified config
│   └── gateway/
│       └── router.go               # HTTP routes mapping to DD v2 API
└── go.mod

# go.work includes:
# use ../../services/auth-service
# use ../../services/finding-service
# use ../../services/product-service
# ... (tất cả services)
# use .
```

## Startup Sequence

```
1. Load config from env/yaml
2. Connect to PostgreSQL (connection pool shared)
3. Connect to NATS JetStream
4. Connect to Redis
5. Connect to OpenSearch
6. Run DB migrations (tất cả services)
7. Start ServiceRegistry
8. Start goroutines (theo dependency order):
   a. auth-service       (no deps)
   b. product-service    (no deps)
   c. vulnerability-service (no deps)
   d. search-service     (depends: OpenSearch)
   e. finding-service    (depends: product-service, auth-service)
   f. scan-service       (depends: finding-service, product-service)
   g. ingestion-service  (depends: vulnerability-service, search-service)
   h. notification-service (depends: finding-service, product-service)
   i. report-service     (depends: finding-service, product-service)
   j. ai-service         (depends: finding-service)
   k. impact-service     (depends: vulnerability-service, finding-service)
   l. integration-service (depends: finding-service, product-service)
   m. unified-gateway    (depends: tất cả services — cuối cùng)
9. Wait for all services healthy
10. Accept traffic
```

## Graceful Shutdown

```go
func (a *App) Shutdown(ctx context.Context) error {
    // 1. Stop unified-gateway (reject new requests)
    a.registry.Stop("unified-gateway")
    
    // 2. Wait for in-flight requests (timeout: 30s)
    
    // 3. Stop ingestion (no new data)
    a.registry.Stop("ingestion-service")
    
    // 4. Stop all async consumers
    a.nats.Drain()
    
    // 5. Stop remaining services
    a.registry.StopAll()
    
    // 6. Close DB connections
    a.db.Close()
    
    return nil
}
```

## Health Check Architecture

```
GET /health           → 200 if all services healthy
GET /health/ready     → 200 if ready to accept traffic  
GET /health/{service} → Individual service health

// Mỗi service implement interface:
type HealthChecker interface {
    Name() string
    Health(ctx context.Context) error
}
```
