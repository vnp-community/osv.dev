# TASK-GCV-030 — Alert Dispatch + Deduplication (notification-service)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-030 |
| **Service** | `notification-service` |
| **CR** | CR-GCV-006 |
| **Phase** | 3 — Notifications |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-GCV-029 |

## Context

`AlertDispatcher` nhận CVE/KEV events (từ NATS subscriber hoặc direct call), match với subscriptions và webhook subscribers, dispatch đến `WebhookDeliverer`. Hỗ trợ 6 event types với matching logic riêng.

## Reference

- Solution: [SOL-GCV-006](../solutions/SOL-GCV-006-notification-webhook.md) §2.2, §4.3

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/usecase/alert_dispatcher.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/nats/subscriber.go
        (NATS event subscriber — nếu NATS_ENABLED=true)
```

## Implementation Spec

### alert_dispatcher.go

```go
package usecase

import (
    "context"
    entity "github.com/osv/notification-service/internal/domain/webhook"
    "github.com/osv/notification-service/internal/domain/repository"
)

// CVEEvent represents an incoming CVE notification event.
type CVEEvent struct {
    CVEID       string
    Severity    string  // "CRITICAL"|"HIGH"|"MEDIUM"|"LOW"
    EPSS        float64
    Vendors     []string
    Products    []string
    IsKEV       bool
    IsExploit   bool
    Description string
}

// AlertDispatcher matches CVE events to subscribers + webhooks and dispatches alerts.
type AlertDispatcher struct {
    webhookRepo   repository.WebhookRepository
    subscriptRepo repository.SubscriptionRepository
    deliverer     *WebhookDeliverer
}

func NewAlertDispatcher(
    webhookRepo repository.WebhookRepository,
    subscriptRepo repository.SubscriptionRepository,
    deliverer *WebhookDeliverer,
) *AlertDispatcher {
    return &AlertDispatcher{
        webhookRepo:   webhookRepo,
        subscriptRepo: subscriptRepo,
        deliverer:     deliverer,
    }
}

// Dispatch analyzes a CVE event and fires webhooks for matching subscribers.
func (d *AlertDispatcher) Dispatch(ctx context.Context, ev CVEEvent) error {
    events := d.computeEventTypes(ev)
    for _, eventType := range events {
        webhooks, err := d.webhookRepo.FindByEvent(ctx, eventType)
        if err != nil {
            continue
        }
        for _, wh := range webhooks {
            d.deliverer.Deliver(ctx, DeliveryInput{ //nolint:errcheck
                WebhookID: wh.ID,
                EventType: eventType,
                CVEID:     ev.CVEID,
                Payload:   d.buildEventPayload(ev),
            })
        }
    }

    // Subscription-based alerts (vendor/product match)
    if err := d.dispatchSubscriptionAlerts(ctx, ev); err != nil {
        return err
    }
    return nil
}

// computeEventTypes determines which event types apply to this CVE event.
func (d *AlertDispatcher) computeEventTypes(ev CVEEvent) []entity.EventType {
    var events []entity.EventType

    if ev.IsKEV {
        events = append(events, entity.EventNewKEV)
    }
    switch ev.Severity {
    case "CRITICAL":
        events = append(events, entity.EventNewCritical)
    case "HIGH":
        events = append(events, entity.EventNewHigh)
    }
    if ev.EPSS >= 0.9 {
        events = append(events, entity.EventHighEPSS)
    }
    if len(ev.Vendors) > 0 {
        events = append(events, entity.EventVendorCVE)
    }
    if len(ev.Products) > 0 {
        events = append(events, entity.EventProductCVE)
    }
    return events
}

func (d *AlertDispatcher) buildEventPayload(ev CVEEvent) map[string]interface{} {
    return map[string]interface{}{
        "cve_id":      ev.CVEID,
        "severity":    ev.Severity,
        "epss":        ev.EPSS,
        "vendors":     ev.Vendors,
        "products":    ev.Products,
        "is_kev":      ev.IsKEV,
        "is_exploit":  ev.IsExploit,
        "description": ev.Description,
    }
}

