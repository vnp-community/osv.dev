# S2-FIND-01 — Thêm SLA Breach Publisher + Cron (finding-service)


## ✅ Execution Status: COMPLETED
## Metadata
- **Task ID**: S2-FIND-01
- **Service**: finding-service
- **Sprint**: 2 (P1)
- **Ước tính**: 3 giờ
- **Dependencies**: S1-FIND-02 (NATS publisher setup)
- **Spec nguồn**: `specs/develop/05_finding-service-upgrade.md` § "P1 — Thêm: SLA Breach Publisher + Cron"

## Context

```bash
# Đọc SLA use cases:
cat services/finding-service/internal/usecase/sla/sla_use_cases.go

# Đọc finding repo để biết query methods:
cat services/finding-service/internal/infra/postgres/finding_repo.go

# Đọc domain SLA entity:
cat services/finding-service/internal/domain/sla/entity.go
cat services/finding-service/internal/domain/finding/entity.go | grep -i "sla\|expire"

# Đọc existing scheduler nếu có:
ls services/finding-service/internal/scheduler/ 2>/dev/null || echo "no scheduler"
cat services/finding-service/cmd/server/main.go
```

## Goal

1. Tạo SLA event publisher với events `finding.sla_breached` và `finding.sla_due_soon`
2. Tạo SLA checker cron job chạy hàng ngày

## Files to Create

### File 1: `services/finding-service/internal/infra/messaging/nats/sla_publisher.go`

```go
package nats

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

const (
	SubjectSLABreached  = "finding.sla_breached"
	SubjectSLADueSoon   = "finding.sla_due_soon"
)

// SLABreachedEvent is published when a finding's SLA is overdue.
type SLABreachedEvent struct {
	FindingID   uuid.UUID `json:"finding_id"`
	ProductID   uuid.UUID `json:"product_id"`
	Severity    string    `json:"severity"`
	ExpiresAt   time.Time `json:"expires_at"`
	DaysOverdue int       `json:"days_overdue"`
}

// SLADueSoonEvent is published when a finding's SLA is about to expire.
type SLADueSoonEvent struct {
	FindingID     uuid.UUID `json:"finding_id"`
	ProductID     uuid.UUID `json:"product_id"`
	Severity      string    `json:"severity"`
	ExpiresAt     time.Time `json:"expires_at"`
	DaysRemaining int       `json:"days_remaining"`
}

// SLAEventPublisher publishes SLA-related events to NATS JetStream.
type SLAEventPublisher struct {
	js  nats.JetStreamContext
	log zerolog.Logger
}

// NewSLAEventPublisher creates a new SLAEventPublisher.
func NewSLAEventPublisher(js nats.JetStreamContext, log zerolog.Logger) *SLAEventPublisher {
	return &SLAEventPublisher{js: js, log: log}
}

// PublishSLABreached publishes a SLABreachedEvent.
func (p *SLAEventPublisher) PublishSLABreached(ctx context.Context, event SLABreachedEvent) {
	go p.publish(SubjectSLABreached, event)
}

// PublishSLADueSoon publishes a SLADueSoonEvent.
func (p *SLAEventPublisher) PublishSLADueSoon(ctx context.Context, event SLADueSoonEvent) {
	go p.publish(SubjectSLADueSoon, event)
}

func (p *SLAEventPublisher) publish(subject string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		p.log.Error().Err(err).Str("subject", subject).Msg("sla_publisher: marshal error")
		return
	}
	if _, err := p.js.Publish(subject, data); err != nil {
		p.log.Warn().Err(err).Str("subject", subject).Msg("sla_publisher: publish failed")
	}
}
```

### File 2: `services/finding-service/internal/scheduler/sla_checker.go`

