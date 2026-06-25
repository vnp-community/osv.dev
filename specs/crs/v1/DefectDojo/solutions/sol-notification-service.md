# ✅ COMPLETED — Solution: notification-service Extension

> **Covers**: CR-DD-007  
> **`notification-service` đã tồn tại** tại `services/notification-service/`. Chỉ cần mở rộng.

---

## Current State Analysis

`notification-service` hiện tại đã có:
- `internal/domain/` — `alert`, `delivery`, `rule`, `subscription`, `webhook`, `integration`
- `internal/usecase/` — partial
- `internal/infra/` — partial implementations
- `internal/integrations/` — có Slack, Teams tích hợp sẵn

→ **Chỉ cần mở rộng** để handle 20+ DefectDojo event types và thêm retry logic.

---

## What Needs to Be Added

### 1. Event Type Extension

```go
// notification-service/internal/domain/rule/event_types.go — THÊM MỚI
const (
    // Events từ DefectDojo CRs (thêm vào danh sách hiện có)
    EventScanAdded                EventType = "scan_added"
    EventFindingAdded             EventType = "finding_added"
    EventFindingStatusChanged     EventType = "finding_status_changed"
    EventJIRAUpdate               EventType = "jira_update"
    EventEngagementAdded          EventType = "engagement_added"
    EventEngagementClosed         EventType = "engagement_closed"
    EventRiskAcceptanceExpiration EventType = "risk_acceptance_expiration"
    EventSLABreach                EventType = "sla_breach"
    EventSLAExpiringSoon          EventType = "sla_expiring_soon"
    EventUserMentioned            EventType = "user_mentioned"
    EventProductAdded             EventType = "product_added"
    EventClosedFindingRemoved     EventType = "closed_finding_removed"
    EventReviewRequested          EventType = "review_requested"
    EventAuditInteraction         EventType = "audit_interaction"
)
```

### 2. NotificationRule Entity Extension

```go
// notification-service/internal/domain/rule/entity.go — MỞ RỘNG
type NotificationRule struct {
    ID        string
    UserID    *string   // NULL = system/global rule
    ProductID *string   // NULL = all products

    // Per-event channel configuration — THÊM fields DefectDojo
    ScanAdded                 []Channel
    TestAdded                 []Channel
    FindingAdded              []Channel
    FindingStatusChanged      []Channel
    JIRAUpdate                []Channel
    EngagementAdded           []Channel
    EngagementClosed          []Channel
    RiskAcceptanceExpiration  []Channel
    SLABreach                 []Channel
    SLAExpiringSoon           []Channel
    UserMentioned             []Channel
    ProductAdded              []Channel
    ClosedFindingRemoved      []Channel
    ReviewRequested           []Channel
    AuditInteraction          []Channel

    CreatedAt time.Time
    UpdatedAt time.Time
}

type Channel string
const (
    ChannelEmail   Channel = "email"
    ChannelSlack   Channel = "slack"
    ChannelTeams   Channel = "msteams"
    ChannelWebhook Channel = "webhook"
    ChannelInApp   Channel = "inapp"
)
```

### 3. NATS Event Handlers (new)

```
notification-service/internal/delivery/event/handlers/  # 🆕 THÊM MỚI
├── scan_completed.go           # scan.import.completed → EventScanAdded
├── finding_created.go          # finding.batch_created → EventFindingAdded
├── finding_status_changed.go   # finding.status_changed → EventFindingStatusChanged
├── sla_breached.go             # sla.breached → EventSLABreach
├── sla_expiring_soon.go        # sla.expiring_soon → EventSLAExpiringSoon
├── engagement_closed.go        # engagement.closed → EventEngagementClosed
├── risk_acceptance_expired.go  # risk_acceptance.expired → EventRiskAcceptanceExpiration
├── jira_update.go              # jira.issue.created/updated → EventJIRAUpdate
└── product_created.go          # product.created → EventProductAdded
```

### 4. Dispatch Use Case (core logic)

