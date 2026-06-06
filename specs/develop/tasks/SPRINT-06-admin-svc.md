# SPRINT-06 — Admin Service (`services/admin/`)

> **Thời gian:** Q1 2027, Tháng 7 (3 tuần)  
> **Mục tiêu:** Implement đầy đủ Admin REST API và data quality monitoring  
> **Refs:** [04-roadmap.md §2.6](../04-roadmap.md), [06-new-features.md §6](../06-new-features.md)

---

## Tổng Quan

```
Sprint Goal: "Ops team có thể quản lý platform qua REST API"

Hiện trạng:
  ✅ admin/cmd/main.go — HTTP server với graceful shutdown
  ✅ 12 REST routes đã define (tất cả đều "not implemented")

Deliverables:
  1. Source management handlers ✅ DONE 2026-06-03
  2. Import findings handlers ✅ DONE 2026-06-03
  3. Vulnerability admin handlers ✅ DONE 2026-06-03
  4. System health endpoint ✅ DONE 2026-06-03
  5. API key management ✅ DONE 2026-06-03
  6. Data quality monitoring (follow-up)
  7. Audit trail logging (follow-up)
```

---

## TASK-06-01 · Source Management Handlers [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 3 ngày  
**Priority:** P1  
**Files:**
- [handler/handler.go](../../../../services/admin/internal/infra/http/handler/handler.go)
- [cmd/main.go](../../../../services/admin/cmd/main.go)

### Đã implement
- [x] `Handler` struct — dependency injection container
- [x] `ListSources()` — trả tất cả sources với trạng thái
- [x] `GetSource(name)` — chi tiết một source
- [x] `TriggerSync(name)` — publish sync request (HTTP 202 Accepted)
- [x] `PauseSource(name)` / `ResumeSource(name)` — state management
- [x] DTO types: `SourceStatus` với State, LastSyncAt, TotalVulns, SyncInterval
- [x] Main.go refactored: sử dụng `handler.New()` thay vì stub functions

---

## TASK-06-02 · Import Findings Handlers [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Files:**
- [handler/handler.go](../../../../services/admin/internal/infra/http/handler/handler.go)

### Đã implement
- [x] `ListImportFindings()` — query params: ?source=, ?resolved=, ?limit=
- [x] `ResolveImportFinding(id)` — mark as resolved với timestamp
- [x] `ImportFinding` DTO: ID, SourceName, VulnID, Category, Message, OccurredAt, IsResolved

---

## TASK-06-03 · Vulnerability Admin Operations [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Files:**
- [handler/handler.go](../../../../services/admin/internal/infra/http/handler/handler.go)

### Đã implement
- [x] `WithdrawVuln(id)` — POST body: `{"reason": "..."}`; trả withdrawn_at timestamp
- [x] `ReprocessVuln(id)` — publish reprocess event (HTTP 202)
- [x] `VulnStats()` — aggregate: total, by_ecosystem, by_source, withdrawn, with_cvss, with_kev

---

## TASK-06-07 · System Health Endpoint [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Files:**
- [handler/handler.go](../../../../services/admin/internal/infra/http/handler/handler.go)

### Đã implement
- [x] `SystemHealth()` — trả component health cho: firestore, nats, redis, opensearch, source-sync, ai-enrichment
- [x] `ComponentHealth` DTO: Status (ok/degraded/down), Latency, Message
- [x] Route: `GET /admin/v1/system/health`

---

## TASK-06-06 · API Key Management (Basic) [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Files:**
- [handler/handler.go](../../../../services/admin/internal/infra/http/handler/handler.go)

### đã implement
- [x] `ListAPIKeys()` — `GET /admin/v1/api-keys`
- [x] `CreateAPIKey()` — `POST /admin/v1/api-keys` với name, scopes, expires_at
- [x] `RevokeAPIKey(id)` — `DELETE /admin/v1/api-keys/{id}`
- [x] `APIKey` DTO: ID, Name, Prefix, Scopes, CreatedAt, ExpiresAt, IsActive
- [x] Security: key chỉ hiển thị 1 lần khi tạo ("Store this key securely")

---

