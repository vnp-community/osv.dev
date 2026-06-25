# ✅ COMPLETED — TASK-DD-019 — SLA Compute Expiry + Breach Detection

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-019 |
| **Service** | `sla-service` |
| **CR** | CR-DD-006 |
| **Phase** | 2 — Security Management |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-018 |
| **Estimated effort** | 1 ngày |

## Context

Core SLA logic: tính toán `sla_expiration_date` cho findings mới và detect breaches. Subscribe NATS events để tính SLA khi finding được tạo. Chạy cron job hàng ngày để detect breaches và gửi `sla.breached` / `sla.expiring_soon` events.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/sla-service/
```

## Files to Create

```
internal/usecase/
├── compute/
│   └── compute_expiry.go       # Compute SLA date for new findings
└── breach/
    └── detect_breaches.go      # Daily breach detection job

internal/delivery/event/
└── subscriber.go               # Subscribe NATS events

internal/infra/grpc/client/
└── finding.go                  # gRPC client → finding-service
```

## Implementation Spec

### `internal/usecase/compute/compute_expiry.go`

```go
package compute

import (
    "context"
    "time"
    "github.com/osv/services/sla-service/internal/domain/config"
)

type ComputeExpiryInput struct {
    FindingID  string
    ProductID  string
    Severity   string
    FoundDate  time.Time  // date the finding was introduced
}

type ComputeExpiryResult struct {
    FindingID      string
    ExpirationDate time.Time
    SLADays        int
}

type ComputeExpiryUseCase struct {
    configRepo     config.Repository
    assignmentRepo config.AssignmentRepository
    findingClient  FindingServiceClient  // gRPC → finding-service
    logRepo        ComputationLogRepository
    eventPub       EventPublisher
}

func (uc *ComputeExpiryUseCase) Execute(ctx context.Context, in ComputeExpiryInput) (*ComputeExpiryResult, error) {
    // 1. Lookup SLA config for this product
    cfg, err := uc.getSLAConfig(ctx, in.ProductID)
    if err != nil || cfg == nil {
        return nil, ErrNoSLAConfig
    }

    // 2. Get SLA days for this severity
    sladays := uc.getSLADays(cfg, in.Severity)

    // 3. Compute expiration date
    expirationDate := in.FoundDate.UTC().Truncate(24*time.Hour).AddDate(0, 0, sladays)

    // 4. Update finding via gRPC → finding-service
    if err := uc.findingClient.BatchUpdateSLADates(ctx, []BatchSLAUpdate{
        {FindingID: in.FindingID, SLAExpirationDate: expirationDate},
    }); err != nil {
        return nil, err
    }

    // 5. Log computation
    uc.logRepo.Save(ctx, &ComputationLog{
        FindingID:          in.FindingID,
        ProductID:          in.ProductID,
        SLAConfigurationID: cfg.ID,
        Severity:           in.Severity,
        SLADays:            sladays,
        FoundDate:          in.FoundDate,
        ComputedExpiry:     expirationDate,
        ComputedAt:         time.Now(),
    })

    return &ComputeExpiryResult{
        FindingID:      in.FindingID,
        ExpirationDate: expirationDate,
        SLADays:        sladays,
    }, nil
}

func (uc *ComputeExpiryUseCase) getSLAConfig(ctx context.Context, productID string) (*config.SLAConfiguration, error) {
    // Check product-specific assignment first
    assignment, _ := uc.assignmentRepo.FindByProduct(ctx, productID)
    if assignment != nil {
        return uc.configRepo.FindByID(ctx, assignment.SLAConfigurationID)
    }
    // Fall back to default
    return uc.configRepo.FindDefault(ctx)
}

func (uc *ComputeExpiryUseCase) getSLADays(cfg *config.SLAConfiguration, severity string) int {
    switch severity {
    case "Critical": return cfg.Critical
    case "High":     return cfg.High
    case "Medium":   return cfg.Medium
    case "Low":      return cfg.Low
    default:         return cfg.Low  // Info treated as Low
    }
}
```

### `internal/usecase/breach/detect_breaches.go`

```go
package breach

import (
    "context"
    "log/slog"
    "time"
)

// DetectBreachesUseCase — runs daily at 05:00 UTC
// Streams active findings with sla_expiration_date < today → publishes sla.breached
// Also detects findings expiring in next 3, 7, 14 days → publishes sla.expiring_soon
type DetectBreachesUseCase struct {
    findingClient FindingServiceClient
    eventPub      EventPublisher
}