```go
// notification-service/internal/usecase/dispatch/dispatch.go — THÊM/MỞ RỘNG
type NotificationEvent struct {
    Type         EventType
    ProductID    *string
    EngagementID *string
    FindingID    *string
    Title        string
    Description  string
    URL          string
    Severity     *string
    Metadata     map[string]interface{}
}

func (uc *DispatchUseCase) Execute(ctx context.Context, event *NotificationEvent) error {
    // 1. Find matching rules (system + user-specific)
    rules, _ := uc.ruleRepo.FindMatchingRules(ctx, &RuleQuery{
        EventType: event.Type,
        ProductID: event.ProductID,
    })

    // 2. Get product members to notify
    var recipients []Recipient
    if event.ProductID != nil {
        members, _ := uc.identityClient.GetUsersForProduct(ctx, *event.ProductID)
        recipients = buildRecipients(rules, members)
    }

    // 3. Deliver to each recipient × channel (async with retry)
    for _, recipient := range recipients {
        for _, channel := range recipient.Channels {
            payload, _ := uc.tmplRenderer.Render(event.Type, channel, &TemplateData{
                Event: event, Recipient: recipient,
            })
            record := uc.deliveryRepo.Create(ctx, &DeliveryRecord{...})
            go uc.deliverWithRetry(ctx, record, channel, payload) // async
        }
        // Always create in-app alert
        uc.alertRepo.Save(ctx, &Alert{UserID: recipient.UserID, ...})
    }
    return nil
}

// Retry với exponential backoff: 30s, 60s, 120s
func (uc *DispatchUseCase) deliverWithRetry(ctx context.Context, record *DeliveryRecord, channel Channel, payload map[string]interface{}) {
    for attempt := 0; attempt < 3; attempt++ {
        err := uc.sendToChannel(ctx, channel, record.Recipient, payload)
        if err == nil {
            record.Status = DeliveryStatusSent
            uc.deliveryRepo.Save(ctx, record)
            return
        }
        record.Attempts++
        record.Status = DeliveryStatusRetrying
        uc.deliveryRepo.Save(ctx, record)
        time.Sleep(30 * time.Second * time.Duration(1<<attempt)) // 30s, 60s, 120s
    }
    record.Status = DeliveryStatusFailed
    uc.deliveryRepo.Save(ctx, record)
}
```

### 5. SSRF Protection (new)

```go
// notification-service/internal/infra/webhook/ssrf.go — 🆕 MỚI
// Ngăn webhook URLs trỏ đến private/localhost IPs
type SSRFChecker struct{}

var privateRanges = []*net.IPNet{
    mustParseCIDR("10.0.0.0/8"),
    mustParseCIDR("172.16.0.0/12"),
    mustParseCIDR("192.168.0.0/16"),
    mustParseCIDR("127.0.0.0/8"),
    mustParseCIDR("::1/128"),
    mustParseCIDR("fc00::/7"),
}

func (c *SSRFChecker) Validate(rawURL string) error {
    u, _ := url.Parse(rawURL)
    ips, _ := net.LookupIP(u.Hostname())
    for _, ip := range ips {
        for _, pr := range privateRanges {
            if pr.Contains(ip) {
                return fmt.Errorf("SSRF protection: %s resolves to private IP %s", rawURL, ip)
            }
        }
    }
    return nil
}
```

### 6. Template System

```
notification-service/templates/  # 🆕 MỚI — DefectDojo event templates
├── scan_added/
│   ├── email.html
│   ├── email.txt
│   ├── slack.json      # Slack Block Kit
│   └── teams.json      # Teams MessageCard
├── finding_added/
│   ├── email.html
│   └── slack.json
├── sla_breach/
│   ├── email.html
│   └── slack.json
├── sla_expiring_soon/
│   └── email.html
├── engagement_closed/
│   └── email.html
└── risk_acceptance_expiration/
    └── email.html
```

### 7. Database Migrations

