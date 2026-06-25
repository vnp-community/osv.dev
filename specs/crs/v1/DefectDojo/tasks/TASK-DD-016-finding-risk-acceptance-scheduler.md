# ✅ COMPLETED — TASK-DD-016 — Risk Acceptance Expiry Scheduler

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-016 |
| **Service** | `finding-service` |
| **CR** | CR-DD-005 |
| **Phase** | 2 — Security Management |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-DD-015 |
| **Estimated effort** | 0.5 ngày |

## Context

Tích hợp `ExpireRiskAcceptancesUseCase` vào scheduler chạy hàng ngày lúc 06:00 UTC trong finding-service.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/
```

## Files to Create/Modify

```
internal/scheduler/
├── scheduler.go              # Scheduler runner (nếu chưa có)
└── risk_acceptance_job.go    # Daily job

internal/app/
└── wire.go                   # DI — wire scheduler vào app
```

## Implementation Spec

### `internal/scheduler/risk_acceptance_job.go`

```go
package scheduler

import (
    "context"
    "log/slog"
    "time"

    rauc "github.com/osv/services/finding-service/internal/usecase/riskacceptance"
)

// RiskAcceptanceExpiryJob — runs daily at 06:00 UTC
type RiskAcceptanceExpiryJob struct {
    expireUC *rauc.ExpireRiskAcceptancesUseCase
}

func (j *RiskAcceptanceExpiryJob) Name() string { return "risk_acceptance_expiry" }

func (j *RiskAcceptanceExpiryJob) Schedule() string { return "0 6 * * *" } // cron: 06:00 daily

func (j *RiskAcceptanceExpiryJob) Run(ctx context.Context) {
    slog.InfoContext(ctx, "running risk acceptance expiry job")
    if err := j.expireUC.Execute(ctx); err != nil {
        slog.ErrorContext(ctx, "risk acceptance expiry job failed", "error", err)
    }
}
```

### `internal/scheduler/scheduler.go`

```go
package scheduler

import (
    "context"
    "log/slog"
    "time"
)

type Job interface {
    Name() string
    Schedule() string  // cron expression "M H D M DoW"
    Run(ctx context.Context)
}

type Scheduler struct {
    jobs []Job
}

func New(jobs ...Job) *Scheduler {
    return &Scheduler{jobs: jobs}
}

// Start begins all scheduled jobs in background goroutines
// Returns when ctx is cancelled
func (s *Scheduler) Start(ctx context.Context) {
    for _, job := range s.jobs {
        go s.runJob(ctx, job)
    }
    <-ctx.Done()
}

func (s *Scheduler) runJob(ctx context.Context, job Job) {
    slog.InfoContext(ctx, "scheduler registered job", "job", job.Name(), "schedule", job.Schedule())
    // Simple implementation: parse cron, compute next run, sleep, run
    // In production: use github.com/robfig/cron/v3
    for {
        next := computeNextRun(job.Schedule())
        select {
        case <-ctx.Done():
            return
        case <-time.After(time.Until(next)):
            job.Run(ctx)
        }
    }
}
```

### Wire into finding-service main

```go
// In internal/app/app.go or cmd/server/main.go, add:
scheduler := scheduler.New(
    &scheduler.RiskAcceptanceExpiryJob{ExpireUC: expireRiskAcceptanceUC},
    &scheduler.AutoCloseEngagementsJob{CloseUC: autoCloseEngagementsUC},
)
go scheduler.Start(ctx)
```

## Acceptance Criteria

- [x] Scheduler starts on service boot
- [x] `RiskAcceptanceExpiryJob` chạy at 06:00 UTC (verified via test with mocked time)
- [x] Job calls `ExpireRiskAcceptancesUseCase.Execute()` successfully
- [x] Job failure doesn't crash the service (error logged, service continues)
- [x] `go build ./...` thành công

## Implementation Status: ✅ DONE

> `finding-service/internal/scheduler/risk_acceptance_scheduler.go` — RiskAcceptanceExpiryJob, cron "0 6 * * *"
> `finding-service/internal/scheduler/sla_checker.go` — SLA checker job
> Scheduler wired vào `finding-service` main application via goroutine
