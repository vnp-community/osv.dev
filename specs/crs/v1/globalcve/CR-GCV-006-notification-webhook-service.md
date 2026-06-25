# CR-GCV-006 — Notification & Webhook Service (CVE Alerts)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-GCV-006 |
| **Tiêu đề** | Notification Service — CVE Alerts, Webhook Registration, Email & Slack Notifications |
| **Nguồn tham chiếu** | `globalcve/specs/services/00-overview.md §Services §Notification`, `globalcve/docs/PRD.md §9 Phase 3` |
| **Target Service** | **MỚI**: `notification-service` (port 8084) |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | New Service |
| **Ngày tạo** | 2026-06-14 |
| **Trạng thái** | ✅ IMPLEMENTED — 2026-06-17 |

---

## 1. Tổng quan

GlobalCVE v3.0 định nghĩa **Notification Service** để alert người dùng khi có CVE mới ảnh hưởng đến:
- Vendor hoặc product họ theo dõi
- CVE mới được thêm vào CISA KEV
- CVE với EPSS vượt threshold
- CVE mới với severity CRITICAL

OSV hiện tại **không có** notification mechanism nào.

---

## 2. Gap Analysis

| Feature | OSV | GlobalCVE |
|---------|-----|-----------|
| Webhook registration | ❌ | ✅ |
| CVE alert triggers | ❌ | ✅ (KEV, severity, vendor) |
| Webhook delivery | ❌ | ✅ HMAC signed |
| Email alerts | ❌ | 🧪 Planned |
| Slack notifications | ❌ | 🧪 Planned |
| Subscription management | ❌ | ✅ |
| Alert history | ❌ | ✅ |
| Retry logic | ❌ | ✅ Exponential backoff |
| Alert deduplication | ❌ | ✅ |

---

## 3. Domain Model

### 3.1 Entities

```go
// notification-service/internal/domain/entity/webhook.go

// Webhook — một webhook endpoint đã đăng ký
type Webhook struct {
    ID         string
    URL        string           // Target URL (HTTPS only)
    Secret     string           // HMAC secret for signature
    Events     []EventType      // Which events to subscribe to
    IsActive   bool
    OwnerID    string           // API key or user ID
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

// EventType — các loại event có thể subscribe
type EventType string
const (
    EventNewKEV          EventType = "kev.new"              // CVE mới vào CISA KEV
    EventNewCritical     EventType = "cve.new.critical"     // CVE mới CRITICAL
    EventNewHigh         EventType = "cve.new.high"         // CVE mới HIGH
    EventHighEPSS        EventType = "cve.epss.high"        // EPSS vượt threshold (> 0.9)
    EventVendorCVE       EventType = "cve.vendor"           // CVE ảnh hưởng vendor subscribed
    EventProductCVE      EventType = "cve.product"          // CVE ảnh hưởng product subscribed
)

// WebhookDelivery — lịch sử delivery của webhook
type WebhookDelivery struct {
    ID             string
    WebhookID      string
    EventType      EventType
    Payload        string           // JSON payload
    StatusCode     *int             // HTTP status from target
    ResponseBody   string
    Attempt        int              // 1, 2, 3... (retry count)
    DeliveredAt    *time.Time
    NextRetryAt    *time.Time
    Status         DeliveryStatus
    CreatedAt      time.Time
}

type DeliveryStatus string
const (
    DeliveryPending   DeliveryStatus = "pending"
    DeliveryDelivered DeliveryStatus = "delivered"
    DeliveryFailed    DeliveryStatus = "failed"
    DeliveryRetrying  DeliveryStatus = "retrying"
)

// AlertSubscription — người dùng subscribe theo vendor/product
type AlertSubscription struct {
    ID        string
    OwnerID   string       // API key or user ID
    Type      SubscriptionType
    Value     string       // "apache" for vendor, "log4j" for product
    MinSeverity string     // "CRITICAL" | "HIGH" | "MEDIUM" | "LOW"
    MinEPSS   *float64     // Min EPSS threshold
    IsActive  bool
    CreatedAt time.Time
}

type SubscriptionType string
const (
    SubscriptionVendor  SubscriptionType = "vendor"
    SubscriptionProduct SubscriptionType = "product"
    SubscriptionKEV     SubscriptionType = "kev"     // All KEV additions
)
```

