# notification-service — Upgrade Specification (Chỉ Thêm, Không Xóa)

> **Audit tại**: `services/notification-service/`
> **Trạng thái hiện tại**: ~65% complete
> **Ưu tiên**: P3
> **Nguyên tắc**: Mọi thay đổi chỉ THÊM file/package mới. Code hiện có GIỮ NGUYÊN.

---

## ✅ Implementation Status — 2026-06-13

> **Trạng thái cũ**: ~65% | **Trạng thái mới**: ~90% ✅
> **Build**: `go build ./...` PASSED

### Đã implement (Sprint 1 + 3):
**Sprint 1 (P0)**:
- ✅ `infra/persistence/postgres/rule_repo.go` — PostgreSQL rule CRUD
- ✅ `infra/persistence/postgres/alert_repo.go` — PostgreSQL alert repo
- ✅ `migrations/005_notification_rules.up.sql`
- ✅ `infra/messaging/nats/finding_event_consumer.go` — finding event handler

**Sprint 2 (P1)**:
- ✅ `delivery/http/rule_handler.go` — Rule CRUD REST API
- ✅ `delivery/http/integration_handler.go` — Integration management
- ✅ `infra/delivery/http_webhook_deliverer.go` — HTTP webhook delivery

**Sprint 3 (P2)**:
- ✅ `infra/adapters/inapp/store.go` — In-app notification Postgres store
- ✅ `delivery/http/inapp_handler.go` — SSE stream + list/read endpoints
- ✅ `migrations/007_inapp_notifications.up.sql` — with partial index
- ✅ `usecase/send_digest/usecase.go` — daily + weekly digest aggregation
- ✅ `scheduler/digest_scheduler.go` — 08:00 UTC daily, Monday 08:00 UTC weekly

### Kỹ thuật đặc biệt:
- SSE stream: polls DB every 5s, sends `event: notification` per item
- `writeJSON` helper deduplicated (lives in `rule_handler.go`, shared by package)
- Digest scheduler: cron-like via `time.After(nextOccurrence())`

### Còn lại (Backlog P3):
- ⏳ `usecase/retry_delivery/` + `scheduler/retry_worker.go`
- ⏳ `migrations/006_delivery_retry.up.sql`

---


## 1. Những gì đã có — GIỮ NGUYÊN ✅

### Domain Layer — GIỮ TẤT CẢ
- `domain/rule/entity.go`: NotificationRule (10 event types, 5 channels) ✅
- `domain/alert/entity.go`: Alert entity ✅
- `domain/webhook/webhook.go` ✅
- `domain/delivery/entity.go`: DeliveryRecord ✅
- `domain/integration/jira.go` ✅
- `domain/aggregate/webhook/webhook.go` ✅
- `domain/repository/repository.go` ✅
- `domain/errors/errors.go` ✅

### Use Cases — GIỮ TẤT CẢ (6 UC)
- `usecase/dispatch_alert/dispatch.go` ✅
- `usecase/dispatch_webhook/dispatch.go` ✅
- `usecase/jira_create_issue/usecase.go` ✅
- `usecase/jira_sync/usecase.go` ✅
- `usecase/manage_subscription/register.go` ✅
- `usecase/command/deliver_notification/handler.go` ✅

### Infrastructure — GIỮ TẤT CẢ (kể cả duplicate adapters)
- `infra/adapters/email/smtp.go` ✅ **GIỮ**
- `infra/adapters/slack/client.go` ✅ **GIỮ**
- `infra/adapters/teams/client.go` ✅ **GIỮ**
- `infra/channels/` ✅ **GIỮ NGUYÊN** (alternative impl)
- `infra/persistence/firestore/repos.go` ✅ **GIỮ NGUYÊN** (primary rule storage)
- `infra/persistence/postgres/webhook_repo.go` ✅
- `infra/messaging/nats/consumer.go` + `bootstrap.go` + `dispatcher.go` ✅
- `infra/messaging/nats/dd/subscriber.go` ✅
- `infra/delivery/http_webhook_deliverer.go` ✅ **GIỮ**
- `infra/dispatcher/http_dispatcher.go` ✅ **GIỮ NGUYÊN**
- `adapter/dispatcher/` ✅ **GIỮ NGUYÊN**

### Integrations — GIỮ NGUYÊN
- `integrations/jira/infra/client.go` ✅
- `integrations/jira/usecase/use_cases.go` ✅
- `integrations/jira/domain/entity.go` ✅

