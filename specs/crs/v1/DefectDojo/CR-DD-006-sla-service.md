# ✅ COMPLETED — CR-DD-006 — SLA Service (Full Implementation)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DD-006 |
| **Tiêu đề** | SLA Service — Configuration, Expiry Computation, Breach Detection, Bulk Recompute |
| **Nguồn tham chiếu** | `django-DefectDojo/specs/services/06-sla-service.md`, `SRS.md §FR-SLA-01 to FR-SLA-05` |
| **Target Service** | **MỚI**: `sla-service` |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | New Service |
| **Ngày tạo** | 2026-06-13 |

---

## 1. Tổng quan

SLA (Service Level Agreement) trong DefectDojo định nghĩa số ngày tối đa để fix vulnerability theo severity. OSV hiện tại có SLA concept nhưng thiếu:

- **SLA Configuration CRUD** (per-organization, assignable to products)
- **Automatic expiry computation** khi finding được tạo
- **Daily breach detection** (scheduled cron)
- **Bulk recompute** khi SLA config thay đổi
- **SLA dashboard** metrics

---

## 2. Gap Analysis

| Feature | OSV | DefectDojo SLA |
|---------|-----|---------------|
| SLA Config CRUD | ⚠️ Partial | ✅ Full CRUD |
| Per-severity days | ⚠️ Partial | ✅ Critical/High/Medium/Low |
| Enforce flags (optional SLA) | ❌ | ✅ Per severity enforce flag |
| Auto-compute on finding.created | ❌ | ✅ Event-driven |
| Daily breach detection | ❌ | ✅ Daily 07:30 cron |
| Expiring-soon notification (7d) | ❌ | ✅ |
| Bulk recompute on config change | ❌ | ✅ Async job |
| `restart_sla_on_reactivation` | ❌ | ✅ |
| SLA compliance % dashboard | ❌ | ✅ |

---

## 3. Service Architecture

```
sla-service/
├── cmd/server/main.go
│
├── internal/
│   ├── domain/
│   │   ├── slaconfig/
│   │   │   ├── entity.go          # SLAConfiguration entity
│   │   │   ├── value_objects.go   # SLADays, SeverityConfig
│   │   │   └── repository.go
│   │   ├── breach/
│   │   │   ├── entity.go          # SLABreachEvent
│   │   │   └── repository.go
│   │   └── assignment/
│   │       ├── entity.go          # ProductSLAAssignment
│   │       └── repository.go
│   │
│   ├── usecase/
│   │   ├── slaconfig/             # CRUD
│   │   ├── computation/
│   │   │   ├── compute_expiry.go       # Per finding
│   │   │   └── bulk_recompute.go      # Async
│   │   └── breach/
│   │       ├── detect_breaches.go     # Daily cron
│   │       └── detect_expiring_soon.go
│   │
│   ├── delivery/
│   │   ├── http/                  # REST handlers
│   │   ├── grpc/                  # gRPC server
│   │   └── event/
│   │       └── subscriber.go      # NATS: finding.created, product.updated
│   │
│   └── infra/
│       ├── postgres/
│       ├── scheduler/             # cron jobs
│       └── grpc/client/
│           └── finding.go         # → finding-service
```

---

## 4. Domain Model

### 4.1 SLAConfiguration Entity

```go
// domain/slaconfig/entity.go
// Mirrors Python: dojo/models.py::SLA_Configuration

type SLAConfiguration struct {
    ID          string
    Name        string  // unique
    Description string

    // Days per severity (number of days from discovery to fix)
    Critical int  // default: 7
    High     int  // default: 30
    Medium   int  // default: 90
    Low      int  // default: 180

    // Enforcement flags (false = don't track SLA for this severity)
    EnforceCritical bool  // default: true
    EnforceHigh     bool  // default: true
    EnforceMedium   bool  // default: false
    EnforceLow      bool  // default: false

    // Behavior flags
    RestartSLAOnReactivation bool  // Reset SLA when finding is reactivated

    // Concurrency lock for bulk updates
    AsyncUpdating bool

    CreatedAt time.Time
    UpdatedAt time.Time
}

// ComputeExpirationDate — mirrors Python: SLA_Configuration.compute_expiration_date()
func (c *SLAConfiguration) ComputeExpirationDate(discoveryDate time.Time, severity Severity) *time.Time {
    days := c.DaysForSeverity(severity)
    if days == 0 {
        return nil // not enforced for this severity
    }
    expiry := discoveryDate.AddDate(0, 0, days)
    return &expiry
}

func (c *SLAConfiguration) DaysForSeverity(severity Severity) int {
    switch severity {
    case SeverityCritical:
        if c.EnforceCritical { return c.Critical }
    case SeverityHigh:
        if c.EnforceHigh { return c.High }
    case SeverityMedium:
        if c.EnforceMedium { return c.Medium }
    case SeverityLow:
        if c.EnforceLow { return c.Low }
    }
    return 0 // not enforced
}

// ProductSLAAssignment — which SLA config applies to a product
type ProductSLAAssignment struct {
    ID               string
    ProductID        string
    SLAConfigurationID string
    CreatedAt        time.Time
}

// SLABreachEvent — audit record of breach detection
type SLABreachEvent struct {
    ID                string
    FindingID         string
    ProductID         string
    Severity          string
    SLAExpirationDate time.Time
    DetectedAt        time.Time
    DaysOverdue       int
    Notified          bool
}
```

