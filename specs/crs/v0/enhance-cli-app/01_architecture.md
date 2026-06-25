# 01 — Kiến trúc tổng thể (Architecture)

> **Căn cứ**: PRD.md, SRS.md, URD.md + codebase audit tại `services/`

---

## 1. Bức tranh toàn cảnh

```
                        ┌─────────────────────────────────────────────┐
                        │            apps/osv/cmd/server               │
                        │         (Single binary, multiple goroutines) │
                        │                                              │
                        │  ┌──────────────┐   ┌───────────────────┐   │
                        │  │ gateway-svc  │   │   identity-svc    │   │
                        │  │ :8080/:9090  │   │   :8081/:50051    │   │
                        │  └──────┬───────┘   └───────────────────┘   │
                        │         │                                     │
                        │  ┌──────▼───────┐   ┌───────────────────┐   │
                        │  │  data-svc    │   │    search-svc     │   │
                        │  │ :8082/:50053 │◄──│   :8083/:50056    │   │
                        │  └──────┬───────┘   └───────────────────┘   │
                        │         │                                     │
                        │  ┌──────▼───────┐   ┌───────────────────┐   │
                        │  │   ai-svc     │   │  notification-svc  │   │
                        │  │ :8086/:50052 │   │     :8084         │   │
                        │  └──────────────┘   └───────────────────┘   │
                        │                                              │
                        │  ┌──────────────┐   ┌───────────────────┐   │
                        │  │ finding-svc  │   │    scan-svc       │   │
                        │  │ :8085/:50060 │   │    :8087          │   │
                        │  └──────────────┘   └───────────────────┘   │
                        │                                              │
                        │  ┌──────────────────────────────────────┐   │
                        │  │           NATS JetStream              │   │
                        │  │  (embedded nats-server or external)  │   │
                        │  └──────────────────────────────────────┘   │
                        └─────────────────────────────────────────────┘

                        ┌─────────────────────────────────────────────┐
                        │          apps/cli (independent commands)     │
                        │                                              │
                        │  cmd/importer  ───NATS/HTTP──► data-svc     │
                        │  cmd/worker    ───gRPC───────► data-svc     │
                        │  cmd/exporter  ───REST───────► data-svc     │
                        │  cmd/relations ───gRPC───────► data-svc     │
                        │  cmd/scan      ───gRPC───────► scan-svc     │
                        │  cmd/query     ───REST───────► gateway-svc  │
                        │  cmd/enrich    ───gRPC───────► ai-svc       │
                        └─────────────────────────────────────────────┘
```

---

## 2. Service Port Map

| Service | Module | HTTP Port | gRPC Port | NATS Subjects |
|---------|--------|-----------|-----------|---------------|
| gateway-service | `github.com/osv/gateway-service` | 8080 | 9090 | — |
| identity-service | `github.com/osv/identity-service` | 8081 | 50051 | — |
| data-service | `github.com/osv/data-service` | 8082 | 50053 | `osv.vuln.*` |
| search-service | `github.com/osv/search-service` | 8083 | 50056 | `osv.search.*` |
| notification-service | `github.com/osv/notification-service` | 8084 | — | `osv.notif.*` |
| finding-service | `github.com/osv/finding-service` | 8085 | 50060 | `defectdojo.*` |
| ai-service | `github.com/osv/ai-service` | 8086 | 50052 | `osv.ai.*` |
| scan-service | `github.com/osv/scan-service` | 8087 | 50055 | `osv.scan.*` |

---

## 3. Giao thức giao tiếp

### 3.1 gRPC (inter-service, latency-sensitive)
- CLI → data-service: `VulnerabilityService`, `AliasService`
- CLI → ai-service: `AIEnrichmentService`
- CLI → scan-service: `ScanService`
- gateway → tất cả services: proxy + BFF aggregation