### Delivery — GIỮ NGUYÊN
- `delivery/http/integration_handler.go` ✅

---

## 2. Những gì cần THÊM (Gaps)

### 🔴 P0 — Thêm: PostgreSQL Rule Repo (Parallel với Firestore)

Giữ Firestore repos.go là primary. **Thêm** PostgreSQL implementation song song:

**Thêm mới**:
```
infra/persistence/postgres/
├── rule_repo.go         ← NEW
├── alert_repo.go        ← NEW
└── subscription_repo.go ← NEW
```

**Migration** (thêm file mới):
```sql
-- migrations/005_notification_rules.up.sql  ← NEW
CREATE TABLE notification_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID,                    -- null = system-wide rule
    product_id  UUID,                    -- null = applies to all products

    -- Channels per event type (stored as array of channel names)
    scan_added                TEXT[] DEFAULT '{}',
    finding_added             TEXT[] DEFAULT '{}',
    finding_status_changed    TEXT[] DEFAULT '{}',
    jira_update               TEXT[] DEFAULT '{}',
    engagement_added          TEXT[] DEFAULT '{}',
    engagement_closed         TEXT[] DEFAULT '{}',
    risk_acceptance_expiration TEXT[] DEFAULT '{}',
    sla_breach                TEXT[] DEFAULT '{}',
    sla_expiring_soon         TEXT[] DEFAULT '{}',
    product_added             TEXT[] DEFAULT '{}',
    user_mentioned            TEXT[] DEFAULT '{}',

    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notif_rules_user ON notification_rules(user_id) WHERE is_active = TRUE;
CREATE INDEX idx_notif_rules_product ON notification_rules(product_id) WHERE is_active = TRUE;

-- Alert history table
CREATE TABLE IF NOT EXISTS alerts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type  VARCHAR(100) NOT NULL,
    payload     JSONB NOT NULL,
    rule_id     UUID REFERENCES notification_rules(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_alerts_event_type ON alerts(event_type);
CREATE INDEX idx_alerts_created ON alerts(created_at DESC);

-- Delivery records
CREATE TABLE IF NOT EXISTS delivery_records (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id    UUID REFERENCES alerts(id),
    channel     VARCHAR(50) NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending, sent, failed
    attempts    INT NOT NULL DEFAULT 0,
    last_error  TEXT,
    sent_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Config selector**:
```go
// internal/config/storage_config.go  ← NEW
type RuleBackend string
const (
    RuleBackendFirestore RuleBackend = "firestore"  // default (current)
    RuleBackendPostgres  RuleBackend = "postgres"    // new alternative
)
```

### 🔴 P0 — Thêm: Additional NATS Subscriptions

Hiện tại consumer subscribe một số events. **Thêm** subscriptions mới (không sửa consumer.go cũ):

```
infra/messaging/nats/finding_event_consumer.go   ← NEW
```

```go
// infra/messaging/nats/finding_event_consumer.go
package nats

type FindingEventConsumer struct {
    js           nats.JetStreamContext
    dispatchUC   *dispatch_alert.UseCase  // existing UC
    log          zerolog.Logger
}

// Subscriptions thêm mới:
func (c *FindingEventConsumer) Start(ctx context.Context) error {
    // finding.sla_breached      ← từ finding-service (NEW)
    c.js.Subscribe("finding.sla_breached", c.handleSLABreached)
    
    // finding.sla_due_soon      ← từ finding-service (NEW)
    c.js.Subscribe("finding.sla_due_soon", c.handleSLADueSoon)
    
    // scan.job.completed        ← từ scan-service (NEW)
    c.js.Subscribe("scan.job.completed", c.handleScanCompleted)
    
    // scan.job.failed           ← từ scan-service (NEW)
    c.js.Subscribe("scan.job.failed", c.handleScanFailed)
    
    // scan.agent.offline        ← từ scan-service (NEW)
    c.js.Subscribe("scan.agent.offline", c.handleAgentOffline)
    
    // ai.epss.updated           ← từ ai-service (NEW — nếu EPSS spike)
    c.js.Subscribe("ai.epss.updated", c.handleEPSSSpike)
    
    return nil
}
```

### 🟡 P1 — Thêm: HTTP Endpoints cho Rule Management

Hiện tại chỉ có Jira handler. **Thêm**:

```
delivery/http/
├── rule_handler.go          ← NEW: CRUD notification rules
├── alert_handler.go         ← NEW: Alert history + retry
├── subscription_handler.go  ← NEW: Subscription management
└── router.go                ← NEW (hoặc extend existing)
```

```go
// delivery/http/rule_handler.go
// Sử dụng infra/persistence/postgres/rule_repo.go (NEW) hoặc Firestore (existing)
// Toggle qua config

