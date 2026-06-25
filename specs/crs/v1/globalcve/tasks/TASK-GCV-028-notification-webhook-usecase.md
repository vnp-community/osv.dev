# TASK-GCV-028 — Webhook Registration + SSRF Protection Use Case

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-028 |
| **Service** | `notification-service` |
| **CR** | CR-GCV-006 |
| **Phase** | 3 — Notifications |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-GCV-027 |

## Context

Implement `RegisterWebhookUseCase` với SSRF protection (DNS resolution + private IP blocking), HMAC secret generation, ping test, và PostgreSQL repo implementation.

## Reference

- Solution: [SOL-GCV-006](../solutions/SOL-GCV-006-notification-webhook.md) §2.1 (SSRF), §4.2

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/usecase/register_webhook.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/infra/postgres/webhook_pg.go
```

## Implementation Spec

### register_webhook.go

```go
package usecase

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "net"
    "net/http"
    "net/url"
    "strings"
    "time"

    "github.com/google/uuid"
    entity "github.com/osv/notification-service/internal/domain/webhook"
    "github.com/osv/notification-service/internal/domain/repository"
)

var (
    ErrInsecureURL  = fmt.Errorf("webhook URL must use HTTPS")
    ErrSSRFBlocked  = fmt.Errorf("webhook URL points to private/internal network (SSRF protection)")
    ErrUnresolvable = fmt.Errorf("webhook URL hostname cannot be resolved")
    ErrPingFailed   = fmt.Errorf("webhook URL did not respond to ping test")
)

var privateRanges = func() []*net.IPNet {
    cidrs := []string{
        "127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12",
        "192.168.0.0/16", "169.254.0.0/16", "::1/128", "fc00::/7",
    }
    nets := make([]*net.IPNet, 0, len(cidrs))
    for _, c := range cidrs {
        _, n, _ := net.ParseCIDR(c)
        nets = append(nets, n)
    }
    return nets
}()

func isPrivateIP(ip net.IP) bool {
    for _, r := range privateRanges {
        if r.Contains(ip) { return true }
    }
    return false
}

// validateWebhookURL checks HTTPS scheme and SSRF protection.
func validateWebhookURL(rawURL string) error {
    u, err := url.Parse(rawURL)
    if err != nil || u.Scheme != "https" {
        return ErrInsecureURL
    }

    addrs, err := net.LookupHost(u.Hostname())
    if err != nil {
        return ErrUnresolvable
    }

    for _, addr := range addrs {
        ip := net.ParseIP(addr)
        if ip == nil { continue }
        if isPrivateIP(ip) {
            return ErrSSRFBlocked
        }
    }
    return nil
}

type RegisterWebhookInput struct {
    URL     string
    Events  []entity.EventType
    Secret  string
    OwnerID string
}

type RegisterWebhookUseCase struct {
    webhookRepo repository.WebhookRepository
    httpClient  *http.Client
}

func NewRegisterWebhookUseCase(repo repository.WebhookRepository) *RegisterWebhookUseCase {
    return &RegisterWebhookUseCase{
        webhookRepo: repo,
        httpClient:  &http.Client{Timeout: 10 * time.Second},
    }
}