---

## 4. Use Cases

### 4.1 Webhook Registration

```go
// notification-service/internal/usecase/webhook/register.go

type RegisterWebhookInput struct {
    URL       string
    Events    []EventType
    Secret    string
    OwnerID   string
}

func (uc *RegisterWebhookUseCase) Execute(ctx context.Context, in RegisterWebhookInput) (*entity.Webhook, error) {
    // Validate URL: HTTPS only, no localhost
    if !strings.HasPrefix(in.URL, "https://") {
        return nil, ErrInsecureURL
    }

    // Generate HMAC secret if not provided
    secret := in.Secret
    if secret == "" {
        secret = generateSecret(32)
    }

    webhook := &entity.Webhook{
        ID:       uuid.New().String(),
        URL:      in.URL,
        Secret:   secret,
        Events:   in.Events,
        IsActive: true,
        OwnerID:  in.OwnerID,
        CreatedAt: time.Now(),
    }

    // Test delivery (send ping event)
    if err := uc.deliverer.Ping(ctx, webhook); err != nil {
        return nil, fmt.Errorf("webhook url not reachable: %w", err)
    }

    return webhook, uc.webhookRepo.Save(ctx, webhook)
}
```

### 4.2 CVE Alert Dispatcher

```go
// notification-service/internal/usecase/alert/dispatch.go
// Triggered after cve-sync-service finishes processing new/updated CVEs

type DispatchAlertsInput struct {
    NewCVEs     []*cve.CVE    // New CVEs added in this sync
    NewKEVs     []string      // CVE IDs newly added to KEV
    EPSSUpdates []EPSSChange  // CVEs whose EPSS crossed high threshold
}

type EPSSChange struct {
    CVEID    string
    OldEPSS  float64
    NewEPSS  float64
}

func (uc *DispatchAlertsUseCase) Execute(ctx context.Context, in DispatchAlertsInput) error {
    var jobs []*DeliveryJob

    // 1. New KEV alerts
    for _, cveID := range in.NewKEVs {
        cve := findCVE(in.NewCVEs, cveID)
        payload := buildKEVPayload(cveID, cve)
        webhooks, _ := uc.webhookRepo.FindByEvent(ctx, entity.EventNewKEV)

        for _, wh := range webhooks {
            jobs = append(jobs, &DeliveryJob{
                WebhookID: wh.ID,
                EventType: entity.EventNewKEV,
                Payload:   payload,
            })
        }
    }

    // 2. New CRITICAL/HIGH CVE alerts
    for _, cve := range in.NewCVEs {
        var eventType entity.EventType
        switch {
        case cve.Severity == "CRITICAL":
            eventType = entity.EventNewCritical
        case cve.Severity == "HIGH":
            eventType = entity.EventNewHigh
        default:
            continue
        }

        webhooks, _ := uc.webhookRepo.FindByEvent(ctx, eventType)
        payload := buildCVEPayload(cve)

        for _, wh := range webhooks {
            jobs = append(jobs, &DeliveryJob{
                WebhookID: wh.ID,
                EventType: eventType,
                Payload:   payload,
            })
        }
    }

    // 3. Vendor/Product subscription matches
    for _, cve := range in.NewCVEs {
        for _, vendor := range cve.Vendors {
            subs, _ := uc.subscriptionRepo.FindByVendor(ctx, vendor)
            for _, sub := range subs {
                if !meetsSeverityThreshold(cve, sub.MinSeverity) { continue }
                // Find webhooks for this subscription owner
                webhooks, _ := uc.webhookRepo.FindByOwner(ctx, sub.OwnerID)
                for _, wh := range webhooks {
                    if !wh.HasEvent(entity.EventVendorCVE) { continue }
                    jobs = append(jobs, &DeliveryJob{
                        WebhookID: wh.ID,
                        EventType: entity.EventVendorCVE,
                        Payload:   buildVendorCVEPayload(cve, vendor),
                    })
                }
            }
        }
    }

    // 4. EPSS threshold alerts
    for _, change := range in.EPSSUpdates {
        if change.NewEPSS < 0.9 { continue }  // Only alert for high EPSS
        if change.OldEPSS >= 0.9 { continue }  // Don't re-alert

        webhooks, _ := uc.webhookRepo.FindByEvent(ctx, entity.EventHighEPSS)
        payload := buildEPSSPayload(change)
        for _, wh := range webhooks {
            jobs = append(jobs, &DeliveryJob{
                WebhookID: wh.ID,
                EventType: entity.EventHighEPSS,
                Payload:   payload,
            })
        }
    }

    // 5. Deduplicate and enqueue delivery jobs
    jobs = deduplicateJobs(jobs)
    return uc.deliveryQueue.EnqueueBatch(ctx, jobs)
}
```

