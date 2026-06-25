# S1-NOTIF-02 — Thêm NATS Finding Event Consumer (notification-service)

## ✅ Execution Status: COMPLETED
- **Executed**: 2026-06-13
- **Result**: `go build` + `go vet` PASSED
- **Files Created**:
  - `internal/infra/messaging/nats/finding_event_consumer.go` ← FindingEventConsumer
  - `internal/usecase/dispatch_alert/finding_events.go` ← new dispatch methods (additive)
- **Key Adjustments vs Spec**:
  - Dùng `jetstream.JetStream` + `CreateOrUpdateConsumer` (không dùng raw `nats.JetStreamContext`) — theo pattern VulnEventConsumer
  - Dispatch package name là `dispatch` (không phải `dispatch_alert`) — match actual file
  - Thêm CloudEvent unwrapper (shared/pkg/nats publish CloudEvents envelope)
  - `FindingEventConsumer` subscribe: sla.breached, sla.expiring_soon, batch_created, status_changed, ai.epss.updated
  - `DispatchEPSSSpike()` chỉ fire khi new_score > 0.7 AND old_score ≤ 0.7 (threshold crossing)
  - Stream name: `DEFECTDOJO-EVENTS` (tương đương `OSV-EVENTS` cho DD namespace)

## Metadata
- **Task ID**: S1-NOTIF-02
- **Service**: notification-service
- **Sprint**: 1 (P0)
- **Ước tính**: 2 giờ
- **Dependencies**: S1-NOTIF-01 (alert repo), S1-FIND-02 (finding publisher phải publish trước)
- **Spec nguồn**: `specs/develop/07_notification-service-upgrade.md` § "P0 — Thêm: Additional NATS Subscriptions"

## Context

```bash
# Đọc existing consumer để biết pattern:
cat services/notification-service/internal/infra/messaging/nats/consumer.go
cat services/notification-service/internal/infra/messaging/nats/bootstrap.go

# Đọc dispatch_alert UC:
cat services/notification-service/internal/usecase/dispatch_alert/dispatch.go
```

## Files to Create

### File: `services/notification-service/internal/infra/messaging/nats/finding_event_consumer.go`

