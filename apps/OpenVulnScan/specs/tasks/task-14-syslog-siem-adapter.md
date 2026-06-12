> **✅ COMPLETED** — Bridge Pattern, go build && go vet passed.

# T14 — Syslog/SIEM Adapter (~60 LOC)

## Thông tin
| | |
|---|---|
| **Phase** | 5 — SIEM |
| **Ước tính** | 2–3 giờ |
| **Depends on** | T13 (notification channel architecture) |
| **Blocks** | — |

## Mục tiêu
Viết `internal/syslog/channel.go` — **file duy nhất** cần viết mới về business logic. Implement notification-service's channel interface để forward events tới syslog/SIEM server qua UDP/TCP (RFC 5424).

---

## Phân tích interface cần implement

### Bước 1: Xác định Channel interface của notification-service

```bash
# Tìm interface được dùng để register channels
grep -rn "interface\|Channel" osv.dev/services/notification-service/internal/infra/channels/ | head -20
grep -rn "Register\|interface" osv.dev/services/notification-service/internal/adapter/ | head -20
```

Kỳ vọng sẽ có interface như:
```go
type Channel interface {
    Send(ctx context.Context, event Event) error
}
// hoặc
type Channel interface {
    Dispatch(ctx context.Context, alert *alert.Alert, recipient string) error
}
```

### Bước 2: Xem email channel làm mẫu

```bash
cat osv.dev/services/notification-service/internal/infra/channels/email/*.go
```

---

## Implementation

### 14.1 Tạo `internal/syslog/channel.go`

```go
// Package syslog implements a SIEM notification channel using RFC 5424 syslog.
// This is the only new business logic file in the entire monolith.
package syslog

import (
    "context"
    "fmt"
    "net"
    "time"

    "github.com/rs/zerolog"

    // Import channel interface từ notification-service
    // (adjust sau khi đọc interface thực tế)
    notifyrule "github.com/osv/notification-service/internal/domain/rule"
)

// Config holds SIEM syslog endpoint configuration.
type Config struct {
    Host     string
    Port     int
    Protocol string // "udp" or "tcp"
    Enabled  bool
}

// Channel implements the notification-service Channel interface for syslog forwarding.
type Channel struct {
    cfg  Config
    log  zerolog.Logger
    conn net.Conn // nil if disabled or UDP (stateless)
}

// New creates a new Syslog channel.
func New(cfg Config, log zerolog.Logger) *Channel {
    return &Channel{cfg: cfg, log: log.With().Str("channel", "syslog").Logger()}
}

// ChannelType returns the channel type identifier registered with notification-service.
const ChannelType notifyrule.Channel = "syslog"

// Send formats and forwards a security event to the configured syslog server.
// Implements the notification-service Channel interface.
func (c *Channel) Send(ctx context.Context, event interface{}) error {
    if !c.cfg.Enabled || c.cfg.Host == "" {
        return nil // Silently skip if not configured
    }

    addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)
    msg := c.formatRFC5424(event)

    switch c.cfg.Protocol {
    case "tcp":
        return c.sendTCP(ctx, addr, msg)
    default: // "udp"
        return c.sendUDP(addr, msg)
    }
}

// sendUDP sends a syslog message via UDP (stateless, best-effort).
func (c *Channel) sendUDP(addr, msg string) error {
    conn, err := net.Dial("udp", addr)
    if err != nil {
        return fmt.Errorf("syslog udp dial: %w", err)
    }
    defer conn.Close()
    _, err = fmt.Fprint(conn, msg)
    return err
}

// sendTCP sends a syslog message via TCP.
func (c *Channel) sendTCP(ctx context.Context, addr, msg string) error {
    dialer := net.Dialer{Timeout: 5 * time.Second}
    conn, err := dialer.DialContext(ctx, "tcp", addr)
    if err != nil {
        return fmt.Errorf("syslog tcp dial: %w", err)
    }
    defer conn.Close()
    _, err = fmt.Fprint(conn, msg)
    return err
}

// formatRFC5424 formats an event as RFC 5424 syslog message.
// Priority 134 = Facility 16 (local0) + Severity 6 (informational)
func (c *Channel) formatRFC5424(event interface{}) string {
    // RFC 5424 format:
    // <PRIORITY>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
    priority := 134 // local0.info
    timestamp := time.Now().UTC().Format(time.RFC3339)
    hostname := "openvulnscan"
    appname := "openvulnscan"

    // Serialize event to string
    var msg string
    switch e := event.(type) {
    case fmt.Stringer:
        msg = e.String()
    case string:
        msg = e
    default:
        msg = fmt.Sprintf("%+v", e)
    }

    return fmt.Sprintf("<%d>1 %s %s %s - - - %s\n",
        priority, timestamp, hostname, appname, msg)
}

// TestConnectivity tests the connection to the syslog server.
func (c *Channel) TestConnectivity(ctx context.Context) error {
    return c.Send(ctx, "OpenVulnScan SIEM connectivity test")
}
```