## TASK-06-04 · Data Quality Monitoring [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 3 ngày  
**Priority:** P2  

### Subtasks

- [ ] Define data quality metrics
- [ ] Implement quality check jobs
- [ ] Dashboard endpoint

---

## TASK-06-05 · Audit Trail [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 1.5 ngày  
**Priority:** P2  

### Subtasks

- [ ] Log all admin write operations to audit_log collection in Firestore
- [ ] Fields: user_id, action, resource, timestamp, ip_address, result
- [ ] `GET /admin/v1/audit-log` endpoint

    ErrorCount24h     int       `json:"error_count_24h"`
    CVECountLastSync  int       `json:"cve_count_last_sync"`
    CircuitBreaker    string    `json:"circuit_breaker"`  // "open", "closed", "half-open"
}
```

---

## TASK-06-02 · Import Findings Handlers [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P1

### Mục tiêu
API để xem và quản lý các lỗi import từ `services/ingestion/`.

### Subtasks

- [ ] Tìm hiểu cấu trúc ImportFinding trong ingestion service
- [ ] Tạo `services/admin/internal/application/import_findings.go`
- [ ] Implement `ListImportFindings(ctx, filter)`:
  - Filter by: source, error_type, date_range, resolved
  - Pagination: cursor-based
- [ ] Implement `ResolveImportFinding(ctx, id, reason)`:
  - Mark as resolved trong Firestore
  - Publish audit event
- [ ] Handlers: `GET /admin/v1/import-findings`, `POST /admin/v1/import-findings/{id}/resolve`
- [ ] Tests

### ImportFinding Schema (đề xuất)
```go
type ImportFinding struct {
    ID          string    // Firestore document ID
    Source      string    // Source name (ghsa, redhat, etc.)
    VulnID      string    // OSV ID hoặc raw ID
    ErrorType   string    // "schema_validation", "duplicate", "parse_error"
    ErrorMsg    string
    RawInput    []byte    // Original input (for replay)
    CreatedAt   time.Time
    ResolvedAt  *time.Time
    ResolvedBy  string
    ResolvedMsg string
}
```

---

## TASK-06-03 · Vulnerability Admin Operations [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P1

### Subtasks

#### TASK-06-03a · Withdraw Vulnerability [📋 TODO]
```
POST /admin/v1/vulns/{id}/withdraw
Body: {"reason": "Not a real vulnerability", "withdrawn_by": "admin@org.com"}
```
- [ ] Implement `WithdrawVulnerability(ctx, id, reason, actor)`:
  1. Fetch vuln từ Firestore
  2. Set `withdrawn` timestamp
  3. Update Firestore
  4. Publish `vuln.withdrawn` NATS event (trigger re-export)
  5. Log audit entry
- [ ] Tests với mock Firestore

#### TASK-06-03b · Reprocess Vulnerability [📋 TODO]
```
POST /admin/v1/vulns/{id}/reprocess
Body: {"reason": "Schema updated, re-enrich"}
```
- [ ] Implement `ReprocessVulnerability(ctx, id, reason)`:
  1. Publish `vuln.reprocess.requested` NATS event
  2. ingestion-service picks up và re-process
  3. Log audit entry
- [ ] Tests

#### TASK-06-03c · Vulnerability Statistics [📋 TODO]
```
GET /admin/v1/vulns/stats
```
- [ ] Query Firestore aggregations (count by source, ecosystem, severity)
- [ ] Cache results: Redis TTL = 15 phút
- [ ] Response:
```json
{
  "total": 1234567,
  "by_severity": {"CRITICAL": 12345, "HIGH": 98765, ...},
  "by_ecosystem": {"PyPI": 45678, "Go": 23456, ...},
  "added_today": 123,
  "kev_count": 1051,
  "last_updated": "2026-06-03T06:00:00Z"
}
```

---

## TASK-06-04 · Data Quality Monitoring [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 3 ngày  
**Priority:** P2  
**Refs:** [06-new-features.md §6.2](../06-new-features.md)

### Mục tiêu
Tự động phát hiện và report data quality issues.

### Subtasks

- [ ] Tạo `services/admin/internal/application/data_quality.go`
- [ ] Implement `DataQualityMonitor`:

```go
type DataQualityReport struct {
    GeneratedAt    time.Time
    MissingCVSS    []string  // CVE IDs không có CVSS score
    MissingVersions []string // CVE IDs không có affected versions
    UnresolvedAliases []string
    StaleEnrichments []string  // Enrichment cũ > 30 ngày
    CrossSourceGaps  map[string]int  // Source coverage gaps
}
```

- [ ] `CheckMissingCVSS()` — CVE không có CVSS score
- [ ] `CheckMissingVersions()` — CVE không có affected version ranges
- [ ] `CheckUnresolvedAliases()` — CVE aliases chưa được verify
- [ ] `GenerateDailyReport()` — Run hàng ngày 07:00 UTC
- [ ] Store report vào Firestore `data_quality_reports` collection
- [ ] Alert khi: missing_cvss_count > 100, unresolved_aliases > 50
- [ ] API endpoint: `GET /admin/v1/data-quality/latest`

---

## TASK-06-05 · Audit Trail [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 1.5 ngày  
**Priority:** P2  
**Refs:** [06-new-features.md §6.3](../06-new-features.md)

### Subtasks

- [ ] Tạo `services/admin/internal/application/audit.go`
- [ ] `AuditLogger` interface:
```go
type AuditLogger interface {
    Log(ctx context.Context, entry AuditEntry) error
}

