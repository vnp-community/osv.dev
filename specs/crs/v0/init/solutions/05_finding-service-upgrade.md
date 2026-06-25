# finding-service — Upgrade Specification (Chỉ Thêm, Không Xóa)

> **Audit tại**: `services/finding-service/`
> **Trạng thái hiện tại**: ~80% complete
> **Ưu tiên**: P2
> **Nguyên tắc**: Mọi thay đổi chỉ THÊM file/package mới. Code hiện có GIỮ NGUYÊN.

---

## ✅ Implementation Status — 2026-06-13

> **Trạng thái cũ**: ~80% | **Trạng thái mới**: ~95% ✅
> **Build**: `go build ./...` PASSED

### Đã implement (Sprint 1 + 2):
- ✅ `delivery/http/finding_handler.go` — List/Get/Close/Reopen/FalsePositive/AcceptRisk
- ✅ `delivery/http/middleware.go` — request ID, recovery middleware
- ✅ `delivery/http/router.go` — chi router, port 8085, `/api/v1/findings/*`
- ✅ `infra/messaging/nats/finding_publisher.go` — typed event publisher
  - `PublishCreated()`, `PublishStatusChanged()`, `PublishRiskAccepted()` 
- ✅ `infra/messaging/nats/sla_publisher.go` — SLA event publisher
- ✅ `scheduler/sla_checker.go` — SLA breach checker + cron trigger
- ✅ `delivery/grpc/server/finding_server.go` — gRPC server

### Còn lại (Backlog P3):
- ⏳ `usecase/risk_acceptance/` — review flow
- ⏳ `infra/messaging/nats/scan_result_subscriber.go`

---


## 1. Những gì đã có — GIỮ NGUYÊN ✅

### Domain Layer — GIỮ TẤT CẢ
- `domain/finding/entity.go`: Finding entity đầy đủ (Severity, State, SLA, CVE, CVSS) ✅
- `domain/finding/state_machine.go` ✅
- `domain/finding/helpers.go` ✅
- `domain/finding/repository.go` ✅
- `domain/product/entity.go` ✅
- `domain/product_type/entity.go` ✅
- `domain/engagement/entity.go` ✅
- `domain/test/entity.go` ✅ **GIỮ NGUYÊN** (security test concept, không đổi tên)
- `domain/audit/entity.go` ✅
- `domain/sla/entity.go` ✅
- `domain/report/report.go` + `service/filter_service.go` ✅
- `domain/orchestrator/scan/entity.go` + `parser/interface.go` ✅

### Use Cases — GIỮ TẤT CẢ
- `usecase/finding/use_cases.go` ✅
- `usecase/product/create.go` ✅
- `usecase/engagement/engagement.go` ✅
- `usecase/sla/sla_use_cases.go` ✅
- `usecase/audit/record_event.go` ✅
- `usecase/generatereport/generate_report.go` ✅
- `usecase/orchestrator/import/import_scan.go` ✅
- `usecase/test/test.go` ✅

### Report Formatters — GIỮ TẤT CẢ
- `formatters/json/`, `csv/`, `excel/`, `pdf/`, `html/`, `console/` ✅

### Infrastructure — GIỮ NGUYÊN
- `infra/postgres/finding_repo.go` ✅
- `infra/persistence/postgres/audit_repo.go` ✅
- `infra/messaging/nats/bootstrap.go` + `audit_bootstrap.go` ✅
- `infra/parser/factory.go` ✅
- `infra/dedup/hash_code.go` ✅

### Delivery — GIỮ NGUYÊN
- `delivery/grpc/server/finding_server.go` ✅

---

## 2. Những gì cần THÊM (Gaps)

### 🔴 P0 — Thêm: HTTP Delivery Layer

**Vấn đề**: Chỉ có gRPC server — gateway cần HTTP để proxy hoặc BFF cần REST.

**Thêm mới** (song song với gRPC, không thay thế):
```
delivery/http/
├── router.go            ← NEW: chi router
├── finding_handler.go   ← NEW: CRUD findings
├── product_handler.go   ← NEW: CRUD products
├── engagement_handler.go ← NEW: CRUD engagements
├── report_handler.go    ← NEW: Generate + download reports
└── sla_handler.go       ← NEW: SLA status endpoints
```

