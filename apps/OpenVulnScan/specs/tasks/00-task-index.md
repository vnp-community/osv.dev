# OpenVulnScan — Task Index (v3 — Goroutine-Per-Service)

> Tổng hợp tất cả tác vụ thực thi từ giải pháp v3.  
> Kiến trúc mới: **mỗi service chạy trong goroutine riêng**, giao tiếp qua **gRPC bufconn**, **NATS JetStream**, hoặc **REST**.

## Quy ước

| Trạng thái | Ký hiệu |
|------------|---------|
| Chưa bắt đầu | `[ ]` |
| Đang thực hiện | `[/]` |
| Hoàn thành | `[x]` |
| Bị chặn | `[!]` |

---

## Danh sách tác vụ theo Phase

### Phase 0 — Foundation (goroutine infrastructure)
| Task | Status | File | Mô tả | Depends on |
|------|--------|------|--------|------------|
| T00 | ✅ | [task-00-goroutine-foundation.md](./task-00-goroutine-foundation.md) | ServiceRunner interface, Registry, bufconn transport | — |
| T01 | ✅ | [task-01-project-scaffold.md](./task-01-project-scaffold.md) | Cấu trúc thư mục, go.mod, go.work | T00 |
| T02 | ✅ | [task-02-infrastructure-setup.md](./task-02-infrastructure-setup.md) | Docker Compose, config, NATS JetStream setup | T01 |
| T03 | ✅ | [task-03-database-migrations.md](./task-03-database-migrations.md) | Merge migrations từ tất cả services (13 files) | T01 |

### Phase 1 — Service Goroutines (Tier 1 — No Dependencies)
| Task | Status | File | Mô tả | Depends on |
|------|--------|------|--------|------------|
| T04 | ✅ | [task-04-auth-runner.md](./task-04-auth-runner.md) | auth-service goroutine: Bridge Pattern + gRPC bufconn | T02, T03 |
| T05 | ✅ | [task-05-vuln-runner.md](./task-05-vuln-runner.md) | vulnerability-service goroutine: Bridge Pattern + MongoDB | T02, T03 |
| T06 | ✅ | [task-06-product-runner.md](./task-06-product-runner.md) | product-service goroutine: Bridge Pattern + gRPC bufconn | T02, T03 |
| T07 | ✅ | [task-07-query-runner.md](./task-07-query-runner.md) | query-service goroutine: Browse CPE handler + Redis L1 + MongoDB L2 | T02, T03 |

### Phase 2 — Service Goroutines (Tier 2 — Depend on Tier 1)
| Task | Status | File | Mô tả | Depends on |
|------|--------|------|--------|------------|
| T08 | ✅ | [task-08-finding-runner.md](./task-08-finding-runner.md) | finding-service goroutine: Bridge Pattern + gRPC bufconn | T04 |
| T09 | ✅ | [task-09-scan-runner.md](./task-09-scan-runner.md) | scan-service goroutine: Bridge Pattern + WorkerPool + HTTP | T08 |
| T10 | ✅ | [task-10-product-service-wiring.md](./task-10-product-service-wiring.md) | product-service wiring: HTTP REST + gRPC ProductService | T08, T06 |
| T11 | ✅ | [task-11-notify-runner.md](./task-11-notify-runner.md) | notification-service goroutine: NATS subscriber + Webhook dispatcher | T04, T08 |
| T12 | ✅ | [task-12-ingestion-agent-pipeline.md](./task-12-ingestion-agent-pipeline.md) | ingestion-service goroutine: OSV API enrichment + idempotency + findings | T05, T08 |

### Phase 3 — API Gateway
| Task | Status | File | Mô tả | Depends on |
|------|--------|------|--------|------------|
| T13 | ✅ | [task-13-api-gateway.md](./task-13-api-gateway.md) | API Gateway: JWT middleware, chi.Router, HTTP→gRPC handlers | T04–T12 |
| T14 | ✅ | [task-14-agent-endpoints.md](./task-14-agent-endpoints.md) | Agent REST endpoints: download + report submission + NATS publish | T09, T12 |