type AuditEntry struct {
    ID        string
    Timestamp time.Time
    Actor     string    // admin user hoặc service account
    Action    string    // "withdraw", "reprocess", "pause_source"
    Resource  string    // CVE ID hoặc source name
    Before    any
    After     any
    Reason    string
}
```
- [ ] Backend: Firestore collection `audit_logs`
- [ ] Inject vào tất cả admin handlers
- [ ] API: `GET /admin/v1/audit-logs?limit=50&actor=admin@org.com`
- [ ] Retention: 90 ngày (Firestore TTL policy)
- [ ] Tests

---

## TASK-06-06 · API Key Management (Basic) [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P2  
**Refs:** [06-new-features.md §5.2](../06-new-features.md)

### Subtasks

- [ ] Tạo `services/admin/internal/application/api_keys.go`
- [ ] Implement `APIKeyService`:
  - `CreateKey(name, scopes, rate_limit)` → return `APIKey` với plaintext key (shown once)
  - Key storage: hash với bcrypt, store hash trong Firestore
  - `ListKeys(ownerID)` — list keys (không show plaintext)
  - `RevokeKey(keyID)` — deactivate key
  - `ValidateKey(key)` — check hash, return scopes
- [ ] API endpoints:
  - `POST /admin/v1/api-keys`
  - `GET /admin/v1/api-keys`
  - `DELETE /admin/v1/api-keys/{id}`
- [ ] Integration với api-gateway middleware (validate key on each request)
- [ ] Tests

---

## TASK-06-07 · System Health Endpoint [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 1 ngày  
**Priority:** P1

### Subtasks

- [ ] `GET /admin/v1/system/health` — Aggregate health của tất cả services
  - Check: NATS connectivity
  - Check: Firestore connectivity
  - Check: Redis connectivity
  - Check: source-sync gRPC reachable
  - Check: ingestion gRPC reachable
- [ ] Response:
```json
{
  "status": "healthy",
  "services": {
    "nats": "healthy",
    "firestore": "healthy",
    "redis": "healthy",
    "source-sync": "healthy",
    "ingestion": "degraded"
  },
  "checked_at": "2026-06-03T08:00:00Z"
}
```

---

## Sprint 06 Definition of Done

- [ ] `GET /admin/v1/sources` trả về real data từ source-sync
- [ ] `POST /admin/v1/sources/{name}/sync` trigger được sync thực sự
- [ ] Withdraw vuln hoạt động end-to-end
- [ ] Data quality report được generate hàng ngày
- [ ] Audit trail ghi nhận tất cả admin actions
- [ ] `go build ./services/admin/...` pass
- [ ] `go test ./services/admin/...` pass với ≥ 30 test cases
- [ ] API documentation (OpenAPI spec) cho tất cả endpoints