```go
package nats

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"

	"github.com/osv/notification-service/internal/usecase/dispatch_alert"
)

// Finding event types subscribed from finding-service
const (
	SubjectFindingSLABreached    = "finding.sla_breached"
	SubjectFindingSLADueSoon     = "finding.sla_due_soon"
	SubjectScanJobCompleted      = "scan.job.completed"
	SubjectScanJobFailed         = "scan.job.failed"
	SubjectScanAgentOffline      = "scan.agent.offline"
	SubjectAIEPSSUpdated         = "ai.epss.updated"
)

// SLABreachedPayload is the event received from finding-service.
type SLABreachedPayload struct {
	FindingID   string    `json:"finding_id"`
	ProductID   string    `json:"product_id"`
	Severity    string    `json:"severity"`
	ExpiresAt   time.Time `json:"expires_at"`
	DaysOverdue int       `json:"days_overdue"`
}

// SLADueSoonPayload is the event received from finding-service.
type SLADueSoonPayload struct {
	FindingID     string `json:"finding_id"`
	DaysRemaining int    `json:"days_remaining"`
}

// ScanCompletedPayload is the event received from scan-service.
type ScanCompletedPayload struct {
	ScanID       string `json:"scan_id"`
	FindingCount int    `json:"finding_count"`
	ProductID    string `json:"product_id"`
}

// ScanFailedPayload is the event received from scan-service.
type ScanFailedPayload struct {
	ScanID  string `json:"scan_id"`
	Reason  string `json:"reason"`
}

// AgentOfflinePayload is the event received from scan-service.
type AgentOfflinePayload struct {
	AgentID   string    `json:"agent_id"`
	Hostname  string    `json:"hostname"`
	LastSeen  time.Time `json:"last_seen"`
}

// EPSSUpdatedPayload is the event received from ai-service.
type EPSSUpdatedPayload struct {
	CVEID      string  `json:"cve_id"`
	OldScore   float64 `json:"old_score"`
	NewScore   float64 `json:"new_score"`
	Percentile float64 `json:"percentile"`
}

// FindingEventConsumer subscribes to cross-service events and triggers notifications.
type FindingEventConsumer struct {
	js         nats.JetStreamContext
	dispatchUC *dispatch_alert.UseCase
	log        zerolog.Logger
}

// NewFindingEventConsumer creates a new FindingEventConsumer.
func NewFindingEventConsumer(
	js nats.JetStreamContext,
	dispatchUC *dispatch_alert.UseCase,
	log zerolog.Logger,
) *FindingEventConsumer {
	return &FindingEventConsumer{
		js:         js,
		dispatchUC: dispatchUC,
		log:        log,
	}
}

// Start subscribes to all relevant NATS subjects.
// This should be called in a goroutine.
func (c *FindingEventConsumer) Start(ctx context.Context) error {
	subs := []struct {
		subject string
		handler func(*nats.Msg)
	}{
		{SubjectFindingSLABreached, c.handleSLABreached},
		{SubjectFindingSLADueSoon, c.handleSLADueSoon},
		{SubjectScanJobCompleted, c.handleScanCompleted},
		{SubjectScanJobFailed, c.handleScanFailed},
		{SubjectScanAgentOffline, c.handleAgentOffline},
		{SubjectAIEPSSUpdated, c.handleEPSSSpike},
	}

	for _, s := range subs {
		if _, err := c.js.Subscribe(s.subject, s.handler, nats.Durable("notification-svc")); err != nil {
			c.log.Error().Err(err).Str("subject", s.subject).Msg("finding_consumer: subscribe failed")
			// Non-fatal: continue subscribing to other subjects
		} else {
			c.log.Info().Str("subject", s.subject).Msg("finding_consumer: subscribed")
		}
	}

	// Block until context cancelled
	<-ctx.Done()
	return ctx.Err()
}

func (c *FindingEventConsumer) handleSLABreached(msg *nats.Msg) {
	var payload SLABreachedPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		c.log.Warn().Err(err).Msg("finding_consumer: bad sla_breached payload")
		msg.Nak()
		return
	}

	c.log.Info().Str("finding_id", payload.FindingID).
		Int("days_overdue", payload.DaysOverdue).
		Msg("SLA breach notification triggered")

	// Dispatch via existing use case
	if err := c.dispatchUC.DispatchSLABreach(context.Background(), dispatch_alert.SLABreachRequest{
		FindingID:   payload.FindingID,
		ProductID:   payload.ProductID,
		Severity:    payload.Severity,
		DaysOverdue: payload.DaysOverdue,
	}); err != nil {
		c.log.Error().Err(err).Msg("finding_consumer: dispatch sla_breach failed")
		msg.Nak()
		return
	}
	msg.Ack()
}

func (c *FindingEventConsumer) handleSLADueSoon(msg *nats.Msg) {
	var payload SLADueSoonPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		msg.Nak()
		return
	}

	c.log.Info().Str("finding_id", payload.FindingID).
		Int("days_remaining", payload.DaysRemaining).
		Msg("SLA due-soon notification triggered")

	// Dispatch warning notification
	if err := c.dispatchUC.DispatchSLADueSoon(context.Background(), dispatch_alert.SLADueSoonRequest{
		FindingID:     payload.FindingID,
		DaysRemaining: payload.DaysRemaining,
	}); err != nil {
		msg.Nak()
		return
	}
	msg.Ack()
}

func (c *FindingEventConsumer) handleScanCompleted(msg *nats.Msg) {
	var payload ScanCompletedPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		msg.Nak()
		return
	}

	c.log.Info().Str("scan_id", payload.ScanID).
		Int("finding_count", payload.FindingCount).
		Msg("Scan completed notification triggered")

	// Notify relevant users/teams
	if err := c.dispatchUC.DispatchScanCompleted(context.Background(), dispatch_alert.ScanCompletedRequest{
		ScanID:       payload.ScanID,
		FindingCount: payload.FindingCount,
		ProductID:    payload.ProductID,
	}); err != nil {
		msg.Nak()
		return
	}
	msg.Ack()
}

func (c *FindingEventConsumer) handleScanFailed(msg *nats.Msg) {
	var payload ScanFailedPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		msg.Nak()
		return
	}
	c.log.Warn().Str("scan_id", payload.ScanID).Str("reason", payload.Reason).
		Msg("Scan failed notification triggered")

	c.dispatchUC.DispatchScanFailed(context.Background(), dispatch_alert.ScanFailedRequest{
		ScanID: payload.ScanID,
		Reason: payload.Reason,
	})
	msg.Ack()
}

func (c *FindingEventConsumer) handleAgentOffline(msg *nats.Msg) {
	var payload AgentOfflinePayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		msg.Nak()
		return
	}
	c.log.Warn().Str("agent_id", payload.AgentID).Msg("Agent offline notification triggered")
	msg.Ack()
}

func (c *FindingEventConsumer) handleEPSSSpike(msg *nats.Msg) {
	var payload EPSSUpdatedPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		msg.Nak()
		return
	}

	// Only notify if EPSS score crosses threshold (e.g., 0.7)
	if payload.NewScore > 0.7 && payload.OldScore <= 0.7 {
		c.log.Info().Str("cve_id", payload.CVEID).
			Float64("score", payload.NewScore).
			Msg("EPSS spike notification triggered")
	}
	msg.Ack()
}
```

