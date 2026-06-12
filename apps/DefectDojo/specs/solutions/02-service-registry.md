# Service Registry & Lifecycle Management

## Service Runner Interface

Mỗi service được wrap trong một `ServiceRunner` để tích hợp vào monolith:

```go
// apps/DefectDojo/internal/app/registry/runner.go

package registry

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/rs/zerolog/log"
)

// ServiceRunner wraps một service thành một goroutine-managed unit.
type ServiceRunner interface {
    // Name returns the unique service identifier.
    Name() string
    
    // Run starts the service and blocks until ctx is done or fatal error.
    Run(ctx context.Context) error
    
    // Health returns nil if service is healthy.
    Health(ctx context.Context) error
}

// Registry manages all service runners in the monolith.
type Registry struct {
    mu       sync.RWMutex
    runners  map[string]ServiceRunner
    statuses map[string]*ServiceStatus
    cancel   map[string]context.CancelFunc
    wg       sync.WaitGroup
}

// ServiceStatus tracks runtime state.
type ServiceStatus struct {
    Name      string
    State     ServiceState
    StartedAt time.Time
    Error     error
    mu        sync.Mutex
}

type ServiceState int

const (
    StateIdle     ServiceState = iota
    StateStarting
    StateRunning
    StateStopping
    StateStopped
    StateFailed
)

func New() *Registry {
    return &Registry{
        runners:  make(map[string]ServiceRunner),
        statuses: make(map[string]*ServiceStatus),
        cancel:   make(map[string]context.CancelFunc),
    }
}

// Register adds a service runner (call before Start).
func (r *Registry) Register(runner ServiceRunner) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.runners[runner.Name()] = runner
    r.statuses[runner.Name()] = &ServiceStatus{
        Name:  runner.Name(),
        State: StateIdle,
    }
}

// Start launches all registered services as goroutines.
func (r *Registry) Start(ctx context.Context) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    for name, runner := range r.runners {
        svcCtx, cancel := context.WithCancel(ctx)
        r.cancel[name] = cancel
        
        status := r.statuses[name]
        status.mu.Lock()
        status.State = StateStarting
        status.StartedAt = time.Now()
        status.mu.Unlock()
        
        r.wg.Add(1)
        go func(name string, runner ServiceRunner, svcCtx context.Context) {
            defer r.wg.Done()
            defer func() {
                if rec := recover(); rec != nil {
                    status := r.statuses[name]
                    status.mu.Lock()
                    status.State = StateFailed
                    status.Error = fmt.Errorf("panic: %v", rec)
                    status.mu.Unlock()
                    log.Error().Str("service", name).Interface("panic", rec).Msg("service panicked")
                }
            }()
            
            log.Info().Str("service", name).Msg("starting service")
            status.mu.Lock()
            status.State = StateRunning
            status.mu.Unlock()
            
            if err := runner.Run(svcCtx); err != nil && err != context.Canceled {
                status.mu.Lock()
                status.State = StateFailed
                status.Error = err
                status.mu.Unlock()
                log.Error().Str("service", name).Err(err).Msg("service failed")
            } else {
                status.mu.Lock()
                status.State = StateStopped
                status.mu.Unlock()
                log.Info().Str("service", name).Msg("service stopped")
            }
        }(name, runner, svcCtx)
    }
}

// Stop gracefully stops a specific service.
func (r *Registry) Stop(name string) {
    r.mu.RLock()
    cancel, ok := r.cancel[name]
    status := r.statuses[name]
    r.mu.RUnlock()
    
    if !ok {
        return
    }
    
    status.mu.Lock()
    status.State = StateStopping
    status.mu.Unlock()
    
    cancel()
}

// StopAll stops all services and waits for completion.
func (r *Registry) StopAll() {
    r.mu.RLock()
    for _, cancel := range r.cancel {
        cancel()
    }
    r.mu.RUnlock()
    r.wg.Wait()
}

// Wait blocks until all services have stopped.
func (r *Registry) Wait() {
    r.wg.Wait()
}

// HealthAll returns health status of all services.
func (r *Registry) HealthAll(ctx context.Context) map[string]error {
    results := make(map[string]error)
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    for name, runner := range r.runners {
        results[name] = runner.Health(ctx)
    }
    return results
}
```

## Service Runner Implementations