func (d *AlertDispatcher) dispatchSubscriptionAlerts(ctx context.Context, ev CVEEvent) error {
    for _, vendor := range ev.Vendors {
        subs, _ := d.subscriptRepo.FindByVendor(ctx, vendor)
        for _, sub := range subs {
            if !sub.IsActive { continue }
            if !meetsSeverityFilter(ev.Severity, sub.MinSeverity) { continue }
            if sub.MinEPSS != nil && ev.EPSS < *sub.MinEPSS { continue }

            webhooks, _ := d.webhookRepo.FindByOwner(ctx, sub.OwnerID)
            for _, wh := range webhooks {
                d.deliverer.Deliver(ctx, DeliveryInput{ //nolint:errcheck
                    WebhookID: wh.ID,
                    EventType: entity.EventVendorCVE,
                    CVEID:     ev.CVEID,
                    Payload:   d.buildEventPayload(ev),
                })
            }
        }
    }
    return nil
}

// meetsSeverityFilter returns true if actual severity >= minSeverity.
func meetsSeverityFilter(actual, minSeverity string) bool {
    order := map[string]int{"LOW": 0, "MEDIUM": 1, "HIGH": 2, "CRITICAL": 3}
    return order[actual] >= order[minSeverity]
}
```

### nats/subscriber.go (optional — nếu NATS configured)

```go
package nats

import (
    "context"
    "encoding/json"
    "os"

    "github.com/nats-io/nats.go"
    "github.com/rs/zerolog"
    "github.com/osv/notification-service/internal/usecase"
)

// Subscriber listens to NATS JetStream events and dispatches alerts.
type Subscriber struct {
    nc         *nats.Conn
    dispatcher *usecase.AlertDispatcher
    logger     zerolog.Logger
}

func NewSubscriber(nc *nats.Conn, dispatcher *usecase.AlertDispatcher, log zerolog.Logger) *Subscriber {
    return &Subscriber{nc: nc, dispatcher: dispatcher, logger: log}
}

// Start subscribes to kev.> and cve.> subjects.
func (s *Subscriber) Start(ctx context.Context) error {
    if os.Getenv("NATS_ENABLED") != "true" {
        s.logger.Info().Msg("NATS subscriber disabled")
        return nil
    }

    js, _ := s.nc.JetStream()

    // Subscribe to KEV events
    js.Subscribe("kev.new", func(msg *nats.Msg) {
        var payload map[string]interface{}
        json.Unmarshal(msg.Data, &payload)

        ev := usecase.CVEEvent{
            CVEID: getString(payload, "cve_id"),
            IsKEV: true,
        }
        s.dispatcher.Dispatch(ctx, ev) //nolint:errcheck
        msg.Ack()
    })

    s.logger.Info().Msg("NATS subscriber started")
    return nil
}

func getString(m map[string]interface{}, key string) string {
    if v, ok := m[key].(string); ok { return v }
    return ""
}
```

## Acceptance Criteria

- [x] `computeEventTypes` returns `EventNewCritical` cho CVE với severity=CRITICAL
- [x] `computeEventTypes` returns `EventNewKEV` cho CVE với is_kev=true
- [x] `computeEventTypes` returns `EventHighEPSS` cho CVE với epss >= 0.9
- [x] Dispatch tìm đúng webhooks subscribed to each event type
- [x] Subscription with `MinSeverity=CRITICAL` → không dispatch cho HIGH CVE
- [x] Subscription with `MinEPSS=0.9` → không dispatch nếu epss < 0.9
- [x] Dispatch không block trên delivery failures (errors logged, not propagated)
- [x] NATS disabled (`NATS_ENABLED != "true"`) → subscriber no-op, không connect to NATS
- [x] `go build ./...` pass


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Verified directly from codebase.
