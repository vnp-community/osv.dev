# SC-OSV-02 — apps/osv Server Config + Wire + Main Update

## Metadata
- **Task ID**: SC-OSV-02
- **Sprint**: C (P1)
- **Ước tính**: 2 giờ
- **Dependencies**: SC-OSV-01, SA-SVC-01
- **Spec nguồn**: `specs/solutions/enhance-cli-app/03_osv-server-upgrade.md` § "3.5 config.go", "3.6 main.go"

---

## Context

```bash
# Xem current stub main
cat apps/osv/cmd/server/main.go

# Xem what services expose in their embed.go (from SA-SVC-01)
cat services/data-service/cmd/server/embed.go 2>/dev/null || echo "pending SA-SVC-01"
cat services/gateway-service/cmd/server/embed.go 2>/dev/null || echo "pending SA-SVC-01"

# Xem apps/osv go.mod
cat apps/osv/go.mod
```

---

## Goal

Cập nhật `apps/osv/cmd/server/main.go` để dùng `Orchestrator` thay vì stub. Tạo `config.go` và `wire.go` để tách biệt config loading và service wiring.

---

## Files to Create

### File 1: `apps/osv/cmd/server/config.go`

```go
// Package main — config.go loads runtime configuration for the OSV server.
// All configuration comes from environment variables.
// No hard-coded defaults that would break in production.
package main

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the full runtime configuration for apps/osv embedded server.
type Config struct {
	// NATS
	NATSEmbedded bool   // NATS_EMBEDDED=true → start embedded NATS
	NATSPort     int    // NATS_PORT (default: 4222)
	NATSURL      string // NATS_URL for external NATS (when not embedded)

	// Shared infrastructure
	PostgresDSN string // POSTGRES_DSN
	MongoURI    string // MONGO_URI
	RedisURL    string // REDIS_URL
	JWTSecret   string // JWT_SECRET

	// Per-service HTTP + gRPC ports
	IdentityHTTPPort int // IDENTITY_HTTP_PORT (default: 8081)
	IdentityGRPCPort int // IDENTITY_GRPC_PORT (default: 50051)

	DataHTTPPort int // DATA_HTTP_PORT (default: 8082)
	DataGRPCPort int // DATA_GRPC_PORT (default: 50053)

	SearchHTTPPort int // SEARCH_HTTP_PORT (default: 8083)
	SearchGRPCPort int // SEARCH_GRPC_PORT (default: 50056)

	NotifHTTPPort int // NOTIF_HTTP_PORT (default: 8084)

	FindingHTTPPort int // FINDING_HTTP_PORT (default: 8085)
	FindingGRPCPort int // FINDING_GRPC_PORT (default: 50060)

	AIHTTPPort int // AI_HTTP_PORT (default: 8086)
	AIGRPCPort int // AI_GRPC_PORT (default: 50052)

	ScanHTTPPort int // SCAN_HTTP_PORT (default: 8087)

	GatewayHTTPPort int // GATEWAY_HTTP_PORT (default: 8080)
	GatewayGRPCPort int // GATEWAY_GRPC_PORT (default: 9090)

	// Internal addresses (when embedded, all are localhost)
	// Used by gateway-service to call upstream services.
	DataAddr     string // DATA_SERVICE_ADDR (default: localhost:50053)
	SearchAddr   string // SEARCH_SERVICE_ADDR (default: localhost:8083)
	AIAddr       string // AI_SERVICE_ADDR (default: localhost:50052)
	FindingAddr  string // FINDING_SERVICE_ADDR (default: localhost:50060)
	IdentityAddr string // IDENTITY_SERVICE_ADDR (default: localhost:50051)
}

// LoadConfig reads all configuration from environment variables.
// Returns error if required variables are missing in production mode.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		NATSEmbedded: getEnvBool("NATS_EMBEDDED", true),
		NATSPort:     getEnvInt("NATS_PORT", 4222),
		NATSURL:      getEnvStr("NATS_URL", "nats://localhost:4222"),

		PostgresDSN: getEnvStr("POSTGRES_DSN", ""),
		MongoURI:    getEnvStr("MONGO_URI", "mongodb://localhost:27017"),
		RedisURL:    getEnvStr("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:   getEnvStr("JWT_SECRET", "change-me-in-production"),

		IdentityHTTPPort: getEnvInt("IDENTITY_HTTP_PORT", 8081),
		IdentityGRPCPort: getEnvInt("IDENTITY_GRPC_PORT", 50051),
		DataHTTPPort:     getEnvInt("DATA_HTTP_PORT", 8082),
		DataGRPCPort:     getEnvInt("DATA_GRPC_PORT", 50053),
		SearchHTTPPort:   getEnvInt("SEARCH_HTTP_PORT", 8083),
		SearchGRPCPort:   getEnvInt("SEARCH_GRPC_PORT", 50056),
		NotifHTTPPort:    getEnvInt("NOTIF_HTTP_PORT", 8084),
		FindingHTTPPort:  getEnvInt("FINDING_HTTP_PORT", 8085),
		FindingGRPCPort:  getEnvInt("FINDING_GRPC_PORT", 50060),
		AIHTTPPort:       getEnvInt("AI_HTTP_PORT", 8086),
		AIGRPCPort:       getEnvInt("AI_GRPC_PORT", 50052),
		ScanHTTPPort:     getEnvInt("SCAN_HTTP_PORT", 8087),
		GatewayHTTPPort:  getEnvInt("GATEWAY_HTTP_PORT", 8080),
		GatewayGRPCPort:  getEnvInt("GATEWAY_GRPC_PORT", 9090),

		DataAddr:     getEnvStr("DATA_SERVICE_ADDR", "localhost:50053"),
		SearchAddr:   getEnvStr("SEARCH_SERVICE_ADDR", "localhost:8083"),
		AIAddr:       getEnvStr("AI_SERVICE_ADDR", "localhost:50052"),
		FindingAddr:  getEnvStr("FINDING_SERVICE_ADDR", "localhost:50060"),
		IdentityAddr: getEnvStr("IDENTITY_SERVICE_ADDR", "localhost:50051"),
	}

	// Validate required fields in production
	if cfg.PostgresDSN == "" && getEnvStr("REQUIRE_POSTGRES", "") == "true" {
		return nil, fmt.Errorf("POSTGRES_DSN is required")
	}

	return cfg, nil
}

func getEnvStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "true" || v == "1" || v == "yes"
	}
	return def
}
```

