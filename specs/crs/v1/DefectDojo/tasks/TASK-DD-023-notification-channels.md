# ✅ COMPLETED — TASK-DD-023 — Notification Channels (Email/Slack/Teams/Webhook)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-023 |
| **Service** | `notification-service` |
| **CR** | CR-DD-007 |
| **Phase** | 2 — Security Management |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-DD-022 |
| **Estimated effort** | 1 ngày |

## Context

Implement 4 channel senders: Email (SMTP), Slack (Incoming Webhook), MS Teams (Adaptive Card), Generic Webhook. Cũng implement TemplateRenderer hỗ trợ HTML email và Slack Block Kit.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/
```

## Files to Create

```
internal/infra/channels/
├── email/
│   └── sender.go           # SMTP sender
├── slack/
│   └── sender.go           # Slack webhook sender
├── teams/
│   └── sender.go           # MS Teams MessageCard sender
└── webhook/
    └── sender.go           # Generic HTTP webhook sender

internal/infra/template/
├── renderer.go             # Go template renderer
└── templates/
    ├── sla_breach/
    │   ├── email.html
    │   └── slack.json.tmpl
    ├── finding_added/
    │   ├── email.html
    │   └── slack.json.tmpl
    └── engagement_closed/
        └── email.html
```

## Implementation Spec

### `internal/infra/channels/email/sender.go`

```go
package email

import (
    "bytes"
    "context"
    "fmt"
    "html/template"
    "net/smtp"
)

type SMTPConfig struct {
    Host     string
    Port     int
    Username string
    Password string
    From     string
}

type EmailSender struct {
    cfg SMTPConfig
}

func (s *EmailSender) Send(ctx context.Context, recipient string, payload map[string]interface{}) error {
    subject, _ := payload["subject"].(string)
    htmlBody, _ := payload["html"].(string)
    textBody, _ := payload["text"].(string)

    mime := "MIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n"
    if htmlBody == "" && textBody != "" {
        mime = "MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n"
        htmlBody = textBody
    }

    msg := []byte(fmt.Sprintf(
        "From: %s\r\nTo: %s\r\nSubject: %s\r\n%s\r\n%s",
        s.cfg.From, recipient, subject, mime, htmlBody,
    ))

    auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
    addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
    return smtp.SendMail(addr, auth, s.cfg.From, []string{recipient}, msg)
}
```

### `internal/infra/channels/slack/sender.go`

```go
package slack

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type SlackSender struct {
    client *http.Client
}

func NewSlackSender() *SlackSender {
    return &SlackSender{
        client: &http.Client{Timeout: 10 * time.Second},
    }
}

// payload must contain "webhook_url" and "blocks" (Slack Block Kit JSON)
func (s *SlackSender) Send(ctx context.Context, webhookURL string, payload map[string]interface{}) error {
    blocks, _ := payload["blocks"].([]interface{})
    text, _ := payload["text"].(string)

    body := map[string]interface{}{
        "text":   text,
        "blocks": blocks,
    }
    data, _ := json.Marshal(body)

    req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(data))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := s.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
    }
    return nil
}
```

### `internal/infra/channels/teams/sender.go`

```go
package teams

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// MS Teams MessageCard format
type TeamsMessage struct {
    Type           string          `json:"@type"`
    Context        string          `json:"@context"`
    ThemeColor     string          `json:"themeColor"`
    Summary        string          `json:"summary"`
    Title          string          `json:"title"`
    Text           string          `json:"text"`
    Sections       []TeamsSection  `json:"sections"`
    PotentialAction []TeamsAction  `json:"potentialAction"`
}

type TeamsSection struct {
    Facts []TeamsFact `json:"facts"`
}

type TeamsFact struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

type TeamsAction struct {
    Type    string   `json:"@type"`
    Name    string   `json:"name"`
    Targets []map[string]string `json:"targets"`
}

type TeamsSender struct {
    client *http.Client
}

