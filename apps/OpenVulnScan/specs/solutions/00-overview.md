# OpenVulnScan — Go Monolithic Application: Solution Overview (v3)

> **Revision v3**: Chuyển sang kiến trúc **goroutine-per-service**.  
> Mỗi service chạy trong goroutine riêng biệt. Giao tiếp qua **gRPC (bufconn)**, **REST nội bộ**, hoặc **NATS JetStream**.  
> Code từ `osv.dev/services/` được import nguyên vẹn — không chỉnh sửa.

---

## 1. Mục tiêu

Xây dựng **OpenVulnScan** thành một **Golang monolithic application** với kiến trúc:

- Mỗi service chạy như một **goroutine độc lập** với vòng đời riêng
- Giao tiếp qua **gRPC bufconn** (in-process, zero-network-overhead), **REST** (HTTP/S), hoặc **NATS JetStream**
- **Không thay đổi** code trong `services/`
- Graceful shutdown với `context.Context`

| Tính năng gốc (Python)                  | Service Go / Goroutine                          |
|------------------------------------------|-------------------------------------------------|
| Dashboard                                | `query-service` goroutine (gRPC bufconn)        |
| Scan (full / discovery / web / agent)   | `scan-service` goroutine (gRPC bufconn + NATS)  |
| Schedule Scan                            | `scan-service` CronWorker goroutine             |
| Asset Management                         | `product-service` goroutine (gRPC bufconn)      |
| Finding / CVE Detail                     | `finding-service` + `vulnerability-service` (gRPC bufconn) |
| PDF Report                               | `report-service` goroutine (gRPC bufconn)       |
| Agent                                    | `scan-service` agent goroutine (REST endpoint)  |
| SIEM / Syslog Forwarding                | `notification-service` + syslog adapter (NATS)  |
| User Management                          | `auth-service` goroutine (gRPC bufconn)         |
| Tag / Asset Management                   | `product-service` goroutine (gRPC bufconn)      |
| Notification (email/slack/teams/webhook) | `notification-service` goroutine (NATS)         |

---

## 2. Kiến trúc Goroutine-Per-Service

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    OpenVulnScan (Single Binary)                              │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                  main.go — Bootstrap & Orchestration                  │  │
│  │  • Load config → start goroutines → wait for signals                 │  │
│  └────────────────────────────┬──────────────────────────────────────────┘  │
│                               │                                              │
│  ┌────────────────────────────▼──────────────────────────────────────────┐  │
│  │            ServiceRegistry — Goroutine Lifecycle Manager              │  │
│  │  Start(ctx) │ Stop(name) │ StopAll() │ Health() │ Wait()             │  │
│  └────────────────────────────┬──────────────────────────────────────────┘  │
│                               │                                              │
│  ┌──────────┐ ┌──────────┐ ┌─┴────────┐ ┌──────────┐ ┌─────────────────┐  │
│  │goroutine │ │goroutine │ │goroutine │ │goroutine │ │   goroutine     │  │
│  │auth-svc  │ │scan-svc  │ │finding   │ │product   │ │ notification    │  │
│  │gRPC srv  │ │gRPC srv  │ │-service  │ │-service  │ │ -service        │  │
│  │bufconn   │ │bufconn   │ │gRPC srv  │ │gRPC srv  │ │ NATS subscriber │  │
│  │          │ │+WorkerPool│ │bufconn   │ │bufconn   │ │ + channels      │  │
│  └──────────┘ │+CronWorker│ └──────────┘ └──────────┘ └─────────────────┘  │
│               └──────────┘                                                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                       │
│  │goroutine │ │goroutine │ │goroutine │ │goroutine │                       │
│  │vuln-svc  │ │report-svc│ │query-svc │ │ingestion │                       │
│  │gRPC srv  │ │gRPC srv  │ │gRPC srv  │ │-service  │                       │
│  │bufconn   │ │bufconn   │ │bufconn   │ │NATS sub  │                       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘                       │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │             API Gateway Goroutine (HTTP :8080)                         │  │
│  │  chi.Router → JWT middleware → gRPC clients → service goroutines      │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │         Shared Infrastructure (kết nối 1 lần, chia sẻ qua DI)        │  │
│  │  PostgreSQL (pgxpool)  │  Redis  │  NATS JetStream  │  MinIO          │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Giao tiếp giữa các Service Goroutines

### 3.1 gRPC bufconn (In-Process) — Ưu tiên số 1

Dùng `google.golang.org/grpc/test/bufconn` cho giao tiếp đồng bộ giữa goroutines **trong cùng process**:

```
API Gateway ──gRPC client──► auth-service goroutine (bufconn :authLis)
API Gateway ──gRPC client──► scan-service goroutine (bufconn :scanLis)
API Gateway ──gRPC client──► finding-service goroutine (bufconn :findingLis)
scan-service ─gRPC client──► finding-service goroutine (bufconn)
scan-service ─gRPC client──► product-service goroutine (bufconn)
```

**Ưu điểm**: Zero network overhead, type-safe, bidirectional streaming, full gRPC features.

### 3.2 NATS JetStream — Async Events

Dùng cho các luồng bất đồng bộ và fan-out:

```
scan-service → NATS: scan.completed
    ├── finding-service subscriber → batch create findings
    └── notification-service subscriber → alert rules → email/slack/syslog

finding-service → NATS: finding.created
    └── notification-service → rule check → alert

agent → NATS: agent.report.submitted
    └── ingestion-service → OSV enrichment
```

### 3.3 REST (HTTP Internal) — Fallback

Dùng khi service chưa có gRPC proto hoặc cần expose endpoint ra ngoài:

```
External Agent ──HTTP POST──► /agent/report endpoint
External Client ──HTTP GET──► /api/v1/* endpoints (API Gateway)
```

---

## 4. Danh sách Goroutines

| Goroutine Name | Service | Protocol | bufconn Listener |
|---|---|---|---|
| `auth-svc` | auth-service | gRPC bufconn | `authLis` |
| `scan-svc` | scan-service | gRPC bufconn + NATS | `scanLis` |
| `finding-svc` | finding-service | gRPC bufconn + NATS | `findingLis` |
| `product-svc` | product-service | gRPC bufconn | `productLis` |
| `vuln-svc` | vulnerability-service | gRPC bufconn | `vulnLis` |
| `report-svc` | report-service | gRPC bufconn | `reportLis` |
| `query-svc` | query-service | gRPC bufconn | `queryLis` |
| `notify-svc` | notification-service | NATS subscriber | — |
| `ingestion-svc` | ingestion-service | NATS subscriber | — |
| `scan-worker-pool` | scan-service/worker | internal channels | — |
| `scan-cron` | scan-service/scheduler | ticker | — |
| `api-gateway` | internal/gateway | HTTP :8080 | — |

---

## 5. Startup Sequence

```
1. Load config
2. Connect shared infra: PostgreSQL pool, NATS, Redis, MinIO
3. Run DB migrations
4. Create bufconn listeners (một per service)
5. Start service goroutines (theo dependency order):
   a. auth-svc        (no deps)
   b. vuln-svc        (no deps)
   c. product-svc     (no deps)
   d. query-svc       (deps: DB)
   e. finding-svc     (deps: product-svc)
   f. scan-svc        (deps: finding-svc, product-svc)
   g. report-svc      (deps: finding-svc, product-svc)
   h. notify-svc      (deps: NATS)
   i. ingestion-svc   (deps: NATS, vuln-svc)
   j. scan-worker-pool (deps: scan-svc)
   k. scan-cron       (deps: scan-svc)
   l. api-gateway     (deps: all — last)
6. Wait for SIGINT/SIGTERM → graceful shutdown
```

---

## 6. Code mới cần viết (tối thiểu)

| File | LOC | Mục đích |
|---|---|---|
| `cmd/server/main.go` | ~120 | Bootstrap, signal handling |
| `internal/app/app.go` | ~200 | ServiceRegistry, wire-up tất cả goroutines |
| `internal/app/registry.go` | ~100 | Goroutine lifecycle manager |
| `internal/gateway/router.go` | ~100 | HTTP API gateway, mount all routes |
| `internal/gateway/middleware.go` | ~40 | JWT middleware dùng auth-service gRPC |
| `internal/transport/bufconn.go` | ~50 | bufconn Dialer factory |
| `internal/syslog/channel.go` | ~60 | SIEM syslog channel adapter |
| `configs/config.yaml` | ~60 | Config |
| `docker-compose.yml` | ~80 | Infrastructure |
| `go.mod` | ~35 | Module + replace directives |
| **Tổng** | **~845 LOC** | **Toàn bộ business logic từ services** |

---

## 7. Tham chiếu chi tiết

| Doc | Nội dung |
|---|---|
| [01-communication-strategy.md](./01-communication-strategy.md) | gRPC bufconn, NATS, REST patterns chi tiết |
| [02-project-structure.md](./02-project-structure.md) | Directory tree, go.mod |
| [03-module-mapping.md](./03-module-mapping.md) | Service → package import paths |
| [04-data-flow.md](./04-data-flow.md) | Luồng dữ liệu giữa goroutines |
| [05-infrastructure.md](./05-infrastructure.md) | Docker Compose, Dockerfile |
| [06-migration-plan.md](./06-migration-plan.md) | Kế hoạch migration |
| [07-goroutine-registry.md](./07-goroutine-registry.md) | ServiceRegistry implementation |