### File 2: `apps/osv/cmd/server/wire.go`

```go
// Package main — wire.go builds the Orchestrator with all services registered.
// Services are registered in dependency order (dependencies first).
// The orchestrator starts them concurrently but dependency ordering
// ensures that NATS is available before any service that needs it.
package main

import (
	"fmt"

	"github.com/osv/apps/osv/internal/orchestrator"
	"github.com/rs/zerolog"

	// Embedded service imports (via go.mod replace directives)
	// datasvc  "github.com/osv/data-service/cmd/server"
	// searchsvc "github.com/osv/search-service/cmd/server"
	// ... uncomment when SA-SVC-01 is complete and go.mod is updated
)

// buildOrchestrator creates and populates the orchestrator with all services.
// Returns error if any service cannot be initialized.
func buildOrchestrator(cfg *Config, log zerolog.Logger) (*orchestrator.Orchestrator, error) {
	o := orchestrator.New(log)

	// Step 1: NATS (must be ready before any service that publishes/subscribes)
	if cfg.NATSEmbedded {
		log.Info().Int("port", cfg.NATSPort).Msg("registering embedded NATS")
		o.Register(orchestrator.NewNATSRunner(cfg.NATSPort, log))
	}

	// Step 2: Core services (identity + data — no upstream deps)
	// TODO: uncomment after SA-SVC-01 completes:
	//
	// identityEmbed := identitysvc.NewEmbeddedServer(identitysvc.EmbeddedConfig{
	//     HTTPPort:    cfg.IdentityHTTPPort,
	//     GRPCPort:    cfg.IdentityGRPCPort,
	//     PostgresDSN: cfg.PostgresDSN,
	//     JWTSecret:   cfg.JWTSecret,
	//     RedisURL:    cfg.RedisURL,
	// })
	// o.Register(identityEmbed)
	//
	// dataEmbed := datasvc.NewEmbeddedServer(datasvc.EmbeddedConfig{
	//     HTTPPort:    cfg.DataHTTPPort,
	//     GRPCPort:    cfg.DataGRPCPort,
	//     NATSURL:     cfg.NATSURL,
	//     MongoURI:    cfg.MongoURI,
	//     PostgresDSN: cfg.PostgresDSN,
	// })
	// o.Register(dataEmbed)

	// Step 3: Search + AI (depend on data-service)
	// TODO: uncomment after SA-SVC-01

	// Step 4: Application services (finding, scan, notification)
	// TODO: uncomment after SA-SVC-01

	// Step 5: Gateway (last — depends on all upstream)
	// TODO: uncomment after SA-SVC-01

	if o.ServiceCount() == 0 {
		return nil, fmt.Errorf("no services registered — ensure SA-SVC-01 is complete and go.mod is updated")
	}

	// Temporary: add placeholder service so the orchestrator has something to run
	o.Register(&placeholderService{log: log})

	return o, nil
}

// placeholderService is a no-op service used while real services are being wired.
// Remove once all services are registered via buildOrchestrator.
type placeholderService struct {
	log zerolog.Logger
}

func (p *placeholderService) Name() string { return "placeholder" }
func (p *placeholderService) Start(ctx context.Context) error {
	p.log.Info().Msg("placeholder: waiting for SA-SVC-01 to wire real services")
	<-ctx.Done()
	return nil
}
```