```go
package scheduler

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/finding-service/internal/domain/finding"
	"github.com/osv/finding-service/internal/domain/repository"
	nats_infra "github.com/osv/finding-service/internal/infra/messaging/nats"
)

// SLAWarningDays defines how many days before breach to send warning.
const SLAWarningDays = 3

// SLACheckerJob checks for SLA breaches and upcoming expirations.
type SLACheckerJob struct {
	findingRepo repository.FindingRepository
	publisher   *nats_infra.SLAEventPublisher
	log         zerolog.Logger
}

// NewSLACheckerJob creates a new SLACheckerJob.
func NewSLACheckerJob(
	findingRepo repository.FindingRepository,
	publisher *nats_infra.SLAEventPublisher,
	log zerolog.Logger,
) *SLACheckerJob {
	return &SLACheckerJob{
		findingRepo: findingRepo,
		publisher:   publisher,
		log:         log,
	}
}

// Run executes the SLA check.
// Call this from a cron scheduler at daily 00:00 UTC.
func (j *SLACheckerJob) Run(ctx context.Context) {
	j.log.Info().Msg("sla_checker: starting daily SLA check")

	now := time.Now().UTC()
	checked := 0

	// === Check 1: Already breached ===
	breachedFindings, err := j.findingRepo.FindSLABreached(ctx)
	if err != nil {
		j.log.Error().Err(err).Msg("sla_checker: failed to query breached SLAs")
	} else {
		for _, f := range breachedFindings {
			daysOverdue := int(now.Sub(f.SLAExpirationDate).Hours() / 24)

			j.publisher.PublishSLABreached(ctx, nats_infra.SLABreachedEvent{
				FindingID:   f.ID,
				ProductID:   f.ProductID,
				Severity:    string(f.Severity),
				ExpiresAt:   f.SLAExpirationDate,
				DaysOverdue: daysOverdue,
			})

			j.log.Warn().
				Str("finding_id", f.ID.String()).
				Int("days_overdue", daysOverdue).
				Msg("sla_checker: SLA breach published")
			checked++
		}
	}

	// === Check 2: Due soon (within SLAWarningDays) ===
	warnDeadline := now.Add(time.Duration(SLAWarningDays) * 24 * time.Hour)
	dueSoonFindings, err := j.findingRepo.FindSLADueSoon(ctx, warnDeadline)
	if err != nil {
		j.log.Error().Err(err).Msg("sla_checker: failed to query due-soon SLAs")
	} else {
		for _, f := range dueSoonFindings {
			daysRemaining := int(time.Until(f.SLAExpirationDate).Hours() / 24)
			if daysRemaining < 0 {
				continue  // already handled in breach check
			}

			j.publisher.PublishSLADueSoon(ctx, nats_infra.SLADueSoonEvent{
				FindingID:     f.ID,
				ProductID:     f.ProductID,
				Severity:      string(f.Severity),
				ExpiresAt:     f.SLAExpirationDate,
				DaysRemaining: daysRemaining,
			})

			j.log.Info().
				Str("finding_id", f.ID.String()).
				Int("days_remaining", daysRemaining).
				Msg("sla_checker: SLA due-soon published")
			checked++
		}
	}

	j.log.Info().Int("checked", checked).Msg("sla_checker: daily check complete")
}
```

## Files to Extend

### Extend: `services/finding-service/internal/domain/repository/repository.go`

Thêm 2 query methods vào FindingRepository interface:

```go
type FindingRepository interface {
    // ... existing methods giữ nguyên ...

    // SLA queries (NEW)
    FindSLABreached(ctx context.Context) ([]finding.Finding, error)
    // Returns: active findings where sla_expiration_date < NOW()

    FindSLADueSoon(ctx context.Context, deadline time.Time) ([]finding.Finding, error)
    // Returns: active findings where sla_expiration_date BETWEEN NOW() AND deadline
}
```

### Extend: `services/finding-service/internal/infra/postgres/finding_repo.go`

Implement các methods mới:

```go
// Thêm vào cuối file:

func (r *findingRepo) FindSLABreached(ctx context.Context) ([]finding.Finding, error) {
    query := `
        SELECT id, product_id, severity, sla_expiration_date, title, status
        FROM findings
        WHERE sla_expiration_date < NOW()
          AND status IN ('active', 'new')  -- adjust based on actual status values
          AND risk_accepted = FALSE
        ORDER BY sla_expiration_date ASC
        LIMIT 1000
    `
    rows, err := r.db.Query(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    return scanFindings(rows)  // implement scanner
}

func (r *findingRepo) FindSLADueSoon(ctx context.Context, deadline time.Time) ([]finding.Finding, error) {
    query := `
        SELECT id, product_id, severity, sla_expiration_date, title, status
        FROM findings
        WHERE sla_expiration_date BETWEEN NOW() AND $1
          AND status IN ('active', 'new')
          AND risk_accepted = FALSE
        ORDER BY sla_expiration_date ASC
        LIMIT 1000
    `
    rows, err := r.db.Query(ctx, query, deadline)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    return scanFindings(rows)
}
```

### Extend: `services/finding-service/cmd/server/main.go`

```go
// Import:
// "github.com/robfig/cron/v3"  (nếu dùng cron library)
// scheduler_pkg "github.com/osv/finding-service/internal/scheduler"

// Sau khi khởi tạo publisher:
slaPublisher := nats_infra.NewSLAEventPublisher(js, logger)
slaChecker := scheduler_pkg.NewSLACheckerJob(findingRepo, slaPublisher, logger)

// Start daily cron:
c := cron.New()
c.AddFunc("0 0 * * *", func() {  // Daily at 00:00 UTC
    slaChecker.Run(context.Background())
})
c.Start()
defer c.Stop()
```

Nếu không dùng cron library, dùng ticker:
```go
go func() {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    // Run once immediately on startup
    slaChecker.Run(ctx)
    for {
        select {
        case <-ticker.C:
            slaChecker.Run(ctx)
        case <-ctx.Done():
            return
        }
    }
}()
```

## Verification

```bash
cd services/finding-service && go build ./...

# Test SLA checker (force run):
# Tạo finding với sla_expiration_date trong quá khứ
# Trigger slaChecker.Run(ctx) manually
# Check NATS: nats sub "finding.sla_*"
# Expected: receive SLABreachedEvent

# Test due-soon:
# Tạo finding với sla_expiration_date = NOW() + 2 days
# Run checker
# Expected: receive SLADueSoonEvent với days_remaining=2
```

## Notes

- Đọc actual `finding.Finding` struct để biết exact field names (ID, SLAExpirationDate, Status)
- Query SQL cần điều chỉnh `status IN ('active', 'new')` theo actual status values trong codebase
- `LIMIT 1000` để tránh quá tải — production nên dùng cursor-based pagination
- Cần `go get github.com/robfig/cron/v3` nếu muốn dùng cron syntax
