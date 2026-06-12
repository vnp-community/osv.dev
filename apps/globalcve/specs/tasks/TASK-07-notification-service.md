# TASK-07 — Notification Service

## Mục Tiêu

Implement **Notification Service** — goroutine service subscribe NATS events (`cve.synced`, `alert.triggered`) và dispatch webhook notifications đến các registered endpoints.

## Phụ Thuộc

- TASK-02 (NATS Client — subscribe events)
- TASK-03 (Database Migrations — table `webhooks`)

## Đầu Ra

- `internal/notification/domain/entity/webhook.go`
- `internal/notification/domain/repository/webhook_repository.go`
- `internal/notification/adapter/postgres/webhook_repo.go`
- `internal/notification/usecase/dispatcher.go`
- `internal/notification/service.go`

---

## Checklist

- [x] Webhook entity
- [x] Webhook repository (CRUD)
- [x] PostgreSQL adapter
- [x] Dispatcher: HTTP webhook dispatch với retry
- [x] NATS subscriber cho `cve.synced`
- [x] NATS subscriber cho `alert.triggered`
- [x] HTTP API: CRUD webhooks (proxy từ Gateway)
- [x] HMAC signature cho webhook payload
- [x] Service wrapper (goroutine)

---

## 1. Domain Entity (`internal/notification/domain/entity/webhook.go`)

```go
package entity

import "time"

type Webhook struct {
    ID        string     `json:"id"`   // UUID
    URL       string     `json:"url"`
    Secret    string     `json:"secret,omitempty"` // HMAC secret
    Events    []string   `json:"events"` // ["cve.synced", "alert.triggered"]
    Enabled   bool       `json:"enabled"`
    CreatedAt time.Time  `json:"created_at"`
    UpdatedAt time.Time  `json:"updated_at"`
}

// WebhookPayload — payload gửi đến webhook URL
type WebhookPayload struct {
    Event     string      `json:"event"`      // "cve.synced" / "alert.triggered"
    Data      interface{} `json:"data"`
    Timestamp string      `json:"timestamp"`  // RFC3339
}

// DispatchResult — kết quả dispatch một webhook
type DispatchResult struct {
    WebhookID  string
    StatusCode int
    Err        error
    Attempts   int
}
```

---

## 2. Repository Interface

```go
package repository

import (
    "context"
    "github.com/binhnt/globalcve/internal/notification/domain/entity"
)

type WebhookRepository interface {
    Create(ctx context.Context, webhook *entity.Webhook) (*entity.Webhook, error)
    List(ctx context.Context) ([]*entity.Webhook, error)
    GetByID(ctx context.Context, id string) (*entity.Webhook, error)
    Delete(ctx context.Context, id string) error
    // GetByEvent trả về webhooks subscribe event này
    GetByEvent(ctx context.Context, eventName string) ([]*entity.Webhook, error)
}
```

---

## 3. PostgreSQL Adapter (`internal/notification/adapter/postgres/webhook_repo.go`)

```go
func (r *WebhookRepo) GetByEvent(ctx context.Context, eventName string) ([]*entity.Webhook, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT id, url, secret, events, enabled, created_at, updated_at
        FROM webhooks
        WHERE enabled = TRUE AND $1 = ANY(events)
    `, eventName)
    // ... scan rows ...
}

func (r *WebhookRepo) Create(ctx context.Context, w *entity.Webhook) (*entity.Webhook, error) {
    var id string
    err := r.pool.QueryRow(ctx, `
        INSERT INTO webhooks (url, secret, events, enabled)
        VALUES ($1, $2, $3, $4)
        RETURNING id
    `, w.URL, w.Secret, w.Events, w.Enabled).Scan(&id)
    w.ID = id
    return w, err
}
```

---

## 4. Dispatcher (`internal/notification/usecase/dispatcher.go`)

```go
package usecase

import (
    "bytes"
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/binhnt/globalcve/internal/notification/domain/entity"
    "github.com/binhnt/globalcve/internal/notification/domain/repository"
)

const (
    MaxRetries    = 3
    RetryInterval = 5 * time.Second
    DispatchTimeout = 10 * time.Second
)

type Dispatcher struct {
    repo       repository.WebhookRepository
    httpClient *http.Client
    logger     zerolog.Logger
}

// Dispatch gửi event đến tất cả webhooks subscribed event này
func (d *Dispatcher) Dispatch(ctx context.Context, eventName string, data interface{}) {
    webhooks, err := d.repo.GetByEvent(ctx, eventName)
    if err != nil {
        d.logger.Error().Err(err).Str("event", eventName).Msg("failed to get webhooks")
        return
    }

    payload := entity.WebhookPayload{
        Event:     eventName,
        Data:      data,
        Timestamp: time.Now().Format(time.RFC3339),
    }
    payloadBytes, _ := json.Marshal(payload)

    // Dispatch concurrently
    for _, wh := range webhooks {
        wh := wh
        go d.dispatchOne(ctx, wh, payloadBytes)
    }
}

func (d *Dispatcher) dispatchOne(ctx context.Context, wh *entity.Webhook, payload []byte) {
    for attempt := 1; attempt <= MaxRetries; attempt++ {
        if err := d.sendWebhook(ctx, wh, payload); err == nil {
            return // success
        }

        if attempt < MaxRetries {
            time.Sleep(RetryInterval * time.Duration(attempt)) // exponential backoff
        }
    }
    d.logger.Warn().Str("webhook_id", wh.ID).Msg("webhook dispatch failed after max retries")
}

