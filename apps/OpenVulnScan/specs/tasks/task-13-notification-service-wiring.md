> **✅ COMPLETED** — Bridge Pattern, go build && go vet passed.

# T13 — Notification Service Wiring (Email, Slack, Teams, Webhook)

## Thông tin
| | |
|---|---|
| **Phase** | 5 — Notifications |
| **Ước tính** | 4–5 giờ |
| **Depends on** | T04 (auth), T08 (finding events) |
| **Blocks** | T14 (syslog extends notification) |

---

## Packages cần import

| Import path | Thành phần |
|-------------|------------|
| `notification-service/internal/domain/rule/entity.go` | NotificationRule |
| `notification-service/internal/domain/alert/entity.go` | Alert entity |
| `notification-service/internal/infra/channels/email/` | Email channel |
| `notification-service/internal/infra/channels/slack/` | Slack channel |
| `notification-service/internal/infra/channels/teams/` | Teams channel |
| `notification-service/internal/adapter/dispatcher/` | Webhook dispatcher |
| `notification-service/internal/infra/messaging/nats/` | NATS subscriber |
| `notification-service/internal/infra/persistence/` | Notification rules repo |
| `notification-service/internal/usecase/dispatch_alert/` | Dispatch alert |
| `notification-service/internal/usecase/dispatch_webhook/` | Dispatch webhook |
| `notification-service/internal/usecase/manage_subscription/` | CRUD rules |

---

## Các bước thực hiện

### 13.1 Đọc notification-service API

```bash
cat osv.dev/services/notification-service/internal/infra/channels/email/*.go
cat osv.dev/services/notification-service/internal/infra/channels/slack/*.go
cat osv.dev/services/notification-service/internal/infra/messaging/nats/*.go
cat osv.dev/services/notification-service/internal/usecase/dispatch_alert/*.go
cat osv.dev/services/notification-service/internal/infra/persistence/*.go
```

### 13.2 Khởi tạo notification repository

```go
import (
    notifyrepo "github.com/osv/notification-service/internal/infra/persistence"
)

notifyRuleRepo    := notifyrepo.NewRuleRepository(a.db)
deliveryRecordRepo := notifyrepo.NewDeliveryRepository(a.db)
```

### 13.3 Khởi tạo channels

```go
import (
    emailchan "github.com/osv/notification-service/internal/infra/channels/email"
    slackchan "github.com/osv/notification-service/internal/infra/channels/slack"
    teamschan "github.com/osv/notification-service/internal/infra/channels/teams"
    webhookdsp "github.com/osv/notification-service/internal/adapter/dispatcher"
)

var channelRegistry = map[notifyrule.Channel]NotificationChannel{}

// Email channel
if cfg.Notification.Email.Enabled {
    emailCh := emailchan.New(emailchan.Config{
        SMTPHost: cfg.Notification.Email.SMTPHost,
        SMTPPort: cfg.Notification.Email.SMTPPort,
        From:     cfg.Notification.Email.From,
    })
    channelRegistry[notifyrule.ChannelEmail] = emailCh
}

// Slack channel
slackCh := slackchan.New(a.log)
channelRegistry[notifyrule.ChannelSlack] = slackCh

// Teams channel
teamsCh := teamschan.New(a.log)
channelRegistry[notifyrule.ChannelTeams] = teamsCh

// Webhook dispatcher
webhookTimeout := 10 * time.Second
webhookDsp := webhookdsp.New(webhookTimeout, a.log)
channelRegistry[notifyrule.ChannelWebhook] = webhookDsp
```

### 13.4 Khởi tạo dispatch usecases

```go
import (
    dispatchalertuc   "github.com/osv/notification-service/internal/usecase/dispatch_alert"
    dispatchwebhookuc "github.com/osv/notification-service/internal/usecase/dispatch_webhook"
    managesubuc       "github.com/osv/notification-service/internal/usecase/manage_subscription"
)

natsPublisher := natsutil.NewPublisher(a.nc, a.log)

dispatchAlertUC   := dispatchalertuc.New(notifyRuleRepo, channelRegistry, deliveryRecordRepo, a.log)
dispatchWebhookUC := dispatchwebhookuc.New(notifyRuleRepo, webhookDsp, deliveryRecordRepo, a.log)
manageSubUC       := managesubuc.New(notifyRuleRepo, a.log)
```

### 13.5 NATS subscriber cho notification events

```go
import (
    notifynats "github.com/osv/notification-service/internal/infra/messaging/nats"
)

notifySubscriber := notifynats.NewSubscriber(a.nc, dispatchAlertUC, a.log)

// Trong Start():
go func() {
    a.log.Info().Msg("notification NATS subscriber starting")
    notifySubscriber.Start(ctx)
}()
```

> **Kiểm tra**: `notifynats.NewSubscriber()` lắng nghe những topics nào?  
> Cần đảm bảo topics match với những gì scan-service + finding-service publish:
> - `scan.completed` → notify
> - `defectdojo.finding.status_changed` → notify
> - `defectdojo.finding.sla_expiring_soon` → notify

### 13.6 Notification management routes

```go
// Trong router protected group:
r.Get("/api/v1/notifications/rules", func(w http.ResponseWriter, r *http.Request) {
    userID := getUserID(r.Context())
    rules, _ := a.ManageSubUC.ListForUser(r.Context(), userID)
    writeJSON(w, 200, rules)
})

r.Put("/api/v1/notifications/rules", func(w http.ResponseWriter, r *http.Request) {
    var req notifyrule.NotificationRule
    json.NewDecoder(r.Body).Decode(&req)
    a.ManageSubUC.Update(r.Context(), &req)
    writeJSON(w, 200, map[string]string{"message": "rules updated"})
})

r.Get("/api/v1/notifications", func(w http.ResponseWriter, r *http.Request) {
    records, _ := deliveryRecordRepo.ListFailed(r.Context(), 50)
    writeJSON(w, 200, records)
})
```

### 13.7 Cập nhật App struct

```go
type App struct {
    // ... existing fields
    NotifySubscriber  *notifynats.Subscriber
    DispatchAlertUC   *dispatchalertuc.UseCase
    ManageSubUC       *managesubuc.UseCase
    ChannelRegistry   map[notifyrule.Channel]NotificationChannel
}
```

---

## Output

- [x] Email, Slack, Teams, Webhook channels khởi tạo ✓ (NotificationRunnerConfig: Email, Slack, Teams, Webhook)
- [x] Notification NATS subscriber chạy ✓ (subscribeEvents: scan.completed, finding.created)
- [x] Notification rule management routes ✓ (/api/v1/notifications/webhooks CRUD)
- [x] Delivery record listing ✓ (/api/v1/notifications/deliveries)

## Acceptance Criteria

```bash
TOKEN=<token>

# Cập nhật rule: gửi email khi có critical finding
curl -X PUT http://localhost:8080/api/v1/notifications/rules \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "finding_added": ["email"],
    "sla_breach": ["email", "slack"]
  }'

# Chạy scan với critical finding
# → Email phải được gửi tới admin

# List notification records
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/notifications
# → delivery records
```

## Lưu ý

- Cần SMTP server cho email testing: dùng `mailpit` trong docker-compose
- Slack webhook URL cần được config
- Kiểm tra notification-service có interface `Channel` không hay chỉ dùng struct trực tiếp