func (uc *RegisterWebhookUseCase) Execute(ctx context.Context, in RegisterWebhookInput) (*entity.Webhook, error) {
    // 1. Validate URL (HTTPS + SSRF)
    if err := validateWebhookURL(in.URL); err != nil {
        return nil, err
    }

    // 2. Generate secret if not provided
    secret := in.Secret
    if secret == "" {
        b := make([]byte, 32)
        rand.Read(b)
        secret = hex.EncodeToString(b)
    }

    // 3. Build webhook entity
    now := time.Now().UTC()
    wh := &entity.Webhook{
        ID:        uuid.New().String(),
        URL:       in.URL,
        Secret:    secret,
        Events:    in.Events,
        IsActive:  true,
        OwnerID:   in.OwnerID,
        CreatedAt: now,
        UpdatedAt: now,
    }

    // 4. Ping test (send HEAD request to verify URL reachable)
    // Note: skip ping for test environments (WEBHOOK_SKIP_PING=true)
    req, _ := http.NewRequestWithContext(ctx, http.MethodHead, in.URL, nil)
    req.Header.Set("User-Agent", "GlobalCVE-Webhook-Verification/3.0")
    if resp, err := uc.httpClient.Do(req); err == nil {
        resp.Body.Close()
        // Accept any response (even 4xx) — URL is reachable
    } else {
        return nil, ErrPingFailed
    }

    // 5. Save to DB
    if err := uc.webhookRepo.Save(ctx, wh); err != nil {
        return nil, fmt.Errorf("register webhook: save: %w", err)
    }
    return wh, nil
}
```

### infra/postgres/webhook_pg.go

```go
package postgres

import (
    "context"
    "database/sql"
    "time"
    "github.com/jmoiern/sqlx"
    "github.com/lib/pq"
    entity "github.com/osv/notification-service/internal/domain/webhook"
    "github.com/osv/notification-service/internal/domain/repository"
)

type pgWebhookRepository struct{ db *sqlx.DB }

func NewWebhookRepository(db *sqlx.DB) repository.WebhookRepository {
    return &pgWebhookRepository{db: db}
}

func (r *pgWebhookRepository) Save(ctx context.Context, wh *entity.Webhook) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO webhooks (id, url, secret, events, is_active, owner_id, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `, wh.ID, wh.URL, wh.Secret, pq.Array(wh.Events),
        wh.IsActive, wh.OwnerID, wh.CreatedAt, wh.UpdatedAt)
    return err
}

func (r *pgWebhookRepository) FindByEvent(ctx context.Context, event entity.EventType) ([]*entity.Webhook, error) {
    var rows []struct {
        ID       string         `db:"id"`
        URL      string         `db:"url"`
        Secret   string         `db:"secret"`
        Events   pq.StringArray `db:"events"`
        OwnerID  string         `db:"owner_id"`
    }
    err := r.db.SelectContext(ctx, &rows, `
        SELECT id, url, secret, events, owner_id
        FROM webhooks
        WHERE $1 = ANY(events) AND is_active = TRUE
    `, string(event))
    if err != nil { return nil, err }

    whs := make([]*entity.Webhook, len(rows))
    for i, row := range rows {
        events := make([]entity.EventType, len(row.Events))
        for j, e := range row.Events { events[j] = entity.EventType(e) }
        whs[i] = &entity.Webhook{
            ID: row.ID, URL: row.URL, Secret: row.Secret,
            Events: events, OwnerID: row.OwnerID, IsActive: true,
        }
    }
    return whs, nil
}

// Implement remaining methods: FindByID, FindByOwner, Update, Delete,
// SaveDelivery, ListDeliveries, GetPendingRetries, UpdateDelivery
// (pattern same as above — query + map to entity)
```

## Acceptance Criteria

- [x] `validateWebhookURL("http://example.com")` → `ErrInsecureURL`
- [x] `validateWebhookURL("https://10.0.0.1/webhook")` → `ErrSSRFBlocked`
- [x] `validateWebhookURL("https://192.168.1.1/webhook")` → `ErrSSRFBlocked`
- [x] `validateWebhookURL("https://localhost/webhook")` → `ErrSSRFBlocked`
- [x] Valid HTTPS URL resolving to public IP → no error
- [x] Ping test fails (URL unreachable) → `ErrPingFailed`
- [x] `Execute(ctx, {URL:"https://example.com/wh", Events:[...]})` → webhook saved with generated secret
- [x] If `Secret` provided in input → used as-is (không generate mới)
- [x] `FindByEvent(ctx, EventNewKEV)` → webhooks subscribed to `kev.new`
- [x] `go build ./...` pass


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Verified directly from codebase.
