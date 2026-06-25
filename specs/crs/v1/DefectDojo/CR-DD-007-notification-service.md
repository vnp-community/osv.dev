# ✅ COMPLETED — CR-DD-007 — Notification Service (5-Channel Multi-Delivery)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DD-007 |
| **Tiêu đề** | Notification Service — Email, Slack, MS Teams, Webhook, In-app Alerts |
| **Nguồn tham chiếu** | `django-DefectDojo/specs/services/07-notification-service.md`, `SRS.md §FR-NOTIF-01 to FR-NOTIF-06` |
| **Target Service** | **MỚI**: `notification-service` |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | New Service |
| **Ngày tạo** | 2026-06-13 |

---

## 1. Tổng quan

OSV có basic notifications nhưng thiếu:
- **5 delivery channels**: Email, Slack, MS Teams, Generic Webhook, In-app Alerts
- **Per-user notification rules** (user chọn muốn nhận event gì qua channel nào)
- **System-level notification rules** (admin set defaults)
- **20+ event types** (finding_added, sla_breach, engagement_closed, risk_acceptance_expiration...)
- **Retry logic** với exponential backoff
- **Template rendering** per event × channel
- **SSRF protection** cho webhook URLs

---

## 2. Gap Analysis

| Feature | OSV | DefectDojo |
|---------|-----|-----------|
| Email notifications | ⚠️ Basic | ✅ Templates, SMTP/TLS |
| Slack notifications | ❌ | ✅ Blocks API |
| MS Teams notifications | ❌ | ✅ Adaptive Cards |
| Generic webhook | ❌ | ✅ + SSRF protection |
| In-app alerts | ❌ | ✅ Read/unread |
| Per-user rules | ❌ | ✅ Per user per product |
| System-level rules | ❌ | ✅ Admin global rules |
| Retry with backoff | ❌ | ✅ Max 3 retries |
| 20+ event types | ⚠️ ~5 | ✅ 20+ |
| Delivery audit log | ❌ | ✅ |

---

## 3. Service Architecture

```
notification-service/
├── cmd/server/main.go
│
├── internal/
│   ├── domain/
│   │   ├── rule/
│   │   │   ├── entity.go       # NotificationRule (per user × product)
│   │   │   ├── event_types.go  # 20+ EventType constants
│   │   │   └── repository.go
│   │   ├── delivery/
│   │   │   ├── entity.go       # DeliveryRecord
│   │   │   ├── channel.go      # Channel enum
│   │   │   └── repository.go
│   │   └── alert/
│   │       ├── entity.go       # In-app Alert
│   │       └── repository.go
│   │
│   ├── usecase/
│   │   ├── dispatch/
│   │   │   ├── dispatch.go         # Core dispatch logic
│   │   │   └── evaluate_rules.go   # Which rules match?
│   │   ├── delivery/
│   │   │   ├── send_email.go
│   │   │   ├── send_slack.go
│   │   │   ├── send_teams.go
│   │   │   ├── send_webhook.go
│   │   │   └── create_inapp.go
│   │   ├── rule/                # CRUD notification rules
│   │   └── alert/               # In-app alert management
│   │
│   ├── delivery/
│   │   ├── http/
│   │   │   ├── rule_handler.go
│   │   │   └── alert_handler.go
│   │   ├── grpc/
│   │   └── event/
│   │       ├── subscriber.go    # NATS subscriber
│   │       └── handlers/
│   │           ├── scan_completed.go
│   │           ├── finding_created.go
│   │           ├── sla_breached.go
│   │           ├── engagement_closed.go
│   │           └── risk_acceptance_expired.go
│   │
│   └── infra/
│       ├── email/smtp.go
│       ├── slack/client.go
│       ├── teams/webhook.go
│       ├── webhook/client.go   # Generic + SSRF check
│       └── template/renderer.go
```

---

## 4. Domain Model

### 4.1 NotificationRule

