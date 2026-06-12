# OSV.dev — Core Services Architecture

> Kết quả sau quá trình merge: **17 active services + 45 archive services → 8 Core Services**
> Mỗi service tuân theo **Clean Architecture** với bounded context rõ ràng.

---

## Danh sách 8 Core Services

| # | Service | Bounded Context | File chi tiết |
|---|---------|----------------|---------------|
| 1 | [identity-service](./01_identity-service.md) | Identity & Access Management | [→](./01_identity-service.md) |
| 2 | [data-service](./02_data-service.md) | Vulnerability Data Management | [→](./02_data-service.md) |
| 3 | [search-service](./03_search-service.md) | Search & Discovery | [→](./03_search-service.md) |
| 4 | [scan-service](./04_scan-service.md) | Scanning Orchestration | [→](./04_scan-service.md) |
| 5 | [finding-service](./05_finding-service.md) | Findings & Remediation | [→](./05_finding-service.md) |
| 6 | [ai-service](./06_ai-service.md) | AI/ML Enrichment | [→](./06_ai-service.md) |
| 7 | [notification-service](./07_notification-service.md) | Notifications & Integrations | [→](./07_notification-service.md) |
| 8 | [gateway-service](./08_gateway-service.md) | API Gateway & BFF | [→](./08_gateway-service.md) |

---

## Merge Summary

```
17 active → 8 core services
────────────────────────────────────────────────────
auth-service                    → identity-service
vulnerability-service
  + ingestion-service           → data-service
search-service
  + query-service
  + dd-search                   → search-service
scan-service
  + schedule-service            → scan-service
finding-service
  + product-service
  + report-service              → finding-service
ai-service                      → ai-service (unchanged)
notification-service
  + integration-service         → notification-service
unified-gateway                 → gateway-service

shared/                         → shared/ (unchanged)
```

---

## Architecture Principles

### Clean Architecture Layers
```
cmd/              ← Entry point (main.go)
internal/
  domain/         ← Business rules & entities (NO external deps)
  usecase/        ← Application logic (orchestrates domain)
  delivery/       ← Transport layer (gRPC, HTTP)
  infra/          ← External systems (DB, cache, messaging)
migrations/       ← DB schema migrations
```

### Communication Patterns
- **Synchronous**: gRPC (internal service-to-service)
- **Asynchronous**: NATS JetStream (event-driven)
- **External**: HTTP REST via gateway-service

### Shared Libraries
- `github.com/osv/shared/pkg` — common utilities
- `github.com/osv/shared/proto` — gRPC contract definitions

---

## Service Communication Matrix

```
Internet
   │
   ▼
┌──────────────────────────────────────────────────────────┐
│                    gateway-service                       │
│  (Auth Proxy · Rate Limit · BFF · Health)                │
└──┬──────┬──────┬──────┬──────┬──────┬────────────────────┘
   │      │      │      │      │      │
   ▼      ▼      ▼      ▼      ▼      ▼
identity data  search  scan  finding  ai    notification
service  svc    svc    svc    svc    svc       svc
   │      │                   │      │           │
   │      └──NATS events──────┴──────┘           │
   │                                             │
   └─────────token validate (gRPC)───────────────┘
```

---

## Database Matrix

| Service | PostgreSQL | MongoDB | Redis | Firestore | NATS |
|---------|:----------:|:-------:|:-----:|:---------:|:----:|
| identity-service | ✅ | ✅ | ✅ | — | — |
| data-service | ✅ | ✅ | — | ✅ | ✅ pub |
| search-service | — | — | ✅ | — | ✅ sub |
| scan-service | ✅ | — | ✅ | — | ✅ pub/sub |
| finding-service | ✅ | ✅ | — | — | ✅ sub |
| ai-service | — | — | ✅ | ✅ | ✅ sub |
| notification-service | ✅ | — | — | — | ✅ sub |
| gateway-service | — | — | ✅ | — | — |

---

## Port Conventions

| Service | gRPC Port | HTTP Port |
|---------|:---------:|:---------:|
| identity-service | 50051 | 8081 |
| data-service | 50052 | 8082 |
| search-service | 50053 | 8083 |
| scan-service | 50054 | 8084 |
| finding-service | 50055 | 8085 |
| ai-service | 50056 | 8086 |
| notification-service | 50057 | 8087 |
| gateway-service | — | 8080 |
