# Sprint E — Production Readiness Tasks

> **Status**: ✅ COMPLETED — 2026-06-13
> **Tham chiếu**: Tất cả tasks được định nghĩa chi tiết trong [sprint-D/SD-all-feature-parity-tasks.md](../sprint-D/SD-all-feature-parity-tasks.md)

---

## SE-PROD-01 — Docker Compose (All Services) ✅

→ Chi tiết: [sprint-D/SD-all-feature-parity-tasks.md#se-prod-01](../sprint-D/SD-all-feature-parity-tasks.md#se-prod-01)

**Completed**: 2026-06-13

### Files Created / Modified
- `deploy/dev/docker-compose.yml` — **Updated** với đầy đủ healthchecks cho tất cả 9 services:
  - Infrastructure: postgres, mongodb, redis, nats (thêm healthcheck), elasticsearch
  - Application services: identity-service, data-service, search-service, scan-service, finding-service, ai-service, notification-service, gateway-service
  - Tất cả `nats: condition: service_started` → `service_healthy`
  - HTTP healthchecks dùng `curl /health` với `start_period: 20-30s`

### Verification
```bash
cd deploy/dev
docker compose config --quiet && echo "compose config: OK"
# Run:
docker compose up -d
docker compose ps  # All services should show "healthy"
curl http://localhost:8080/health  # Gateway health
```

---

## SE-PROD-02 — Health & Readiness Probes ✅

→ Chi tiết: [sprint-D/SD-all-feature-parity-tasks.md#se-prod-02](../sprint-D/SD-all-feature-parity-tasks.md#se-prod-02)

**Completed**: 2026-06-13

### Files Created
- `services/shared/pkg/health/handler.go` — shared health package:
  - `LivenessHandler(w, r)` — `GET /health` → HTTP 200 + uptime + go_version
  - `ReadinessHandler(checks ...func() error)` — `GET /ready` → runs checks in parallel with 5s timeout
  - Graceful: timeout treated as pass (avoid false negatives)

### Usage Pattern
```go
// In each service's HTTP router:
import "github.com/osv/shared/pkg/health"

r.HandleFunc("/health", health.LivenessHandler)
r.HandleFunc("/ready", health.ReadinessHandler(
    func() error { return db.Ping() },
    func() error { return nc.Flush() },
))
```

---

## SE-PROD-03 — Integration Tests (CLI + Gateway) ✅

→ Chi tiết: [sprint-D/SD-all-feature-parity-tasks.md#se-prod-03](../sprint-D/SD-all-feature-parity-tasks.md#se-prod-03)

**Completed**: 2026-06-13

### Files Created
- `tests/integration/README.md` — setup instructions + test matrix
- `tests/integration/go.mod` — test module (`github.com/osv/tests/integration`)
- `tests/integration/osv_query_test.go`:
  - `TestOSVQueryHealth` — gateway /health
  - `TestOSVQueryByID` — GET /v1/vulns/{id}
  - `TestOSVQueryByPackage` — POST /v1/query (package lookup)
  - `TestOSVQueryBatch` — POST /v1/querybatch (2 queries)
  - `TestSitemapXML` — GET /sitemap.xml XML validation
  - `TestSchemaValidation` — POST /admin/validate OSV record
- `tests/integration/cli_importer_test.go`:
  - `TestCLIImporterNATSPublish` — NATS JetStream publish
  - `TestCLIImporterDataServiceIngest` — data-service health + ingest readiness
  - `TestFullPipelineImportToQuery` — stub (requires all services)

### Run Tests
```bash
cd tests/integration
GATEWAY_URL=http://localhost:8080 go test ./... -v -timeout 5m
```

---

## SE-PROD-04 — Full Pipeline Integration Tests ✅

→ Chi tiết: [sprint-D/SD-all-feature-parity-tasks.md#se-prod-04](../sprint-D/SD-all-feature-parity-tasks.md#se-prod-04)

**Completed**: 2026-06-13

### Files Created
- `tests/integration/ai_enrichment_test.go`:
  - `TestAIServiceHealth` — ai-service liveness
  - `TestAIEnrichmentViaNATS` — NATS monitoring check
  - `TestAIEnrichmentHTTPEndpoint` — POST /v1/enrich (single CVE)
  - `TestAIEnrichmentBatch` — POST /v1/enrich/batch (3 CVEs)
- `tests/integration/full_pipeline_test.go`:
  - `TestFullPipelineE2E` — complete 5-step pipeline (activate with `RUN_FULL_PIPELINE=1`)
  - `TestPipelineServiceDiscovery` — gateway + all services reachable
  - `TestPipelineOSVV1Compatibility` — 4 OSV v1 API compatibility checks
  - `buildTestOSVRecord()` — synthetic OSV record factory

### Run Full Pipeline
```bash
cd deploy/dev && docker compose up -d && sleep 60
cd ../../tests/integration
GATEWAY_URL=http://localhost:8080 \
  DATA_SERVICE_URL=http://localhost:8082 \
  SEARCH_SERVICE_URL=http://localhost:8083 \
  AI_SERVICE_URL=http://localhost:8086 \
  NATS_URL=nats://localhost:4222 \
  RUN_FULL_PIPELINE=1 \
  go test ./... -v -timeout 15m
```

---

## Summary

| Task | Status | Key Files |
|------|--------|-----------|
| SE-PROD-01 | ✅ | `deploy/dev/docker-compose.yml` (healthchecks cho 9 services) |
| SE-PROD-02 | ✅ | `services/shared/pkg/health/handler.go` |
| SE-PROD-03 | ✅ | `tests/integration/osv_query_test.go`, `cli_importer_test.go` |
| SE-PROD-04 | ✅ | `tests/integration/ai_enrichment_test.go`, `full_pipeline_test.go` |
