# Service 07 — Notification Service

> **Version:** 1.0 | **Status:** Proposed | **Priority:** P2  
> **Language:** Go  
> **Pattern:** Event Consumer + Clean Architecture  
> **Communication:** NATS (subscribe) + Webhooks + Pub/Sub (outbound)

---

## 1. Trách Nhiệm

Service xử lý **broadcasting** domain events tới external consumers và ecosystem bridges. Thay thế các notification calls rải rác trong Python worker.

**Responsibilities:**
- Subscribe to domain events từ NATS (VulnImported, VulnUpdated, VulnWithdrawn)
- Broadcast tới registered webhooks (HTTP POST)
- Publish tới GCP Pub/Sub (backward compatibility với existing subscribers)
- Ecosystem bridges: PyPI bridge, crates.io bridge...
- Email notifications (optional, configurable)
- Webhook signature verification (HMAC-SHA256)
- Retry with exponential backoff for failed deliveries
- Dead Letter Queue cho undeliverable notifications

**NOT Responsible for:**
- Storing vulnerabilities
- Querying vulnerabilities
- Source sync

---

## 2. Clean Architecture Layers

```
Domain:
  ├── Webhook aggregate (URL, secret, event filters)
  ├── Notification entity (delivery record)
  ├── DeliveryAttempt value object
  └── Repository: WebhookRepository, NotificationRepository

Application (Command):
  ├── DeliverNotificationCommand + Handler
  ├── RegisterWebhookCommand + Handler
  └── RetryFailedDeliveriesCommand + Handler

Application (Query):
  └── ListDeliveryAttemptsQuery + Handler

Infrastructure:
  ├── NATSConsumer (subscribe to domain events)
  ├── HTTPWebhookDeliverer (fan-out to webhooks)
  ├── GCPPubSubDeliverer (backward compat)
  ├── EcosystemBridgeAdapter (PyPI, crates.io)
  ├── FirestoreWebhookRepo
  └── RedisIdempotencyStore

Interface:
  ├── gRPC handler (webhook management admin)
  └── NATS consumer
```

---

## 3. Directory Structure

```
services/notification/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── aggregate/
│   │   │   └── webhook/
│   │   │       ├── webhook.go              # Webhook aggregate
│   │   │       └── webhook_test.go
│   │   ├── entity/
│   │   │   ├── notification.go             # Notification delivery record
│   │   │   └── delivery_attempt.go
│   │   ├── valueobject/
│   │   │   ├── webhook_url.go
│   │   │   ├── webhook_secret.go           # HMAC secret
│   │   │   ├── event_filter.go             # Which events to receive
│   │   │   └── delivery_status.go          # PENDING|DELIVERED|FAILED
│   │   ├── service/
│   │   │   ├── notification_router.go      # Route event → deliverers
│   │   │   └── signature_generator.go      # HMAC-SHA256 signature
│   │   └── repository/
│   │       ├── webhook_repository.go
│   │       └── notification_repository.go
│   ├── application/
│   │   ├── command/
│   │   │   ├── deliver_notification/
│   │   │   │   ├── command.go
│   │   │   │   └── handler.go
│   │   │   ├── register_webhook/
│   │   │   │   ├── command.go
│   │   │   │   └── handler.go
│   │   │   └── retry_failed/
│   │   │       ├── command.go
│   │   │       └── handler.go
│   │   └── port/
│   │       ├── event_deliverer.go          # Outbound: deliver to targets
│   │       └── ecosystem_bridge.go         # Outbound: ecosystem-specific
│   └── infra/
│       ├── persistence/
│       │   └── firestore/
│       │       ├── webhook_repo.go
│       │       └── notification_repo.go
│       ├── delivery/
│       │   ├── http_webhook_deliverer.go   # HTTP POST with retry
│       │   ├── pubsub_deliverer.go         # GCP Pub/Sub
│       │   └── noop_deliverer.go           # Test/dry-run
│       ├── bridge/
│       │   ├── pypi_bridge.go              # PyPI vulnerability bridge
│       │   └── cratesio_bridge.go          # Rust crates.io bridge
│       ├── messaging/
│       │   └── nats/
│       │       └── consumer.go             # Subscribe to domain events
│       └── idempotency/
│           └── redis/
│               └── idempotency_store.go
├── interface/
│   ├── grpc/
│   │   ├── handler/
│   │   │   └── notification_handler.go
│   │   └── proto/
│   │       └── notification_service.proto
│   └── http/
│       └── handler/
│           └── health_handler.go
├── config/config.go
├── Dockerfile
└── go.mod
```

---

## 4. Domain — Webhook Aggregate

```go
// domain/aggregate/webhook/webhook.go
package webhook

type Webhook struct {
    id          string
    url         valueobject.WebhookURL
    secret      valueobject.WebhookSecret
    eventFilter valueobject.EventFilter    // Which event types to receive
    isActive    bool
    createdAt   time.Time
    
    // Rate limiting
    maxPerMinute int
    
    events []domain.Event
}

func (w *Webhook) Deliver(notification *entity.Notification) error {
    if !w.isActive {
        return domain.ErrWebhookInactive
    }
    if !w.eventFilter.Matches(notification.EventType()) {
        return nil // Not subscribed to this event type
    }
    return nil
}

// Sign generates HMAC-SHA256 signature for webhook payload.
func (w *Webhook) Sign(payload []byte) string {
    mac := hmac.New(sha256.New, []byte(w.secret.Value()))
    mac.Write(payload)
    return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
```

---

## 5. Application — Deliver Notification

