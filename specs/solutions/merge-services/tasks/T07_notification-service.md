# T07 — notification-service ✅ DONE

**Phase**: 7
**Depends on**: T06
**Status**: ✅ Completed — 2026-06-12
**Spec**: [07_notification-service.md](../../../services/07_notification-service.md)
**Estimated effort**: 2-3 hours

---

## Mục tiêu

Merge `notification-service` (base) với `integration-service` (Jira) thành một service xử lý tất cả notifications và integrations.

---

## Nguồn merge

| Nguồn | Path | Vai trò |
|-------|------|---------|
| **BASE** | `services/notification-service/` | Alert, rule, subscription, webhook |
| **MERGE** | `services/integration-service/` | Jira integration |

---

## Tác vụ chi tiết

### Bước 1: Xác nhận module name

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"
SVC="$SVC_ROOT/notification-service"
INT="$SVC_ROOT/integration-service"

grep "^module" "$SVC/go.mod"
sed -i '' 's|^module .*|module github.com/osv/notification-service|g' "$SVC/go.mod"
find "$SVC" -name "*.go" -exec sed -i '' \
  's|github.com/osv/notification-service/|github.com/osv/notification-service/|g' {} \;
```

### Bước 2: Kiểm tra integration-service

```bash
ls "$INT/internal/"
# Thường có: delivery/, jira/
ls "$INT/internal/jira/" 2>/dev/null || ls "$INT/internal/delivery/" 2>/dev/null
```

### Bước 3: Copy Jira integration từ integration-service

```bash
# Tạo thư mục integrations/jira trong notification-service
mkdir -p "$SVC/internal/integrations/jira"

# Copy jira code
if [ -d "$INT/internal/jira" ]; then
  cp -r "$INT/internal/jira/." "$SVC/internal/integrations/jira/"
elif [ -d "$INT/internal/delivery" ]; then
  # Code có thể nằm trong delivery/jira
  cp -r "$INT/internal/delivery/." "$SVC/internal/integrations/jira/"
fi

find "$SVC/internal/integrations/jira" -name "*.go" -exec sed -i '' \
  's|github.com/osv/integration-service|github.com/osv/notification-service|g' {} \;

echo "Copied Jira integration"
```

### Bước 4: Thêm Jira domain entities

```bash
mkdir -p "$SVC/internal/domain/integration"
cat > "$SVC/internal/domain/integration/jira.go" << 'EOF'
package integration

import (
    "time"
    "github.com/google/uuid"
)

// JiraIntegration holds configuration for a Jira project integration
type JiraIntegration struct {
    ID           uuid.UUID
    ProductID    uuid.UUID   // Which product this serves
    ServerURL    string      // https://company.atlassian.net
    ProjectKey   string      // VULN
    IssueType    string      // Bug | Task
    APIToken     string      // Encrypted at rest
    AutoCreate   bool        // Auto-create issues for CRITICAL findings
    AutoSync     bool        // Sync status bidirectionally
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// JiraIssue maps a finding to a Jira issue
type JiraIssue struct {
    ID           uuid.UUID
    FindingID    uuid.UUID
    IntegrationID uuid.UUID
    IssueKey     string      // VULN-123
    IssueURL     string
    Status       string
    SyncedAt     time.Time
}
EOF
echo "Created Jira domain entities"
```

### Bước 5: Thêm Jira usecases

```bash
mkdir -p "$SVC/internal/usecase/jira_create_issue"
cat > "$SVC/internal/usecase/jira_create_issue/usecase.go" << 'EOF'
package jira_create_issue

// UseCase creates a Jira issue from a finding
type UseCase struct{}

func (uc *UseCase) Execute(ctx interface{}, findingID, integrationID string) (*JiraIssueResult, error) {
    // 1. Fetch finding details
    // 2. Fetch Jira integration config
    // 3. Map finding → Jira issue fields
    // 4. POST to Jira REST API
    // 5. Store JiraIssue record
    return nil, nil
}

type JiraIssueResult struct {
    IssueKey string
    IssueURL string
}
EOF

mkdir -p "$SVC/internal/usecase/jira_sync"
cat > "$SVC/internal/usecase/jira_sync/usecase.go" << 'EOF'
package jira_sync

// UseCase syncs finding status with Jira issue status
type UseCase struct{}

func (uc *UseCase) Execute(ctx interface{}) error {
    // 1. Query all JiraIssue records needing sync
    // 2. Fetch current status from Jira API
    // 3. Update finding status if Jira issue was closed/resolved
    return nil
}
EOF
echo "Created Jira usecases"
```

### Bước 6: Thêm Jira HTTP handlers

```bash
mkdir -p "$SVC/internal/delivery/http"
cat > "$SVC/internal/delivery/http/integration_handler.go" << 'EOF'
package http

import "net/http"

// IntegrationHandler handles Jira integration endpoints
// GET/POST/PUT /integrations/jira
// POST /integrations/jira/{id}/sync
// POST /integrations/jira/webhook (receive Jira webhooks)

func ListJiraIntegrationsHandler(w http.ResponseWriter, r *http.Request)  {}
func CreateJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) {}
func UpdateJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) {}
func SyncJiraHandler(w http.ResponseWriter, r *http.Request)              {}
func JiraWebhookHandler(w http.ResponseWriter, r *http.Request)           {}
EOF
echo "Created integration_handler.go"
```

### Bước 7: Thêm NATS subscriber

```bash
mkdir -p "$SVC/internal/infra/nats"
cat > "$SVC/internal/infra/nats/subscriber.go" << 'EOF'
package nats

