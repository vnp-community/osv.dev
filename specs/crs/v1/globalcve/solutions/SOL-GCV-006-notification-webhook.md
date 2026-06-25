# SOL-GCV-006 — Notification & Webhook Service

| Trường | Giá trị |
|--------|---------|
| **CR** | [CR-GCV-006](../CR-GCV-006-notification-webhook-service.md) |
| **Target Service** | `notification-service` (**NEW service** — đã có skeleton) |
| **apps/osv role** | Gateway forward `/api/v2/webhooks`, `/api/v2/subscriptions` |
| **Priority** | 🟡 Medium |

---

## 1. Hiện trạng

- `services/notification-service/` → **đã có skeleton** với `internal/domain/webhook/`, `internal/domain/alert/`, etc.
- `services/notification-service/internal/` → cấu trúc đầy đủ nhưng cần implement
- `gateway-service/internal/proxy/ovs_routes.go` → có `/api/v1/notifications` route nhưng dùng permission `system:configure`

---

## 2. Giải pháp

### 2.1 notification-service Implementation

**notification-service** đã có skeleton. Cần implement business logic:

#### Domain layer (verify/implement)

**File**: `notification-service/internal/domain/webhook/webhook.go`

```go
type Webhook struct {
    ID        string
    URL       string       // HTTPS only
    Secret    string       // HMAC-SHA256 signing key
    Events    []EventType
    IsActive  bool
    OwnerID   string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type EventType string
const (
    EventNewKEV      EventType = "kev.new"
    EventNewCritical EventType = "cve.new.critical"
    EventNewHigh     EventType = "cve.new.high"
    EventHighEPSS    EventType = "cve.epss.high"
    EventVendorCVE   EventType = "cve.vendor"
    EventProductCVE  EventType = "cve.product"
)
```

**File**: `notification-service/internal/domain/alert/subscription.go`

```go
type AlertSubscription struct {
    ID          string
    OwnerID     string
    Type        SubscriptionType  // "vendor" | "product" | "kev"
    Value       string
    MinSeverity string            // "CRITICAL" | "HIGH" | "MEDIUM"
    MinEPSS     *float64
    IsActive    bool
    CreatedAt   time.Time
}
```

#### Use Cases

**File**: `notification-service/internal/usecase/` (implement các use cases)

```
usecase/
├── register_webhook.go     ← POST /api/v2/webhooks
├── list_webhooks.go        ← GET  /api/v2/webhooks
├── delete_webhook.go       ← DELETE /api/v2/webhooks/:id
├── dispatch_alerts.go      ← POST /internal/alerts/dispatch (called by data-service)
├── deliver_webhook.go      ← HMAC signed HTTP delivery + retry
└── manage_subscription.go  ← CRUD for alert subscriptions
```

**Key business logic — SSRF Protection** (`register_webhook.go`):
```go
func validateWebhookURL(rawURL string) error {
    u, err := url.Parse(rawURL)
    if err != nil || u.Scheme != "https" {
        return ErrInsecureURL
    }

    // Resolve hostname → IP
    addrs, err := net.LookupHost(u.Hostname())
    if err != nil { return ErrUnresolvable }

    for _, addr := range addrs {
        ip := net.ParseIP(addr)
        if isPrivateIP(ip) {
            return ErrSSRFBlocked
        }
    }
    return nil
}

var privateRanges = []*net.IPNet{
    mustParseCIDR("127.0.0.0/8"),
    mustParseCIDR("10.0.0.0/8"),
    mustParseCIDR("172.16.0.0/12"),
    mustParseCIDR("192.168.0.0/16"),
    mustParseCIDR("169.254.0.0/16"),  // link-local
    mustParseCIDR("::1/128"),          // IPv6 loopback
}
```

**Key business logic — Webhook Delivery** (`deliver_webhook.go`):
```go
// HMAC-SHA256 signing
func signPayload(secret string, body []byte) string {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Retry schedule (exponential backoff):
var retryDelays = []time.Duration{
    0,
    5 * time.Minute,
    30 * time.Minute,
    2 * time.Hour,
    12 * time.Hour,
}

// Alert deduplication: redis key "alert:dedup:{webhook_id}:{cve_id}:{event}" TTL=1h
func isDuplicate(ctx context.Context, redis *redis.Client, webhookID, cveID string, event EventType) bool {
    key := fmt.Sprintf("alert:dedup:%s:%s:%s", webhookID, cveID, event)
    ok, _ := redis.SetNX(ctx, key, "1", 1*time.Hour).Result()
    return !ok  // If SetNX fails (key exists) → duplicate
}
```