---

## 5. Use Cases

### 5.1 ComputeExpiry (triggered by finding.created event)

```go
// usecase/computation/compute_expiry.go

type ComputeExpiryInput struct {
    FindingID     string
    ProductID     string
    Severity      Severity
    DiscoveryDate time.Time
    IsReactivated bool
}

func (uc *ComputeExpiryUseCase) Execute(ctx context.Context, in ComputeExpiryInput) error {
    // 1. Get SLA config for this product
    assignment, err := uc.assignmentRepo.GetForProduct(ctx, in.ProductID)
    if err != nil || assignment == nil {
        return nil // No SLA config → no computation
    }

    config, _ := uc.slaConfigRepo.FindByID(ctx, assignment.SLAConfigurationID)

    // 2. If reactivated and restart_sla=false → keep existing date
    if in.IsReactivated && !config.RestartSLAOnReactivation {
        return nil
    }

    // 3. Compute expiration date
    expiryDate := config.ComputeExpirationDate(in.DiscoveryDate, in.Severity)
    if expiryDate == nil {
        return nil // Not enforced for this severity
    }

    // 4. Update finding via gRPC
    _, err = uc.findingClient.BatchUpdateSLADates(ctx, &findingv1.BatchUpdateSLADatesRequest{
        Updates: []*findingv1.SLADateUpdate{{
            FindingId:         in.FindingID,
            SlaExpirationDate: expiryDate.Format("2006-01-02"),
        }},
    })
    return err
}
```

### 5.2 DetectBreaches (Daily 07:30)

```go
// usecase/breach/detect_breaches.go
// Mirrors Python: dojo/celery_tasks.py::async_sla_compute_and_notify()

func (uc *DetectBreachesUseCase) Execute(ctx context.Context) error {
    today := time.Now().Truncate(24 * time.Hour)

    // 1. Find breached findings (sla_expiration_date < today, active=true)
    stream, _ := uc.findingClient.ListFindingsForSLACheck(ctx, &findingv1.ListFindingsForSLACheckRequest{
        ActiveOnly:     true,
        BreachedBefore: today.Format("2006-01-02"),
    })

    var breachedCount int
    for {
        finding, err := stream.Recv()
        if err == io.EOF { break }
        if err != nil { return err }

        daysOverdue := int(today.Sub(parseSLADate(finding.SlaExpirationDate)).Hours() / 24)

        // Record breach event
        uc.breachRepo.Save(ctx, &SLABreachEvent{
            FindingID:         finding.Id,
            ProductID:         finding.ProductId,
            Severity:          finding.Severity,
            SLAExpirationDate: parseSLADate(finding.SlaExpirationDate),
            DetectedAt:        time.Now(),
            DaysOverdue:       daysOverdue,
        })

        // Publish event → Notification Service
        uc.eventPub.Publish(ctx, &events.SLABreached{
            FindingID:         finding.Id,
            ProductID:         finding.ProductId,
            Severity:          finding.Severity,
            SLAExpirationDate: finding.SlaExpirationDate,
            DaysOverdue:       daysOverdue,
        })
        breachedCount++
    }

    // 2. Find expiring soon (next 7 days)
    expiringStream, _ := uc.findingClient.ListFindingsForSLACheck(ctx, &findingv1.ListFindingsForSLACheckRequest{
        ActiveOnly: true,
        ExpiringBetween: &findingv1.DateRange{
            From: today.AddDate(0, 0, 1).Format("2006-01-02"),
            To:   today.AddDate(0, 0, 7).Format("2006-01-02"),
        },
    })

    for {
        finding, err := expiringStream.Recv()
        if err == io.EOF { break }

        daysLeft := int(parseSLADate(finding.SlaExpirationDate).Sub(today).Hours() / 24)
        uc.eventPub.Publish(ctx, &events.SLAExpiringSoon{
            FindingID:    finding.Id,
            ProductID:    finding.ProductId,
            Severity:     finding.Severity,
            DaysRemaining: daysLeft,
        })
    }

    uc.logger.Info("SLA breach detection completed", "breached", breachedCount)
    return nil
}
```