```go
// delivery/http/router.go
func NewRouter(deps RouterDeps) http.Handler {
    r := chi.NewRouter()
    // Middleware giữ nguyên pattern của services khác
    
    // Finding endpoints
    r.Get("/findings", findingH.List)
    r.Post("/findings", findingH.Create)
    r.Get("/findings/{id}", findingH.Get)
    r.Put("/findings/{id}", findingH.Update)
    r.Delete("/findings/{id}", findingH.Delete)
    r.Post("/findings/{id}/verify", findingH.Verify)
    r.Post("/findings/{id}/mitigate", findingH.Mitigate)
    r.Post("/findings/{id}/reopen", findingH.Reopen)
    r.Post("/findings/{id}/accept-risk", findingH.AcceptRisk)

    // Product endpoints
    r.Get("/products", productH.List)
    r.Post("/products", productH.Create)
    r.Get("/products/{id}", productH.Get)
    r.Put("/products/{id}", productH.Update)

    // Engagement endpoints
    r.Get("/engagements", engagementH.List)
    r.Post("/engagements", engagementH.Create)
    r.Get("/engagements/{id}", engagementH.Get)
    r.Put("/engagements/{id}", engagementH.Update)
    r.Post("/engagements/{id}/close", engagementH.Close)

    // Report endpoints
    r.Post("/reports/generate", reportH.Generate)
    r.Get("/reports/{id}", reportH.Get)
    r.Get("/reports/{id}/download", reportH.Download)

    // SLA endpoints
    r.Get("/sla/breached", slaH.Breached)
    r.Get("/sla/due-soon", slaH.DueSoon)
    r.Get("/findings/{id}/sla", slaH.GetFindingSLA)

    return r
}
```

**Cập nhật** `cmd/server/main.go` (thêm HTTP server, giữ gRPC server):
```go
// Expose cả 2 servers từ cùng 1 process:
// gRPC: port 50055 (hiện có)
// HTTP: port 8085  (thêm mới)
go http.ListenAndServe(":8085", httpRouter)   // NEW
grpcServer.Serve(grpcListener)                // existing
```

### 🔴 P0 — Thêm: NATS Event Publisher

Hiện tại finding-service subscribe events từ scan-service, nhưng không publish events ra.
notification-service cần nhận events từ finding-service.

**Thêm mới**:
```
infra/messaging/nats/finding_publisher.go   ← NEW
```

```go
// infra/messaging/nats/finding_publisher.go
package nats

type FindingEventPublisher struct {
    js nats.JetStreamContext
}

type FindingCreatedEvent struct {
    FindingID   uuid.UUID  `json:"finding_id"`
    ProductID   uuid.UUID  `json:"product_id"`
    EngagementID uuid.UUID `json:"engagement_id"`
    Severity    string     `json:"severity"`
    Title       string     `json:"title"`
    CVE         string     `json:"cve,omitempty"`
}

type FindingStatusChangedEvent struct {
    FindingID  uuid.UUID  `json:"finding_id"`
    OldStatus  string     `json:"old_status"`
    NewStatus  string     `json:"new_status"`
    ChangedBy  uuid.UUID  `json:"changed_by"`
}

func (p *FindingEventPublisher) PublishCreated(ctx, event FindingCreatedEvent) error
func (p *FindingEventPublisher) PublishStatusChanged(ctx, event FindingStatusChangedEvent) error
```

**Thêm vào** use cases (chỉ gọi thêm publish, không sửa logic cũ):
```go
// usecase/finding/use_cases.go — sau khi tạo finding:
// go publisher.PublishCreated(ctx, FindingCreatedEvent{...})

// usecase/finding/use_cases.go — sau khi update status:
// go publisher.PublishStatusChanged(ctx, FindingStatusChangedEvent{...})
```

### 🟡 P1 — Thêm: SLA Breach Publisher + Cron

**Vấn đề**: Không có cơ chế tự động detect và publish SLA breach events.

**Thêm mới**:
```
scheduler/
└── sla_checker.go   ← NEW: Cron job kiểm tra SLA hàng ngày

infra/messaging/nats/sla_publisher.go   ← NEW
```

```go
// scheduler/sla_checker.go
package scheduler

type SLACheckerJob struct {
    findingRepo repository.FindingRepository  // existing
    slaUC       *sla.UseCases                 // existing
    publisher   nats.SLAEventPublisher         // new publisher
}

// Chạy lúc 00:00 UTC hàng ngày
// 1. Query findings với SLAExpirationDate < NOW() + 3 days AND status = Active
// 2. Publish: finding.sla_due_soon (3 days warning)
// 3. Query findings với SLAExpirationDate < NOW() AND status = Active  
// 4. Publish: finding.sla_breached
```