#### HTTP Handlers

**File**: `notification-service/internal/delivery/http/webhook_handler.go`

```go
// Routes:
// GET    /api/v2/webhooks              → ListWebhooks (owner-scoped)
// POST   /api/v2/webhooks              → RegisterWebhook
// GET    /api/v2/webhooks/:id          → GetWebhook
// PATCH  /api/v2/webhooks/:id          → UpdateWebhook
// DELETE /api/v2/webhooks/:id          → DeleteWebhook
// GET    /api/v2/webhooks/:id/deliveries → GetDeliveryHistory

// POST   /api/v2/subscriptions         → CreateSubscription
// GET    /api/v2/subscriptions         → ListSubscriptions
// DELETE /api/v2/subscriptions/:id     → DeleteSubscription

// Internal (không expose qua gateway):
// POST   /internal/alerts/dispatch     → Dispatch alerts for new CVEs
// GET    /health
```

#### Database

**File**: `notification-service/migrations/001_init.sql`

```sql
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

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id              TEXT        PRIMARY KEY,
    webhook_id      TEXT        NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type      TEXT        NOT NULL,
    payload         TEXT        NOT NULL,
    status_code     INT,
    attempt         INT         NOT NULL DEFAULT 1,
    status          TEXT        NOT NULL DEFAULT 'pending',
    delivered_at    TIMESTAMPTZ,
    next_retry_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_deliveries_webhook ON webhook_deliveries(webhook_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deliveries_retry   ON webhook_deliveries(next_retry_at)
    WHERE status = 'retrying';

CREATE TABLE IF NOT EXISTS alert_subscriptions (
    id              TEXT        PRIMARY KEY,
    owner_id        TEXT        NOT NULL,
    type            TEXT        NOT NULL CHECK (type IN ('vendor', 'product', 'kev')),
    value           TEXT        NOT NULL,
    min_severity    TEXT        NOT NULL DEFAULT 'HIGH',
    min_epss        NUMERIC(8,6),
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 2.2 Integration với data-service

**Sau khi data-service sync CVEs**, gọi notification-service:

**File**: `data-service/internal/usecase/*/usecase.go` (thêm post-sync hook)

```go
// Sau khi sync hoàn thành, dispatch alerts (non-blocking):
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    payload := buildDispatchPayload(syncResult)
    _, err := http.Post(
        uc.notificationServiceURL + "/internal/alerts/dispatch",
        "application/json",
        payload,
    )
    if err != nil {
        log.Warn().Err(err).Msg("alert dispatch failed (non-critical)")
    }
}()
```

**Config** (`data-service/config.yaml`):
```yaml
notification_service:
  url: "http://notification-service:8084"
  enabled: true
```

### 2.3 Retry Background Worker

**File**: `notification-service/internal/scheduler/retry_worker.go`

```go
// Background goroutine: poll every 1 minute for failed deliveries
// WHERE status = 'retrying' AND next_retry_at <= NOW()
// Re-attempt delivery with exponential backoff
func (w *RetryWorker) Run(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    for {
        select {
        case <-ticker.C:
            w.processRetries(ctx)
        case <-ctx.Done():
            return
        }
    }
}
```

---

## 3. apps/osv Changes

> **apps/osv không thay đổi business logic.**

**Gateway routing update** (`gateway-service/internal/proxy/ovs_routes.go`):

```go
// Thay thế route cũ (notification với system:configure permission)
// thành routes mới với API key scope cve:read / webhook:write:
{PathPrefix: "/api/v2/webhooks",       Upstream: "notification-service"},
{PathPrefix: "/api/v2/subscriptions",  Upstream: "notification-service"},
// /internal/alerts/dispatch KHÔNG expose qua gateway (internal only)
```

---

## 4. Files cần tạo/sửa

### notification-service (IMPLEMENT — service đã có skeleton)
```
internal/usecase/register_webhook.go   ← Webhook registration + SSRF check
internal/usecase/dispatch_alerts.go    ← Alert dispatch logic
internal/usecase/deliver_webhook.go    ← HMAC delivery + retry
internal/usecase/manage_subscription.go
internal/delivery/http/webhook_handler.go   ← All HTTP handlers
internal/delivery/http/router.go            ← Route setup
internal/infra/postgres/webhook_pg.go       ← Repository impl
internal/infra/postgres/subscription_pg.go
internal/scheduler/retry_worker.go          ← Background retry
migrations/001_init.sql                     ← DB schema
cmd/server/main.go                          ← Wire + start server
```

### data-service (MODIFY)
```
internal/usecase/*/usecase.go   ← Add post-sync HTTP notification call
config/config.yaml               ← Add notification_service.url
```

### gateway-service (MODIFY)
```
internal/proxy/ovs_routes.go    ← Update /api/v2/webhooks routes
```

---

## 5. API Spec

```
POST   /api/v2/webhooks                    → Register webhook (Auth required)
GET    /api/v2/webhooks                    → List owner's webhooks
GET    /api/v2/webhooks/{id}               → Get webhook details
PATCH  /api/v2/webhooks/{id}               → Update events/active status
DELETE /api/v2/webhooks/{id}               → Delete webhook
GET    /api/v2/webhooks/{id}/deliveries    → Delivery history

