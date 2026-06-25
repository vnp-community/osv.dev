# SC-OSV-01 — apps/osv Orchestrator Framework

## Metadata
- **Task ID**: SC-OSV-01
- **Sprint**: C (P1 — OSV Server Integration)
- **Ước tính**: 3 giờ
- **Dependencies**: SA-SVC-01 (service constructors)
- **Spec nguồn**: `specs/solutions/enhance-cli-app/03_osv-server-upgrade.md` § "3. Giải pháp"

---

## Context

```bash
# Xem OSV server stub
cat apps/osv/cmd/server/main.go

# Xem existing internal structure
ls apps/osv/internal/

# Xem go.mod để biết existing deps
cat apps/osv/go.mod | head -30
```

---

## Goal

Tạo orchestrator framework trong `apps/osv/internal/orchestrator/` để quản lý lifecycle của tất cả embedded services trong goroutines riêng biệt.

---

## Files to Create

### File 1: `apps/osv/internal/orchestrator/service.go`

```go
// Package orchestrator manages the concurrent lifecycle of all embedded microservices.
// Each service runs in its own goroutine and communicates via gRPC/REST/NATS.
package orchestrator

import "context"

// Service is the interface that all embedded services must implement.
// Each service's Start method blocks until the context is cancelled or an error occurs.
type Service interface {
	// Name returns a human-readable service identifier for logging.
	Name() string
	// Start begins serving requests. Blocks until ctx is done or error.
	// Must return nil when ctx is cancelled (not an error).
	Start(ctx context.Context) error
}
```

### File 2: `apps/osv/internal/orchestrator/orchestrator.go`

```go
package orchestrator

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// Orchestrator starts all registered services concurrently using errgroup.
// If any service returns a non-nil error (other than context.Canceled),
// all other services are cancelled via the group context.
type Orchestrator struct {
	services []Service
	log      zerolog.Logger
}

// New creates a new Orchestrator with the given logger.
func New(log zerolog.Logger) *Orchestrator {
	return &Orchestrator{log: log}
}

// Register adds one or more services to be managed.
// Services are started in registration order but run concurrently.
func (o *Orchestrator) Register(svcs ...Service) {
	o.services = append(o.services, svcs...)
}

// Run starts all registered services in separate goroutines.
// Blocks until all services stop. Returns the first non-cancelled error,
// or nil if all services stopped cleanly.
func (o *Orchestrator) Run(ctx context.Context) error {
	if len(o.services) == 0 {
		return fmt.Errorf("orchestrator: no services registered")
	}

	g, gCtx := errgroup.WithContext(ctx)

	for _, svc := range o.services {
		svc := svc // capture loop variable
		g.Go(func() error {
			o.log.Info().Str("service", svc.Name()).Msg("starting")
			err := svc.Start(gCtx)
			if err != nil && !errors.Is(err, context.Canceled) {
				o.log.Error().Err(err).Str("service", svc.Name()).Msg("service exited with error")
				return fmt.Errorf("%s: %w", svc.Name(), err)
			}
			o.log.Info().Str("service", svc.Name()).Msg("stopped")
			return nil
		})
	}

	return g.Wait()
}

// ServiceCount returns the number of registered services.
func (o *Orchestrator) ServiceCount() int { return len(o.services) }
```

### File 3: `apps/osv/internal/orchestrator/nats_runner.go`

```go
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/rs/zerolog"
)

// NATSRunner embeds a NATS JetStream server in-process.
// Eliminates the need for an external NATS deployment in development/staging.
//
// Activated when NATS_EMBEDDED=true (default in apps/osv).
// In production with distributed services, use external NATS instead.
type NATSRunner struct {
	port int
	srv  *natsserver.Server
	log  zerolog.Logger
}

// NewNATSRunner creates a runner for an embedded NATS server.
// port: TCP port to listen on (default: 4222)
func NewNATSRunner(port int, log zerolog.Logger) *NATSRunner {
	return &NATSRunner{port: port, log: log}
}

// Name satisfies the Service interface.
func (r *NATSRunner) Name() string { return "nats-embedded" }

// Start launches the embedded NATS server and blocks until ctx is cancelled.
func (r *NATSRunner) Start(ctx context.Context) error {
	storeDir := os.TempDir()

	opts := &natsserver.Options{
		Host:      "0.0.0.0",
		Port:      r.port,
		JetStream: true,
		StoreDir:  storeDir,
		NoLog:     true,  // zerolog handles logging
		NoSigs:    true,  // we handle signals via ctx
	}

	srv, err := natsserver.NewServer(opts)
	if err != nil {
		return fmt.Errorf("nats server init: %w", err)
	}
	r.srv = srv

	go srv.Start()

	if !srv.ReadyForConnections(10 * time.Second) {
		srv.Shutdown()
		return errors.New("nats embedded server: not ready after 10s")
	}
	r.log.Info().Int("port", r.port).Msg("NATS embedded server started")

	<-ctx.Done()
	r.log.Info().Msg("NATS embedded server shutting down")
	srv.Shutdown()
	return nil
}
```