### AuthService Runner

```go
// apps/DefectDojo/internal/runners/auth_runner.go

package runners

import (
    "context"
    "net"
    
    authInternal "github.com/osv/auth-service/internal"
    "google.golang.org/grpc"
    "google.golang.org/grpc/test/bufconn"
)

type AuthServiceRunner struct {
    cfg    AuthConfig
    lis    *bufconn.Listener  // in-process gRPC listener
    server *grpc.Server
}

func NewAuthServiceRunner(cfg AuthConfig) *AuthServiceRunner {
    return &AuthServiceRunner{
        cfg: cfg,
        lis: bufconn.Listen(1 << 20), // 1 MB buffer
    }
}

func (r *AuthServiceRunner) Name() string { return "auth-service" }

func (r *AuthServiceRunner) Run(ctx context.Context) error {
    // 1. Init dependencies
    db, err := authInternal.NewDB(ctx, r.cfg.PostgresURL)
    if err != nil {
        return fmt.Errorf("auth-service: db init: %w", err)
    }
    defer db.Close()
    
    redis, err := authInternal.NewRedis(r.cfg.RedisURL)
    if err != nil {
        return fmt.Errorf("auth-service: redis init: %w", err)
    }
    
    // 2. Wire repositories & use cases (reuse auth-service internal code)
    userRepo := authInternal.NewUserRepository(db)
    sessionRepo := authInternal.NewSessionRepository(redis)
    authUC := authInternal.NewAuthUseCase(userRepo, sessionRepo, r.cfg.JWTSecret)
    
    // 3. Start gRPC server (on bufconn listener for in-process)
    r.server = grpc.NewServer()
    authInternal.RegisterAuthServiceServer(r.server, authInternal.NewGRPCHandler(authUC))
    
    errCh := make(chan error, 1)
    go func() {
        if err := r.server.Serve(r.lis); err != nil {
            errCh <- err
        }
    }()
    
    // 4. Also start HTTP REST handler for auth endpoints
    httpSrv := authInternal.NewHTTPServer(authUC, r.cfg.HTTPPort)
    go httpSrv.ListenAndServe()
    
    select {
    case <-ctx.Done():
        r.server.GracefulStop()
        httpSrv.Shutdown(context.Background())
        return nil
    case err := <-errCh:
        return err
    }
}

func (r *AuthServiceRunner) Health(ctx context.Context) error {
    // ping gRPC server via bufconn
    conn, err := grpc.DialContext(ctx, "auth-service",
        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
            return r.lis.DialContext(ctx)
        }),
        grpc.WithInsecure(),
    )
    if err != nil {
        return err
    }
    conn.Close()
    return nil
}

// Listener returns the bufconn listener for other services to connect.
func (r *AuthServiceRunner) Listener() *bufconn.Listener { return r.lis }
```

### FindingService Runner

```go
// apps/DefectDojo/internal/runners/finding_runner.go

package runners

import (
    "context"
    
    findingDelivery "github.com/defectdojo/finding-service/internal/delivery"
    findingInfra    "github.com/defectdojo/finding-service/internal/infra"
    findingUseCase  "github.com/defectdojo/finding-service/internal/usecase"
    "google.golang.org/grpc/test/bufconn"
)

type FindingServiceRunner struct {
    cfg       FindingConfig
    nats      *nats.Conn
    lis       *bufconn.Listener
    server    *grpc.Server
}

func (r *FindingServiceRunner) Name() string { return "finding-service" }

func (r *FindingServiceRunner) Run(ctx context.Context) error {
    db, err := pgxpool.New(ctx, r.cfg.PostgresURL)
    if err != nil {
        return fmt.Errorf("finding-service db: %w", err)
    }
    defer db.Close()
    
    // Wire using finding-service internal packages
    repo := findingInfra.NewPostgresRepository(db)
    slaRepo := findingInfra.NewSLARepository(db)
    auditRepo := findingInfra.NewAuditRepository(db)
    
    findingUC := findingUseCase.NewFindingUseCase(repo, slaRepo, auditRepo, r.nats)
    slaUC := findingUseCase.NewSLAUseCase(repo, slaRepo, r.nats)
    
    // gRPC server
    r.server = grpc.NewServer(grpc.UnaryInterceptor(loggingInterceptor))
    findingDelivery.RegisterFindingServiceServer(r.server, findingDelivery.NewGRPCHandler(findingUC))
    
    // SLA ticker goroutine
    go slaUC.RunSLAChecker(ctx, 1*time.Hour)
    
    errCh := make(chan error, 1)
    go func() {
        errCh <- r.server.Serve(r.lis)
    }()
    
    select {
    case <-ctx.Done():
        r.server.GracefulStop()
        return nil
    case err := <-errCh:
        return err
    }
}
```

