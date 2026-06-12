# Migration Plan (v2) — Tối đa tái sử dụng services

## Nguyên tắc

- Mỗi phase: setup infra → wire-up services → test
- **Không viết business logic mới** — chỉ wire-up và test
- Thứ tự: auth → scan core → findings → assets/reports → notifications → dashboard

---

## Phase 1: Khung sườn, Infrastructure & Auth (Tuần 1)

### Mục tiêu
- Project setup với `go.mod replace`
- Kết nối DB, NATS, Redis
- Auth hoạt động đầy đủ

### Tasks

- [ ] Tạo `apps/OpenVulnScan/` với cấu trúc tối giản
- [ ] Viết `go.mod` với replace directives cho tất cả services
- [ ] Cập nhật `go.work` trong `services/`
- [ ] Viết `internal/app/app.go`:
  - [ ] Kết nối PostgreSQL (`shared/pkg/database`)
  - [ ] Kết nối NATS (`shared/pkg/nats`)
  - [ ] Kết nối Redis
- [ ] Chạy migrations (merge từ auth-service/migrations)
- [ ] Wire-up auth-service:
  - [ ] `login` usecase
  - [ ] `register` usecase
  - [ ] `validate_token` usecase → JWT middleware
  - [ ] `oauth` usecase (Google)
- [ ] Viết `internal/middleware/auth.go` (~30 LOC)
- [ ] Test: login, logout, token refresh, Google OAuth callback

### Deliverables
- `GET /healthz` → 200
- `POST /api/v1/auth/login` → JWT token
- `GET /api/v1/auth/google` → OAuth redirect

---

## Phase 2: Scan Core (Tuần 1-2)

### Mục tiêu
- Tạo, chạy, list, cancel scan
- Scheduled scan (cron)

### Tasks

- [ ] Chạy scan-service migrations
- [ ] Wire-up scan-service dependencies:
  - [ ] `scanrepo.NewScanRepo(db)`
  - [ ] `scanrepo.NewFindingRepo(db)`
  - [ ] `scancreate.New(...)` → CreateScanUseCase
  - [ ] `scanexecute.New(...)` → ExecuteScanUseCase
  - [ ] `scanworker.NewWorkerPool(...)` ← đã có sẵn pool.go
  - [ ] `scancron.CronWorker` ← đã có sẵn cron_worker.go
- [ ] Mount scan handler trong router:
  ```go
  r.Mount("/api/v1", scanhttp.NewRouter(scanHandler, log))
  ```
- [ ] Start goroutines:
  ```go
  go pool.Start(ctx)
  go cronWorker.Start(ctx)
  ```
- [ ] Wire-up agent usecase:
  - [ ] `scan-service/usecase/agent/submit_report`
  - [ ] `GET /agent/download` (render Python agent script)
  - [ ] `POST /agent/report`
- [ ] Test: full scan lifecycle (nmap), discovery scan, scheduled scan

### Deliverables
- `POST /api/v1/scans` → scan chạy async, trả về scan_id
- `GET /api/v1/scans/{id}` → scan status + findings
- `DELETE /api/v1/scans/{id}` → cancel
- Cron trigger scheduled scan

---

## Phase 3: Findings & CVE Enrichment (Tuần 2)

### Mục tiêu
- Finding processing sau scan
- CVE detail với CVSS, remediation

### Tasks

- [ ] Chạy finding-service + vulnerability-service migrations
- [ ] Wire-up finding-service:
  - [ ] `findinguc.NewBatchCreate(repo, nats)` → NATS subscriber
  - [ ] `findinguc.NewStatusTransition(repo, nats)`
  - [ ] `findinguc.NewBatchUpdateSLADates(repo, nats)`
  - [ ] NATS subscriber: `"scan.completed"` → batch create findings
- [ ] Wire-up vulnerability-service:
  - [ ] `vulnhttp.NewRouter()` → mount CVE routes
  - [ ] `lookupcves` usecase cho finding enrichment
- [ ] Start finding NATS subscriber goroutine
- [ ] Test: scan → findings appear → CVE details populated

### Deliverables
- `GET /api/v1/findings` → list với severity filter
- `PATCH /api/v1/findings/{id}/status` → state transition
- `GET /api/v1/cves/{id}` → CVE detail với CVSS

---

## Phase 4: Assets (Product), Agent Enrichment & PDF (Tuần 2-3)

### Mục tiêu
- Asset management với vulnerability tracking
- Agent package report + OSV enrichment
- PDF report

### Tasks

- [ ] Chạy product-service + ingestion-service migrations
- [ ] Wire-up product-service:
  - [ ] `producthttp.NewRouter()` → mount product/asset routes
  - [ ] Asset upsert từ `scan-service/usecase/asset/upsert_asset`
- [ ] Wire-up ingestion-service:
  - [ ] `ingestion-service/internal/fetcher` → OSV API client
  - [ ] `ingestion-service/internal/converter`
  - [ ] NATS subscriber: `"agent.report.submitted"` → OSV enrichment pipeline
- [ ] Wire-up report-service:
  - [ ] `reportuc.New(...)` → GenerateReport usecase
  - [ ] `GET /api/v1/scans/{id}/pdf`
- [ ] Test: agent Python script download, report submission, asset view, PDF download

