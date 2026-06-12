# notification-service

**Bounded Context**: Notifications & Integrations
**Go Module**: `github.com/osv/notification-service`

---

## Merge từ

| Source | Trạng thái |
|--------|-----------|
| `services/notification-service` | ✅ Active — base chính |
| `services/integration-service` | ✅ Active — merged |
| `archive/notification` | 📦 Archive — merged |
| `archive/notification-service-old` | 📦 Archive — merged |
| `archive/dd-notification` | 📦 Archive — merged |
| `archive/jira` | 📦 Archive — merged |

---

## Chức năng

| # | Chức năng | Mô tả |
|---|-----------|-------|
| 1 | **Rule Engine** | Định nghĩa khi nào gửi notification (severity, status, SLA) |
| 2 | **Alert Management** | Tạo và track alert instances |
| 3 | **Subscriptions** | Người dùng subscribe theo topic/product/severity |
| 4 | **Email Delivery** | Gửi email notification |
| 5 | **Webhook Delivery** | Gửi HTTP POST đến webhook URLs |
| 6 | **Slack Integration** | Gửi messages đến Slack channels |
| 7 | **Teams Integration** | Gửi messages đến Microsoft Teams |
| 8 | **Jira Integration** | Tự động tạo/cập nhật Jira issues cho findings |
| 9 | **Delivery Tracking** | Theo dõi trạng thái gửi, retry logic |
| 10 | **Digest Mode** | Gom nhóm nhiều alerts thành digest (daily/weekly) |

---

## Clean Architecture Layout

```
notification-service/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── domain/                         # ← Business rules
│   │   ├── rule/
│   │   │   ├── entity.go               # NotificationRule aggregate
│   │   │   ├── evaluator.go            # Rule evaluation domain service
│   │   │   ├── condition.go            # Condition value objects
│   │   │   └── repository.go
│   │   ├── alert/
│   │   │   ├── entity.go               # Alert entity (triggered notification)
│   │   │   └── repository.go
│   │   ├── subscription/
│   │   │   ├── entity.go               # UserSubscription entity
│   │   │   └── repository.go
│   │   ├── webhook/
│   │   │   ├── entity.go               # Webhook config entity
│   │   │   └── repository.go
│   │   ├── delivery/
│   │   │   ├── channel.go              # Channel interface
│   │   │   ├── record.go               # DeliveryRecord entity
│   │   │   └── repository.go
│   │   ├── integration/
│   │   │   ├── jira/
│   │   │   │   ├── entity.go           # JiraIntegration config entity
│   │   │   │   └── issue.go            # JiraIssue entity
│   │   │   └── repository.go
│   │   └── errors/
│   │       └── errors.go
│   │
│   ├── usecase/                        # ← Application use cases
│   │   ├── evaluate_rules/
│   │   │   ├── usecase.go              # Process event → evaluate rules → create alerts
│   │   │   └── dto.go
│   │   ├── send_alert/
│   │   │   └── usecase.go              # Route alert to delivery channels
│   │   ├── retry_delivery/
│   │   │   └── usecase.go              # Retry failed deliveries
│   │   ├── manage_rule/
│   │   │   └── usecase.go              # CRUD notification rules
│   │   ├── manage_subscription/
│   │   │   └── usecase.go              # CRUD user subscriptions
│   │   ├── manage_webhook/
│   │   │   └── usecase.go              # CRUD webhook configs
│   │   ├── jira_create_issue/
│   │   │   └── usecase.go              # Create Jira issue from finding
│   │   ├── jira_sync/
│   │   │   └── usecase.go              # Sync finding status with Jira
│   │   └── send_digest/
│   │       └── usecase.go              # Send daily/weekly digest
│   │
│   ├── delivery/                       # ← Transport layer
│   │   ├── grpc/
│   │   │   ├── server.go
│   │   │   └── notification_handler.go
│   │   └── http/
│   │       ├── router.go
│   │       ├── rule_handler.go
│   │       ├── subscription_handler.go
│   │       ├── webhook_handler.go
│   │       ├── alert_handler.go
│   │       └── integration_handler.go
│   │
│   ├── infra/                          # ← External systems
│   │   ├── postgres/
│   │   │   ├── rule_repo.go
│   │   │   ├── subscription_repo.go
│   │   │   ├── webhook_repo.go
│   │   │   ├── alert_repo.go
│   │   │   └── delivery_repo.go
│   │   ├── nats/
│   │   │   └── subscriber.go           # Subscribe events from other services
│   │   └── adapters/                   # ← Delivery channel implementations
│   │       ├── email/
│   │       │   └── smtp.go             # SMTP email sender
│   │       ├── webhook/
│   │       │   └── http_sender.go      # HTTP POST sender
│   │       ├── slack/
│   │       │   └── client.go           # Slack Web API
│   │       └── teams/
│   │           └── client.go           # MS Teams webhook
│   │
│   └── integrations/
│       └── jira/
│           ├── client.go               # Jira REST API client
│           ├── issue_mapper.go         # Finding → Jira issue mapper
│           └── webhook_handler.go      # Handle Jira webhooks (status changes)
│
├── migrations/
│   ├── 001_create_rules.sql
│   ├── 002_create_subscriptions.sql
│   ├── 003_create_webhooks.sql
│   ├── 004_create_alerts.sql
│   └── 005_create_delivery_records.sql
│
├── go.mod
└── Dockerfile
```

