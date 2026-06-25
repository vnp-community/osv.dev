# ✅ COMPLETED — Solution: sla-service (New Service)

> **Covers**: CR-DD-006  
> **Lý do tạo service mới**: SLA là một domain hoàn toàn tách biệt — có scheduler riêng, database riêng (partitioned tables), không phụ thuộc vào finding domain logic. Nó subscribe NATS events và gọi finding-service qua gRPC để cập nhật SLA dates.

---

## Justification

`sla-service` cần tồn tại độc lập vì:

1. **Scheduler riêng**: Daily 07:30 breach detection cần process hàng ngàn findings — không muốn block finding-service
2. **Partitioned tables**: `sla_breach_events` dùng PostgreSQL range partitioning — tốt nhất là DB riêng
3. **Bulk recompute async**: Khi SLA config thay đổi, cần recompute toàn bộ findings — heavy task cần isolate
4. **Scale độc lập**: SLA computation có thể cần scale out riêng (e.g., nhiều products × nhiều findings)

---

## Service Structure

```
services/sla-service/           # NEW SERVICE
├── cmd/server/main.go
├── Dockerfile
├── go.mod
├── migrations/
│   ├── 001_sla_configurations.sql
│   ├── 002_product_sla_assignments.sql
│   └── 003_sla_breach_events_partitioned.sql
│
└── internal/
    ├── domain/
    │   ├── slaconfig/
    │   │   ├── entity.go          # SLAConfiguration
    │   │   ├── value_objects.go   # SLADays, SeverityConfig
    │   │   └── repository.go
    │   ├── breach/
    │   │   ├── entity.go          # SLABreachEvent
    │   │   └── repository.go
    │   └── assignment/
    │       ├── entity.go          # ProductSLAAssignment
    │       └── repository.go
    │
    ├── usecase/
    │   ├── slaconfig/
    │   │   ├── create.go
    │   │   ├── update.go
    │   │   ├── delete.go
    │   │   └── list.go
    │   ├── computation/
    │   │   ├── compute_expiry.go       # Per finding (triggered by NATS)
    │   │   └── bulk_recompute.go       # Async (triggered by config change)
    │   └── breach/
    │       ├── detect_breaches.go      # Daily cron 07:30
    │       └── detect_expiring_soon.go # Daily 07:30
    │
    ├── delivery/
    │   ├── http/
    │   │   ├── server.go
    │   │   ├── slaconfig_handler.go
    │   │   ├── dashboard_handler.go
    │   │   └── violations_handler.go
    │   ├── grpc/
    │   │   └── sla_server.go
    │   └── event/
    │       └── subscriber.go      # NATS subscriber
    │
    └── infra/
        ├── postgres/
        │   ├── slaconfig_repo.go
        │   ├── breach_repo.go
        │   └── assignment_repo.go
        ├── scheduler/
        │   └── cron.go            # cron jobs
        └── grpc/client/
            └── finding.go         # → finding-service gRPC
```

---

## Domain Model

### SLAConfiguration Entity

```go
// sla-service/internal/domain/slaconfig/entity.go
type SLAConfiguration struct {
    ID          string
    Name        string  // unique
    Description string

    // Days per severity (từ discovery đến fix)
    Critical int  // default: 7
    High     int  // default: 30
    Medium   int  // default: 90
    Low      int  // default: 180

    // Enforcement flags
    EnforceCritical bool  // default: true
    EnforceHigh     bool  // default: true
    EnforceMedium   bool  // default: false
    EnforceLow      bool  // default: false

    RestartSLAOnReactivation bool
    AsyncUpdating            bool  // lock during bulk recompute

    CreatedAt time.Time
    UpdatedAt time.Time
}

// ComputeExpirationDate — mirrors Python: SLA_Configuration.compute_expiration_date()
func (c *SLAConfiguration) ComputeExpirationDate(discoveryDate time.Time, severity Severity) *time.Time {
    days := c.DaysForSeverity(severity)
    if days == 0 {
        return nil // not enforced
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
```

### ProductSLAAssignment Entity

```go
// sla-service/internal/domain/assignment/entity.go
type ProductSLAAssignment struct {
    ID                 string
    ProductID          string
    SLAConfigurationID string
    CreatedAt          time.Time
}
```

### SLABreachEvent Entity