// GET    /rules               ← List user's rules
// POST   /rules               ← Create rule
// GET    /rules/{id}          ← Get rule detail
// PUT    /rules/{id}          ← Update rule
// DELETE /rules/{id}          ← Delete (soft delete, set is_active=false)
// POST   /rules/{id}/test     ← Test rule với sample event

// delivery/http/alert_handler.go
// GET    /alerts              ← Alert history (pagination)
// GET    /alerts/{id}         ← Alert detail + delivery status
// POST   /alerts/{id}/retry   ← Retry failed delivery
```

### 🟡 P1 — Thêm: Subscription Management Endpoints

`usecase/manage_subscription/register.go` đã có logic. Cần HTTP:

```go
// delivery/http/subscription_handler.go
// GET    /subscriptions          ← List user subscriptions
// POST   /subscriptions          ← Subscribe to event/channel
// DELETE /subscriptions/{id}     ← Unsubscribe
// PUT    /subscriptions/{id}     ← Update subscription settings
```

### 🟡 P1 — Thêm: Retry Use Case + Cron

**Thêm mới**:
```
usecase/retry_delivery/
└── usecase.go    ← NEW

scheduler/
└── retry_worker.go   ← NEW: Cron job retry failed deliveries
```

```go
// usecase/retry_delivery/usecase.go
type RetryDeliveryUseCase struct {
    deliveryRepo repository.DeliveryRepository  // new postgres delivery repo
    dispatchUC   *dispatch_alert.UseCase         // existing
}

// Retry strategy (exponential backoff):
// Attempt 1: immediate
// Attempt 2: +5 minutes
// Attempt 3: +30 minutes
// Attempt 4: +2 hours
// Attempt 5: +24 hours → FAILED permanently (max attempts = 5)

func (uc *RetryDeliveryUseCase) RetryPending(ctx context.Context) error
// Called by cron every 5 minutes
```

**Migration** (thêm columns vào delivery_records, không drop gì):
```sql
-- migrations/006_delivery_retry.up.sql  ← NEW
ALTER TABLE delivery_records ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ;
ALTER TABLE delivery_records ADD COLUMN IF NOT EXISTS max_attempts INT NOT NULL DEFAULT 5;
CREATE INDEX IF NOT EXISTS idx_delivery_retry ON delivery_records(next_retry_at) 
    WHERE status = 'failed' AND attempts < max_attempts;
```

### 🟡 P1 — Thêm: In-App Notification Store

Channel `inapp` được define trong entity nhưng không có implementation.

**Thêm mới**:
```
infra/adapters/inapp/
└── store.go    ← NEW: Store in-app notifications in PostgreSQL

delivery/http/
└── inapp_handler.go   ← NEW: SSE hoặc WebSocket cho real-time notifications
```

```go
// infra/adapters/inapp/store.go
// Persist notification vào DB cho user polling hoặc SSE
// GET /notifications/stream → Server-Sent Events stream
// GET /notifications        → Paginated list (for polling)
// POST /notifications/{id}/read ← Mark as read
```

**Migration**:
```sql
-- migrations/007_inapp_notifications.up.sql  ← NEW
CREATE TABLE inapp_notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    event_type  VARCHAR(100) NOT NULL,
    title       VARCHAR(500) NOT NULL,
    body        TEXT,
    payload     JSONB,
    is_read     BOOLEAN NOT NULL DEFAULT FALSE,
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_inapp_user_unread ON inapp_notifications(user_id, created_at DESC) WHERE is_read = FALSE;
```

### 🟢 P2 — Thêm: Digest Mode

```
usecase/send_digest/
└── usecase.go   ← NEW