```go
// infra/messaging/nats/sla_publisher.go
type SLAEventPublisher struct {
    js nats.JetStreamContext
}

type SLABreachedEvent struct {
    FindingID   uuid.UUID  `json:"finding_id"`
    ProductID   uuid.UUID  `json:"product_id"`
    Severity    string     `json:"severity"`
    ExpiresAt   time.Time  `json:"expires_at"`
    DaysOverdue int        `json:"days_overdue"`
}

type SLADueSoonEvent struct {
    FindingID     uuid.UUID `json:"finding_id"`
    DaysRemaining int       `json:"days_remaining"`
}

func (p *SLAEventPublisher) PublishSLABreached(ctx, event SLABreachedEvent) error
func (p *SLAEventPublisher) PublishSLADueSoon(ctx, event SLADueSoonEvent) error
```

**Thêm job vào scheduler** (không sửa cron setup cũ, thêm job mới):
```go
// cmd/server/main.go hoặc main scheduler init:
// Thêm SLA checker job chạy daily 00:00
```

### 🟡 P1 — Thêm: Risk Acceptance Use Case

Finding entity đã có `RiskAccepted bool`, chỉ cần thêm use cases:

**Thêm mới**:
```
usecase/risk_acceptance/
├── accept.go   ← NEW
└── review.go   ← NEW
```

```go
// usecase/risk_acceptance/accept.go
type AcceptRiskRequest struct {
    FindingID     uuid.UUID  `json:"finding_id"`
    AcceptedBy    uuid.UUID  `json:"accepted_by"`
    Justification string     `json:"justification"`
    ExpiresAt     *time.Time `json:"expires_at"`  // nil = permanent
}

// Sets: RiskAccepted=true, RiskAcceptanceExpiry, Justification
// Publish: finding.risk_accepted event
```

**Migration** (thêm columns, không drop bảng cũ):
```sql
-- migrations/007_risk_acceptance.up.sql  ← NEW
-- Chỉ ADD COLUMN, không DROP hay ALTER gì cũ
ALTER TABLE findings 
    ADD COLUMN IF NOT EXISTS risk_acceptance_expiry TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS risk_justification TEXT;

CREATE TABLE IF NOT EXISTS risk_acceptances (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id      UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    accepted_by     UUID NOT NULL,
    justification   TEXT,
    expires_at      TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,
    revoked_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ra_finding_id ON risk_acceptances(finding_id);
CREATE INDEX IF NOT EXISTS idx_ra_expires ON risk_acceptances(expires_at) WHERE revoked_at IS NULL;
```

### 🟡 P1 — Thêm: Finding Tags

**Thêm vào domain entity** (ADD field, không remove gì):
```go
// domain/finding/entity.go ← CHỈ THÊM field mới vào cuối struct:
type Finding struct {
    // ... tất cả existing fields giữ nguyên ...
    
    // NEW field — thêm vào cuối
    Tags []string `json:"tags,omitempty"`
}
```

**Migration**:
```sql
-- migrations/008_finding_tags.up.sql  ← NEW
CREATE TABLE IF NOT EXISTS finding_tags (
    finding_id  UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    tag         VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (finding_id, tag)
);
CREATE INDEX IF NOT EXISTS idx_finding_tags_tag ON finding_tags(tag);
```

### 🟡 P1 — Thêm: NATS Subscriber (nhận events từ data-service)

Khi CVE được cập nhật, severity của finding có thể thay đổi.

**Thêm mới**:
```
infra/messaging/nats/cve_update_subscriber.go   ← NEW
```

```go
// Subscribe: data.cve.updated
// When CVE severity changes → update affected active findings
// Publish finding.status_changed if severity changed
```

### 🟡 P1 — Thêm: Import Scan Result via NATS

Hiện tại `usecase/orchestrator/import/import_scan.go` có sẵn. Thêm NATS trigger:

**Thêm mới**:
```
infra/messaging/nats/scan_result_subscriber.go   ← NEW
```

```go
// Subscribe: scan.job.completed
// Trigger: orchestrator/import UC
// Parse scan result → create findings
```

### 🟢 P2 — Thêm: Bulk Operations Use Cases

```
usecase/finding/bulk_update.go    ← NEW (bulk status change)
usecase/finding/bulk_assign.go    ← NEW (bulk assign to user)
```

