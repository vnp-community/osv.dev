# TASK-GCV-027 — notification-service Domain + DB Schema

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-027 |
| **Service** | `notification-service` |
| **CR** | CR-GCV-006 |
| **Phase** | 3 — Notifications |
| **Priority** | 🟡 Medium |
| **Prerequisites** | — |

## Context

`notification-service` đã có skeleton. Task này implement domain entities (`Webhook`, `AlertSubscription`, `WebhookDelivery`) và tạo DB migration với đầy đủ indexes. Foundation cho TASK-GCV-028/029/030/031.

## Reference

- Solution: [SOL-GCV-006](../solutions/SOL-GCV-006-notification-webhook.md) §2.1

## Files to Create/Modify

```
MODIFY hoặc CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/domain/webhook/
                    (tìm cấu trúc domain hiện có, implement entities)
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/migrations/001_init.sql
```

**Đọc trước**: `notification-service/internal/domain/` để xác định cấu trúc skeleton hiện có.

## Implementation Spec

### Domain Entities

Implement/verify các entity files trong `notification-service/internal/domain/`:

```go
// webhook/webhook.go
type Webhook struct {
    ID        string
    URL       string
    Secret    string       // HMAC-SHA256 signing key
    Events    []EventType  // subscribed event types
    IsActive  bool
    OwnerID   string
    CreatedAt time.Time
    UpdatedAt time.Time
}

// EventType constants
type EventType string
const (
    EventNewKEV      EventType = "kev.new"
    EventNewCritical EventType = "cve.new.critical"
    EventNewHigh     EventType = "cve.new.high"
    EventHighEPSS    EventType = "cve.epss.high"
    EventVendorCVE   EventType = "cve.vendor"
    EventProductCVE  EventType = "cve.product"
)

// HasEvent returns true if webhook subscribes to this event type.
func (w *Webhook) HasEvent(e EventType) bool {
    for _, ev := range w.Events {
        if ev == e { return true }
    }
    return false
}

// WebhookDelivery records each delivery attempt.
type WebhookDelivery struct {
    ID          string
    WebhookID   string
    EventType   EventType
    Payload     string         // JSON payload
    StatusCode  *int
    Attempt     int
    Status      DeliveryStatus // "pending"|"delivered"|"failed"|"retrying"
    DeliveredAt *time.Time
    NextRetryAt *time.Time
    CreatedAt   time.Time
}

type DeliveryStatus string
const (
    DeliveryPending   DeliveryStatus = "pending"
    DeliveryDelivered DeliveryStatus = "delivered"
    DeliveryFailed    DeliveryStatus = "failed"
    DeliveryRetrying  DeliveryStatus = "retrying"
)

// AlertSubscription for vendor/product/kev alerts.
type AlertSubscription struct {
    ID          string
    OwnerID     string
    Type        SubscriptionType // "vendor"|"product"|"kev"
    Value       string           // e.g. "apache" for vendor
    MinSeverity string           // "CRITICAL"|"HIGH"|"MEDIUM"|"LOW"
    MinEPSS     *float64
    IsActive    bool
    CreatedAt   time.Time
}

type SubscriptionType string
const (
    SubscriptionVendor  SubscriptionType = "vendor"
    SubscriptionProduct SubscriptionType = "product"
    SubscriptionKEV     SubscriptionType = "kev"
)
```

### Repository Interfaces

```go
// domain/repository/webhook_repo.go
type WebhookRepository interface {
    Save(ctx context.Context, wh *entity.Webhook) error
    FindByID(ctx context.Context, id, ownerID string) (*entity.Webhook, error)
    FindByOwner(ctx context.Context, ownerID string) ([]*entity.Webhook, error)
    FindByEvent(ctx context.Context, event entity.EventType) ([]*entity.Webhook, error)
    Update(ctx context.Context, wh *entity.Webhook) error
    Delete(ctx context.Context, id, ownerID string) error
    SaveDelivery(ctx context.Context, d *entity.WebhookDelivery) error
    ListDeliveries(ctx context.Context, webhookID string, limit int) ([]*entity.WebhookDelivery, error)
    GetPendingRetries(ctx context.Context) ([]*entity.WebhookDelivery, error)
    UpdateDelivery(ctx context.Context, d *entity.WebhookDelivery) error
}

// domain/repository/subscription_repo.go
type SubscriptionRepository interface {
    Save(ctx context.Context, s *entity.AlertSubscription) error
    FindByOwner(ctx context.Context, ownerID string) ([]*entity.AlertSubscription, error)
    FindByVendor(ctx context.Context, vendor string) ([]*entity.AlertSubscription, error)
    Delete(ctx context.Context, id, ownerID string) error
}
```

### migrations/001_init.sql

```sql
-- notification-service initial schema

-- Webhooks
CREATE TABLE IF NOT EXISTS webhooks (
    id          TEXT        PRIMARY KEY,
    url         TEXT        NOT NULL,
    secret      TEXT        NOT NULL,
    events      TEXT[]      NOT NULL DEFAULT '{}',
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    owner_id    TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_webhooks_owner  ON webhooks(owner_id) WHERE is_active;
CREATE INDEX IF NOT EXISTS idx_webhooks_events ON webhooks USING GIN(events);

-- Webhook deliveries (audit trail)
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id              TEXT        PRIMARY KEY,
    webhook_id      TEXT        NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type      TEXT        NOT NULL,
    payload         TEXT        NOT NULL,
    status_code     INT         DEFAULT NULL,
    attempt         INT         NOT NULL DEFAULT 1,
    status          TEXT        NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','delivered','failed','retrying')),
    delivered_at    TIMESTAMPTZ DEFAULT NULL,
    next_retry_at   TIMESTAMPTZ DEFAULT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_deliveries_webhook
    ON webhook_deliveries(webhook_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deliveries_retry
    ON webhook_deliveries(next_retry_at)
    WHERE status = 'retrying';

-- Alert subscriptions
CREATE TABLE IF NOT EXISTS alert_subscriptions (
    id              TEXT        PRIMARY KEY,
    owner_id        TEXT        NOT NULL,
    type            TEXT        NOT NULL CHECK (type IN ('vendor','product','kev')),
    value           TEXT        NOT NULL,
    min_severity    TEXT        NOT NULL DEFAULT 'HIGH'
                    CHECK (min_severity IN ('CRITICAL','HIGH','MEDIUM','LOW')),
    min_epss        NUMERIC(8,6) DEFAULT NULL,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_owner
    ON alert_subscriptions(owner_id) WHERE is_active;
CREATE INDEX IF NOT EXISTS idx_subscriptions_vendor
    ON alert_subscriptions(type, lower(value))
    WHERE type IN ('vendor','product');
```

## Acceptance Criteria

- [x] `Webhook` entity có `HasEvent(EventType) bool` method
- [x] 6 EventType constants defined correctly
- [x] `WebhookDelivery.Status` có 4 valid states
- [x] `AlertSubscription` entity có `MinSeverity` và `MinEPSS` fields
- [x] `WebhookRepository` interface có đủ 11 methods
- [x] `SubscriptionRepository` interface có 4 methods
- [x] Migration tạo 3 tables với indexes và constraints đúng
- [x] Migration idempotent (IF NOT EXISTS everywhere)
- [x] CHECK constraints trên `status`, `type`, `min_severity`
- [x] `go build ./...` pass


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Verified directly from codebase.