### 5.3 BulkRecompute (triggered by sla.config.updated event)

```go
// usecase/computation/bulk_recompute.go
// Mirrors Python: dojo/celery_tasks.py::async_update_sla_expiration_dates_sla_config_sync()

func (uc *BulkRecomputeUseCase) Execute(ctx context.Context, in BulkRecomputeInput) error {
    config, _ := uc.slaConfigRepo.FindByID(ctx, in.SLAConfigID)

    // Lock
    config.AsyncUpdating = true
    uc.slaConfigRepo.Save(ctx, config)
    defer func() {
        config.AsyncUpdating = false
        uc.slaConfigRepo.Save(ctx, config)
    }()

    // Find all products assigned this SLA config
    assignments, _ := uc.assignmentRepo.FindBySLAConfigID(ctx, in.SLAConfigID)

    // Process in batches
    batchSize := 1000
    for _, assignment := range assignments {
        offset := 0
        for {
            findings, _ := uc.findingClient.ListFindingsForSLACheck(ctx, &findingv1.ListFindingsForSLACheckRequest{
                ProductIds: []string{assignment.ProductID},
                ActiveOnly: true,
                Limit:      int32(batchSize),
                Offset:     int32(offset),
            })

            if len(findings) == 0 { break }

            updates := make([]*findingv1.SLADateUpdate, 0, len(findings))
            for _, f := range findings {
                newDate := config.ComputeExpirationDate(parseDate(f.Date), Severity(f.Severity))
                updates = append(updates, &findingv1.SLADateUpdate{
                    FindingId:         f.Id,
                    SlaExpirationDate: formatDate(newDate),
                })
            }

            uc.findingClient.BatchUpdateSLADates(ctx, &findingv1.BatchUpdateSLADatesRequest{Updates: updates})

            offset += batchSize
            if len(findings) < batchSize { break }
        }
    }

    uc.eventPub.Publish(ctx, &events.SLABulkRecomputeCompleted{SLAConfigID: in.SLAConfigID})
    return nil
}
```

---

## 6. gRPC Contract

```protobuf
// proto/sla/v1/sla.proto
syntax = "proto3";
package sla.v1;

service SLAService {
    // SLA Configuration CRUD
    rpc CreateSLAConfig(CreateSLAConfigRequest) returns (CreateSLAConfigResponse);
    rpc GetSLAConfig(GetSLAConfigRequest) returns (GetSLAConfigResponse);
    rpc UpdateSLAConfig(UpdateSLAConfigRequest) returns (UpdateSLAConfigResponse);
    rpc DeleteSLAConfig(DeleteSLAConfigRequest) returns (DeleteSLAConfigResponse);
    rpc ListSLAConfigs(ListSLAConfigsRequest) returns (ListSLAConfigsResponse);

    // Computation
    rpc ComputeExpiryForFinding(ComputeExpiryRequest) returns (ComputeExpiryResponse);
    rpc BulkRecomputeForProduct(BulkRecomputeRequest) returns (BulkRecomputeResponse);

    // Dashboard
    rpc GetSLADashboard(GetSLADashboardRequest) returns (GetSLADashboardResponse);
}

message CreateSLAConfigRequest {
    string name        = 1;
    string description = 2;
    int32 critical     = 3;  // days
    int32 high         = 4;
    int32 medium       = 5;
    int32 low          = 6;
    bool enforce_critical = 7;
    bool enforce_high     = 8;
    bool enforce_medium   = 9;
    bool enforce_low      = 10;
    bool restart_sla_on_reactivation = 11;
}

message GetSLADashboardResponse {
    int32 total_active_findings = 1;
    int32 breached_count        = 2;
    int32 expiring_soon_count   = 3;
    float sla_compliance_pct    = 4;
    map<string, int32> breached_by_severity = 5;
}
```

---