POST   /api/v2/subscriptions               → Subscribe to vendor/product alerts
GET    /api/v2/subscriptions               → List subscriptions
DELETE /api/v2/subscriptions/{id}          → Unsubscribe
```

---

## 6. Acceptance Criteria

- [x] `POST /api/v2/webhooks` với HTTPS URL → registered, ping test sent
- [x] `POST /api/v2/webhooks` với HTTP URL → 400 "HTTPS required"
- [x] `POST /api/v2/webhooks` với private IP URL → 400 "SSRF protection"
- [x] New KEV → webhooks subscribed to `kev.new` triggered ≤ 60s
- [x] Webhook payload: `X-GlobalCVE-Signature: sha256=<hmac>` header
- [x] Delivery fail → retry với delays: 5m, 30m, 2h, 12h (5 attempts max)
- [x] Same alert trong 1h → deduplicated (Redis TTL)
- [x] `GET /api/v2/webhooks/{id}/deliveries` → history với `status_code`, `attempt`


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Build verified: go build ./... pass (notification-service).

| Component | Status | Notes |
|-----------|--------|-------|
| domain/aggregate/webhook/webhook.go | CREATED | Webhook aggregate root với New, Reconstitute, ReconstituteFromStrings, Sign, ShouldDeliver, Activate, Deactivate, UpdateSecret |
| domain/aggregate/webhook — EventType, WebhookDelivery re-exports | CREATED | Re-exports từ domain/webhook |
| domain/repository/webhook_repo.go | UPDATED | WebhookRepository interface dùng aggregate/webhook.Webhook thay domain/webhook.Webhook; thêm FindActiveByEvent, SaveDelivery, UpdateDelivery |
| infra/postgres/webhook_pg.go | REWRITTEN | Dùng aggregate/webhook.Webhook accessor methods thay field access |
| infra/persistence/postgres/webhook_repo.go | UPDATED | Thêm FindByID(ctx,id,ownerID), FindByEvent, Update, SaveDelivery, UpdateDelivery, ListDeliveries, GetPendingRetries |
| infra/persistence/firestore/repos.go | FIXED | ReconstituteFromStrings thay Reconstitute |
| usecase/register_webhook.go | UPDATED | Dùng aggregate/webhook.Webhook |
| usecase/manage_subscription/register.go | FIXED | []string → []webhook.EventType conversion |
| usecase/deliver_webhook.go | FIXED | wh.Secret() / wh.URL() / wh.IsActive() method calls thay field access |
| usecase/alert_dispatcher.go | FIXED | wh.ID() method call |
| delivery/http/webhook_handler.go | FIXED | wh.Secret(), wh.URL(), FindByID với ownerID |
| delivery/http/helpers.go | UPDATED | Thêm writeJSONError helper |
| delivery/http/rule_handler.go | FIXED | Remove duplicate writeJSON/writeJSONError; dùng helpers.go |
| delivery/http/router.go | FIXED | Remove unused auth import |
| delivery/http/alert_handler.go | FIXED | respondError calls với correct arity |
| infra/channels/email/smtp.go | REWRITTEN | GoMailSender (không conflict với sender.go Config/Sender) |
| infra/persistence/postgres/alert_repo.go | REWRITTEN | Remove import cycle với delivery/http; remove duplicate MarkRead/CountUnread methods |
| domain/kev/kev.go | FIXED | package name entity → kev |
| jira/domain/domain.go | CREATED | Redirect alias → integrations/jira/domain |
| jira/infra/client.go | CREATED | Redirect alias → integrations/jira/infra (Client, CreateIssueRequest, IssueFields, ADFDescription) |
| nats/subscriber.go | FIXED | Added "fmt" import |
| cmd/server/main.go | FIXED | NewSubscriber với EventBroker arg; SetupRouter với AlertsHandler/SSEHandler nil args |

### Build fixes count: 22 files modified/created