func (uc *DetectBreachesUseCase) Execute(ctx context.Context) error {
    today := time.Now().UTC().Truncate(24 * time.Hour)

    // Stream all active findings with expired SLA (server-side stream)
    stream, err := uc.findingClient.ListFindingsForSLACheck(ctx, &SLACheckRequest{
        ActiveOnly:     true,
        BreachedBefore: today.Format("2006-01-02"),
    })
    if err != nil {
        return err
    }

    breachCount := 0
    for {
        finding, err := stream.Recv()
        if err != nil { break } // EOF or error

        uc.eventPub.Publish(ctx, "sla.breached", map[string]any{
            "finding_id":          finding.ID,
            "product_id":          finding.ProductID,
            "severity":            finding.Severity,
            "sla_expiration_date": finding.SLAExpirationDate,
            "days_overdue":        int(today.Sub(finding.ParsedSLADate()).Hours() / 24),
            "_service":            "sla-service",
        })
        breachCount++
    }
    slog.InfoContext(ctx, "SLA breach detection completed", "breached_count", breachCount)

    // Detect "expiring soon" (within next 7 days)
    soonDeadline := today.AddDate(0, 0, 7)
    soonStream, _ := uc.findingClient.ListFindingsForSLACheck(ctx, &SLACheckRequest{
        ActiveOnly:     true,
        BreachedBefore: soonDeadline.Format("2006-01-02"),
    })
    for {
        finding, err := soonStream.Recv()
        if err != nil { break }
        daysLeft := int(finding.ParsedSLADate().Sub(today).Hours() / 24)
        if daysLeft >= 0 { // only future expiries (not already breached)
            uc.eventPub.Publish(ctx, "sla.expiring_soon", map[string]any{
                "finding_id": finding.ID,
                "product_id": finding.ProductID,
                "severity":   finding.Severity,
                "days_left":  daysLeft,
                "_service":   "sla-service",
            })
        }
    }

    return nil
}
```

### `internal/delivery/event/subscriber.go`

```go
package event

import (
    "context"
    "encoding/json"
    "log/slog"
    "github.com/nats-io/nats.go"
    "github.com/osv/services/sla-service/internal/usecase/compute"
)

type EventSubscriber struct {
    nc            *nats.Conn
    computeExpiry *compute.ComputeExpiryUseCase
}

func (s *EventSubscriber) Start(ctx context.Context) error {
    // Subscribe to finding batch created — compute SLA for each new finding
    _, err := s.nc.QueueSubscribe("finding.batch_created", "sla-service", func(msg *nats.Msg) {
        var event struct {
            FindingIDs []string `json:"finding_ids"`
            ProductID  string   `json:"product_id"`
        }
        if err := json.Unmarshal(msg.Data, &event); err != nil {
            slog.Error("failed to parse finding.batch_created event", "error", err)
            return
        }
        // Fetch finding details (severity, found_date) from finding-service
        // then compute SLA for each finding
        for _, fid := range event.FindingIDs {
            go s.processNewFinding(ctx, fid, event.ProductID)
        }
    })

    // Subscribe to finding status changed — when finding reopened, reset SLA
    s.nc.QueueSubscribe("finding.status_changed", "sla-service", func(msg *nats.Msg) {
        var event struct {
            FindingID string `json:"finding_id"`
            NewState  string `json:"new_state"`
        }
        json.Unmarshal(msg.Data, &event)
        if event.NewState == "active" { // reopened
            go s.processReopenedFinding(ctx, event.FindingID)
        }
    })

    return err
}
```

## Cron Schedule

```
ComputeExpiryUseCase   → triggered by NATS events (real-time)
DetectBreachesUseCase  → daily cron at "0 5 * * *" (05:00 UTC)
```

## Acceptance Criteria

- [x] `finding.batch_created` event → SLA computed cho mỗi finding trong < 100ms
- [x] Critical finding with found_date=today → sla_expiration_date = today + 7
- [x] High finding with found_date=today → sla_expiration_date = today + 30
- [x] Product-specific SLA config overrides default config
- [x] DetectBreachesUseCase: finding với expired SLA → `sla.breached` event published
- [x] DetectBreachesUseCase: finding expiring in 7 days → `sla.expiring_soon` event
- [x] Finding reopened (status=active) → SLA date recomputed
- [x] `sla.breached` event có đúng fields: finding_id, product_id, severity, days_overdue
- [x] Breach detection cron chạy daily at 05:00 UTC

## Implementation Status: ✅ DONE

> `sla-service/internal/usecase/compute/bulk_recompute.go` — BulkRecomputeUseCase (500-finding batch flush)
> `sla-service/internal/usecase/sla_compute.go` — ComputeExpiryUseCase, DetectBreachesUseCase
> NATS subscriber: `finding.batch_created` → ComputeExpiry cho mỗi finding
> NATS subscriber: `finding.status_changed` → recompute khi status=active
> Cron: DetectBreachesUseCase daily at 05:00 UTC, expiring_soon window = 7 days
