# Module Mapping: Service → Monolith (v2)

## Nguyên tắc

> **Không tạo module mới.** Tất cả tính năng đều được phục vụ bởi code từ services hiện có.  
> "Module" ở đây chỉ là cách tổ chức route/handler trong `router.go`, không phải package mới.

---

## 1. `auth-service` → Auth & User Routes

### Thành phần import

| Package path                                          | Dùng cho                                 |
|-------------------------------------------------------|------------------------------------------|
| `auth-service/internal/usecase/login/`                | POST /api/v1/auth/login                  |
| `auth-service/internal/usecase/register/`             | POST /api/v1/auth/register               |
| `auth-service/internal/usecase/logout/`               | POST /api/v1/auth/logout                 |
| `auth-service/internal/usecase/refresh_token/`        | POST /api/v1/auth/refresh                |
| `auth-service/internal/usecase/manage_api_key/`       | POST/DELETE /api/v1/auth/api-keys        |
| `auth-service/internal/usecase/oauth/`                | GET /api/v1/auth/google + callback       |
| `auth-service/internal/usecase/validate_token/`       | Middleware JWT validation                |
| `auth-service/internal/provider/` (chain.go, local.go)| Auth provider chain (local + OAuth)     |
| `auth-service/internal/infra/auth/`                   | JWT signer, Redis session store          |

**Routes:**
```
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
POST   /api/v1/auth/register
POST   /api/v1/auth/refresh
GET    /api/v1/auth/google
GET    /api/v1/auth/google/callback
POST   /api/v1/auth/api-keys
DELETE /api/v1/auth/api-keys/{key}
```

---

## 2. `scan-service` → Scan, Agent, Schedule Routes

### Thành phần import

| Package path                                                  | Dùng cho                                     |
|---------------------------------------------------------------|----------------------------------------------|
| `scan-service/internal/adapters/handler/http/scan_handler.go` | HTTP handler — đã có tất cả CRUD             |
| `scan-service/internal/adapters/handler/grpc/scan_grpc_handler.go` | gRPC (optional, cho external clients)  |
| `scan-service/internal/usecase/create_scan/create_scan.go`   | CreateScan business logic                    |
| `scan-service/internal/usecase/execute_scan/execute_scan.go` | ExecuteScan (nmap + zap)                     |
| `scan-service/internal/usecase/agent/submit_report/`         | Agent report submission                      |
| `scan-service/internal/usecase/asset/upsert_asset/`          | Asset upsert sau khi scan xong               |
| `scan-service/internal/usecase/scan/create_scan/`            | Scan creation (nested usecases)              |
| `scan-service/internal/adapters/worker/pool.go`              | WorkerPool (goroutine pool)                  |
| `scan-service/internal/adapters/scanner/nmap/nmap_scanner.go`| Nmap scan executor                           |
| `scan-service/internal/adapters/scanner/zap/`                | OWASP ZAP scan executor                      |
| `scan-service/internal/adapters/repository/postgres/`        | PostgreSQL repos (scan, finding, asset)      |
| `scan-service/internal/adapters/repository/redis/`           | Redis cache (scan status)                    |
| `scan-service/internal/scheduler/cron_worker.go`             | Scheduled scan cron                          |
| `scan-service/internal/domain/scan/entity/scan.go`           | Entities: Scan, Finding, Port, Service, WebAlert |
| `scan-service/internal/domain/scan/entity/scan.go`           | ScanStatus state machine                     |

**Routes (mount nguyên `scanhttp.NewRouter()`):**
```
POST   /api/v1/scans                     → ScanHandler.CreateScan
GET    /api/v1/scans                     → ScanHandler.ListScans
GET    /api/v1/scans/{id}                → ScanHandler.GetScan
DELETE /api/v1/scans/{id}                → ScanHandler.CancelScan
GET    /api/v1/scans/{id}/findings       → ScanHandler.GetFindings
POST   /api/v1/scans/{id}/schedule       → CronWorker (via DB + cron)
GET    /api/v1/scans/scheduled           → scanRepo.ListScheduled
DELETE /api/v1/scans/scheduled/{id}      → scanRepo.DeleteScheduled
GET    /agent/download                   → agentDownloadHandler (render Python script)
POST   /agent/report                     → agent submit_report usecase
GET    /api/v1/agent/reports             → scanRepo.ListAgentReports
GET    /api/v1/agent/reports/{id}        → scanRepo.GetAgentReport
```

**Background workers:**
```
WorkerPool.Start(ctx)      → goroutine pool executing scans
CronWorker.Start(ctx)      → cron_worker.go trigger scheduled scans
```

---

## 3. `finding-service` → Finding Routes

### Thành phần import