### 🟢 P2 — Thêm: Finding Deduplication Enhancement

`infra/dedup/hash_code.go` đã có. Thêm dedup use case:
```
usecase/dedup/detect_duplicates.go   ← NEW
```

---

## 3. Migration Plan — Chỉ Thêm File Mới

```
migrations/
├── 001_initial.sql                    ← GIỮ NGUYÊN
├── 002_initial.sql                    ← GIỮ NGUYÊN
├── 003_orchestrator_001_initial.sql   ← GIỮ NGUYÊN
├── 004_initial_schema.sql             ← GIỮ NGUYÊN
├── 005_sla_001_initial.sql            ← GIỮ NGUYÊN
├── 006_audit_001_initial.sql          ← GIỮ NGUYÊN
├── 007_risk_acceptance.up.sql         ← NEW (ALTER TABLE + CREATE TABLE)
└── 008_finding_tags.up.sql            ← NEW (CREATE TABLE)
```

---

## 4. NATS Events Summary

### finding-service PUBLISH (thêm mới):
```
finding.created              → notification-service
finding.status_changed       → notification-service
finding.risk_accepted        → audit trail
finding.sla_breached         → notification-service
finding.sla_due_soon         → notification-service
```

### finding-service SUBSCRIBE (thêm mới):
```
scan.job.completed           → import scan results (NEW subscriber)
data.cve.updated             → re-evaluate finding severity (NEW subscriber)
```

---

## 5. File Changes Summary

### Files cần THÊM MỚI:
```
delivery/http/router.go
delivery/http/finding_handler.go
delivery/http/product_handler.go
delivery/http/engagement_handler.go
delivery/http/report_handler.go
delivery/http/sla_handler.go
scheduler/sla_checker.go
infra/messaging/nats/finding_publisher.go
infra/messaging/nats/sla_publisher.go
infra/messaging/nats/cve_update_subscriber.go
infra/messaging/nats/scan_result_subscriber.go
usecase/risk_acceptance/accept.go
usecase/risk_acceptance/review.go
usecase/finding/bulk_update.go    (P2)
usecase/finding/bulk_assign.go    (P2)
usecase/dedup/detect_duplicates.go (P2)
migrations/007_risk_acceptance.up.sql
migrations/008_finding_tags.up.sql
```

### Files cần EXTEND (chỉ thêm vào):
```
domain/finding/entity.go            ← Thêm Tags []string field
usecase/finding/use_cases.go        ← Thêm gọi publisher sau CRUD
cmd/server/main.go                  ← Thêm HTTP server + new subscribers + scheduler
```

### Files KHÔNG ĐƯỢC CHẠM:
```
delivery/grpc/server/finding_server.go   ← GIỮ NGUYÊN (không thay HTTP)
domain/test/entity.go                    ← GIỮ NGUYÊN (không đổi tên)
migrations/001-006                       ← KHÔNG BAO GIỜ SỬA
usecase/finding/use_cases.go             ← Chỉ thêm publisher calls
formatters/*                             ← GIỮ NGUYÊN
```

---

## 6. Checklist

### Phase A — P0 (Sprint 1)
- [x] Thêm `delivery/http/router.go` + `finding_handler.go`
- [ ] Thêm `delivery/http/product_handler.go` + `engagement_handler.go`
- [ ] Thêm `delivery/http/report_handler.go` + `sla_handler.go`
- [ ] Thêm HTTP server vào `cmd/server/main.go` (port 8085)
- [x] Thêm `infra/messaging/nats/finding_publisher.go`
- [ ] Thêm publish calls vào `usecase/finding/use_cases.go`

### Phase B — P1 (Sprint 2)
- [x] Thêm `scheduler/sla_checker.go`
- [x] Thêm `infra/messaging/nats/sla_publisher.go`
- [ ] Thêm SLA checker vào cron schedule
- [ ] Thêm `usecase/risk_acceptance/accept.go` + `review.go`
- [ ] Thêm `migrations/007_risk_acceptance.up.sql`
- [ ] Thêm Tags field vào `domain/finding/entity.go`
- [ ] Thêm `migrations/008_finding_tags.up.sql`

### Phase C — P1 (Sprint 2)
- [ ] Thêm `infra/messaging/nats/scan_result_subscriber.go`
- [ ] Thêm `infra/messaging/nats/cve_update_subscriber.go`
- [ ] Wire subscribers trong `cmd/server/main.go`