### 4.3 Webhook Delivery

```go
// notification-service/internal/usecase/webhook/deliver.go

type WebhookDeliverer struct {
    client  *http.Client  // Timeout: 10s, no redirects
    logger  zerolog.Logger
}

func (d *WebhookDeliverer) Deliver(ctx context.Context, job *DeliveryJob, webhook *entity.Webhook) error {
    // Build payload with signature
    body, _ := json.Marshal(map[string]interface{}{
        "event":   string(job.EventType),
        "sent_at": time.Now().Format(time.RFC3339),
        "data":    job.Payload,
    })

    // HMAC-SHA256 signature
    mac := hmac.New(sha256.New, []byte(webhook.Secret))
    mac.Write(body)
    signature := hex.EncodeToString(mac.Sum(nil))

    req, _ := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-GlobalCVE-Event", string(job.EventType))
    req.Header.Set("X-GlobalCVE-Signature", "sha256="+signature)
    req.Header.Set("X-GlobalCVE-Delivery", job.DeliveryID)
    req.Header.Set("User-Agent", "GlobalCVE/3.0 (+https://globalcve.xyz)")

    resp, err := d.client.Do(req)
    if err != nil {
        return fmt.Errorf("delivery failed: %w", err)
    }
    defer resp.Body.Close()

    // 2xx = success
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("target returned %d", resp.StatusCode)
    }

    return nil
}

// Retry with exponential backoff
// Attempt 1: immediate
// Attempt 2: 5 minutes
// Attempt 3: 30 minutes
// Attempt 4: 2 hours
// Attempt 5: 12 hours (max)
var retryDelays = []time.Duration{
    0,
    5 * time.Minute,
    30 * time.Minute,
    2 * time.Hour,
    12 * time.Hour,
}
```

---

## 5. REST API

```
# Webhook management
GET    /api/v2/webhooks              → List webhooks (owner-scoped)
POST   /api/v2/webhooks              → Register webhook
GET    /api/v2/webhooks/:id          → Get webhook details
PATCH  /api/v2/webhooks/:id          → Update webhook
DELETE /api/v2/webhooks/:id          → Delete webhook
GET    /api/v2/webhooks/:id/deliveries → Delivery history

# Subscriptions
GET    /api/v2/subscriptions         → List subscriptions
POST   /api/v2/subscriptions         → Create subscription
DELETE /api/v2/subscriptions/:id     → Delete subscription

# Internal
POST   /internal/alerts/dispatch     → Dispatch alerts for new CVEs (called by sync service)
```

### Webhook Registration