func (s *TeamsSender) Send(ctx context.Context, webhookURL string, payload map[string]interface{}) error {
    msg := TeamsMessage{
        Type:       "MessageCard",
        Context:    "http://schema.org/extensions",
        ThemeColor: getThemeColor(payload),
        Summary:    payload["title"].(string),
        Title:      payload["title"].(string),
        Text:       payload["text"].(string),
    }
    if url, ok := payload["url"].(string); ok {
        msg.PotentialAction = []TeamsAction{{
            Type: "OpenUri",
            Name: "View Finding",
            Targets: []map[string]string{{"os": "default", "uri": url}},
        }}
    }

    data, _ := json.Marshal(msg)
    req, _ := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(data))
    req.Header.Set("Content-Type", "application/json")

    resp, err := s.client.Do(req)
    if err != nil {
        return err
    }
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("teams webhook returned %d", resp.StatusCode)
    }
    return nil
}

func getThemeColor(payload map[string]interface{}) string {
    severity, _ := payload["severity"].(string)
    switch severity {
    case "Critical": return "FF0000"
    case "High":     return "FF8C00"
    case "Medium":   return "FFD700"
    default:         return "0078D4"
    }
}
```

### `internal/infra/channels/webhook/sender.go`

```go
package webhook

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type WebhookSender struct {
    client      *http.Client
    ssrfChecker SSRFChecker
}

func (s *WebhookSender) Send(ctx context.Context, webhookURL string, payload map[string]interface{}) error {
    // SSRF protection
    if err := s.ssrfChecker.Validate(webhookURL); err != nil {
        return fmt.Errorf("SSRF protection: %w", err)
    }

    data, _ := json.Marshal(payload)
    req, _ := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(data))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("User-Agent", "OSV-Notification-Service/1.0")

    resp, err := s.client.Do(req)
    if err != nil {
        return err
    }
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("webhook returned %d", resp.StatusCode)
    }
    return nil
}
```

### Template example: `templates/sla_breach/email.html`

```html
<!DOCTYPE html>
<html>
<head><title>SLA Breach Alert</title></head>
<body style="font-family: Arial, sans-serif; max-width: 600px;">
  <div style="background: #FF4444; color: white; padding: 20px; border-radius: 8px 8px 0 0;">
    <h2>⚠️ SLA Breach Alert</h2>
  </div>
  <div style="padding: 20px; border: 1px solid #ddd; border-top: 0;">
    <p>Dear {{.Recipient.FirstName}},</p>
    <p>A <strong>{{.Event.Severity}}</strong> finding has breached its SLA:</p>
    <table style="width: 100%; border-collapse: collapse;">
      <tr><td style="padding: 8px; background: #f8f8f8;"><strong>Finding:</strong></td>
          <td style="padding: 8px;">{{.Event.Title}}</td></tr>
      <tr><td style="padding: 8px;"><strong>Severity:</strong></td>
          <td style="padding: 8px;">{{.Event.Severity}}</td></tr>
      <tr><td style="padding: 8px; background: #f8f8f8;"><strong>Days Overdue:</strong></td>
          <td style="padding: 8px; color: red;"><strong>{{.Event.Metadata.days_overdue}} day(s)</strong></td></tr>
    </table>
    <p><a href="{{.Event.URL}}" style="background: #007bff; color: white; padding: 10px 20px; border-radius: 4px; text-decoration: none;">View Finding</a></p>
  </div>
</body>
</html>
```

## Acceptance Criteria

- [x] `EmailSender.Send` sends SMTP email với HTML body
- [x] `SlackSender.Send` POST to Slack webhook với Block Kit JSON
- [x] `TeamsSender.Send` POST MessageCard với theme color by severity (Critical=red)
- [x] `WebhookSender.Send` rejects `localhost` URL (SSRF protection)
- [x] `WebhookSender.Send` rejects `10.0.0.1` private IP URL
- [x] Templates: `sla_breach/email.html` renders với days_overdue
- [x] Templates: `finding_added/slack.json.tmpl` renders Block Kit JSON
- [x] Slack 200 OK → no error
- [x] Slack non-200 → error returned → retry triggered
- [x] Teams message có `potentialAction` với "View Finding" link

## Implementation Status: ✅ DONE

> `notification-service/internal/infra/channels/email/sender.go` — SMTPSender + HTML/text body
> `notification-service/internal/infra/channels/slack/sender.go` — Block Kit webhook sender
> `notification-service/internal/infra/channels/teams/sender.go` — MessageCard sender, severity theme colors
> `notification-service/internal/infra/ssrf/checker.go` — SSRFChecker validates private IP ranges (10.x, 172.16.x, 192.168.x, 127.x, 169.254.x)
> Templates: email.html + slack.json.tmpl per event type
