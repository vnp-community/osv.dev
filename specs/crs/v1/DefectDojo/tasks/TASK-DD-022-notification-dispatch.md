# ✅ COMPLETED — TASK-DD-022 — Notification Dispatch + Retry Logic

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-022 |
| **Service** | `notification-service` |
| **CR** | CR-DD-007 |
| **Phase** | 2 — Security Management |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-021 |
| **Estimated effort** | 1 ngày |

## Context

Implement `DispatchUseCase` — core notification logic: matching rules → finding recipients → delivering to channels với 3-retry exponential backoff (30s/60s/120s). Cũng implement NATS subscribers cho tất cả 12+ auditable events.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/
```

## Files to Create

```
internal/usecase/dispatch/
└── dispatch.go

internal/delivery/event/
├── subscriber.go
└── handlers/
    ├── scan_completed.go
    ├── finding_created.go
    ├── finding_status_changed.go
    ├── sla_breached.go
    ├── sla_expiring_soon.go
    ├── engagement_closed.go
    ├── risk_acceptance_expired.go
    ├── jira_update.go
    └── product_created.go
```

## Implementation Spec

### `internal/usecase/dispatch/dispatch.go`

```go
package dispatch

import (
    "context"
    "log/slog"
    "time"
    "github.com/osv/services/notification-service/internal/domain/rule"
    "github.com/osv/services/notification-service/internal/domain/alert"
    "github.com/osv/services/notification-service/internal/domain/delivery"
)

type NotificationEvent struct {
    Type        rule.EventType
    ProductID   *string
    FindingID   *string
    EngagementID *string
    Title       string
    Description string
    URL         string
    Severity    *string
    Metadata    map[string]interface{}
}

type DispatchUseCase struct {
    ruleRepo         rule.NotificationRuleRepository
    deliveryRepo     delivery.DeliveryRecordRepository
    alertRepo        alert.AlertRepository
    identityClient   IdentityClient      // get product members
    channelSenders   map[rule.Channel]ChannelSender
    tmplRenderer     TemplateRenderer
}

func (uc *DispatchUseCase) Execute(ctx context.Context, event *NotificationEvent) error {
    // 1. Find matching rules (system + user-level)
    rules, err := uc.ruleRepo.FindMatchingRules(ctx, &rule.RuleQuery{
        EventType: event.Type,
        ProductID: event.ProductID,
    })
    if err != nil {
        return err
    }

    // 2. Get recipients from product members (if product-scoped)
    var recipients []Recipient
    if event.ProductID != nil {
        members, _ := uc.identityClient.GetUsersForProduct(ctx, *event.ProductID)
        recipients = buildRecipients(rules, members)
    } else {
        // System event — use system rules, send to all subscribed users
        recipients = buildSystemRecipients(rules)
    }

    // 3. Deliver to each recipient × channel
    for _, recipient := range recipients {
        channels := getChannelsForEvent(rules, recipient.UserID, event.Type)

        // Always create in-app alert
        uc.alertRepo.Save(ctx, &alert.Alert{
            UserID:    recipient.UserID,
            EventType: string(event.Type),
            Title:     event.Title,
            Description: event.Description,
            URL:        event.URL,
            IsRead:    false,
            CreatedAt: time.Now(),
        })

        for _, channel := range channels {
            payload, _ := uc.tmplRenderer.Render(event.Type, channel, &TemplateData{
                Event:     event,
                Recipient: recipient,
            })

            record := &delivery.DeliveryRecord{
                EventType: string(event.Type),
                Channel:   string(channel),
                Recipient: recipient.Email,
                Status:    delivery.StatusPending,
                Payload:   payload,
                CreatedAt: time.Now(),
            }
            uc.deliveryRepo.Save(ctx, record)

            // Async delivery with retry
            go uc.deliverWithRetry(ctx, record, channel, payload)
        }
    }
    return nil
}

