# ✅ COMPLETED — TASK-DD-020 — SLA Bulk Recompute + Dashboard API

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-020 |
| **Service** | `sla-service` |
| **CR** | CR-DD-006 |
| **Phase** | 2 — Security Management |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-DD-019 |
| **Estimated effort** | 1 ngày |

## Context

Implement `BulkRecomputeUseCase` (chạy khi SLA config được thay đổi/assign mới) và Dashboard/Violations REST API.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/sla-service/
```

## Files to Create

```
internal/usecase/
├── compute/
│   └── bulk_recompute.go
└── dashboard/
    └── sla_dashboard.go

internal/delivery/http/
├── dashboard_handler.go
└── violations_handler.go
```

## Implementation Spec

### `internal/usecase/compute/bulk_recompute.go`

```go
package compute

import (
    "context"
    "log/slog"
    "github.com/osv/services/sla-service/internal/domain/config"
)

// BulkRecomputeUseCase — recomputes SLA dates for all active findings in a product
// Triggered when: new SLA config assigned to product, SLA config updated
type BulkRecomputeUseCase struct {
    findingClient FindingServiceClient
    computeExpiry *ComputeExpiryUseCase
}

func (uc *BulkRecomputeUseCase) Execute(ctx context.Context, productID string, cfg *config.SLAConfiguration) error {
    slog.InfoContext(ctx, "starting SLA bulk recompute", "product_id", productID)

    // Stream all active findings for product via gRPC server-stream
    stream, err := uc.findingClient.ListFindingsForSLACheck(ctx, &SLACheckRequest{
        ActiveOnly: true,
        ProductID:  &productID,
    })
    if err != nil {
        return err
    }

    var batchUpdates []BatchSLAUpdate
    processed := 0

    for {
        finding, err := stream.Recv()
        if err != nil {
            break // EOF or error
        }

        sladays := computeSLADays(cfg, finding.Severity)
        expirationDate := finding.FoundDate.UTC().Truncate(24*time.Hour).AddDate(0, 0, sladays)

        batchUpdates = append(batchUpdates, BatchSLAUpdate{
            FindingID:         finding.ID,
            SLAExpirationDate: expirationDate,
        })
        processed++

        // Flush batch every 500 findings
        if len(batchUpdates) >= 500 {
            uc.findingClient.BatchUpdateSLADates(ctx, batchUpdates)
            batchUpdates = batchUpdates[:0]
        }
    }

    // Flush remaining
    if len(batchUpdates) > 0 {
        uc.findingClient.BatchUpdateSLADates(ctx, batchUpdates)
    }

    slog.InfoContext(ctx, "SLA bulk recompute completed", "product_id", productID, "processed", processed)
    return nil
}
```

### `internal/usecase/dashboard/sla_dashboard.go`

```go
package dashboard

import (
    "context"
)

type SLADashboardOutput struct {
    TotalActive       int
    WithinSLA         int     // active + sla_expiration_date >= today
    SLABreached       int     // active + sla_expiration_date < today
    BreachRatePct     float64 // SLABreached/TotalActive * 100
    ExpiringSoon7Days int     // expiring within 7 days
    BySeverity        map[string]SLASeverityStat
    ByProduct         []SLAProductStat
}

type SLASeverityStat struct {
    Total       int
    Breached    int
    BreachPct   float64
    AvgDaysLeft float64
}

type SLAProductStat struct {
    ProductID   string
    ProductName string
    BreachCount int
    TotalActive int
    BreachPct   float64
}

type SLADashboardUseCase struct {
    findingClient FindingServiceClient
}

func (uc *SLADashboardUseCase) Execute(ctx context.Context, productID *string) (*SLADashboardOutput, error) {
    // Query finding-service for aggregate stats via gRPC
    counts, _ := uc.findingClient.GetSeverityCounts(ctx, &SeverityCountRequest{
        ProductID:  productID,
        ActiveOnly: true,
    })

    breachedStream, _ := uc.findingClient.ListFindingsForSLACheck(ctx, &SLACheckRequest{
        ActiveOnly:     true,
        BreachedBefore: time.Now().UTC().Format("2006-01-02"),
        ProductID:      productID,
    })

    // Aggregate into dashboard stats
    // ...

    return &SLADashboardOutput{
        TotalActive:   int(counts.Total),
        // ... computed from breached stream
    }, nil
}
```

### REST Endpoints (`dashboard_handler.go` + `violations_handler.go`)

```go
// GET /api/v2/sla-dashboard?product_id=uuid
// Response: SLADashboardOutput JSON
// {
//   "total_active": 245,
//   "within_sla": 198,
//   "sla_breached": 47,
//   "breach_rate_pct": 19.2,
//   "expiring_soon_7_days": 12,
//   "by_severity": {
//     "Critical": {"total": 5, "breached": 2, "breach_pct": 40},
//     "High": {"total": 42, "breached": 15, "breach_pct": 35.7},
//     ...
//   }
// }

// GET /api/v2/sla-violations?severity=High&days_overdue_min=1
// Response: paginated list of breached findings with SLA context
// {
//   "count": 47,
//   "results": [{
//     "finding_id": "uuid",
//     "title": "CVE-2021-44228",
//     "severity": "Critical",
//     "sla_expiration_date": "2026-01-07",
//     "days_overdue": 159,
//     "product_id": "uuid"
//   }]
// }

// GET /api/v2/sla-violations/{product_id}
// Response: violations for specific product
```

## Acceptance Criteria

- [x] `BulkRecomputeUseCase` processes 10,000 findings without timeout
- [x] Batch flush every 500 findings (not one-by-one)
- [x] `GET /api/v2/sla-dashboard` → dashboard stats object
- [x] `GET /api/v2/sla-dashboard?product_id=X` → product-scoped stats
- [x] `breach_rate_pct` correctly computed as percentage
- [x] `GET /api/v2/sla-violations` → list của breached findings sorted by `days_overdue` DESC
- [x] `GET /api/v2/sla-violations/{product_id}` → product-specific violations
- [x] `GET /api/v2/sla-violations?severity=Critical` → filter by severity
- [x] BulkRecompute logs start + completion with count

## Implementation Status: ✅ DONE

> `sla-service/internal/usecase/compute/bulk_recompute.go` — BulkRecomputeUseCase: gRPC stream, 500-finding batch, slog logging
> `sla-service/internal/usecase/dashboard/sla_dashboard.go` — SLADashboardUseCase, ViolationRepository interface, ViolationFilter
> `sla-service/internal/delivery/http/dashboard_handler.go` — DashboardHandler + ViolationsHandler (List + ListByProduct)
> Violation filter supports: product_id, severity, days_overdue_min, limit/offset pagination