```go
// sla-service/internal/domain/breach/entity.go
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

## Use Cases

### ComputeExpiry (triggered by NATS finding.created)

```go
// sla-service/internal/usecase/computation/compute_expiry.go
func (uc *ComputeExpiryUseCase) Execute(ctx context.Context, in ComputeExpiryInput) error {
    // 1. Get SLA assignment for product
    assignment, err := uc.assignmentRepo.GetForProduct(ctx, in.ProductID)
    if err != nil || assignment == nil {
        return nil // No SLA config → skip
    }

    config, _ := uc.slaConfigRepo.FindByID(ctx, assignment.SLAConfigurationID)

    // 2. If reactivated and restart_sla=false → keep existing SLA date
    if in.IsReactivated && !config.RestartSLAOnReactivation {
        return nil
    }

    // 3. Compute expiration date
    expiryDate := config.ComputeExpirationDate(in.DiscoveryDate, in.Severity)
    if expiryDate == nil {
        return nil // Not enforced for this severity
    }

    // 4. Update finding via gRPC → finding-service
    _, err = uc.findingClient.BatchUpdateSLADates(ctx, &findingv1.BatchUpdateSLADatesRequest{
        Updates: []*findingv1.SLADateUpdate{{
            FindingId:         in.FindingID,
            SlaExpirationDate: expiryDate.Format("2006-01-02"),
        }},
    })
    return err
}
```

### DetectBreaches (cron daily 07:30)

```go
// sla-service/internal/usecase/breach/detect_breaches.go
func (uc *DetectBreachesUseCase) Execute(ctx context.Context) error {
    today := time.Now().Truncate(24 * time.Hour)

    // 1. Stream breached findings (sla_expiration_date < today, active=true)
    stream, _ := uc.findingClient.ListFindingsForSLACheck(ctx, &findingv1.ListFindingsForSLACheckRequest{
        ActiveOnly:     true,
        BreachedBefore: today.Format("2006-01-02"),
    })

    for {
        finding, err := stream.Recv()
        if err == io.EOF { break }
        daysOverdue := int(today.Sub(parseDate(finding.SlaExpirationDate)).Hours() / 24)

        // Record breach
        uc.breachRepo.Save(ctx, &SLABreachEvent{...})

        // Notify via NATS → notification-service will pick up
        uc.eventPub.Publish(ctx, &events.SLABreached{
            FindingID:    finding.Id,
            ProductID:    finding.ProductId,
            DaysOverdue:  daysOverdue,
        })
    }

    // 2. Also check expiring soon (1-7 days from now)
    // → publish sla.expiring_soon events
    return nil
}
```

### BulkRecompute (triggered by sla.config.updated event)

```go
// sla-service/internal/usecase/computation/bulk_recompute.go
// Async: locks config.AsyncUpdating=true during recompute
// Processes in batches of 1000 findings per product
```

---

## NATS Event Subscriptions

```go
// sla-service/internal/delivery/event/subscriber.go
var subscriptions = []struct {
    Subject string
    Handler func(msg *nats.Msg)
}{
    {"finding.created",       uc.OnFindingCreated},     // → ComputeExpiry
    {"finding.status_changed", uc.OnFindingChanged},    // → ComputeExpiry (if reactivated)
    {"product.updated",       uc.OnProductUpdated},     // → OnProductSLAChanged
    {"sla.config.updated",    uc.OnSLAConfigUpdated},   // → BulkRecompute
}
```

---

## gRPC Contract

```protobuf
// sla-service/proto/sla/v1/sla.proto
syntax = "proto3";
package sla.v1;