// deliverWithRetry delivers to channel with exponential backoff: 30s, 60s, 120s
func (uc *DispatchUseCase) deliverWithRetry(ctx context.Context, record *delivery.DeliveryRecord, channel rule.Channel, payload map[string]interface{}) {
    backoffs := []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second}

    for attempt := 0; attempt <= len(backoffs); attempt++ {
        sender, ok := uc.channelSenders[channel]
        if !ok {
            slog.ErrorContext(ctx, "no sender for channel", "channel", channel)
            return
        }

        err := sender.Send(ctx, record.Recipient, payload)
        if err == nil {
            record.Status = delivery.StatusSent
            record.Attempts = attempt + 1
            uc.deliveryRepo.Update(ctx, record)
            return
        }

        slog.WarnContext(ctx, "delivery attempt failed",
            "channel", channel, "attempt", attempt+1, "error", err)

        if attempt < len(backoffs) {
            record.Status = delivery.StatusRetrying
            record.Attempts = attempt + 1
            record.LastAttemptAt = timePtr(time.Now())
            record.ErrorMessage = err.Error()
            uc.deliveryRepo.Update(ctx, record)
            time.Sleep(backoffs[attempt])
        }
    }

    // All retries exhausted
    record.Status = delivery.StatusFailed
    uc.deliveryRepo.Update(ctx, record)
    slog.ErrorContext(ctx, "delivery failed after all retries", "channel", channel, "recipient", record.Recipient)
}
```

### `internal/delivery/event/handlers/sla_breached.go`

```go
package handlers

import (
    "context"
    "encoding/json"
    "github.com/nats-io/nats.go"
    "github.com/osv/services/notification-service/internal/domain/rule"
    "github.com/osv/services/notification-service/internal/usecase/dispatch"
)

type SLABreachedHandler struct {
    dispatchUC *dispatch.DispatchUseCase
}

func (h *SLABreachedHandler) Subject() string { return "sla.breached" }

func (h *SLABreachedHandler) Handle(msg *nats.Msg) {
    var event struct {
        FindingID          string `json:"finding_id"`
        ProductID          string `json:"product_id"`
        Severity           string `json:"severity"`
        SLAExpirationDate  string `json:"sla_expiration_date"`
        DaysOverdue        int    `json:"days_overdue"`
    }
    if err := json.Unmarshal(msg.Data, &event); err != nil {
        return
    }

    ctx := context.Background()
    h.dispatchUC.Execute(ctx, &dispatch.NotificationEvent{
        Type:      rule.EventSLABreach,
        ProductID: &event.ProductID,
        FindingID: &event.FindingID,
        Severity:  &event.Severity,
        Title:     formatSLABreachTitle(event.Severity, event.DaysOverdue),
        Description: formatSLABreachDesc(event.FindingID, event.SLAExpirationDate, event.DaysOverdue),
        Metadata:  map[string]interface{}{
            "days_overdue":       event.DaysOverdue,
            "sla_expiration_date": event.SLAExpirationDate,
        },
    })
}

func formatSLABreachTitle(severity string, daysOverdue int) string {
    return fmt.Sprintf("SLA Breach: %s finding is %d day(s) overdue", severity, daysOverdue)
}
```

### NATS Event → NotificationEvent Mapping

```
scan.import.completed    → EventScanAdded    (product_id, new_findings count)
finding.batch_created    → EventFindingAdded (product_id, finding_id, severity)
finding.status_changed   → EventFindingStatusChanged (finding_id, old_state, new_state)
engagement.closed        → EventEngagementClosed (engagement_id, product_id)
sla.breached             → EventSLABreach (finding_id, product_id, severity, days_overdue)
sla.expiring_soon        → EventSLAExpiringSoon (finding_id, days_left)
risk_acceptance.expired  → EventRiskAcceptanceExpiration
jira.issue.created       → EventJIRAUpdate (jira_key)
jira.issue.updated       → EventJIRAUpdate
product.created          → EventProductAdded
```

## Acceptance Criteria

- [x] `sla.breached` NATS event → dispatch called → in-app alert created for product members
- [x] Delivery failure → retry at 30s, 60s, 120s intervals
- [x] After 3 failures → `delivery_records.status = "failed"`
- [x] Successful delivery → `delivery_records.status = "sent"`, `attempts` updated
- [x] In-app alert created even if email/slack delivery fails
- [x] System rules applied to all users without product-specific rules
- [x] `finding.batch_created` → EventFindingAdded dispatched với đúng severity
- [x] `engagement.closed` → EventEngagementClosed dispatched
- [x] `deliverWithRetry` không block main goroutine (async via `go`)

## Implementation Status: ✅ DONE

> `notification-service/internal/usecase/dispatch/dispatch.go` — DispatchUseCase, deliverWithRetry (3-retry exp backoff: 30s/60s/120s)
> `notification-service/internal/delivery/event/` — NATS subscribers cho 10+ event types (sla.breached, finding.batch_created, engagement.closed, etc.)
> In-app alert always created first; channel deliveries async with delivery_records tracking