### ProductService Runner

```go
// apps/DefectDojo/internal/runners/product_runner.go

package runners

import (
    productDelivery "github.com/defectdojo/product-service/internal/delivery"
    productDomain   "github.com/defectdojo/product-service/internal/domain"
    productInfra    "github.com/defectdojo/product-service/internal/infra"
    productUseCase  "github.com/defectdojo/product-service/internal/usecase"
)

type ProductServiceRunner struct {
    cfg ProductConfig
    lis *bufconn.Listener
}

func (r *ProductServiceRunner) Name() string { return "product-service" }

func (r *ProductServiceRunner) Run(ctx context.Context) error {
    db, _ := pgxpool.New(ctx, r.cfg.PostgresURL)
    defer db.Close()
    
    // Reuse product-service internal packages
    productRepo := productInfra.NewProductRepository(db)
    engagementRepo := productInfra.NewEngagementRepository(db)
    testRepo := productInfra.NewTestRepository(db)
    
    orchestrator := productDomain.NewOrchestrator(productRepo, engagementRepo, testRepo)
    productUC := productUseCase.New(orchestrator)
    
    r.server = grpc.NewServer()
    productDelivery.RegisterProductServiceServer(r.server, productDelivery.NewGRPCHandler(productUC))
    
    go r.server.Serve(r.lis)
    
    <-ctx.Done()
    r.server.GracefulStop()
    return nil
}
```

## Main Application Bootstrap

```go
// apps/DefectDojo/cmd/defectdojo/main.go

package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/defectdojo/apps/defectdojo/internal/app"
    "github.com/defectdojo/apps/defectdojo/internal/config"
    "github.com/rs/zerolog/log"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatal().Err(err).Msg("failed to load config")
    }
    
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    application, err := app.New(cfg)
    if err != nil {
        log.Fatal().Err(err).Msg("failed to create application")
    }
    
    // Start all services
    if err := application.Start(ctx); err != nil {
        log.Fatal().Err(err).Msg("failed to start application")
    }
    
    // Wait for signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    log.Info().Msg("shutting down...")
    
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer shutdownCancel()
    
    if err := application.Shutdown(shutdownCtx); err != nil {
        log.Error().Err(err).Msg("shutdown error")
    }
    
    log.Info().Msg("shutdown complete")
}
```

## Dependency Injection Map

```
┌─────────────────────────────────────────────────────────────┐
│                    Shared Infrastructure                     │
│                                                             │
│  pgxpool.Pool  →  auth, product, finding, scan, vuln,      │
│                   notification, report, integration          │
│                                                             │
│  *nats.Conn    →  finding, scan, vuln, notification,       │
│                   ingestion, ai, impact, integration         │
│                                                             │
│  *redis.Client →  auth, unified-gateway (rate limit)        │
│                                                             │
│  opensearch    →  search, ingestion                         │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                  In-Process gRPC Connections                 │
│                                                             │
│  auth-service.lis  ←──── unified-gateway (JWT validate)    │
│  auth-service.lis  ←──── finding-service (auth check)      │
│                                                             │
│  product-service.lis ←── finding-service (product lookup)  │
│  product-service.lis ←── scan-service (engagement lookup)  │
│  product-service.lis ←── report-service                    │
│  product-service.lis ←── unified-gateway                   │
│                                                             │
│  finding-service.lis ←── scan-service (batch create)       │
│  finding-service.lis ←── report-service (stream)           │
│  finding-service.lis ←── notification-service              │
│  finding-service.lis ←── ai-service                        │
│  finding-service.lis ←── unified-gateway                   │
│                                                             │
│  scan-service.lis  ←──── unified-gateway                   │
│  vuln-service.lis  ←──── unified-gateway                   │
│  search-service.lis ←─── unified-gateway                   │
└─────────────────────────────────────────────────────────────┘
```