## Files to Extend

### Extend: `services/notification-service/internal/usecase/dispatch_alert/dispatch.go`

Thêm các dispatch methods còn thiếu vào UseCase struct (không sửa existing):

```go
// Thêm methods mới vào UseCase:

type SLABreachRequest struct {
    FindingID   string
    ProductID   string
    Severity    string
    DaysOverdue int
}

func (uc *UseCase) DispatchSLABreach(ctx context.Context, req SLABreachRequest) error {
    // 1. Find rules matching product_id that have sla_breach channels
    // 2. For each channel, dispatch via existing channel adapters
    return nil  // implement based on existing dispatch pattern
}

type ScanCompletedRequest struct {
    ScanID       string
    FindingCount int
    ProductID    string
}

func (uc *UseCase) DispatchScanCompleted(ctx context.Context, req ScanCompletedRequest) error {
    return nil  // implement
}

func (uc *UseCase) DispatchScanFailed(ctx context.Context, req ScanFailedRequest) error {
    return nil  // implement
}

func (uc *UseCase) DispatchSLADueSoon(ctx context.Context, req SLADueSoonRequest) error {
    return nil  // implement
}
```

### Extend: `services/notification-service/cmd/server/main.go`

```go
// Khởi tạo consumer:
findingConsumer := nats_infra.NewFindingEventConsumer(js, dispatchAlertUC, logger)

// Start consumer (non-blocking):
go func() {
    if err := findingConsumer.Start(ctx); err != nil && err != context.Canceled {
        log.Error().Err(err).Msg("finding consumer stopped")
    }
}()
```

## Verification

```bash
cd services/notification-service && go build ./...

# Test flow:
# 1. Start notification-service
# 2. Publish test event via NATS CLI:
nats pub finding.sla_breached '{"finding_id":"xxx","severity":"CRITICAL","days_overdue":3}'
# 3. Check notification-service logs for "SLA breach notification triggered"
```

## Notes

- Nếu `dispatchUC.DispatchSLABreach()` chưa có, thêm method stub trước (return nil)
- Sử dụng `nats.Durable("notification-svc")` để đảm bảo at-least-once delivery
- `msg.Nak()` cho phép NATS retry sau một khoảng thời gian
