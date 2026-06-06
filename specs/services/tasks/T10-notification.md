# Task T10 — Notification Service

> **Priority:** P2 | **Phase:** 3 | **Spec:** `specs/services/07-notification-service.md`  
> **Depends on:** T00-shared-libs, T12-infrastructure (NATS, Firestore, Redis)

## Mục Tiêu
Broadcasting domain events tới external consumers (webhooks, GCP Pub/Sub, ecosystem bridges). Thay thế notification logic rải rác trong Python worker.

## Trách Nhiệm
- Subscribe domain events từ NATS (VulnImported, VulnUpdated, VulnWithdrawn)
- Fan-out tới registered webhooks (HTTP POST, HMAC-SHA256 signed)
- Publish tới GCP Pub/Sub (backward compatibility)
- Ecosystem bridges: PyPI bridge, crates.io bridge
- Retry w/ exponential backoff (3 retries), Dead Letter Queue
- Admin gRPC API: register/manage webhooks

## Không Làm
- Store/query vulns, source sync

## Cấu Trúc File

```
services/notification/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── aggregate/webhook/
│   │   │   ├── webhook.go          # {URL, Secret, EventFilter, isActive, maxPerMinute}
│   │   │   └── webhook_test.go
│   │   ├── entity/
│   │   │   ├── notification.go     # Delivery record
│   │   │   └── delivery_attempt.go
│   │   ├── valueobject/
│   │   │   ├── webhook_url.go      # Validate HTTPS URL
│   │   │   ├── webhook_secret.go   # HMAC secret (min 32 bytes)
│   │   │   ├── event_filter.go     # Which event types to receive
│   │   │   └── delivery_status.go  # PENDING | DELIVERED | FAILED
│   │   ├── service/
│   │   │   ├── notification_router.go    # Route event → deliverers
│   │   │   └── signature_generator.go   # HMAC-SHA256
│   │   └── repository/
│   │       ├── webhook_repository.go
│   │       └── notification_repository.go
│   ├── application/
│   │   ├── command/
│   │   │   ├── deliver_notification/{command,handler}.go
│   │   │   ├── register_webhook/{command,handler}.go
│   │   │   └── retry_failed/{command,handler}.go
│   │   ├── query/list_delivery_attempts/{query,handler}.go
│   │   └── port/
│   │       ├── event_deliverer.go
│   │       └── ecosystem_bridge.go
│   └── infra/
│       ├── persistence/firestore/
│       │   ├── webhook_repo.go
│       │   └── notification_repo.go
│       ├── delivery/
│       │   ├── http_webhook_deliverer.go  # HTTP POST with retry
│       │   ├── pubsub_deliverer.go        # GCP Pub/Sub
│       │   └── noop_deliverer.go          # test/dry-run
│       ├── bridge/
│       │   ├── pypi_bridge.go             # PyPI vuln bridge
│       │   └── cratesio_bridge.go         # crates.io bridge
│       ├── messaging/nats/consumer.go     # Subscribe domain events
│       └── idempotency/redis/idempotency_store.go
├── interface/
│   ├── grpc/
│   │   ├── handler/notification_handler.go
│   │   └── proto/notification_service.proto
│   └── http/handler/health_handler.go
└── config/config.go
```

## Webhook Aggregate

```go
// domain/aggregate/webhook/webhook.go
type Webhook struct {
    id          string
    url         valueobject.WebhookURL    // must be HTTPS
    secret      valueobject.WebhookSecret // HMAC secret >= 32 bytes
    eventFilter valueobject.EventFilter   // set of event types: {"vuln.imported", "vuln.updated"}
    isActive    bool
    maxPerMinute int  // rate limit per webhook
    createdAt   time.Time
    events      []domain.Event
}

func (w *Webhook) Sign(payload []byte) string:
  // Header: "X-OSV-Signature-256: sha256={hmac_hex}"
  mac := hmac.New(sha256.New, secret)
  mac.Write(payload)
  return "sha256=" + hex.EncodeToString(mac.Sum(nil))

func (w *Webhook) ShouldDeliver(eventType string) bool:
  return w.isActive && w.eventFilter.Matches(eventType)
```

## Deliver Notification Handler

```go
// application/command/deliver_notification/handler.go
func Handle(ctx, cmd Command) error:
  // 1. Idempotency check (Redis key = event_id, TTL 24h)
  // 2. Load all active webhooks matching eventType
  // 3. Fan-out: concurrent deliveries with semaphore(10)
  //    - HTTP webhook deliveries
  //    - GCP Pub/Sub delivery (backward compat)
  //    - Ecosystem bridges
  // 4. Mark event as processed (idempotency)
  // Errors: log per-webhook failures, don't fail whole operation
```