### Phase 4 — SIEM & Dashboard
| Task | Status | File | Mô tả | Depends on |
|------|--------|------|--------|------------|
| T15 | ✅ | [task-14-syslog-siem-adapter.md](./task-14-syslog-siem-adapter.md) | Syslog channel RFC 5424, SIEM config API routes | T11 |
| T16 | ✅ | [task-15-dashboard-routes.md](./task-15-dashboard-routes.md) | Dashboard routes: aggregated stats, timeline, top-CVEs, scan activity | T09, T13 |
| Bonus | ✅ | [task-12-report-service-wiring.md](./task-12-report-service-wiring.md) | ReportRunner: JSON/CSV/HTML report generation + download endpoints | T08, T10 |

### Phase 5 — Testing & Polish
| Task | Status | File | Mô tả | Depends on |
|------|--------|------|--------|------------|
| T17 | ✅ | [task-16-integration-tests.md](./task-16-integration-tests.md) | Integration tests: goroutine lifecycle, testutil helpers, HTTP endpoint tests | T00–T16 |
| T18 | ✅ | [task-17-security-hardening.md](./task-17-security-hardening.md) | Rate limiting (60 req/min), security headers, body size limit, audit log | T13 |
| T19 | ✅ | [task-18-docker-production.md](./task-18-docker-production.md) | Dockerfile multi-stage (alpine), docker-compose.prod.yml, .env.example | T01–T16 |
| T20 | ✅ | [task-19-openapi-docs.md](./task-19-openapi-docs.md) | OpenAPI 3.0.3 spec (40+ endpoints), Swagger UI embedded | T13–T16 |

---

## Tiến độ tổng thể

```
Phase 0 (Foundation)        [x][x][x][x]        4/4   ✅
Phase 1 (Tier 1 goroutines) [x][x][x][x]        4/4   ✅
Phase 2 (Tier 2 goroutines) [x][x][x][x][x]     5/5   ✅
Phase 3 (API Gateway)       [x][x]               2/2   ✅
Phase 4 (SIEM/Dashboard)    [x][x][x]            3/3   ✅
Phase 5 (Test/Polish)       [x][x][x][x]         4/4   ✅
──────────────────────────────────────────────────────
TOTAL                                            22/22 (100%) 🎉

Cập nhật: 2026-06-10 | Sprint 3 — ALL COMPLETE
```

---

## Dependency Graph

```
T00 (foundation)
  └── T01 (scaffold)
        └── T02 (infra) ──── T03 (migrations)
              │
    ┌─────────┴──────────────────────────────────┐
    ▼         ▼         ▼         ▼              │
   T04      T05       T06       T07              │
 (auth)    (vuln)  (product)  (query)            │
    │         │       │                          │
    └────┬────┘       │                          │
         ▼            │                          │
        T08 (finding) │                          │
         │   ─────────┘                          │
    ┌────┴──────────────────┐                    │
    ▼         ▼      ▼      ▼                    │
   T09       T10    T11    T12                   │
  (scan)  (report)(notify)(ingestion)            │
    │         │      │      │                    │
    └────┬────┴──────┴──────┘                    │
         ▼                                        │
        T13 (API Gateway) ←──────────────────────┘
         │
    ┌────┴──────┐
    ▼           ▼
   T14         T16
  (agent)   (dashboard)
    │
    ▼
   T15 (syslog)
    │
    ▼
  T17-T20 (test/polish)
```

---

## Tham chiếu giải pháp

| Solution doc | Nội dung |
|---|---|
| [00-overview.md](../solutions/00-overview.md) | Tổng quan kiến trúc goroutine-per-service |
| [01-communication-strategy.md](../solutions/01-communication-strategy.md) | gRPC bufconn, NATS, REST chi tiết |
| [02-project-structure.md](../solutions/02-project-structure.md) | Directory tree, go.mod, registry |
| [03-module-mapping.md](../solutions/03-module-mapping.md) | Package import paths từng service |
| [04-data-flow.md](../solutions/04-data-flow.md) | Luồng dữ liệu giữa goroutines |
| [05-infrastructure.md](../solutions/05-infrastructure.md) | Docker Compose, NATS JetStream |
| [06-migration-plan.md](../solutions/06-migration-plan.md) | Kế hoạch migration |
| [07-goroutine-registry.md](../solutions/07-goroutine-registry.md) | Registry + Runner pattern chi tiết |