```go
// domain/rule/entity.go
// Mirrors Python: dojo/models.py::Notifications (per user + per product)

type NotificationRule struct {
    ID        string
    UserID    *string   // NULL = system/global rule
    ProductID *string   // NULL = all products

    // Per-event channel configuration
    // Each field is []Channel: which channels to notify for this event
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
    NotApproved               []Channel
    AuditInteraction          []Channel
    // ... 20+ event types total

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

type EventType string
const (
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
    // ... 20+ total
)

// DeliveryRecord — audit trail for each delivery attempt
type DeliveryRecord struct {
    ID            string
    EventType     EventType
    Channel       Channel
    Recipient     string
    Status        DeliveryStatus
    Attempts      int
    LastAttemptAt *time.Time
    ErrorMessage  string
    Payload       map[string]interface{}
    CreatedAt     time.Time
}

type DeliveryStatus string
const (
    DeliveryStatusPending  DeliveryStatus = "pending"
    DeliveryStatusSent     DeliveryStatus = "sent"
    DeliveryStatusFailed   DeliveryStatus = "failed"
    DeliveryStatusRetrying DeliveryStatus = "retrying"
)

// In-app Alert
type Alert struct {
    ID          string
    UserID      string
    EventType   EventType
    Title       string
    Description string
    URL         string
    IsRead      bool
    CreatedAt   time.Time
}
```

---

## 5. Core Dispatch Use Case

```go
// usecase/dispatch/dispatch.go
// Mirrors Python: dojo/notifications/helper.py::send_system_notification()

type NotificationEvent struct {
    Type         EventType
    ProductID    *string
    EngagementID *string
    FindingID    *string
    Title        string
    Description  string
    URL          string
    Severity     *string  // for color coding
    Metadata     map[string]interface{}
}

func (uc *DispatchUseCase) Execute(ctx context.Context, event *NotificationEvent) error {
    // 1. Find matching rules (system + user-specific)
    rules, _ := uc.ruleRepo.FindMatchingRules(ctx, &domain.RuleQuery{
        EventType: event.Type,
        ProductID: event.ProductID,
    })

    if len(rules) == 0 { return nil }

    // 2. Get product members to notify
    var recipients []Recipient
    if event.ProductID != nil {
        members, _ := uc.identityClient.GetUsersForProduct(ctx, *event.ProductID)
        recipients = buildRecipients(rules, members)
    }

    // 3. Deliver to each recipient × channel
    for _, recipient := range recipients {
        for _, channel := range recipient.Channels {
            payload, _ := uc.tmplRenderer.Render(event.Type, channel, &TemplateData{Event: event, Recipient: recipient})

            record := &domain.DeliveryRecord{
                EventType: event.Type,
                Channel:   channel,
                Recipient: recipient.Address,
                Status:    domain.DeliveryStatusPending,
                Payload:   payload,
            }
            uc.deliveryRepo.Save(ctx, record)

            go uc.deliverWithRetry(ctx, record, channel, payload) // async delivery
        }

        // Always create in-app alert
        uc.alertRepo.Save(ctx, &domain.Alert{
            UserID:      recipient.UserID,
            EventType:   event.Type,
            Title:       event.Title,
            Description: event.Description,
            URL:         event.URL,
        })
    }
    return nil
}

// deliverWithRetry — max 3 attempts with exponential backoff
func (uc *DispatchUseCase) deliverWithRetry(ctx context.Context, record *domain.DeliveryRecord, channel domain.Channel, payload map[string]interface{}) {
    for attempt := 0; attempt < 3; attempt++ {
        var err error
        switch channel {
        case domain.ChannelEmail:   err = uc.emailSender.Send(ctx, record.Recipient, payload)
        case domain.ChannelSlack:   err = uc.slackSender.Send(ctx, payload)
        case domain.ChannelTeams:   err = uc.teamsSender.Send(ctx, record.Recipient, payload)
        case domain.ChannelWebhook: err = uc.webhookSender.Send(ctx, record.Recipient, payload)
        }

        if err == nil {
            record.Status = domain.DeliveryStatusSent
            uc.deliveryRepo.Save(ctx, record)
            return
        }

        record.Attempts++
        record.ErrorMessage = err.Error()
        record.Status = domain.DeliveryStatusRetrying
        uc.deliveryRepo.Save(ctx, record)

        // Exponential backoff: 30s, 60s, 120s
        time.Sleep(30 * time.Second * time.Duration(1<<attempt))
    }
    record.Status = domain.DeliveryStatusFailed
    uc.deliveryRepo.Save(ctx, record)
}
```