### 3.2 REST/HTTP (external-facing, human tools)
- CLI `cmd/exporter` → `data-service` `/api/v1/vulns` (bulk download)
- CLI `cmd/query` → `gateway-service` `/v1/vulns/{id}`
- `apps/osv` internal: services call nhau qua REST cho non-critical flows

### 3.3 NATS JetStream (async events, decoupled)
- `cmd/importer` publish `osv.vuln.imported` → data-service consumes
- data-service publish `osv.vuln.updated` → ai-service, search-service consume
- ai-service publish `osv.ai.enrichment.completed` → search-service (reindex)
- finding-service publish `defectdojo.finding.created` → notification-service

---

## 4. Environment Configuration

```bash
# apps/osv/cmd/server — phải set trước khi chạy

# Backend selector
BACKEND=microservices              # "microservices" | "gcp" (default: gcp giữ nguyên)

# NATS
NATS_URL=nats://localhost:4222
NATS_EMBEDDED=true                 # Chạy NATS embedded trong process

# Service ports (khi chạy embedded)
GATEWAY_HTTP_PORT=8080
GATEWAY_GRPC_PORT=9090
DATA_SERVICE_HTTP=8082
DATA_SERVICE_GRPC=50053
SEARCH_SERVICE_HTTP=8083
SEARCH_SERVICE_GRPC=50056
AI_SERVICE_HTTP=8086
AI_SERVICE_GRPC=50052
FINDING_SERVICE_HTTP=8085
FINDING_SERVICE_GRPC=50060
IDENTITY_SERVICE_HTTP=8081
IDENTITY_SERVICE_GRPC=50051
NOTIFICATION_SERVICE_HTTP=8084
SCAN_SERVICE_HTTP=8087

# External mode (khi dùng distributed)
DATA_SERVICE_ADDR=data-service:50053
SEARCH_SERVICE_ADDR=search-service:50056
AI_SERVICE_ADDR=ai-service:50052
GATEWAY_ADDR=gateway-service:8080

# Storage
MONGO_URI=mongodb://localhost:27017
POSTGRES_DSN=postgres://...
FIRESTORE_PROJECT=...

# Auth
JWT_SECRET=...
REDIS_URL=redis://localhost:6379
```

---

## 5. Dependency Graph (Apps → Services)

```
apps/cli:
  + add: replace github.com/osv/shared/proto => ../../services/shared/proto
  + add: replace github.com/osv/gateway-service => (for client types)

apps/osv:
  + add: replace github.com/osv/data-service => ../../services/data-service
  + add: replace github.com/osv/search-service => ../../services/search-service
  + add: replace github.com/osv/ai-service => ../../services/ai-service
  + add: replace github.com/osv/finding-service => ../../services/finding-service
  + add: replace github.com/osv/identity-service => ../../services/identity-service
  + add: replace github.com/osv/notification-service => ../../services/notification-service
  + add: replace github.com/osv/scan-service => ../../services/scan-service
  + add: replace github.com/osv/gateway-service => ../../services/gateway-service
  + add: replace github.com/osv/shared/proto => ../../services/shared/proto
```

---

## 6. Python OSV Library — Mapping sang Go Services

| Python (`osv/`) | Go Service tương đương |
|-----------------|----------------------|
| `bug.py` — BugStatus, normalize_tag | `data-service/domain/vuln/` |
| `ecosystems/` — PyPI, Maven, npm... | `search-service/internal/usecase/osv_query/` |
| `impact.py` — commit bisection | `data-service/internal/usecase/ingest/` |
| `models.py` — Vulnerability datastore | `data-service/infra/persistence/mongo/` |
| `sources.py` — source repo management | `data-service/internal/fetcher/` |
| `gcs.py` — GCS bucket operations | `data-service/infra/storage/gcs/` |
| `pubsub.py` — GCP Pub/Sub | NATS JetStream (`shared/pkg/nats/`) |
| `repos.py` — git repo management | `data-service/infra/git/` |
| `ecosystems/alpine.py` etc | `search-service` ecosystem adapters |