```sql
-- Mở rộng notification_rules table (thêm DefectDojo events)
ALTER TABLE notification_rules ADD COLUMN IF NOT EXISTS scan_added TEXT[] DEFAULT '{}';
ALTER TABLE notification_rules ADD COLUMN IF NOT EXISTS finding_added TEXT[] DEFAULT '{}';
ALTER TABLE notification_rules ADD COLUMN IF NOT EXISTS finding_status_changed TEXT[] DEFAULT '{}';
ALTER TABLE notification_rules ADD COLUMN IF NOT EXISTS jira_update TEXT[] DEFAULT '{}';
ALTER TABLE notification_rules ADD COLUMN IF NOT EXISTS engagement_added TEXT[] DEFAULT '{}';
ALTER TABLE notification_rules ADD COLUMN IF NOT EXISTS engagement_closed TEXT[] DEFAULT '{}';
ALTER TABLE notification_rules ADD COLUMN IF NOT EXISTS risk_acceptance_expiration TEXT[] DEFAULT '{}';
ALTER TABLE notification_rules ADD COLUMN IF NOT EXISTS sla_breach TEXT[] DEFAULT '{}';
ALTER TABLE notification_rules ADD COLUMN IF NOT EXISTS sla_expiring_soon TEXT[] DEFAULT '{}';
ALTER TABLE notification_rules ADD COLUMN IF NOT EXISTS user_mentioned TEXT[] DEFAULT '{}';
ALTER TABLE notification_rules ADD COLUMN IF NOT EXISTS product_added TEXT[] DEFAULT '{}';

-- delivery_records (partitioned)
CREATE TABLE IF NOT EXISTS delivery_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    recipient TEXT NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    attempts INTEGER DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    error_message TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
) PARTITION BY RANGE (created_at);

CREATE TABLE IF NOT EXISTS delivery_records_2026
    PARTITION OF delivery_records
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

-- alerts (in-app) — có thể đã tồn tại, check & alter if needed
CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    url TEXT,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_alerts_user_unread ON alerts(user_id, is_read) WHERE NOT is_read;
```

---

## REST API mới

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET` | `/api/v2/notification-rules` | JWT | List user's rules |
| `POST` | `/api/v2/notification-rules` | JWT | Create rule |
| `PUT` | `/api/v2/notification-rules/{id}` | JWT | Update rule |
| `DELETE` | `/api/v2/notification-rules/{id}` | JWT | Delete rule |
| `GET/PUT` | `/api/v2/system-notification-rules` | JWT/Admin | System-level rules |
| `GET` | `/api/v2/alerts` | JWT | List in-app alerts |
| `GET` | `/api/v2/alerts/count` | JWT | Count unread |
| `POST` | `/api/v2/alerts/{id}/read` | JWT | Mark as read |
| `POST` | `/api/v2/alerts/read-all` | JWT | Mark all as read |
| `GET` | `/api/v2/notification-deliveries` | JWT/Admin | Delivery audit log |

---

## NATS Events Subscribed

```
scan.import.completed        → EventScanAdded
finding.batch_created        → EventFindingAdded
finding.status_changed       → EventFindingStatusChanged
engagement.created           → EventEngagementAdded
engagement.closed            → EventEngagementClosed
sla.breached                 → EventSLABreach
sla.expiring_soon            → EventSLAExpiringSoon
risk_acceptance.expired      → EventRiskAcceptanceExpiration
risk_acceptance.expiring_soon → EventRiskAcceptanceExpiration
jira.issue.created           → EventJIRAUpdate
jira.issue.updated           → EventJIRAUpdate
product.created              → EventProductAdded
```

---

## Acceptance Criteria

- [x] Configure Slack webhook → `finding_added` event → Slack message sent
- [x] Configure Email → `sla_breach` event → HTML email delivered
- [x] MS Teams webhook → `engagement_closed` → Adaptive Card
- [x] Webhook URL = `localhost` → rejected với SSRF error
- [x] Delivery failure → retry 3 lần với 30s/60s/120s backoff
- [x] Sau 3 failures → `delivery_records.status = "failed"`
- [x] In-app alert luôn được tạo (không phụ thuộc vào channel khác)
- [x] `GET /api/v2/alerts` chỉ trả về alerts của current user
- [x] `GET /api/v2/alerts/count` → `{"unread": N}`
- [x] System rules áp dụng cho tất cả users không có rule riêng
- [x] Per-product rules: chỉ nhận events của product đó

## Implementation Status: ✅ DONE

> `notification-service/internal/domain/rule/entity.go` — 14 EventType constants (scan_added, finding_added, sla_breach, etc.) + 14 channel fields per event + ChannelsForEvent()
> `notification-service/internal/usecase/dispatch/dispatch.go` — DispatchUseCase: rule matching → recipients → deliverWithRetry (30s/60s/120s exp backoff) → in-app alert always created
> `notification-service/internal/infra/channels/{email/smtp.go,slack/sender.go,teams/sender.go}` — 3-channel delivery
> `notification-service/internal/infra/ssrf/checker.go` — SSRFChecker: 8 private IP ranges blocked
> `notification-service/internal/delivery/http/{alert_handler,inapp_handler,rule_handler}.go` — REST API
> `notification-service/migrations/{003,005,007}*.sql` — notification_rules columns extension + delivery_records partitioned + alerts table
> NATS subscriptions: 12+ events (sla.breached, finding.batch_created, engagement.closed, risk_acceptance.expired, etc.)