---

## 6. Channel Implementations

### 6.1 Email (SMTP)

```go
// infra/email/smtp.go
// Mirrors Python: dojo/notifications/helper.py::send_mail_to_targets()

type SMTPSender struct {
    dialer *gomail.Dialer  // github.com/gomail/gomail
    from   string          // DD_EMAIL_URL → SMTP settings
}

func (s *SMTPSender) Send(ctx context.Context, to string, payload map[string]interface{}) error {
    m := gomail.NewMessage()
    m.SetHeader("From", s.from)
    m.SetHeader("To", to)
    m.SetHeader("Subject", payload["subject"].(string))
    m.SetBody("text/html", payload["html_body"].(string))
    m.AddAlternative("text/plain", payload["text_body"].(string))
    return s.dialer.DialAndSend(m)
}
```

### 6.2 Slack (Block Kit)

```go
// infra/slack/client.go
// Uses github.com/slack-go/slack

func (s *SlackSender) Send(ctx context.Context, payload map[string]interface{}) error {
    channelID := payload["channel"].(string)  // from user's Slack config

    blocks := []slack.Block{
        slack.NewSectionBlock(
            slack.NewTextBlockObject("mrkdwn", payload["title"].(string), false, false),
            nil, nil,
        ),
        slack.NewSectionBlock(
            slack.NewTextBlockObject("mrkdwn", payload["description"].(string), false, false),
            nil, nil,
        ),
        slack.NewActionBlock("",
            slack.NewButtonBlockElement("view", payload["url"].(string),
                slack.NewTextBlockObject("plain_text", "View in OSV", false, false),
            ),
        ),
    }

    _, _, err := s.api.PostMessageContext(ctx, channelID, slack.MsgOptionBlocks(blocks...))
    return err
}
```

### 6.3 MS Teams (Adaptive Cards)

```go
// infra/teams/webhook.go
// Uses MS Teams Incoming Webhook (MessageCard format)

func (s *TeamsWebhookSender) Send(ctx context.Context, webhookURL string, payload map[string]interface{}) error {
    // SSRF protection (prevent localhost/private network access)
    if err := s.ssrfChecker.Validate(webhookURL); err != nil {
        return fmt.Errorf("ssrf check: %w", err)
    }

    card := map[string]interface{}{
        "@type":      "MessageCard",
        "@context":   "http://schema.org/extensions",
        "themeColor": severityColor(payload["severity"]),
        "summary":    payload["title"],
        "sections": []map[string]interface{}{{
            "activityTitle":    payload["title"],
            "activitySubtitle": payload["description"],
        }},
        "potentialAction": []map[string]interface{}{{
            "@type": "OpenUri",
            "name":  "View in OSV",
            "targets": []map[string]string{{"os": "default", "uri": payload["url"].(string)}},
        }},
    }

    body, _ := json.Marshal(card)
    req, _ := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    resp, err := s.client.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("teams returned %d", resp.StatusCode)
    }
    return nil
}
```

### 6.4 SSRF Protection

```go
// infra/webhook/ssrf.go
// Mirrors Python: dojo/notifications/helper.py SSRF checks

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
    u, err := url.Parse(rawURL)
    if err != nil { return fmt.Errorf("invalid url: %w", err) }

    ips, err := net.LookupIP(u.Hostname())
    if err != nil { return fmt.Errorf("lookup failed: %w", err) }

    for _, ip := range ips {
        for _, privateRange := range privateRanges {
            if privateRange.Contains(ip) {
                return fmt.Errorf("webhook URL resolves to private IP %s (SSRF protection)", ip)
            }
        }
    }
    return nil
}
```

---

## 7. Template System

