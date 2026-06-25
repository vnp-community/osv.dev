# S1-FIND-02 — Thêm NATS Event Publisher (finding-service)

## ✅ Execution Status: COMPLETED
- **Executed**: 2026-06-13
- **Result**: `go build` PASSED


## Metadata
- **Task ID**: S1-FIND-02
- **Service**: finding-service
- **Sprint**: 1 (P0)
- **Ước tính**: 1.5 giờ
- **Dependencies**: Không có (độc lập với S1-FIND-01)
- **Spec nguồn**: `specs/develop/05_finding-service-upgrade.md` § "P0 — Thêm: NATS Event Publisher"

## Context

```bash
cat services/finding-service/internal/infra/messaging/nats/bootstrap.go
cat services/finding-service/internal/usecase/finding/use_cases.go
# Đọc NATS publisher pattern từ data-service:
cat services/data-service/internal/infra/messaging/nats/alias_publisher.go
```

## Files to Create

### File: `services/finding-service/internal/infra/messaging/nats/finding_publisher.go`

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
	SubjectFindingCreated       = "finding.created"
	SubjectFindingStatusChanged = "finding.status_changed"
	SubjectFindingRiskAccepted  = "finding.risk_accepted"
)

// FindingCreatedEvent is published when a new finding is created.
type FindingCreatedEvent struct {
	FindingID    uuid.UUID `json:"finding_id"`
	ProductID    uuid.UUID `json:"product_id"`
	EngagementID uuid.UUID `json:"engagement_id"`
	Severity     string    `json:"severity"`
	Title        string    `json:"title"`
	CVE          string    `json:"cve,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// FindingStatusChangedEvent is published when a finding changes state.
type FindingStatusChangedEvent struct {
	FindingID  uuid.UUID `json:"finding_id"`
	ProductID  uuid.UUID `json:"product_id"`
	OldStatus  string    `json:"old_status"`
	NewStatus  string    `json:"new_status"`
	ChangedBy  uuid.UUID `json:"changed_by"`
	ChangedAt  time.Time `json:"changed_at"`
}

// FindingRiskAcceptedEvent is published when risk is accepted for a finding.
type FindingRiskAcceptedEvent struct {
	FindingID     uuid.UUID  `json:"finding_id"`
	AcceptedBy    uuid.UUID  `json:"accepted_by"`
	Justification string     `json:"justification"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	AcceptedAt    time.Time  `json:"accepted_at"`
}

// FindingEventPublisher publishes finding lifecycle events to NATS JetStream.
type FindingEventPublisher struct {
	js  nats.JetStreamContext
	log zerolog.Logger
}

// NewFindingEventPublisher creates a new FindingEventPublisher.
func NewFindingEventPublisher(js nats.JetStreamContext, log zerolog.Logger) *FindingEventPublisher {
	return &FindingEventPublisher{js: js, log: log}
}

// PublishCreated publishes a FindingCreatedEvent asynchronously.
func (p *FindingEventPublisher) PublishCreated(ctx context.Context, event FindingCreatedEvent) {
	go p.publish(SubjectFindingCreated, event)
}

// PublishStatusChanged publishes a FindingStatusChangedEvent asynchronously.
func (p *FindingEventPublisher) PublishStatusChanged(ctx context.Context, event FindingStatusChangedEvent) {
	go p.publish(SubjectFindingStatusChanged, event)
}

// PublishRiskAccepted publishes a FindingRiskAcceptedEvent asynchronously.
func (p *FindingEventPublisher) PublishRiskAccepted(ctx context.Context, event FindingRiskAcceptedEvent) {
	go p.publish(SubjectFindingRiskAccepted, event)
}

func (p *FindingEventPublisher) publish(subject string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		p.log.Error().Err(err).Str("subject", subject).Msg("finding_publisher: marshal error")
		return
	}
	if _, err := p.js.Publish(subject, data); err != nil {
		p.log.Warn().Err(err).Str("subject", subject).Msg("finding_publisher: publish failed")
	}
}
```

## Files to Extend

### Extend: `services/finding-service/internal/usecase/finding/use_cases.go`

```go
// 1. Thêm import:
import nats_infra "github.com/osv/finding-service/internal/infra/messaging/nats"

// 2. Thêm publisher field vào UseCases struct:
type UseCases struct {
    // ... existing fields giữ nguyên ...
    publisher *nats_infra.FindingEventPublisher  // optional
}

// 3. Thêm setter (không sửa constructor cũ):
func (uc *UseCases) WithPublisher(p *nats_infra.FindingEventPublisher) *UseCases {
    uc.publisher = p
    return uc
}

// 4. Trong Create() method, sau khi save thành công:
if uc.publisher != nil {
    uc.publisher.PublishCreated(ctx, nats_infra.FindingCreatedEvent{
        FindingID: created.ID,
        Severity:  string(created.Severity),
        Title:     created.Title,
        CVE:       created.CVE,
        CreatedAt: created.CreatedAt,
    })
}

// 5. Trong state transition methods (Verify, Mitigate, Reopen), sau khi save:
if uc.publisher != nil {
    uc.publisher.PublishStatusChanged(ctx, nats_infra.FindingStatusChangedEvent{
        FindingID: id,
        OldStatus: oldStatus,
        NewStatus: newStatus,
        ChangedAt: time.Now(),
    })
}
```

### Extend: `services/finding-service/internal/infra/messaging/nats/bootstrap.go`

```go
// Thêm JetStream stream cho finding events:
_, err = js.AddStream(&nats.StreamConfig{
    Name:     "FINDING_EVENTS",
    Subjects: []string{"finding.*"},
    MaxAge:   30 * 24 * time.Hour,  // 30 days
    Storage:  nats.FileStorage,
})
```

### Extend: `services/finding-service/cmd/server/main.go`

```go
// Khởi tạo publisher:
findingPublisher := nats_infra.NewFindingEventPublisher(js, logger)

// Wire vào use cases:
findingUC = findingUC.WithPublisher(findingPublisher)
```

## Verification

```bash
cd services/finding-service && go build ./...

# Test với NATS:
nats sub "finding.>"
# Tạo finding qua API → expect FindingCreatedEvent trên NATS
```
