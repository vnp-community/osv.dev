# 08 — Implementation Tasks (Sprint Plan)

> **Ưu tiên**: P0 = Blocking, P1 = Important, P2 = Nice-to-have, P3 = Future

---

## Sprint A — Foundation (P0 Blocking)

### A1: Shared Event Types
**File**: `services/shared/pkg/events/types.go`
- [ ] Define `VulnImportedEvent`, `VulnUpdatedEvent`, `AIEnrichmentCompletedEvent`
- [ ] Define all subject constants
- [ ] Add `services/shared/pkg/nats/streams.go` with JetStream stream configs
- [ ] `go build ./...` phải pass

### A2: Shared gRPC Client Layer
**Dir**: `services/shared/pkg/clients/grpc/`
- [ ] `options.go` — DefaultDialOptions, DialTimeout
- [ ] `data_client.go` — DataClient (LookupByPackage, GetByID, Close)
- [ ] `ai_client.go` — AIClient (EnrichCVE, GetEPSS, BatchEnrich, Close)
- [ ] `search_client.go` — SearchClient (Search via REST)
- [ ] `go build ./...` phải pass

### A3: OSV REST Client
**File**: `services/shared/pkg/clients/rest/osv_client.go`
- [ ] `QueryByPackage` (POST /v1/query)
- [ ] `GetVuln` (GET /v1/vulns/{id})
- [ ] `QueryBatch` (POST /v1/querybatch)
- [ ] Unit tests với mock HTTP server

### A4: Service Expose — `NewServer()` Constructors
Mỗi service cần expose `NewServer(cfg Config) Service` để apps/osv có thể embed:
- [ ] `data-service/cmd/server`: export `NewServer(cfg Config) orchestrator.Service`
- [ ] `search-service/cmd/server`: export `NewServer(cfg Config) orchestrator.Service`
- [ ] `ai-service/cmd/server`: export `NewServer(cfg Config) orchestrator.Service`
- [ ] `finding-service/cmd/server`: export `NewServer(cfg Config) orchestrator.Service`
- [ ] `identity-service/cmd/server`: export `NewServer(cfg Config) orchestrator.Service`
- [ ] `notification-service/cmd/server`: export `NewServer(cfg Config) orchestrator.Service`
- [ ] `scan-service/cmd/server`: export `NewServer(cfg Config) orchestrator.Service`
- [ ] `gateway-service/cmd/server`: export `NewServer(cfg Config) orchestrator.Service`

---

## Sprint B — CLI Enhancement (P1)

### B1: CLI NATS Publisher
**File**: `apps/cli/internal/importer/nats_publisher.go`
- [ ] `NATSPublisher` struct implementing publisher interface
- [ ] Connect to JetStream
- [ ] Publish `osv.vuln.imported` events
- [ ] Add `CLI_BACKEND` selector to `cmd/importer/main.go`
- [ ] Integration test: publish → data-service receives

### B2: CLI AI Enricher
**File**: `apps/cli/internal/worker/pipeline/ai_enricher.go`
- [ ] `AIEnricher` implementing `Enricher` interface
- [ ] gRPC call to ai-service `EnrichCVE`
- [ ] Non-destructive enrichment (only fill empty fields)
- [ ] Add `AI_ENRICHER_ADDR` selector to `cmd/worker/main.go`

### B3: CLI Relations gRPC Backend
**File**: `apps/cli/cmd/relations/grpc_backend.go`
- [ ] `GRPCRelationsBackend` struct
- [ ] Call data-service `GetAliases` RPC
- [ ] Add `DATA_SERVICE_ADDR` selector to `cmd/relations/relations.go`

### B4: CLI Exporter REST Backend
**File**: `apps/cli/cmd/exporter/api_downloader.go`
- [ ] `APIDownloader` struct
- [ ] Paginated GET from data-service REST API
- [ ] Add `EXPORTER_BACKEND` selector to `cmd/exporter/exporter.go`

### B5: [NEW] `cmd/scan`
**Dir**: `apps/cli/cmd/scan/`
- [ ] `main.go` — CLI flags (--target, --type image|dir|sbom)
- [ ] gRPC call to scan-service `SubmitScan`
- [ ] Poll scan result via `GetScanResult`
- [ ] Output: JSON or table format

### B6: [NEW] `cmd/query`
**Dir**: `apps/cli/cmd/query/`
- [ ] `main.go` — CLI flags (--package, --version, --ecosystem, --cve, --commit)
- [ ] REST call to gateway-service OSV v1 API
- [ ] Output: JSON (--format json) or human-readable table
- [ ] Support `--format osv` for OSV schema output

### B7: [NEW] `cmd/enrich`
**Dir**: `apps/cli/cmd/enrich/`
- [ ] `main.go` — flags (--cve, --batch, --file)
- [ ] gRPC call to ai-service `EnrichCVE` or `BatchEnrich`
- [ ] Output: enrichment summary

---

## Sprint C — OSV Server Integration (P1)