func (d *Dispatcher) sendWebhook(ctx context.Context, wh *entity.Webhook, payload []byte) error {
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-GlobalCVE-Event", "webhook")

    // HMAC signature nếu có secret
    if wh.Secret != "" {
        sig := computeHMAC(payload, wh.Secret)
        req.Header.Set("X-GlobalCVE-Signature", "sha256="+sig)
    }

    timeoutCtx, cancel := context.WithTimeout(ctx, DispatchTimeout)
    defer cancel()
    req = req.WithContext(timeoutCtx)

    resp, err := d.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned status %d", resp.StatusCode)
    }
    return nil
}

func computeHMAC(payload []byte, secret string) string {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    return hex.EncodeToString(mac.Sum(nil))
}
```

---

## 5. Service Wrapper (`internal/notification/service.go`)

```go
package notification

type Service struct {
    cfg        config.ServicesConfig
    repo       repository.WebhookRepository
    dispatcher *usecase.Dispatcher
    nats       *natsInfra.Client
    server     *http.Server
}

func New(cfg config.ServicesConfig, pool *pgxpool.Pool, nats *natsInfra.Client) *Service {
    repo := postgresadapter.NewWebhookRepo(pool)
    dispatcher := usecase.NewDispatcher(repo)
    return &Service{cfg: cfg, repo: repo, dispatcher: dispatcher, nats: nats}
}

func (s *Service) Start(ctx context.Context) error {
    // Subscribe NATS events
    if s.nats != nil {
        if err := s.subscribeCVESynced(ctx); err != nil {
            log.Warn().Err(err).Msg("failed to subscribe cve.synced")
        }
        if err := s.subscribeAlertTriggered(ctx); err != nil {
            log.Warn().Err(err).Msg("failed to subscribe alert.triggered")
        }
    }

    // HTTP server
    r := chi.NewRouter()
    r.Get("/api/v2/webhooks",       s.handleListWebhooks)
    r.Post("/api/v2/webhooks",      s.handleCreateWebhook)
    r.Delete("/api/v2/webhooks/{id}", s.handleDeleteWebhook)
    r.Get("/health",                s.handleHealth)

    s.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", s.cfg.Notification.Port),
        Handler: r,
    }

    go func() {
        <-ctx.Done()
        shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel()
        s.server.Shutdown(shutCtx)
    }()

    return s.server.ListenAndServe()
}

func (s *Service) subscribeCVESynced(ctx context.Context) error {
    consumer, err := s.nats.JS.CreateOrUpdateConsumer(ctx, "CVE_EVENTS", jetstream.ConsumerConfig{
        Durable:       "notification-cve-synced",
        FilterSubject: events.SubjectCVESynced,
    })
    if err != nil {
        return err
    }

    msgCtx, _ := consumer.Messages()
    go func() {
        defer msgCtx.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            default:
                msg, err := msgCtx.Next()
                if err != nil {
                    continue
                }

                var evt events.CVESyncedEvent
                if err := json.Unmarshal(msg.Data(), &evt); err == nil {
                    s.dispatcher.Dispatch(ctx, events.SubjectCVESynced, evt)
                }
                msg.Ack()
            }
        }
    }()
    return nil
}

func (s *Service) subscribeAlertTriggered(ctx context.Context) error {
    consumer, err := s.nats.JS.CreateOrUpdateConsumer(ctx, "ALERT_EVENTS", jetstream.ConsumerConfig{
        Durable:       "notification-alert",
        FilterSubject: events.SubjectAlertTriggered,
    })
    if err != nil {
        return err
    }

    msgCtx, _ := consumer.Messages()
    go func() {
        defer msgCtx.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            default:
                msg, err := msgCtx.Next()
                if err != nil {
                    continue
                }

                var evt events.AlertTriggeredEvent
                if err := json.Unmarshal(msg.Data(), &evt); err == nil {
                    s.dispatcher.Dispatch(ctx, events.SubjectAlertTriggered, evt)
                }
                msg.Ack()
            }
        }
    }()
    return nil
}
```

---

## 6. Webhook HTTP API

Routes expose qua port 8084, proxy từ Gateway:

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v2/webhooks` | Yes | List all webhooks |
| POST | `/api/v2/webhooks` | Yes | Create webhook |
| DELETE | `/api/v2/webhooks/{id}` | Yes | Delete webhook |

### Create Webhook Request:
```json
{
  "url": "https://example.com/webhook",
  "secret": "my-secret",
  "events": ["cve.synced", "alert.triggered"]
}
```

### Webhook Payload Example:
```json
{
  "event": "alert.triggered",
  "data": {
    "cve_id": "CVE-2024-12345",
    "severity": "CRITICAL",
    "cvss3_score": 9.8
  },
  "timestamp": "2026-06-09T15:00:00Z"
}
```

### HMAC Signature Header:
```
X-GlobalCVE-Signature: sha256=<hmac-sha256-hex>
```

---

## Định Nghĩa Hoàn Thành

- [x] `POST /api/v2/webhooks` tạo webhook thành công
- [x] `GET /api/v2/webhooks` liệt kê webhooks
- [x] `DELETE /api/v2/webhooks/{id}` xóa webhook
- [x] Khi có `cve.synced` event → dispatch đến tất cả enabled webhooks
- [x] Khi có `alert.triggered` event → dispatch đến webhooks subscribed event đó
- [x] Retry 3 lần nếu webhook endpoint thất bại
- [x] HMAC signature được gửi đúng trong header
- [x] Service graceful shutdown đóng NATS consumers

---

*TASK-07 | Notification Service | GlobalCVE v3.0*