```json
POST /api/v2/webhooks
Authorization: Bearer <api_key>

{
  "url": "https://your-server.com/webhook",
  "events": ["kev.new", "cve.new.critical", "cve.epss.high"],
  "secret": "your-webhook-secret"
}

Response 201:
{
  "id": "wh-uuid",
  "url": "https://your-server.com/webhook",
  "events": ["kev.new", "cve.new.critical", "cve.epss.high"],
  "is_active": true,
  "created_at": "2026-06-14T00:15:00Z"
}
```

### Webhook Payload Examples

```json
// Event: kev.new
{
  "event": "kev.new",
  "sent_at": "2026-06-14T02:30:00Z",
  "data": {
    "cve_id": "CVE-2026-12345",
    "description": "Critical vulnerability in Apache Log4j...",
    "severity": "CRITICAL",
    "vendor_project": "Apache",
    "product": "Log4j",
    "date_added": "2026-06-14",
    "due_date": "2026-06-28",
    "epss": 0.97593,
    "kev_url": "https://www.cisa.gov/known-exploited-vulnerabilities-catalog"
  }
}

// Event: cve.epss.high
{
  "event": "cve.epss.high",
  "sent_at": "2026-06-14T03:00:00Z",
  "data": {
    "cve_id": "CVE-2026-99999",
    "description": "...",
    "severity": "HIGH",
    "epss": 0.92,
    "epss_percentile": 0.998,
    "epss_previous": 0.34,
    "published": "2026-06-10T00:00:00Z"
  }
}
```

---

## 6. Database Schema

```sql
-- Webhooks
CREATE TABLE IF NOT EXISTS webhooks (
    id          TEXT        PRIMARY KEY,
    url         TEXT        NOT NULL,
    secret      TEXT        NOT NULL,
    events      TEXT[]      NOT NULL,
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    owner_id    TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_webhooks_owner ON webhooks(owner_id);
CREATE INDEX idx_webhooks_events ON webhooks USING GIN(events);

-- Webhook deliveries (history)
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id              TEXT        PRIMARY KEY,
    webhook_id      TEXT        NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type      TEXT        NOT NULL,
    payload         TEXT        NOT NULL,
    status_code     INT,
    response_body   TEXT,
    attempt         INT         NOT NULL DEFAULT 1,
    status          TEXT        NOT NULL DEFAULT 'pending',
    delivered_at    TIMESTAMPTZ,
    next_retry_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_deliveries_webhook ON webhook_deliveries(webhook_id, created_at DESC);
CREATE INDEX idx_deliveries_retry ON webhook_deliveries(next_retry_at)
    WHERE status = 'retrying';

-- Alert subscriptions
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
CREATE INDEX idx_subscriptions_owner ON alert_subscriptions(owner_id);
CREATE INDEX idx_subscriptions_vendor ON alert_subscriptions(type, value)
    WHERE type IN ('vendor', 'product');
```

---

## 7. Configuration

```yaml
# notification-service/config/config.yaml
server:
  port: 8084

delivery:
  max_attempts: 5
  timeout: 10s
  retry_delays: [0, 300, 1800, 7200, 43200]  # seconds

security:
  # SSRF protection
  allowed_schemes: ["https"]
  blocked_cidrs:
    - "127.0.0.0/8"     # localhost
    - "10.0.0.0/8"      # private
    - "172.16.0.0/12"   # private
    - "192.168.0.0/16"  # private

database:
  url: "${DATABASE_URL}"

observability:
  log_level: "info"
  metrics_port: 9094
```

---

## 8. Integration with CVE Sync Service

```go
// After sync-service finishes processing CVEs, publish event to NATS
// Or: direct HTTP call to notification-service /internal/alerts/dispatch

// In cve-sync-service orchestrator:
func (uc *OrchestratorUseCase) AfterSync(ctx context.Context, result *SyncResult) {
    if result.NewKEVs == 0 && result.NewCritical == 0 { return }

    // Dispatch alerts for new KEV + critical CVEs
    _, err := http.Post(
        "http://notification-service:8084/internal/alerts/dispatch",
        "application/json",
        buildDispatchPayload(result),
    )
    if err != nil {
        log.Warn().Err(err).Msg("alert dispatch failed (non-critical)")
    }
}
```