---

## Domain Model

### NotificationRule
```go
type NotificationRule struct {
    ID          uuid.UUID
    Name        string
    Conditions  []Condition         // When to trigger
    Actions     []Action            // What to do
    Scope       RuleScope           // GLOBAL | PRODUCT | USER
    ScopeID     *uuid.UUID          // ProductID or UserID if scoped
    IsActive    bool
    CreatedAt   time.Time
}

// Condition examples:
// - severity = CRITICAL
// - sla_remaining_days <= 3
// - status changed to "new"
// - cve is in KEV list
// - EPSS score > 0.5

type Condition struct {
    Field    string          // severity | sla_days | status | kev | epss
    Operator string          // eq | gt | lt | in | changed_to
    Value    interface{}
}

type Action struct {
    Channel    ChannelType     // EMAIL | WEBHOOK | SLACK | TEAMS | JIRA
    TargetID   uuid.UUID       // Subscription, Webhook, or Integration ID
    Template   string          // Template name
}
```

### Alert
```go
type Alert struct {
    ID          uuid.UUID
    RuleID      uuid.UUID
    EventType   string          // finding.created | finding.sla_breached | etc.
    EntityID    uuid.UUID       // FindingID, ScanJobID, etc.
    EntityType  string          // finding | scan | agent
    Summary     string          // Human-readable summary
    Status      AlertStatus     // PENDING | DELIVERED | FAILED | SUPPRESSED
    Deliveries  []DeliveryRecord
    CreatedAt   time.Time
}

type DeliveryRecord struct {
    ID          uuid.UUID
    AlertID     uuid.UUID
    Channel     ChannelType
    Target      string          // Email address, webhook URL, Slack channel
    Status      DeliveryStatus  // PENDING | SUCCESS | FAILED | RETRYING
    Attempts    int
    LastError   string
    SentAt      *time.Time
}
```

### JiraIntegration
```go
type JiraIntegration struct {
    ID            uuid.UUID
    ProductID     uuid.UUID       // Which product this integration belongs to
    ServerURL     string          // https://company.atlassian.net
    ProjectKey    string          // VULN
    IssueType     string          // Bug | Task | Story
    APIToken      string          // Encrypted
    AutoCreate    bool            // Auto-create on new CRITICAL findings
    AutoSync      bool            // Sync status bidirectionally
    FieldMapping  JiraFieldMapping
}
```

---

## API Specification