### C1: Orchestrator Framework
**Dir**: `apps/osv/internal/orchestrator/`
- [ ] `service.go` — Service interface
- [ ] `orchestrator.go` — errgroup-based lifecycle manager
- [ ] `nats_runner.go` — Embedded NATS server
- [ ] `health.go` — Aggregated health check handler
- [ ] `grpc_pool.go` — Shared gRPC connection pool

### C2: Service Configuration
**Files**: `apps/osv/cmd/server/config.go`, `wire.go`
- [ ] `Config` struct with all service configs
- [ ] `LoadConfig()` from environment variables
- [ ] `buildOrchestrator()` wiring all services
- [ ] Add `apps/osv/go.mod` replace directives for all services

### C3: Update `apps/osv/cmd/server/main.go`
- [ ] Use `orchestrator.Orchestrator` instead of stub
- [ ] Proper shutdown sequence
- [ ] Health check at `GET /health`

### C4: Complete gateway-service OSV Routes
- [ ] `POST /v1/query` → cvedb.LookupCVEs gRPC
- [ ] `POST /v1/querybatch` → parallel LookupCVEs
- [ ] `GET /v1/search` → search-service REST
- [ ] Fix `GET /v1/vulns/{id}` stub → real gRPC call
- [ ] Add `GraphQL /graphql` to gateway router

---

## Sprint D — Feature Parity (P2)

### D1: Commit-based Query
- [ ] Add `QueryByCommit` RPC to data-service proto
- [ ] Implement in data-service handler
- [ ] Wire to gateway-service `POST /v1/query` when `commit` field present

### D2: Sitemap Generation
- [ ] `gateway-service`: `GET /sitemap.xml` endpoint
- [ ] Query data-service for all CVE IDs
- [ ] Stream XML output

### D3: Web Interface Serving
- [ ] gateway-service: serve static files for Next.js frontend
- [ ] Or proxy to Next.js dev server via `FRONTEND_URL` env

### D4: Prometheus Metrics
- [ ] Add `prometheus/client_golang` to each service
- [ ] Expose `GET /metrics` on each service
- [ ] Key metrics: request count, latency, error rate

### D5: OSV Schema Validation
- [ ] data-service: `POST /admin/validate` endpoint
- [ ] Validate OSV JSON against official schema
- [ ] CLI `cmd/osv-linter-worker` → REST call instead of inline

---

## Sprint E — Production Readiness (P3)

### E1: Docker Compose (all services)
**File**: `deploy/dev/docker-compose.yml` (hoặc thêm vào existing)
```yaml
services:
  nats:
    image: nats:2.10-alpine
    command: --js
  data-service:
    build: ./services/data-service
    env_file: .env
  search-service:
    build: ./services/search-service
  # ... etc
```

### E2: Embedded Mode vs Distributed Mode
- [ ] `DEPLOYMENT_MODE=embedded|distributed`
- [ ] Embedded: tất cả services trong 1 process (apps/osv)
- [ ] Distributed: mỗi service Docker container riêng

### E3: Health & Readiness Probes
- [ ] `GET /health` — liveness check
- [ ] `GET /ready` — readiness (all services up)
- [ ] Kubernetes-compatible probe format

### E4: Integration Tests
```
tests/integration/
├── cli_importer_test.go      ← importer → NATS → data-service
├── osv_query_test.go         ← gateway /v1/query → data-service
├── ai_enrichment_test.go     ← worker → ai-service → search-service
└── full_pipeline_test.go     ← import → enrich → search → query
```

---

## Prioritized File Creation Order

```
Phase 1 (Foundation):
  services/shared/pkg/events/types.go
  services/shared/pkg/nats/streams.go
  services/shared/pkg/clients/grpc/options.go
  services/shared/pkg/clients/grpc/data_client.go
  services/shared/pkg/clients/grpc/ai_client.go
  services/shared/pkg/clients/rest/osv_client.go

Phase 2 (CLI):
  apps/cli/internal/importer/nats_publisher.go
  apps/cli/internal/worker/pipeline/ai_enricher.go
  apps/cli/cmd/relations/grpc_backend.go
  apps/cli/cmd/exporter/api_downloader.go
  apps/cli/cmd/scan/main.go
  apps/cli/cmd/query/main.go
  apps/cli/cmd/enrich/main.go

Phase 3 (OSV Server):
  apps/osv/internal/orchestrator/service.go
  apps/osv/internal/orchestrator/orchestrator.go
  apps/osv/internal/orchestrator/nats_runner.go
  apps/osv/internal/orchestrator/health.go
  apps/osv/cmd/server/config.go
  apps/osv/cmd/server/wire.go
  [UPDATE] apps/osv/cmd/server/main.go
  [UPDATE] apps/osv/go.mod (add service replaces)

Phase 4 (Service Expose):
  [UPDATE] each service cmd/server: add NewServer() export
  [UPDATE] gateway-service: complete OSV v1 router

Phase 5 (Features):
  gateway-service: POST /v1/query, /v1/querybatch
  data-service: GetCVE, QueryByCommit RPCs
  search-service: proto definition
  scan-service: proto definition
```