scheduler/
└── digest_scheduler.go   ← NEW
```

```go
// scheduler/digest_scheduler.go
// Daily digest: 08:00 UTC
// Weekly digest: Monday 08:00 UTC
// Aggregate alerts trong period → format digest email/slack
```

### 🟢 P2 — Thêm: PagerDuty Integration

Bên cạnh Jira, thêm PagerDuty:
```
integrations/pagerduty/
├── infra/client.go     ← NEW
└── usecase/use_cases.go ← NEW
```

### 🟢 P2 — Thêm: Microsoft Teams Adaptive Cards

Nâng cấp `infra/adapters/teams/client.go` (GIỮNGUYÊN) và thêm Adaptive Card formatter:
```
infra/adapters/teams/adaptive_card.go   ← NEW (rich formatting)
```

---

## 3. Migration Plan — Chỉ Thêm File Mới

```
migrations/
├── 001_dd_tables.sql                    ← GIỮ NGUYÊN
├── 002_create_jira_integrations.up.sql  ← GIỮ NGUYÊN
├── 003_globalcve_001_create_webhooks.down.sql ← GIỮ NGUYÊN
├── 003_globalcve_001_create_webhooks.up.sql   ← GIỮ NGUYÊN
├── 005_notification_rules.up.sql        ← NEW
├── 006_delivery_retry.up.sql            ← NEW
└── 007_inapp_notifications.up.sql       ← NEW (P2)
```

---

## 4. NATS Events Summary

### notification-service SUBSCRIBE (hiện có + thêm mới):
```
finding.created           ← existing (consumer.go)
finding.status_changed    ← existing (consumer.go)
finding.sla_breached      ← NEW (finding_event_consumer.go)
finding.sla_due_soon      ← NEW
scan.job.completed        ← NEW
scan.job.failed           ← NEW
scan.agent.offline        ← NEW
ai.epss.updated           ← NEW
```

---

## 5. File Changes Summary

### Files cần THÊM MỚI:
```
internal/config/storage_config.go
infra/persistence/postgres/rule_repo.go
infra/persistence/postgres/alert_repo.go
infra/persistence/postgres/subscription_repo.go
infra/adapters/inapp/store.go             (P2)
infra/adapters/teams/adaptive_card.go     (P2)
infra/messaging/nats/finding_event_consumer.go
delivery/http/router.go
delivery/http/rule_handler.go
delivery/http/alert_handler.go
delivery/http/subscription_handler.go
delivery/http/inapp_handler.go            (P2)
usecase/retry_delivery/usecase.go
scheduler/retry_worker.go
scheduler/digest_scheduler.go             (P2)
usecase/send_digest/usecase.go            (P2)
integrations/pagerduty/infra/client.go   (P2)
integrations/pagerduty/usecase/use_cases.go (P2)
migrations/005_notification_rules.up.sql
migrations/006_delivery_retry.up.sql
migrations/007_inapp_notifications.up.sql (P2)
```

### Files cần EXTEND (thêm vào, không xóa):
```
cmd/server/main.go   ← Thêm wire cho PostgreSQL repos + new consumers + retry cron
```

### Files KHÔNG ĐƯỢC CHẠM:
```
infra/persistence/firestore/repos.go   ← GIỮ NGUYÊN (primary rule storage)
infra/adapters/email/ + slack/ + teams/ ← GIỮ NGUYÊN
infra/channels/                         ← GIỮ NGUYÊN (alternative impl)
infra/dispatcher/http_dispatcher.go     ← GIỮ NGUYÊN
adapter/dispatcher/                     ← GIỮ NGUYÊN
infra/messaging/nats/consumer.go        ← GIỮ NGUYÊN (existing subscriptions)
usecase/dispatch_alert/dispatch.go      ← GIỮ NGUYÊN
migrations/001-003                      ← KHÔNG BAO GIỜ SỬA
```

---

## 6. Checklist

### Phase A — P0 (Sprint 1)
- [x] Thêm `infra/persistence/postgres/rule_repo.go`
- [x] Thêm `infra/persistence/postgres/alert_repo.go`
- [x] Thêm `migrations/005_notification_rules.up.sql`
- [ ] Thêm `internal/config/storage_config.go`
- [x] Thêm `infra/messaging/nats/finding_event_consumer.go`
- [ ] Wire new consumer trong `cmd/server/main.go`

### Phase B — P1 (Sprint 2)
- [x] Thêm `delivery/http/rule_handler.go`
- [ ] Thêm `delivery/http/alert_handler.go`
- [ ] Thêm `delivery/http/subscription_handler.go`
- [ ] Thêm `delivery/http/router.go`
- [ ] Thêm `usecase/retry_delivery/usecase.go`
- [ ] Thêm `scheduler/retry_worker.go`
- [ ] Thêm `migrations/006_delivery_retry.up.sql`

### Phase C — P2 (Sprint 3+)
- [x] Thêm `infra/adapters/inapp/store.go`
- [x] Thêm `delivery/http/inapp_handler.go` (SSE)
- [x] Thêm `migrations/007_inapp_notifications.up.sql`
- [ ] Thêm digest scheduler
- [ ] Thêm PagerDuty integration