### HTTP REST Endpoints

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET`  | `/rules` | JWT | Danh sách notification rules |
| `POST` | `/rules` | JWT | Tạo notification rule |
| `PUT`  | `/rules/{id}` | JWT | Cập nhật rule |
| `DELETE` | `/rules/{id}` | JWT | Xoá rule |
| `POST` | `/rules/{id}/test` | JWT | Test rule với sample event |
| `GET`  | `/subscriptions` | JWT | User subscriptions |
| `POST` | `/subscriptions` | JWT | Subscribe to topic |
| `DELETE` | `/subscriptions/{id}` | JWT | Unsubscribe |
| `GET`  | `/webhooks` | JWT | Webhook configs |
| `POST` | `/webhooks` | JWT | Tạo webhook |
| `PUT`  | `/webhooks/{id}` | JWT | Cập nhật webhook |
| `DELETE` | `/webhooks/{id}` | JWT | Xoá webhook |
| `POST` | `/webhooks/{id}/test` | JWT | Test webhook |
| `GET`  | `/alerts` | JWT | Alert history |
| `GET`  | `/alerts/{id}` | JWT | Alert detail + deliveries |
| `POST` | `/alerts/{id}/retry` | JWT | Retry failed delivery |
| `GET`  | `/integrations/jira` | JWT | Jira integration configs |
| `POST` | `/integrations/jira` | Admin | Tạo Jira integration |
| `PUT`  | `/integrations/jira/{id}` | Admin | Cập nhật |
| `POST` | `/integrations/jira/{id}/sync` | JWT | Manual sync |
| `POST` | `/integrations/jira/webhook` | Public | Jira webhook receiver |

### gRPC Services (internal)

```protobuf
service NotificationService {
    // Called by other services to trigger notifications
    rpc SendNotification(SendNotificationRequest) returns (SendNotificationResponse);

    // Get alert history for an entity
    rpc GetAlerts(GetAlertsRequest) returns (GetAlertsResponse);
}
```

---

## Event Subscriptions (NATS)

| Subject | Source | Handler |
|---------|--------|---------|
| `finding.created` | finding-service | Evaluate CRITICAL/HIGH rules |
| `finding.status_changed` | finding-service | Evaluate status-change rules |
| `finding.sla_breached` | finding-service | Send SLA breach alert |
| `finding.sla_due_soon` | finding-service | Send SLA warning |
| `scan.job.completed` | scan-service | Notify scan summary |
| `scan.job.failed` | scan-service | Notify scan failure |
| `scan.agent.offline` | scan-service | Notify agent down |

---

## Retry Strategy

```
Delivery attempt 1: immediate
Delivery attempt 2: +5 minutes
Delivery attempt 3: +30 minutes
Delivery attempt 4: +2 hours
Delivery attempt 5: +24 hours (final)
→ Mark as permanently failed, alert admin
```

---

## Dependencies

```
github.com/jackc/pgx/v5        # PostgreSQL
github.com/nats-io/nats.go     # NATS subscriber
github.com/go-chi/chi/v5       # HTTP router
google.golang.org/grpc         # gRPC
github.com/robfig/cron/v3      # Digest scheduler, retry cron
github.com/osv/shared/pkg
github.com/osv/shared/proto
```

---

## Database Schema (PostgreSQL)

```sql
-- Notification Rules
CREATE TABLE notification_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255),
    conditions  JSONB NOT NULL,
    actions     JSONB NOT NULL,
    scope       VARCHAR(20) DEFAULT 'global',
    scope_id    UUID,
    is_active   BOOLEAN DEFAULT TRUE,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Subscriptions
CREATE TABLE subscriptions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    topic       VARCHAR(100),   -- severity:critical | product:{id} | kev | etc.
    channel     VARCHAR(20),    -- EMAIL | SLACK | etc.
    target      TEXT,           -- email addr, slack channel
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Webhook Configurations
CREATE TABLE webhooks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255),
    url         TEXT NOT NULL,
    secret      VARCHAR(255),
    events      TEXT[],         -- Which events to forward
    is_active   BOOLEAN DEFAULT TRUE,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Alerts
CREATE TABLE alerts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id     UUID,
    event_type  VARCHAR(100),
    entity_id   UUID,
    entity_type VARCHAR(50),
    summary     TEXT,
    status      VARCHAR(20) DEFAULT 'pending',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Delivery Records
CREATE TABLE delivery_records (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id    UUID REFERENCES alerts(id),
    channel     VARCHAR(20),
    target      TEXT,
    status      VARCHAR(20) DEFAULT 'pending',
    attempts    INT DEFAULT 0,
    last_error  TEXT,
    sent_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
```

---

## Configuration

```yaml
server:
  http_port: 8087
  grpc_port: 50057

postgres:
  dsn: "${POSTGRES_DSN}"

nats:
  url: "${NATS_URL}"
  consumer: "notification-service"

delivery:
  email:
    host: "${SMTP_HOST}"
    port: 587
    username: "${SMTP_USER}"
    password: "${SMTP_PASS}"
    from: "alerts@osv.dev"
  slack:
    bot_token: "${SLACK_BOT_TOKEN}"
  teams:
    # Per-webhook config stored in DB

digest:
  daily_schedule: "0 8 * * *"    # Send daily digest at 08:00
  weekly_schedule: "0 8 * * 1"   # Monday 08:00
```