| Package path                                                   | Dùng cho                              |
|----------------------------------------------------------------|---------------------------------------|
| `finding-service/internal/usecase/finding/use_cases.go`       | BatchCreate, CloseOld, StatusTransition, SLA |
| `finding-service/internal/usecase/audit/`                     | Audit log recording                   |
| `finding-service/internal/usecase/sla/`                       | SLA deadline computation              |
| `finding-service/internal/domain/finding/entity.go`           | Finding entity + state machine        |
| `finding-service/internal/domain/finding/state_machine.go`    | State transitions (Open→Mitigated etc)|
| `finding-service/internal/delivery/grpc/`                     | gRPC handler (wrap thành HTTP)        |
| `finding-service/internal/infra/postgres/`                    | Repository                            |
| `finding-service/internal/infra/messaging/`                   | NATS subscriber (batch_created events)|

**Routes:**
```
GET    /api/v1/findings                  → listFindings (filter by scan, severity, status)
GET    /api/v1/findings/{id}             → getFinding (chi tiết + CVE enrichment)
PATCH  /api/v1/findings/{id}/status      → StatusTransitionUseCase
POST   /api/v1/findings/{id}/accept-risk → AcceptRisk
POST   /api/v1/findings/{id}/false-positive → MarkFalsePositive
GET    /api/v1/findings/summary          → stats by severity (dùng repo query)
```

**Background worker:**
```
FindingNATSSubscriber: Nhận "scan.completed" → BatchCreateFindingsUseCase
                       Nhận "defectdojo.finding.batch_created" → SLA compute
```

---

## 4. `product-service` → Asset & Tag Routes

### Thành phần import

| Package path                                              | Dùng cho                                |
|-----------------------------------------------------------|-----------------------------------------|
| `product-service/internal/domain/product/`               | Product entity, tag management          |
| `product-service/internal/domain/engagement/`            | Engagement (tương đương "Asset scan session") |
| `product-service/internal/usecase/product/`              | Product CRUD                            |
| `product-service/internal/usecase/engagement/`           | Engagement CRUD                         |
| `product-service/internal/delivery/http/`                | HTTP handler — mount nguyên             |
| `product-service/internal/delivery/grpc/`                | gRPC handler (optional)                 |
| `product-service/internal/infra/`                        | Repository                              |

**Routes (mount `producthttp.NewRouter()`):**
```
GET    /api/v1/products                  → ListProducts (= Assets)
GET    /api/v1/products/{id}             → GetProduct (with vulnerability count)
POST   /api/v1/products                  → CreateProduct
PUT    /api/v1/products/{id}             → UpdateProduct
DELETE /api/v1/products/{id}             → DeleteProduct
POST   /api/v1/products/{id}/tags        → AddTag
DELETE /api/v1/products/{id}/tags/{tag}  → RemoveTag
GET    /api/v1/tags                      → ListTags
POST   /api/v1/engagements               → CreateEngagement (= scan session)
GET    /api/v1/engagements/{id}          → GetEngagement
```

---

## 5. `vulnerability-service` → CVE Routes

### Thành phần import

| Package path                                                  | Dùng cho                              |
|---------------------------------------------------------------|---------------------------------------|
| `vulnerability-service/internal/usecase/cve/`                | CVE lookup by ID                      |
| `vulnerability-service/internal/usecase/lookupcves/`         | Batch CVE lookup (từ finding list)    |
| `vulnerability-service/internal/usecase/searchbycpe/`        | Search by CPE (asset enrichment)      |
| `vulnerability-service/internal/usecase/getrecent/`          | Recent CVEs (dashboard)               |
| `vulnerability-service/internal/usecase/kev/`                | KEV (Known Exploited Vulnerabilities) |
| `vulnerability-service/internal/delivery/http/`              | HTTP handler                          |
| `vulnerability-service/internal/delivery/grpc/`              | gRPC handler                          |
| `vulnerability-service/internal/usecase/query/`              | Full-text search                      |

**Routes (mount `vulnhttp.NewRouter()`):**
```
GET /api/v1/cves/{id}                    → GetCVE (CVSS, KEV, remediation)
GET /api/v1/cves                         → SearchCVEs (query, filter)
GET /api/v1/cves/recent                  → GetRecentCVEs
GET /api/v1/cves/kev                     → GetKEVList
GET /api/v1/cves/by-cpe                  → SearchByCPE
```

**Tích hợp nội bộ:**
- `FindingWorker` gọi `lookupcves` để enrich findings sau khi scan
- Dashboard gọi `getrecent` để hiển thị CVE mới nhất

---

## 6. `report-service` → PDF Report Routes

### Thành phần import

| Package path                                                  | Dùng cho                            |
|---------------------------------------------------------------|-------------------------------------|
| `report-service/internal/usecase/generatereport/`            | PDF generation                      |
| `report-service/internal/usecase/generateavailablefix/`      | Available fixes summary             |
| `report-service/internal/domain/`                            | Report entity                       |
| `report-service/internal/formatters/`                        | HTML/PDF formatter                  |
| `report-service/internal/infrastructure/`                    | Storage (MinIO/S3)                   |
| `report-service/adapter/`                                    | Data fetcher (scan + findings)      |

**Routes:**
```
GET /api/v1/scans/{id}/pdf               → GenerateReportUseCase → FileResponse
GET /api/v1/scans/{id}/fixes             → GenerateAvailableFixUseCase
```

---

## 7. `notification-service` → Notification + SIEM Routes

### Thành phần import