### Deliverables
- `GET /api/v1/products` → assets with vulnerability count
- `POST /agent/report` → trigger OSV enrichment
- `GET /api/v1/scans/{id}/pdf` → PDF download

---

## Phase 5: Notification, SIEM & Dashboard (Tuần 3)

### Mục tiêu
- Email, Slack, Teams, Webhook notifications
- SIEM syslog forwarding
- Dashboard stats aggregation

### Tasks

- [ ] Chạy notification-service migrations
- [ ] Wire-up notification-service:
  - [ ] `email`, `slack`, `teams` channels từ `infra/channels/`
  - [ ] `http_dispatcher.go` → webhook channel
  - [ ] Dispatcher: register channels
  - [ ] NATS subscriber: `infra/messaging/nats/` → dispatch alerts
- [ ] Viết `internal/syslog/channel.go` (~60 LOC):
  - [ ] Implement notification channel interface
  - [ ] UDP/TCP syslog (RFC 5424)
  - [ ] Register: `dispatcher.Register("syslog", syslogCh)`
- [ ] SIEM config routes (lưu config vào DB, không cần service riêng)
- [ ] Dashboard routes trong `router/dashboard_routes.go` (~40 LOC):
  - [ ] Wire-up query-service `browse` + `rank` usecases
  - [ ] Wire-up `vulnUC.GetRecent()`
  - [ ] Wire-up `scanRepo.CountByStatus()`
- [ ] Test: email notification on scan complete, syslog forward, dashboard stats

### Deliverables
- Email on critical finding
- `GET /api/v1/dashboard` → stats JSON
- SIEM events in syslog

---

## Phase 6: Testing, Polish & Documentation (Tuần 4)

### Tasks

- [ ] Integration test: scan → finding → notify pipeline
- [ ] Load test: 10 concurrent scans
- [ ] Security review: JWT, input validation, CORS
- [ ] OpenAPI spec (`/api/v1/docs`)
- [ ] Update Docker Compose (prod-ready)
- [ ] Admin guide

### Deliverables
- Test coverage > 60%
- Docker image buildable
- `/api/v1/docs` Swagger UI

---

## Feature Parity Matrix (v2)

| Feature Python gốc                  | Go Monolith source                                      | Phase | Status |
|--------------------------------------|----------------------------------------------------------|-------|--------|
| Dashboard stats                      | `query-service` browse/rank + scanRepo                  | 5     | ✅     |
| Scan (full/discovery/web/agent)      | `scan-service` handler + worker + nmap/zap adapters     | 2     | ✅     |
| Schedule scan                        | `scan-service` cron_worker.go                           | 2     | ✅     |
| View scan detail                     | `scan-service` handler + `finding-service`              | 2+3   | ✅     |
| CVE detail (CVSS, remediation)       | `vulnerability-service` delivery/http                   | 3     | ✅     |
| PDF report                           | `report-service` generatereport usecase                 | 4     | ✅     |
| Asset listing + vuln count           | `product-service` delivery/http                         | 4     | ✅     |
| Tag management                       | `product-service` domain/product (tags)                 | 4     | ✅     |
| Agent download                       | `scan-service` agent handler + Python script render     | 4     | ✅     |
| Agent report (package → CVE)         | `ingestion-service` pipeline + `vulnerability-service`  | 4     | ✅     |
| SIEM syslog forwarding               | `notification-service` channel + `syslog/channel.go`   | 5     | ✅     |
| Email notification                   | `notification-service` infra/channels/email             | 5     | ✅     |
| Slack notification                   | `notification-service` infra/channels/slack             | 5     | ✅     |
| Teams notification                   | `notification-service` infra/channels/teams             | 5     | ✅     |
| Webhook notification                 | `notification-service` adapter/dispatcher               | 5     | ✅     |
| Local auth                           | `auth-service` usecase/login, register                  | 1     | ✅     |
| Google OAuth                         | `auth-service` usecase/oauth, provider/chain.go         | 1     | ✅     |
| User management                      | `auth-service` usecases                                 | 1     | ✅     |
| API key management                   | `auth-service` usecase/manage_api_key                   | 1     | ✅     |
| Finding SLA tracking                 | `finding-service` usecase/sla                           | 3     | ✅     |
| Finding audit log                    | `finding-service` usecase/audit                         | 3     | ✅     |
| KEV (Known Exploited Vulns)          | `vulnerability-service` usecase/kev                     | 3     | ✅     |

---

## Rủi ro và Mitigation (v2)

| Rủi ro                                       | Mức | Mitigation                                              |
|----------------------------------------------|-----|----------------------------------------------------------|
| `finding-service` module path khác (`defectdojo`) | 🔴 Cao | Kiểm tra go.mod của finding-service, set replace đúng |
| Services dùng khác nhau DB schema            | 🟡 Trung | Merge migrations thủ công, kiểm tra conflicts         |
| `notification-service` chưa có syslog channel | 🟢 Thấp | Thêm 1 file adapter nhỏ, implement interface hiện có  |
| `query-service` không có browse/rank cho scans | 🟡 Trung | Browse/rank có thể chỉ cho CVE — kiểm tra lại usecase |
| NATS subject naming không nhất quán          | 🟡 Trung | Define constants trong `shared/pkg`, dùng chung       |
| nmap cần quyền root                          | 🟡 Trung | Docker `--cap-add=NET_RAW` hoặc setuid nmap           |