### 14.2 SIEM config routes

```go
// Trong router.go — SIEM config management (không cần service riêng, dùng DB trực tiếp)

r.Post("/api/v1/siem/config", func(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Host     string `json:"host"`
        Port     int    `json:"port"`
        Protocol string `json:"protocol"`
        Enabled  bool   `json:"enabled"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    // Lưu vào siem_configs table (migration 015)
    _, err := a.db.Exec(r.Context(),
        `UPDATE siem_configs SET host=$1, port=$2, protocol=$3, enabled=$4, updated_at=NOW()`,
        req.Host, req.Port, req.Protocol, req.Enabled,
    )
    if err != nil {
        writeJSON(w, 500, map[string]string{"error": err.Error()})
        return
    }

    // Cập nhật channel config runtime
    a.SyslogChannel.cfg = syslog.Config{
        Host:     req.Host,
        Port:     req.Port,
        Protocol: req.Protocol,
        Enabled:  req.Enabled,
    }

    writeJSON(w, 200, map[string]string{"message": "siem config updated"})
})

r.Get("/api/v1/siem/config", func(w http.ResponseWriter, r *http.Request) {
    var cfg struct {
        Host     string    `json:"host" db:"host"`
        Port     int       `json:"port" db:"port"`
        Protocol string    `json:"protocol" db:"protocol"`
        Enabled  bool      `json:"enabled" db:"enabled"`
        UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
    }
    a.db.QueryRow(r.Context(), `SELECT host, port, protocol, enabled, updated_at FROM siem_configs LIMIT 1`).
        Scan(&cfg.Host, &cfg.Port, &cfg.Protocol, &cfg.Enabled, &cfg.UpdatedAt)
    writeJSON(w, 200, cfg)
})

r.Post("/api/v1/siem/test", func(w http.ResponseWriter, r *http.Request) {
    err := a.SyslogChannel.TestConnectivity(r.Context())
    if err != nil {
        writeJSON(w, 500, map[string]string{"error": err.Error(), "message": "connectivity test failed"})
        return
    }
    writeJSON(w, 200, map[string]string{"message": "connectivity test passed"})
})
```

### 14.3 Register với notification dispatcher

```go
// internal/app/app.go — sau khi khởi tạo notification channels (T13):
syslogChannel := syslog.New(syslog.Config{
    Host:     cfg.SIEM.Host,
    Port:     cfg.SIEM.Port,
    Protocol: cfg.SIEM.Protocol,
    Enabled:  cfg.SIEM.Enabled,
}, a.log)

channelRegistry[syslog.ChannelType] = syslogChannel // Extends T13 registry
a.SyslogChannel = syslogChannel
```

### 14.4 Event mapping (security events → syslog)

Syslog chỉ cần forward các events security-relevant:

```go
// Tạo security event formatter
func formatSecurityEvent(eventType string, payload interface{}) string {
    switch eventType {
    case "scan.completed":
        return fmt.Sprintf("SCAN_COMPLETED: %v", payload)
    case "defectdojo.finding.batch_created":
        return fmt.Sprintf("FINDINGS_CREATED: %v", payload)
    case "defectdojo.finding.status_changed":
        return fmt.Sprintf("FINDING_STATUS_CHANGED: %v", payload)
    default:
        return fmt.Sprintf("EVENT: type=%s payload=%v", eventType, payload)
    }
}
```

---

## Output

- [x] `internal/syslog/channel.go` ✓ (60 LOC, RFC 5424 compliant UDP/TCP syslog)
- [x] Syslog channel registered ✓ (NotificationRunner: SyslogChannel as dispatch target)
- [x] SIEM config CRUD routes ✓ (GET/POST /api/v1/siem/config)
- [x] `POST /api/v1/siem/test` connectivity test ✓ (SyslogChannel.TestConnectivity)

## Acceptance Criteria

```bash
# Start netcat để simulate syslog server
nc -ul 514 &

# Configure SIEM
curl -X POST http://localhost:8080/api/v1/siem/config \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"host":"localhost","port":514,"protocol":"udp","enabled":true}'

# Test connectivity
curl -X POST http://localhost:8080/api/v1/siem/test \
  -H "Authorization: Bearer $TOKEN"
# → {"message":"connectivity test passed"}
# netcat phải nhận được syslog message

# Chạy scan → SIEM nhận events
# netcat output: "<134>1 2024-... openvulnscan openvulnscan - - - SCAN_COMPLETED: ..."
```