| Package path                                                     | Dùng cho                              |
|------------------------------------------------------------------|---------------------------------------|
| `notification-service/internal/domain/rule/entity.go`           | NotificationRule (events → channels)  |
| `notification-service/internal/domain/alert/entity.go`          | Alert entity                          |
| `notification-service/internal/infra/channels/email/`           | Email delivery                        |
| `notification-service/internal/infra/channels/slack/`           | Slack delivery                        |
| `notification-service/internal/infra/channels/teams/`           | Teams delivery                        |
| `notification-service/internal/adapter/dispatcher/http_dispatcher.go` | Webhook delivery              |
| `notification-service/internal/infra/messaging/nats/`           | NATS subscriber                       |
| `notification-service/internal/infra/persistence/`              | Notification rules repo               |
| `notification-service/internal/usecase/dispatch_alert/`         | Dispatch alert                        |
| `notification-service/internal/usecase/dispatch_webhook/`       | Dispatch webhook                      |
| `notification-service/internal/usecase/manage_subscription/`    | Subscription CRUD                     |
| **`internal/syslog/channel.go` (file mới ~60 LOC)**             | **Syslog adapter (SIEM)**             |

**Routes:**
```
GET  /api/v1/notifications              → List delivery records
PUT  /api/v1/notifications/rules        → Update notification rules
GET  /api/v1/notifications/rules        → Get notification rules
POST /api/v1/siem/config                → Save syslog endpoint config
GET  /api/v1/siem/config                → Get syslog config
POST /api/v1/siem/test                  → Test syslog connectivity
```

**Background worker:**
```
NotifyNATSSubscriber.Start(ctx)         → Listen NATS, dispatch via configured channels
                                        → Channels: email, slack, teams, webhook, syslog
```

---

## 8. `query-service` → Dashboard Routes

### Thành phần import

| Package path                                              | Dùng cho                                    |
|-----------------------------------------------------------|---------------------------------------------|
| `query-service/internal/usecase/browse/`                  | Browse/filter CVEs và findings              |
| `query-service/internal/usecase/rank/`                    | Rank by severity (top CVEs)                 |
| `query-service/internal/usecase/lookup/`                  | Lookup details                              |
| `query-service/internal/infra/`                           | Repository                                  |

**Routes (không có router riêng — compose trong `dashboard_routes.go`):**
```
GET /api/v1/dashboard                   → Aggregate stats:
                                          - scanRepo.CountByStatus()
                                          - browseUC.Execute(filter{severity:critical})
                                          - rankUC.Execute(limit:10)
                                          - vulnUC.GetRecent(limit:5)
GET /api/v1/dashboard/timeline          → scanRepo.ListRecent(limit:30)
GET /api/v1/dashboard/top-vulnerabilities → rankUC.Execute(by:cvss_score)
```

---

## 9. `ingestion-service` → Agent Enrichment (Background)

### Thành phần import

| Package path                                            | Dùng cho                                  |
|---------------------------------------------------------|-------------------------------------------|
| `ingestion-service/internal/pipeline/`                  | OSV/NVD enrichment pipeline               |
| `ingestion-service/internal/domain/`                   | CVE ingestion entity                      |
| `ingestion-service/internal/usecase/sync/`              | Sync CVE data                             |
| `ingestion-service/internal/fetcher/`                   | OSV API fetcher (dùng cho agent reports)  |
| `ingestion-service/internal/converter/`                 | Format converter (OSV → internal)         |

**Không có HTTP routes** — chỉ chạy như background pipeline.

**Background worker:**
```
AgentNATSWorker: Subscribe "agent.report.submitted"
    → ingestion-service fetcher (query OSV API per package)
    → converter (OSV → CVE entity)
    → vulnerability-service repo (persist CVEs)
    → finding-service (create findings for agent scan)
```

---

## 10. Dependency Graph tổng thể

```
HTTP Router
    │
    ├── /api/v1/auth/*     → auth-service usecases
    ├── /api/v1/scans/*    → scan-service HTTP handler (nguyên)
    ├── /api/v1/findings/* → finding-service delivery (wrap)
    ├── /api/v1/products/* → product-service HTTP handler (nguyên)
    ├── /api/v1/cves/*     → vulnerability-service HTTP handler (nguyên)
    ├── /api/v1/reports/*  → report-service usecase
    ├── /api/v1/notif/*    → notification-service usecases
    ├── /api/v1/dashboard  → query-service usecases (compose)
    └── /agent/*           → scan-service agent usecases

Background Goroutines
    ├── scan-service:         WorkerPool + CronWorker
    ├── finding-service:      NATS subscriber (scan.completed)
    ├── notification-service: NATS subscriber (→ email/slack/teams/webhook/syslog)
    └── ingestion-service:    NATS subscriber (agent.report.submitted → OSV)

Shared Infrastructure (1 instance, dùng chung)
    ├── PostgreSQL  (shared/pkg/database)
    ├── Redis       (shared/pkg hoặc service-specific)
    ├── NATS        (shared/pkg/nats)
    └── MinIO       (report-service/infrastructure)
```