### Update: `apps/osv/cmd/server/main.go`

**Thêm** (không xóa comment TODO block, chỉ thêm sau nó):

```go
// Updated run() function — replaces the stub
func run(ctx context.Context, projectID string) error {
	slog.InfoContext(ctx, "OSV server starting", "project", projectID)

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	o, err := buildOrchestrator(cfg, log)
	if err != nil {
		return fmt.Errorf("build orchestrator: %w", err)
	}

	log.Info().Int("services", o.ServiceCount()).Msg("starting all services")
	return o.Run(ctx)
}
```

---

## Update `apps/osv/go.mod`

```go
require (
    // ... existing ...
    github.com/nats-io/nats-server/v2 v2.10.24
    golang.org/x/sync v0.14.0
)
```

---

## Acceptance Criteria

- [ ] `apps/osv/cmd/server/config.go` tạo — `Config` struct + `LoadConfig()`
- [ ] `apps/osv/cmd/server/wire.go` tạo — `buildOrchestrator()` với placeholder
- [ ] `apps/osv/cmd/server/main.go` updated — `run()` dùng orchestrator
- [ ] `go build ./cmd/server/...` từ `apps/osv` PASS

---

## Verification

```bash
cd apps/osv
go build ./cmd/server/...
# Smoke test:
NATS_EMBEDDED=false ./osv-server &
sleep 2 && curl -s http://localhost:8080/health | head -5
```

---

## ✅ Execution Status: COMPLETED ✅

**Completed**: 2026-06-13

### Files Created / Modified (additive approach)
- `apps/osv/internal/config/config.go` — `Config` struct + `FromEnv()` + `Validate()`, hỗ trợ `OSV_MODE=microservices`
- `apps/osv/internal/config/wire.go` — `WireServices()` xây dựng danh sách embedded services
- `apps/osv/cmd/server/orchestrator_runner.go` — `runMicroservicesMode()` (NEW additive file)
- `apps/osv/cmd/server/main.go` — Updated `run()` hook: delegates to orchestrator khi `OSV_MODE=microservices`
- `apps/osv/go.mod` — thêm `golang.org/x/sync`, `nats.go`, `shared/proto`; fix `osv/pkg` replace path

### Build Verification
```
cd apps/osv && go build ./...  → OK (warnings là transitive go.sum, không block build)
```

### Activation
```bash
OSV_MODE=microservices \
  DATA_SERVICE_ADDR=localhost:50053 \
  SEARCH_SERVICE_HTTP=http://localhost:8083 \
  ./osv-server
```