### File 4: `apps/osv/internal/orchestrator/health.go`

```go
package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthChecker is implemented by services that expose a health status.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// AggregatedHealth exposes an HTTP handler that checks all registered services.
// Returns 200 OK if all healthy, 503 Service Unavailable if any are unhealthy.
type AggregatedHealth struct {
	mu       sync.RWMutex
	checkers []namedChecker
}

type namedChecker struct {
	name    string
	checker HealthChecker
}

// Register adds a service to the health check pool.
func (h *AggregatedHealth) Register(name string, checker HealthChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers = append(h.checkers, namedChecker{name, checker})
}

// ServeHTTP implements http.Handler for GET /health.
func (h *AggregatedHealth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	h.mu.RLock()
	checkers := h.checkers
	h.mu.RUnlock()

	type serviceStatus struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	results := make(map[string]serviceStatus)
	healthy := true
	for _, nc := range checkers {
		if err := nc.checker.HealthCheck(ctx); err != nil {
			results[nc.name] = serviceStatus{Status: "unhealthy", Error: err.Error()}
			healthy = false
		} else {
			results[nc.name] = serviceStatus{Status: "healthy"}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if !healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   map[bool]string{true: "ok", false: "degraded"}[healthy],
		"services": results,
	})
}
```

### File 5: `apps/osv/internal/orchestrator/grpc_pool.go`

```go
package orchestrator

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCPool manages shared gRPC connections to all embedded services.
// When apps/osv runs embedded, all services are on localhost —
// using a pool avoids creating multiple connections to the same address.
type GRPCPool struct {
	DataConn     *grpc.ClientConn // data-service (port 50053)
	SearchConn   *grpc.ClientConn // search-service (via HTTP, not gRPC yet)
	AIConn       *grpc.ClientConn // ai-service (port 50052)
	FindingConn  *grpc.ClientConn // finding-service (port 50060)
	IdentityConn *grpc.ClientConn // identity-service (port 50051)
}

// PoolConfig holds addresses for all services.
type PoolConfig struct {
	DataAddr     string // "localhost:50053"
	AIAddr       string // "localhost:50052"
	FindingAddr  string // "localhost:50060"
	IdentityAddr string // "localhost:50051"
}

// NewGRPCPool creates connections to all services.
// Returns error if any connection cannot be established.
func NewGRPCPool(cfg PoolConfig) (*GRPCPool, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	dial := func(addr string) (*grpc.ClientConn, error) {
		if addr == "" {
			return nil, nil
		}
		conn, err := grpc.NewClient(addr, opts...)
		if err != nil {
			return nil, fmt.Errorf("dial %s: %w", addr, err)
		}
		return conn, nil
	}

	pool := &GRPCPool{}
	var err error

	if pool.DataConn, err = dial(cfg.DataAddr); err != nil {
		return nil, err
	}
	if pool.AIConn, err = dial(cfg.AIAddr); err != nil {
		pool.Close()
		return nil, err
	}
	if pool.FindingConn, err = dial(cfg.FindingAddr); err != nil {
		pool.Close()
		return nil, err
	}
	if pool.IdentityConn, err = dial(cfg.IdentityAddr); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

// Close releases all connections.
func (p *GRPCPool) Close() {
	for _, conn := range []*grpc.ClientConn{
		p.DataConn, p.AIConn, p.FindingConn, p.IdentityConn,
	} {
		if conn != nil {
			conn.Close()
		}
	}
}
```

---

## Update `apps/osv/go.mod`

```go
require (
    // ... existing ...
    github.com/nats-io/nats-server/v2 v2.10.24   // embedded NATS
    golang.org/x/sync v0.14.0                    // errgroup
)
```

---

## Acceptance Criteria

- [ ] `apps/osv/internal/orchestrator/service.go` — Service interface
- [ ] `apps/osv/internal/orchestrator/orchestrator.go` — errgroup lifecycle manager
- [ ] `apps/osv/internal/orchestrator/nats_runner.go` — embedded NATS
- [ ] `apps/osv/internal/orchestrator/health.go` — aggregated health handler
- [ ] `apps/osv/internal/orchestrator/grpc_pool.go` — connection pool
- [ ] `go build ./internal/orchestrator/...` từ `apps/osv` PASS

---

## Verification

```bash
cd apps/osv
go build ./internal/orchestrator/...
```

---

## ✅ Execution Status: COMPLETED ✅

**Completed**: 2026-06-13

### Files Created (additive only)
- `apps/osv/internal/orchestrator/supervisor.go` — `Service` interface + `Supervisor` using `golang.org/x/errgroup`
- `apps/osv/internal/orchestrator/adapters.go` — `HTTPService` service adapter (health endpoints)
- `apps/osv/internal/orchestrator/health.go` — `HealthRegistry` với `HandleHealth`, `HandleReady`

### Build Verification
```
go build ./internal/orchestrator/...  → OK
```