service SLAService {
    // CRUD
    rpc CreateSLAConfig(CreateSLAConfigRequest) returns (CreateSLAConfigResponse);
    rpc GetSLAConfig(GetSLAConfigRequest) returns (GetSLAConfigResponse);
    rpc UpdateSLAConfig(UpdateSLAConfigRequest) returns (UpdateSLAConfigResponse);
    rpc DeleteSLAConfig(DeleteSLAConfigRequest) returns (DeleteSLAConfigResponse);
    rpc ListSLAConfigs(ListSLAConfigsRequest) returns (ListSLAConfigsResponse);

    // Assignment
    rpc AssignToProduct(AssignToProductRequest) returns (AssignToProductResponse);
    rpc GetAssignment(GetAssignmentRequest) returns (GetAssignmentResponse);

    // Computation
    rpc ComputeExpiryForFinding(ComputeExpiryRequest) returns (ComputeExpiryResponse);
    rpc BulkRecomputeForProduct(BulkRecomputeRequest) returns (BulkRecomputeResponse);

    // Dashboard
    rpc GetSLADashboard(GetSLADashboardRequest) returns (GetSLADashboardResponse);
    rpc GetViolations(GetViolationsRequest) returns (GetViolationsResponse);
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

## REST API

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET` | `/api/v2/sla-configurations` | JWT | List SLA configs |
| `POST` | `/api/v2/sla-configurations` | JWT/Admin | Create |
| `GET` | `/api/v2/sla-configurations/{id}` | JWT | Get |
| `PUT` | `/api/v2/sla-configurations/{id}` | JWT/Admin | Update |
| `DELETE` | `/api/v2/sla-configurations/{id}` | JWT/Admin | Delete |
| `POST` | `/api/v2/sla-configurations/{id}/assign/{product_id}` | JWT/Admin | Assign to product |
| `GET` | `/api/v2/sla-dashboard` | JWT | SLA compliance overview |
| `GET` | `/api/v2/sla-violations` | JWT | List current violations |
| `GET` | `/api/v2/sla-violations/{product_id}` | JWT | Violations per product |

---

## Scheduler

```go
// sla-service/internal/infra/scheduler/cron.go
scheduler.AddFunc("30 7 * * *", uc.DetectBreaches.Execute)
scheduler.AddFunc("30 7 * * *", uc.DetectExpiringSoon.Execute)
```

---

## Database Schema

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

-- sla_breach_events (partitioned by month)
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

-- Initial partition
CREATE TABLE sla_breach_events_2026
    PARTITION OF sla_breach_events
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE INDEX idx_sla_breach_product ON sla_breach_events(product_id, detected_at DESC);
CREATE INDEX idx_sla_breach_finding ON sla_breach_events(finding_id);
```

---

## NATS Events Published

```
sla.config.created           {sla_config_id, name}
sla.config.updated           {sla_config_id, changes}
sla.config.deleted           {sla_config_id}
sla.breached                 {finding_id, product_id, severity, expiration_date, days_overdue}
sla.expiring_soon            {finding_id, product_id, severity, expiration_date, days_remaining}
sla.bulk_recompute.started   {sla_config_id}
sla.bulk_recompute.completed {sla_config_id, updated_count}
```

---

## Acceptance Criteria

- [x] `POST /api/v2/sla-configurations` tạo config với Critical=7d, High=30d
- [x] Assign SLA config to product → mọi finding mới tự động có `sla_expiration_date`
- [x] `finding.created` NATS event → sla-service compute expiry trong < 1s
- [x] Daily 07:30: overdue findings → `sla.breached` events published
- [x] Daily 07:30: findings sắp hết hạn 7d → `sla.expiring_soon` events
- [x] Update SLA config → bulk recompute async cho tất cả findings
- [x] `GET /api/v2/sla-dashboard`: trả về compliance % + violation counts
- [x] `EnforceMedium=false`: Medium findings không có `sla_expiration_date`
- [x] `restart_sla_on_reactivation=false`: reactivated finding giữ SLA date cũ

## Implementation Status: ✅ DONE

> `sla-service/internal/domain/slaconfig/entity.go` — SLAConfiguration: ComputeExpirationDate, DaysForSeverity, EnforceCritical/High/Medium/Low flags
> `sla-service/internal/domain/{breach,assignment}/entity.go` — SLABreachEvent, ProductSLAAssignment
> `sla-service/internal/usecase/{config,compute,dashboard}/` — full CRUD + ComputeExpiry + BulkRecompute + DetectBreaches
> `sla-service/internal/infra/scheduler/cron.go` — daily 07:30 cron: DetectBreaches + DetectExpiringSoon
> `sla-service/migrations/001_sla_configurations.sql` — sla_configurations + product_sla_assignments tables
> `sla-service/migrations/002_sla_breach_events.sql` — sla_breach_events partitioned by RANGE(detected_at)
> `sla-service/internal/delivery/{http,grpc,event}/` — REST CRUD, gRPC SLAService, NATS subscriber