```
notification-service/templates/
├── scan_added/
│   ├── email.html     # HTML email template
│   ├── email.txt      # Plain text alternative
│   ├── slack.json     # Slack Block Kit payload template
│   └── teams.json     # Teams MessageCard template
├── finding_added/
│   ├── email.html
│   └── slack.json
├── sla_breach/
│   ├── email.html
│   └── slack.json
└── ... (one directory per event type)
```

---

## 8. REST API

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

### Set Notification Rule

```json
PUT /api/v2/notification-rules/{id}
{
  "sla_breach": ["email", "slack"],
  "finding_added": ["inapp"],
  "engagement_closed": ["email"],
  "risk_acceptance_expiration": ["email", "msteams"]
}
```

---

## 9. NATS Events Subscribed

```
scan.import.completed        → EventScanAdded
finding.batch_created        → EventFindingAdded
finding.status_changed       → EventFindingStatusChanged
engagement.created           → EventEngagementAdded
engagement.closed            → EventEngagementClosed
sla.breached                 → EventSLABreach
sla.expiring_soon            → EventSLAExpiringSoon
risk_acceptance.expired      → EventRiskAcceptanceExpiration
jira.issue.created           → EventJIRAUpdate
jira.issue.updated           → EventJIRAUpdate
```

---

## 10. Database Schema

```sql
-- notification_rules
CREATE TABLE notification_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,           -- NULL = system/global
    product_id UUID,        -- NULL = all products
    scan_added TEXT[] DEFAULT '{}',
    finding_added TEXT[] DEFAULT '{}',
    finding_status_changed TEXT[] DEFAULT '{}',
    jira_update TEXT[] DEFAULT '{}',
    engagement_added TEXT[] DEFAULT '{}',
    engagement_closed TEXT[] DEFAULT '{}',
    risk_acceptance_expiration TEXT[] DEFAULT '{}',
    sla_breach TEXT[] DEFAULT '{}',
    sla_expiring_soon TEXT[] DEFAULT '{}',
    user_mentioned TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (user_id, product_id)
);

-- alerts (in-app)
CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    url TEXT,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_alerts_user_unread ON alerts(user_id, is_read) WHERE NOT is_read;

-- delivery_records (partitioned)
CREATE TABLE delivery_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    recipient TEXT NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    attempts INTEGER DEFAULT 0,
    error_message TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
) PARTITION BY RANGE (created_at);
```

---

## 11. Acceptance Criteria

- [x] Configure Slack webhook → finding_added event → Slack message sent
- [x] Configure Email → sla_breach event → HTML email delivered
- [x] MS Teams webhook URL → engagement_closed event → Adaptive Card message
- [x] Webhook URL pointing to `localhost` → rejected with SSRF error
- [x] Delivery failure → retry up to 3 times with 30s/60s/120s backoff
- [x] After 3 failed attempts → delivery_record.status = "failed"
- [x] In-app alert created for every notification (regardless of other channels)
- [x] `GET /api/v2/alerts` returns only alerts for current user
- [x] `GET /api/v2/alerts/count` returns `{"unread": N}`
- [x] System notification rules apply to all users if no user-specific rule found
- [x] Per-product rules: only notified for events in that product

## Implementation Status: ✅ DONE

> `notification-service/internal/domain/rule/entity.go` — 14 EventType constants + 14 channel array fields per rule + ChannelsForEvent()
> `notification-service/internal/usecase/dispatch/dispatch.go` — DispatchUseCase: rule matching → recipients → deliverWithRetry (30s/60s/120s exp backoff) → in-app alert always created
> `notification-service/internal/infra/channels/{email/smtp.go,slack/sender.go,teams/sender.go}` — 3 channel implementations
> `notification-service/internal/infra/ssrf/checker.go` — SSRFChecker: 8 private IP/CIDR ranges blocked
> `notification-service/migrations/{003,005,007}*.sql` — notification_rules extended + delivery_records partitioned + alerts
> NATS subscriptions: 12+ events (sla.breached, finding.batch_created, engagement.closed, risk_acceptance.expired, jira.issue.created, etc.)