## HTTP Webhook Deliverer

```go
// infra/delivery/http_webhook_deliverer.go
func Deliver(ctx, webhook, payload []byte) error:
  signature := webhook.Sign(payload)
  req := http.NewRequestWithContext(ctx, "POST", webhook.URL(), bytes.NewReader(payload))
  req.Header.Set("Content-Type", "application/json")
  req.Header.Set("X-OSV-Signature-256", signature)
  req.Header.Set("X-OSV-Event", eventType)
  req.Header.Set("User-Agent", "osv.dev/notification-service/1.0")
  
  // Retry with exponential backoff (max 3 attempts)
  for attempt := 0; attempt < 3; attempt++:
    resp, err := client.Do(req)
    if resp.StatusCode 200-299: return nil  // success
    if resp.StatusCode 400-499: return error  // client error, no retry
    // 5xx: retry with backoff(attempt): 1s, 2s, 4s
  return ErrMaxRetriesExceeded
```

## NATS Consumer

```go
// infra/messaging/nats/consumer.go
// Subscribe: "osv.vuln.>" (imported, updated, withdrawn)
// Also: "osv.source.sync.completed" (source-level notifications)
// For each event: dispatch DeliverNotificationCommand
// Durable consumer: MaxDeliver=10, AckWait=30s
```

## Events Consumed & Format

```go
// On VulnImported/Updated/Withdrawn:
// Payload to webhooks:
type WebhookPayload struct {
    EventType string    `json:"event_type"`  // "osv.vuln.imported"
    OccurredAt time.Time `json:"occurred_at"`
    VulnID    string    `json:"vuln_id"`
    Source    string    `json:"source,omitempty"`
    // Full vuln data (optional, configurable per webhook)
}
```

## Dead Letter Queue
```
Failed deliveries (after 3 retries) → Firestore collection "dlq-notifications"
  {webhook_id, event_id, event_type, payload, error, failed_at}
Admin command: RetryFailedDeliveries (re-queue from DLQ)
Alert: DLQ count > 100 → Prometheus alert
```

## gRPC Admin API
```protobuf
service NotificationService {
  rpc RegisterWebhook(RegisterWebhookRequest) returns (RegisterWebhookResponse);
  rpc UpdateWebhook(UpdateWebhookRequest) returns (UpdateWebhookResponse);
  rpc DeleteWebhook(DeleteWebhookRequest) returns (DeleteWebhookResponse);
  rpc ListWebhooks(ListWebhooksRequest) returns (ListWebhooksResponse);
  rpc ListDeliveryAttempts(ListAttemptsRequest) returns (ListAttemptsResponse);
  rpc RetryFailed(RetryFailedRequest) returns (RetryFailedResponse);
}
```

## SLO Targets
- Notification delivery P50: <5s after event
- P99: <60s
- Delivery success rate: >99% for healthy endpoints
- Retry success rate: >80% for initially failed
- Fan-out throughput: 1000 webhooks/event

## Checklist Thực Thi

> **Status: ✅ COMPLETED (Core)** — 2026-06-01

- [x] Implement `Webhook` aggregate (Sign, ShouldDeliver, business rules)
- [x] Implement `SignatureGenerator` (HMAC-SHA256 via Webhook.Sign)
- [x] Implement `HTTPWebhookDeliverer` (POST + 3-retry exponential backoff, 4xx vs 5xx differentiation)
- [x] Implement Redis idempotency store (SETNX, TTL 24h)
- [x] Implement NATS consumer (osv.vuln.> → DeliveryDispatcher)
- [x] `go.mod` + `Dockerfile`
- [x] Unit tests: Webhook.Sign, ShouldDeliver, retry logic (`webhook_test.go`)
- [ ] Implement `NotificationRouter` (route event → all deliverers)
- [ ] Implement `GCPPubSubDeliverer` (backward compat)
- [ ] Implement `PyPIBridge` + `CratesIOBridge` (ecosystem-specific adapters)
- [ ] Implement Firestore `WebhookRepo` + `NotificationRepo`
- [ ] Implement `DeliverNotificationHandler` (fan-out with semaphore)
- [ ] Implement `RegisterWebhookHandler` (validate URL, generate secret)
- [ ] Implement `RetryFailedHandler` (re-queue DLQ items)
- [ ] gRPC admin handler + proto
- [ ] `cmd/server/main.go` + `config/config.go`
- [ ] Integration tests: mock webhook server + NATS
- [ ] Makefile