## 7. REST API

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET/POST` | `/api/v2/sla-configurations` | JWT | CRUD SLA configs |
| `GET/PUT/DELETE` | `/api/v2/sla-configurations/{id}` | JWT | CRUD |
| `POST` | `/api/v2/sla-configurations/{id}/assign/{product_id}` | JWT/Admin | Assign to product |
| `GET` | `/api/v2/sla-dashboard` | JWT | SLA compliance overview |
| `GET` | `/api/v2/sla-violations` | JWT | List current violations |
| `GET` | `/api/v2/sla-violations/{product_id}` | JWT | Violations per product |

---

## 8. Scheduler

```go
// infra/scheduler/cron.go
scheduler.AddFunc("30 7 * * *", DetectBreaches)   // Daily 07:30
scheduler.AddFunc("30 7 * * *", DetectExpiringSoon) // Daily 07:30
```

---

## 9. NATS Events

### Published
```
sla.config.created       {sla_config_id, name, critical_days, ...}
sla.config.updated       {sla_config_id, changes}
sla.breached             {finding_id, product_id, severity, expiration_date, days_overdue}
sla.expiring_soon        {finding_id, product_id, severity, expiration_date, days_remaining}
sla.bulk_recompute.started    {sla_config_id}
sla.bulk_recompute.completed  {sla_config_id, updated_count}
```

### Subscribed
```
finding.created          → ComputeExpiryForFinding
finding.status_changed   → ComputeExpiryForFinding (if reactivated)
product.updated          → OnProductSLAChanged (if sla_configuration_id changed)
sla.config.updated       → BulkRecomputeForProduct
```

---

## 10. Database Schema

```sql
-- sla_configurations
CREATE TABLE sla_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    critical INTEGER NOT NULL DEFAULT 7,
    high INTEGER NOT NULL DEFAULT 30,
    medium INTEGER NOT NULL DEFAULT 90,
    low INTEGER NOT NULL DEFAULT 180,
    enforce_critical BOOLEAN DEFAULT TRUE,
    enforce_high BOOLEAN DEFAULT TRUE,
    enforce_medium BOOLEAN DEFAULT FALSE,
    enforce_low BOOLEAN DEFAULT FALSE,
    restart_sla_on_reactivation BOOLEAN DEFAULT FALSE,
    async_updating BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- product_sla_assignments
CREATE TABLE product_sla_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL UNIQUE,
    sla_configuration_id UUID NOT NULL REFERENCES sla_configurations(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- sla_breach_events (partitioned for performance)
CREATE TABLE sla_breach_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL,
    product_id UUID NOT NULL,
    severity VARCHAR(20) NOT NULL,
    sla_expiration_date DATE NOT NULL,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    days_overdue INTEGER NOT NULL,
    notified BOOLEAN DEFAULT FALSE
) PARTITION BY RANGE (detected_at);

CREATE TABLE sla_breach_events_2026
    PARTITION OF sla_breach_events
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE INDEX idx_sla_breach_product ON sla_breach_events(product_id, detected_at DESC);
CREATE INDEX idx_sla_breach_finding ON sla_breach_events(finding_id);
```

---

## 11. Acceptance Criteria

- [x] `POST /api/v2/sla-configurations` tạo SLA config với Critical=7d, High=30d
- [x] Assign SLA config to product → mọi finding mới trong product có SLA computed
- [x] finding.created event → sla-service tự động compute `sla_expiration_date`
- [x] Daily cron lúc 07:30: findings quá hạn → NATS `sla.breached` event
- [x] Daily cron: findings sắp hết hạn 7 ngày → NATS `sla.expiring_soon` event
- [x] Update SLA config → bulk recompute async cho tất cả findings liên quan
- [x] `GET /api/v2/sla-dashboard`: trả về compliance % và violation counts
- [x] `EnforceMedium=false`: Medium findings không có `sla_expiration_date`
- [x] `restart_sla_on_reactivation=false`: reactivated finding giữ nguyên SLA date cũ

## Implementation Status: ✅ DONE

> `sla-service/internal/domain/slaconfig/entity.go` — SLAConfiguration: ComputeExpirationDate(), DaysForSeverity(), EnforceCritical/High/Medium/Low flags, AsyncUpdating
> `sla-service/internal/usecase/{config,compute,dashboard,breach}/` — CRUD + ComputeExpiry + BulkRecompute + DetectBreaches + DetectExpiringSoon
> `sla-service/internal/infra/scheduler/cron.go` — daily 07:30 cron: DetectBreaches + DetectExpiringSoon
> `sla-service/migrations/001_sla_configurations.sql` — sla_configurations + product_sla_assignments
> `sla-service/migrations/002_sla_breach_events.sql` — sla_breach_events PARTITION BY RANGE(detected_at)
> NATS subscriber: finding.created → ComputeExpiry, sla.config.updated → BulkRecompute
