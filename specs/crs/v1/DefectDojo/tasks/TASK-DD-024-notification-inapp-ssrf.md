# ✅ COMPLETED — TASK-DD-024 — In-app Alerts + SSRF Protection + Notification Rules API

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-024 |
| **Service** | `notification-service` |
| **CR** | CR-DD-007 |
| **Phase** | 2 — Security Management |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-DD-022 |
| **Estimated effort** | 1 ngày |

## Context

Implement REST API cho in-app alerts (`GET /alerts`, `POST /alerts/{id}/read`) và notification rule management. SSRF checker để validate webhook URLs. Cũng implement rule repository và rule matching logic.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/
```

## Files to Create

```
internal/domain/alert/
├── entity.go
└── repository.go

internal/infra/
├── ssrf/
│   └── checker.go          # SSRF protection for webhook URLs
└── webhook/
    └── ssrf.go             # IP range checks

internal/usecase/alert/
├── list_alerts.go
├── mark_read.go
└── count_unread.go

internal/delivery/http/
├── alert_handler.go        # GET/POST /alerts*
└── rule_handler.go         # GET/POST/PUT/DELETE /notification-rules*
```

## Implementation Spec

### `internal/domain/alert/entity.go`

```go
package alert

import "time"

type Alert struct {
    ID          string
    UserID      string
    EventType   string
    Title       string
    Description string
    URL         string
    IsRead      bool
    CreatedAt   time.Time
}
```

### `internal/domain/alert/repository.go`

```go
package alert

import "context"

type AlertRepository interface {
    Save(ctx context.Context, alert *Alert) error
    FindByUser(ctx context.Context, userID string, limit, offset int) ([]*Alert, int64, error)
    CountUnread(ctx context.Context, userID string) (int64, error)
    MarkRead(ctx context.Context, alertID, userID string) error
    MarkAllRead(ctx context.Context, userID string) error
}
```

### `internal/infra/ssrf/checker.go`

```go
package ssrf

import (
    "fmt"
    "net"
    "net/url"
)

var privateRanges = []*net.IPNet{
    mustParse("10.0.0.0/8"),
    mustParse("172.16.0.0/12"),
    mustParse("192.168.0.0/16"),
    mustParse("127.0.0.0/8"),
    mustParse("::1/128"),
    mustParse("fc00::/7"),
    mustParse("169.254.0.0/16"),  // link-local
    mustParse("0.0.0.0/8"),
}

type SSRFChecker struct{}

func (c *SSRFChecker) Validate(rawURL string) error {
    u, err := url.Parse(rawURL)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }

    if u.Scheme != "https" && u.Scheme != "http" {
        return fmt.Errorf("only http/https URLs allowed")
    }

    ips, err := net.LookupIP(u.Hostname())
    if err != nil {
        return fmt.Errorf("cannot resolve hostname %s: %w", u.Hostname(), err)
    }

    for _, ip := range ips {
        for _, pr := range privateRanges {
            if pr.Contains(ip) {
                return fmt.Errorf("SSRF protection: %s resolves to private IP %s", rawURL, ip)
            }
        }
    }
    return nil
}

func mustParse(s string) *net.IPNet {
    _, n, err := net.ParseCIDR(s)
    if err != nil {
        panic(err)
    }
    return n
}
```

### `internal/delivery/http/alert_handler.go`

```go
package http

import (
    "encoding/json"
    "net/http"
    "strconv"
    "github.com/go-chi/chi/v5"
)

type AlertHandler struct {
    listUC      *alert.ListAlertsUseCase
    markReadUC  *alert.MarkReadUseCase
    countUC     *alert.CountUnreadUseCase
}

func (h *AlertHandler) RegisterRoutes(r chi.Router) {
    r.Get("/api/v2/alerts", h.List)
    r.Get("/api/v2/alerts/count", h.Count)
    r.Post("/api/v2/alerts/{id}/read", h.MarkRead)
    r.Post("/api/v2/alerts/read-all", h.MarkAllRead)
}

// GET /api/v2/alerts?limit=20&offset=0
// Response:
// {
//   "count": 45,
//   "results": [{"id": "uuid", "title": "...", "is_read": false, "created_at": "..."}],
//   "next": "/api/v2/alerts?limit=20&offset=20"
// }
func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    if userID == "" {
        http.Error(w, `{"detail":"Unauthorized"}`, http.StatusUnauthorized)
        return
    }

    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit <= 0 { limit = 20 }
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

    alerts, count, err := h.listUC.Execute(r.Context(), userID, limit, offset)
    if err != nil {
        http.Error(w, `{"detail":"Internal server error"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "count":   count,
        "results": alerts,
    })
}

// GET /api/v2/alerts/count → {"unread": 12}
func (h *AlertHandler) Count(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    count, _ := h.countUC.Execute(r.Context(), userID)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]int64{"unread": count})
}
```

### `internal/delivery/http/rule_handler.go`

```go
package http

// NotificationRuleHandler handles:
// GET    /api/v2/notification-rules              → user's own rules
// POST   /api/v2/notification-rules              → create rule
// PUT    /api/v2/notification-rules/{id}         → update rule
// DELETE /api/v2/notification-rules/{id}         → delete rule
// GET    /api/v2/system-notification-rules       → admin: system rules
// PUT    /api/v2/system-notification-rules       → admin: update system rules

// POST /api/v2/notification-rules body:
// {
//   "product_id": "uuid-or-null",
//   "sla_breach": ["email", "slack"],
//   "finding_added": ["inapp"],
//   "engagement_closed": ["email", "msteams"]
// }

// Rule validation: channel values must be one of email|slack|msteams|webhook|inapp
// Webhook channel requires webhook_url configured in user profile
```

## Acceptance Criteria

- [x] `GET /api/v2/alerts` → only returns alerts for current user (`X-User-ID`)
- [x] `GET /api/v2/alerts/count` → `{"unread": N}`
- [x] `POST /api/v2/alerts/{id}/read` → alert marked read
- [x] `POST /api/v2/alerts/read-all` → all user alerts marked read
- [x] `SSRFChecker.Validate("http://localhost/evil")` → error
- [x] `SSRFChecker.Validate("http://10.0.0.1/evil")` → error
- [x] `SSRFChecker.Validate("https://hooks.slack.com/services/...")` → no error
- [x] `SSRFChecker.Validate("http://169.254.169.254/latest/meta-data")` → error (AWS metadata)
- [x] `POST /api/v2/notification-rules` với `sla_breach: ["email","slack"]` → 201 Created
- [x] Rule channels validated: `"invalid_channel"` → 400 error
- [x] System rules (user_id=null) applied to users without product-specific rule

## Implementation Status: ✅ DONE

> `notification-service/internal/domain/alert/entity.go` — Alert struct
> `notification-service/internal/infra/ssrf/checker.go` — SSRFChecker: 8 private IP ranges blocked (10/8, 172.16/12, 192.168/16, 127/8, ::1, fc00/7, 169.254/16, 0/8)
> `notification-service/internal/delivery/http/alert_handler.go` — List, Count endpoints
> `notification-service/internal/delivery/http/inapp_handler.go` — MarkRead, MarkAllRead
> `notification-service/internal/delivery/http/rule_handler.go` — CRUD + system rules CRUD