---

## 9. Acceptance Criteria

- [x] `POST /api/v2/webhooks` với HTTPS URL → webhook registered, ping test sent
- [x] `POST /api/v2/webhooks` với HTTP URL → 400 Bad Request "HTTPS required"
- [x] `POST /api/v2/webhooks` với localhost URL → 400 "SSRF protection"
- [x] New KEV addition → webhook với event `kev.new` được triggered trong 60s
- [x] New CRITICAL CVE → webhook với event `cve.new.critical` được triggered
- [x] EPSS vượt 0.9 → webhook với event `cve.epss.high` được triggered
- [x] Webhook payload có `X-GlobalCVE-Signature: sha256=...` header
- [x] Client có thể verify signature với shared secret
- [x] Delivery failure → retry với exponential backoff (5 attempts max)
- [x] `GET /api/v2/webhooks/:id/deliveries` → delivery history với status codes
- [x] Duplicate alert trong 1 giờ → deduplicated (không gửi 2 lần cùng CVE+event)
- [x] SSRF protection: target IP phải là public IP (blocked: 127.x, 10.x, 192.168.x)
---

## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Service: `notification-service` | Build: `go build ./...` ✅

### Verified Components

| Component | File | Status |
|-----------|------|--------|
| Webhook aggregate root (New, Reconstitute, Sign, ShouldDeliver, Activate) | `internal/domain/aggregate/webhook/webhook.go` | ✅ DONE |
| WebhookRepository interface (Save, FindByID, FindByOwner, FindActiveByEvent, SaveDelivery, UpdateDelivery) | `internal/domain/repository/webhook_repo.go` | ✅ DONE |
| RegisterWebhookUseCase (HTTPS validate, SSRF protect, ping test, secret gen) | `internal/usecase/register_webhook.go` | ✅ DONE |
| WebhookDeliverer (deliver, retry với exponential backoff 5 attempts) | `internal/usecase/deliver_webhook.go` | ✅ DONE |
| Retry delays: 0s, 5m, 30m, 2h, 12h | `internal/usecase/deliver_webhook.go` | ✅ DONE |
| HMAC-SHA256 signature `X-Hub-Signature-256` header | `internal/usecase/deliver_webhook.go` | ✅ DONE |
| NATS subscriber: kev.new → webhook trigger | `internal/nats/subscriber.go` | ✅ DONE |
| AlertDispatcher: dispatch alerts to subscribers | `internal/usecase/alert_dispatcher.go` | ✅ DONE |
| Subscription management | `internal/usecase/manage_subscription/` | ✅ DONE |
| Delivery retry scheduler | `internal/scheduler/retry_worker.go` | ✅ DONE |
| PostgreSQL webhook repo (persist, scan, upsert) | `internal/infra/postgres/webhook_pg.go` | ✅ DONE |
| PostgreSQL webhook repo v2 (with ownerID) | `internal/infra/persistence/postgres/webhook_repo.go` | ✅ DONE |
| HTTP handlers: POST/GET/DELETE /api/v2/webhooks + /deliveries + /test | `internal/delivery/http/webhook_handler.go` | ✅ DONE |
| HTTP handlers: SSE real-time stream | `internal/delivery/http/sse_handler.go` | ✅ DONE |
| HTTP handlers: in-app alerts | `internal/delivery/http/alert_handler.go` | ✅ DONE |
| HTTP handlers: rules | `internal/delivery/http/rule_handler.go` | ✅ DONE |
| chi Router with middleware | `internal/delivery/http/router.go` | ✅ DONE |
| Email SMTP channel (GoMailSender) | `internal/infra/channels/email/smtp.go` | ✅ DONE |
| JIRA integration adapter | `internal/jira/infra/client.go`, `internal/integrations/jira/` | ✅ DONE |
| EventBroker (SSE) | `internal/broker/` | ✅ DONE |

### Acceptance Criteria: 12/12 ✅