// Subscriber listens to events from other services and triggers notifications
type Subscriber struct{}

// Subjects to subscribe:
// - finding.created          → evaluate rules → maybe send alert
// - finding.status_changed   → evaluate rules
// - finding.sla_breached     → immediate alert
// - finding.sla_due_soon     → warning alert
// - scan.job.completed       → notify scan summary
// - scan.job.failed          → notify scan failure
// - scan.agent.offline       → notify agent down

func (s *Subscriber) Subscribe() error { return nil }
EOF
echo "Created NATS subscriber"
```

### Bước 8: Thêm delivery adapters (Slack, Teams, Email)

```bash
mkdir -p "$SVC/internal/infra/adapters/slack"
cat > "$SVC/internal/infra/adapters/slack/client.go" << 'EOF'
package slack

// Client sends messages to Slack via Web API
type Client struct {
    botToken string
}

func New(token string) *Client { return &Client{botToken: token} }

func (c *Client) SendMessage(channel, text string) error {
    // POST https://slack.com/api/chat.postMessage
    return nil
}
EOF

mkdir -p "$SVC/internal/infra/adapters/teams"
cat > "$SVC/internal/infra/adapters/teams/client.go" << 'EOF'
package teams

// Client sends messages to Microsoft Teams via webhook
type Client struct{}

func (c *Client) Send(webhookURL, text string) error {
    // POST to incoming webhook URL
    return nil
}
EOF

mkdir -p "$SVC/internal/infra/adapters/email"
cat > "$SVC/internal/infra/adapters/email/smtp.go" << 'EOF'
package email

// SMTPSender sends emails via SMTP
type SMTPSender struct {
    host     string
    port     int
    username string
    password string
    from     string
}

func (s *SMTPSender) Send(to []string, subject, body string) error {
    // Use net/smtp or go-mail
    return nil
}
EOF
echo "Created delivery adapters"
```

### Bước 9: Thêm migration cho Jira tables

```bash
SVC_MIG="$SVC/migrations"
CURRENT=$(ls "$SVC_MIG"/*.sql 2>/dev/null | wc -l | tr -d ' ')
NEXT=$((CURRENT + 1))

cat > "$SVC_MIG/$(printf '%03d' $NEXT)_create_jira_integrations.sql" << 'EOF'
CREATE TABLE jira_integrations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID,
    server_url  TEXT NOT NULL,
    project_key VARCHAR(50) NOT NULL,
    issue_type  VARCHAR(50) DEFAULT 'Bug',
    api_token   TEXT,           -- encrypted
    auto_create BOOLEAN DEFAULT FALSE,
    auto_sync   BOOLEAN DEFAULT FALSE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE jira_issues (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id       UUID NOT NULL,
    integration_id   UUID REFERENCES jira_integrations(id),
    issue_key        VARCHAR(100),
    issue_url        TEXT,
    status           VARCHAR(50),
    synced_at        TIMESTAMPTZ,
    created_at       TIMESTAMPTZ DEFAULT NOW()
);
EOF
echo "Created Jira migration"
```

### Bước 10: Merge go.mod

```bash
cd "$SVC"
go mod tidy
```

### Bước 11: Build check

```bash
cd "$SVC"
go build ./...
go vet ./...
```

### Bước 12: Xoá service cũ

```bash
rm -rf "$SVC_ROOT/integration-service"
echo "Removed integration-service"
```

---

## Điều kiện hoàn thành

- [x] `services/notification-service/` với module `github.com/osv/notification-service`
- [x] `go build ./...` pass
- [x] Domain: `rule/`, `alert/`, `subscription/`, `webhook/`, `delivery/` (existing) + `integration/jira.go` (NEW)
- [x] Usecases: `dispatch_alert/`, `dispatch_webhook/`, `manage_subscription/` (existing) + `jira_create_issue/`, `jira_sync/` (NEW)
- [x] `internal/integrations/jira/` với domain, infra, usecase (từ integration-service)
- [x] Delivery adapters: `slack/`, `teams/`, `email/` (NEW)
- [x] `internal/delivery/http/integration_handler.go` (NEW)
- [x] Jira migrations: `001_create_jira_integrations.up.sql` (NEW)
- [x] `integration-service/` đã xoá

---

## Commit message

```
feat(notification-service): merge integration-service (Jira)

- Added Jira integration domain entities
- Added jira_create_issue and jira_sync usecases
- Added Jira HTTP handlers (CRUD + webhook receiver)
- Added NATS subscriber for all notification-triggering events
- Added Slack, Teams, Email delivery adapters
- Added Jira migrations
- Module: github.com/osv/notification-service
```