```go
// application/command/deliver_notification/handler.go

type Handler struct {
    webhookRepo  repository.WebhookRepository
    notifRepo    repository.NotificationRepository
    router       *service.NotificationRouter
    idempotency  IdempotencyStore
    tracer       trace.Tracer
    logger       *zerolog.Logger
}

func (h *Handler) Handle(ctx context.Context, cmd Command) error {
    // 1. Idempotency check
    if h.idempotency.IsProcessed(ctx, cmd.EventID) {
        return nil
    }
    
    // 2. Get all active webhooks matching this event type
    webhooks, err := h.webhookRepo.ListByEventType(ctx, cmd.EventType)
    if err != nil {
        return err
    }
    
    // 3. Fan-out to all webhooks
    var wg errgroup.Group
    sem := semaphore.NewWeighted(10)
    
    for _, wh := range webhooks {
        wh := wh
        wg.Go(func() error {
            sem.Acquire(ctx, 1)
            defer sem.Release(1)
            return h.router.Deliver(ctx, wh, cmd.Payload)
        })
    }
    
    // 4. Also deliver to GCP Pub/Sub (backward compat)
    wg.Go(func() error {
        return h.router.DeliverToPubSub(ctx, cmd)
    })
    
    // 5. Ecosystem bridges
    wg.Go(func() error {
        return h.router.DeliverToEcosystemBridges(ctx, cmd)
    })
    
    wg.Wait()
    h.idempotency.MarkProcessed(ctx, cmd.EventID)
    return nil
}
```

---

## 6. HTTP Webhook Deliverer

```go
// infra/delivery/http_webhook_deliverer.go

type HTTPWebhookDeliverer struct {
    client     *http.Client
    maxRetries int
    tracer     trace.Tracer
}

func (d *HTTPWebhookDeliverer) Deliver(
    ctx context.Context,
    webhook *aggregate.Webhook,
    payload []byte,
) error {
    signature := webhook.Sign(payload)
    
    req, _ := http.NewRequestWithContext(ctx, "POST", webhook.URL(), bytes.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-OSV-Signature-256", signature)
    req.Header.Set("X-OSV-Event", string(webhook.EventType()))
    req.Header.Set("User-Agent", "osv.dev/notification-service")
    
    // Retry with exponential backoff
    var lastErr error
    for attempt := 0; attempt < d.maxRetries; attempt++ {
        resp, err := d.client.Do(req)
        if err != nil {
            lastErr = err
            time.Sleep(backoff(attempt))
            continue
        }
        resp.Body.Close()
        
        if resp.StatusCode >= 200 && resp.StatusCode < 300 {
            return nil
        }
        
        // 4xx = client error, don't retry
        if resp.StatusCode >= 400 && resp.StatusCode < 500 {
            return fmt.Errorf("webhook rejected with %d", resp.StatusCode)
        }
        
        // 5xx = server error, retry
        lastErr = fmt.Errorf("webhook server error %d", resp.StatusCode)
        time.Sleep(backoff(attempt))
    }
    
    return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func backoff(attempt int) time.Duration {
    base := time.Second
    return base * time.Duration(math.Pow(2, float64(attempt)))
}
```

---

## 7. SLO Targets

| Metric | Target |
|--------|--------|
| Availability | 99.9% |
| Notification delivery latency P50 | < 5s after event |
| Notification delivery latency P99 | < 60s after event |
| Delivery success rate | > 99% for healthy endpoints |
| Retry success rate | > 80% for initially failed deliveries |
| Webhook fan-out throughput | 1000 webhooks/event |

---

## 8. Implementation Status

> **Status:** ✅ Core Implemented | **Updated:** 2026-06-01

### Implemented
- [x] `domain/aggregate/webhook/webhook.go` — Webhook aggregate (Sign HMAC-SHA256, ShouldDeliver, activate/deactivate, EventFilter)
- [x] `domain/aggregate/webhook/webhook_test.go` — 8 unit tests (Sign, ShouldDeliver, HMAC correctness)
- [x] `infra/delivery/http_webhook_deliverer.go` — HTTP POST + 3-retry exponential backoff (1s→2s→4s), 4xx≠5xx
- [x] `infra/idempotency/redis/idempotency_store.go` — Redis SETNX idempotency (24h TTL)
- [x] `infra/messaging/nats/consumer.go` — NATS JetStream consumer (osv.vuln.> → dispatcher)
- [x] `infra/messaging/nats/dispatcher.go` — NotificationDispatcher (fan-out to webhooks + idempotency guard)
- [x] `cmd/server/main.go` — Service entry point, NATS consumer, HTTP health endpoints
- [x] `go.mod`, `Dockerfile`

### Pending
- [ ] `domain/service/notification_router.go` — Fan-out router (semaphore(10) for webhook concurrency)
- [ ] `infra/delivery/pubsub_deliverer.go` — GCP Pub/Sub delivery (backward compat)
- [ ] `infra/bridge/pypi_bridge.go` + `cratesio_bridge.go` — Ecosystem bridges
- [ ] `infra/persistence/firestore/webhook_repo.go` + `notification_repo.go` — Firestore persistence
- [ ] `application/command/deliver_notification/handler.go` — Full command handler
- [ ] `application/command/register_webhook/handler.go` — Webhook registration
- [ ] `application/command/retry_failed/handler.go` — Retry failed deliveries
- [ ] `interface/grpc/handler/notification_handler.go` — gRPC admin handler
- [ ] `config/config.go` — Config struct
- [ ] Integration tests, Makefile

### Deviations from Spec
- NATS consumer (osv.vuln.>) implemented; GCP Pub/Sub deliverer pending
- Dispatcher is infra-level (not application command handler); will be refactored when full handler is implemented
