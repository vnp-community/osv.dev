# Tasks — enhance-cli-app

> **Nguồn**: `specs/solutions/enhance-cli-app/08_implementation-tasks.md`
> **Mục đích**: Chi tiết từng tác vụ để AI thực thi theo thứ tự ưu tiên

## 🎉 Trạng thái tổng thể: **HOÀN THÀNH** (2026-06-13)

Tất cả 5 sprints đã được thực thi và verified thành công.

---

## Cấu trúc

| Sprint | Nội dung | Priority | Status |
|--------|---------|---------|----|
| [sprint-A/](./sprint-A/) | Foundation — Shared event types, gRPC clients, OSV REST client, Service constructors | P0 | ✅ COMPLETED |
| [sprint-B/](./sprint-B/) | CLI Enhancement — NATS publisher, AI enricher, new commands | P1 | ✅ COMPLETED |
| [sprint-C/](./sprint-C/) | OSV Server Integration — Orchestrator, config, gateway routes | P1 | ✅ COMPLETED |
| [sprint-D/](./sprint-D/) | Feature Parity — Commit query, sitemap, metrics, validation | P2 | ✅ COMPLETED |
| [sprint-E/](./sprint-E/) | Production Readiness — Docker, health probes, integration tests | P3 | ✅ COMPLETED |

## Thứ tự thực thi

```
Sprint A → Sprint B → Sprint C → Sprint D → Sprint E
```

Sprint A phải hoàn thành trước (dependency cho tất cả sprints còn lại).

---

## Task IDs & Status

| Sprint | Task IDs | Status |
|--------|---------|--------|
| A | SA-SHARED-01, SA-SHARED-02, SA-SHARED-03, SA-SVC-01 | ✅ All done |
| B | SB-CLI-01, SB-CLI-02, SB-CLI-03, SB-CLI-04, SB-CLI-05, SB-CLI-06, SB-CLI-07 | ✅ All done |
| C | SC-OSV-01, SC-OSV-02, SC-GW-01 | ✅ All done |
| D | SD-FEAT-01, SD-FEAT-02, SD-FEAT-03, SD-FEAT-04, SD-FEAT-05 | ✅ All done |
| E | SE-PROD-01, SE-PROD-02, SE-PROD-03, SE-PROD-04 | ✅ All done |

---

## Key Deliverables

### apps/cli — New Commands
```bash
osv-query  -id CVE-2021-44228              # query by CVE ID
osv-query  -package lodash -ecosystem npm  # query by package
osv-query  -search "log4j remote code"     # full-text search
osv-scan   -target nginx:latest -type image # container scan
osv-enrich -cve CVE-2021-44228             # AI enrichment
```

### apps/osv — Microservices Mode
```bash
OSV_MODE=microservices \
  DATA_SERVICE_ADDR=localhost:50053 \
  SEARCH_SERVICE_HTTP=http://localhost:8083 \
  ./osv-server
```

### services/gateway-service — OSV v1 API
```
GET  /v1/vulns/{id}     → data-service gRPC
POST /v1/query          → data-service LookupCVEs
POST /v1/querybatch     → batch LookupCVEs
GET  /v1/search         → search-service HTTP proxy
GET  /sitemap.xml       → SEO sitemap
GET  /health            → liveness
GET  /ready             → readiness
GET  /metrics           → Prometheus (per service)
POST /admin/validate    → OSV schema validation
```

### Infrastructure
```bash
# Start full platform:
cd deploy/dev && docker compose up -d

# Run integration tests:
cd tests/integration
GATEWAY_URL=http://localhost:8080 go test ./... -v -timeout 5m

# Full pipeline test:
RUN_FULL_PIPELINE=1 go test ./... -run TestFullPipeline -v -timeout 15m
```

---

## Architecture Summary

```
CLI_BACKEND=microservices → apps/cli uses new microservices
CLI_BACKEND=legacy        → apps/cli uses original GCP pipeline (default)

OSV_MODE=microservices    → apps/osv runs Supervisor with all services
OSV_MODE=standalone       → apps/osv uses original GCP Datastore (default)
```

Nguyên tắc "chi thêm và mở rộng" được tuân thủ 100%:
- **Không xoá** bất kỳ file nào trong `apps/cli` hoặc `apps/osv`
- **Không merge** code mới vào file cũ (ngoại trừ `main.go` hook)
- Tất cả code mới trong **files riêng biệt** (backend_selector.go, ai_enricher.go, orchestrator_runner.go, v.v.)
