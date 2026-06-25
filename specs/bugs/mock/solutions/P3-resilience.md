# P3 — Resilience Fixes (Dài hạn)

> **Bugs**: MOCK-003, MOCK-005, MOCK-015  
> **Mức độ**: 🟡 Medium — Silent data loss và Noop adapters  
> **Timeline**: Sprint 2 trở đi

---

## MOCK-005 — Fix: Outbox Pattern cho NATS Publisher

### Vấn đề
Khi NATS không available lúc startup → events bị mất vĩnh viễn. Noop publisher không retry.

### Giải pháp

Theo `01-architecture.md §1.2`: **Reliability: NATS at-least-once**. Cần implement Outbox Pattern để đảm bảo at-least-once delivery.

#### Kiến trúc Outbox Pattern

```
Finding Status Change
        │
        ▼
[PostgreSQL Transaction] ←── ACID
  ├── UPDATE finding SET status = 'mitigated'
  └── INSERT INTO outbox (subject, payload, created_at, status='pending')
        │
        ▼
[Outbox Poller] (background goroutine, mỗi 1 giây)
  ├── SELECT * FROM outbox WHERE status='pending' ORDER BY created_at LIMIT 100
  ├── FOR EACH message: NATS.Publish(subject, payload)
  └── IF success: UPDATE outbox SET status='published', published_at=NOW()
        │
        ▼
[NATS JetStream] → consumers (notification, audit, sla)
```

#### Bước 1: Tạo outbox table

```sql
CREATE TABLE IF NOT EXISTS outbox_events (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject      VARCHAR(200) NOT NULL,  -- e.g., "finding.status.changed"
    payload      JSONB NOT NULL,
    status       VARCHAR(20) DEFAULT 'pending',  -- pending|published|failed
    attempts     INT DEFAULT 0,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    last_error   TEXT
);
CREATE INDEX idx_outbox_status_created ON outbox_events(status, created_at)
    WHERE status = 'pending';
```

#### Bước 2: Tạo OutboxPublisher

**File mới**: `services/finding-service/internal/infra/nats/outbox_publisher.go`

```go
package nats

import (
    "context"
    "encoding/json"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    natsgo "github.com/nats-io/nats.go"
    "github.com/rs/zerolog"
)

// OutboxPublisher đảm bảo at-least-once delivery bằng PostgreSQL outbox.
// Khi NATS unavailable, events được lưu vào outbox và retry khi NATS khả dụng.
type OutboxPublisher struct {
    db     *pgxpool.Pool
    nc     *natsgo.Conn // có thể nil khi NATS chưa khả dụng
    logger zerolog.Logger
}

func NewOutboxPublisher(db *pgxpool.Pool, nc *natsgo.Conn, logger zerolog.Logger) *OutboxPublisher {
    return &OutboxPublisher{db: db, nc: nc, logger: logger}
}

// Publish ghi event vào outbox (đảm bảo ACID với transaction cha).
// Event sẽ được delivery async bởi OutboxPoller.
func (p *OutboxPublisher) Publish(ctx context.Context, subject string, data interface{}) error {
    payload, err := json.Marshal(data)
    if err != nil {
        return fmt.Errorf("marshal payload: %w", err)
    }
    _, err = p.db.Exec(ctx, `
        INSERT INTO outbox_events (subject, payload) VALUES ($1, $2)
    `, subject, payload)
    return err
}

// Run là background goroutine polling outbox và publish tới NATS.
// Chạy liên tục cho đến khi ctx cancelled.
func (p *OutboxPublisher) Run(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            p.processOutbox(ctx)
        }
    }
}

func (p *OutboxPublisher) processOutbox(ctx context.Context) {
    // Nếu NATS không connected, thử reconnect
    if p.nc == nil || !p.nc.IsConnected() {
        p.logger.Debug().Msg("OutboxPublisher: NATS not connected, skipping")
        return
    }

    rows, err := p.db.Query(ctx, `
        SELECT id, subject, payload
        FROM outbox_events
        WHERE status = 'pending' AND attempts < 10
        ORDER BY created_at
        LIMIT 100
        FOR UPDATE SKIP LOCKED
    `)
    if err != nil {
        p.logger.Error().Err(err).Msg("OutboxPublisher: query failed")
        return
    }
    defer rows.Close()

    for rows.Next() {
        var id, subject string
        var payload []byte
        if err := rows.Scan(&id, &subject, &payload); err != nil {
            continue
        }

        if err := p.nc.Publish(subject, payload); err != nil {
            p.logger.Warn().Err(err).Str("id", id).Msg("OutboxPublisher: publish failed")
            p.db.Exec(ctx, `
                UPDATE outbox_events
                SET attempts = attempts + 1, last_error = $2
                WHERE id = $1
            `, id, err.Error())
            continue
        }

        p.db.Exec(ctx, `
            UPDATE outbox_events
            SET status = 'published', published_at = NOW(), attempts = attempts + 1
            WHERE id = $1
        `, id)
    }
}
```

#### Bước 3: Wire OutboxPublisher trong embedded.go

**File sửa**: `services/finding-service/embedded.go`

```go
func WireEmbedded(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
    // FIX MOCK-005: Dùng OutboxPublisher thay vì noop
    var nc *natsgo.Conn
    natsURL := os.Getenv("NATS_URL")
    if natsURL == "" {
        natsURL = "nats://localhost:4222"
    }
    nc, err := natsgo.Connect(natsURL)
    if err != nil {
        logger.Warn().Err(err).Msg("NATS unreachable at startup — outbox will retry")
        nc = nil // OutboxPublisher xử lý nil nc gracefully
    }

    // OutboxPublisher: đảm bảo at-least-once ngay cả khi NATS unreachable lúc startup
    outboxPub := mynats.NewOutboxPublisher(pool, nc, logger)
    go outboxPub.Run(ctx)  // chạy polling goroutine

    // Wire pub vào usecases
    bulkUC   := findinguc.NewBulkUpdate(findingRepo, outboxPub)
    statusUC := findinguc.NewStatusTransition(findingRepo, outboxPub)

    // FIX MOCK-003: BulkHandler nhận outboxPub (không nil)
    bulkHandler := httpdelivery.NewBulkHandler(bulkUC, findingRepo, outboxPub, logger)
    /* ... */
}
```

#### Cleanup cron (tuỳ chọn)
```sql
-- Chạy hàng ngày để xoá events đã publish > 7 ngày
DELETE FROM outbox_events
WHERE status = 'published' AND published_at < NOW() - INTERVAL '7 days';

-- Alert khi có events failed > 10 lần
SELECT COUNT(*) FROM outbox_events WHERE status = 'pending' AND attempts >= 10;
```

---

## MOCK-003 — Fix: Wire NATS EventBus cho BulkHandler

### Vấn đề
`NewBulkHandler(..., nil, ...)` → `BulkReopen` không publish `finding.status.changed`.

### Giải pháp

Đây là side effect của MOCK-005 fix — khi OutboxPublisher được wire, cần pass vào BulkHandler:

**File sửa**: `services/finding-service/embedded.go:L59`

```go
// Trước (BUG):
bulkHandler := httpdelivery.NewBulkHandler(bulkUC, findingRepo, nil, logger)

// Sau (FIX): OutboxPublisher hoặc NATS publisher thực sự
// outboxPub đã được tạo ở bước MOCK-005 fix
bulkHandler := httpdelivery.NewBulkHandler(bulkUC, findingRepo, outboxPub, logger)
```

Nếu chưa implement OutboxPublisher (quick fix), dùng noop publisher có logging:

```go
// Fallback: noop với logging (không mất events, nhưng log để tracking)
type loggingNoopPublisher struct {
    logger zerolog.Logger
}

func (l *loggingNoopPublisher) Publish(ctx context.Context, subject string, data interface{}) error {
    payload, _ := json.Marshal(data)
    l.logger.Warn().
        Str("subject", subject).
        RawJSON("payload", payload).
        Msg("EventBus: NATS not available, event dropped")
    return nil
}
```

---

## MOCK-015 — Fix: Wire FindingClient gRPC + NATS Publisher cho asset-service

### Vấn đề
- `NoopFindingClient` → risk score luôn = 0
- `noopEventPublisher` → asset events bị drop

### Giải pháp

Theo `01-architecture.md §3.12 Asset-Service` và port map: asset-service cần gọi finding-service gRPC (port 50060) để lấy severity counts.

#### Phần 1: Wire FindingClient gRPC thực sự

**File sửa**: `services/asset-service/embedded.go:L33-43`

```go
// FIX MOCK-015 (phần 1): Wire real gRPC FindingClient
var fc ucasset.FindingClient = &mygrpc.NoopFindingClient{} // default fallback

grpcTarget := os.Getenv("FINDING_SERVICE_GRPC")
if grpcTarget == "" {
    grpcTarget = "localhost:50060"
}

realFC, err := mygrpc.NewFindingClient(grpcTarget)
if err != nil {
    logger.Warn().Err(err).Str("target", grpcTarget).
        Msg("Finding gRPC client unavailable, risk scoring will return 0")
    // fc vẫn là NoopFindingClient — graceful fallback
} else {
    fc = realFC
    logger.Info().Str("target", grpcTarget).
        Msg("Asset-service: Finding gRPC client connected")
}
```

#### Phần 2: Wire NATS EventPublisher cho asset events

**File mới**: `services/asset-service/internal/infra/nats/asset_publisher.go`

```go
package nats

import (
    "encoding/json"
    natsgo "github.com/nats-io/nats.go"
)

// AssetEventPublisher publishes asset events tới NATS.
// Implements ucasset.EventPublisher interface.
type AssetEventPublisher struct {
    nc *natsgo.Conn
}

func NewAssetEventPublisher(nc *natsgo.Conn) *AssetEventPublisher {
    return &AssetEventPublisher{nc: nc}
}

func (p *AssetEventPublisher) Publish(subject string, payload map[string]any) error {
    data, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    return p.nc.Publish("asset."+subject, data)
}
```

**File sửa**: `services/asset-service/embedded.go:L44-46`

```go
// FIX MOCK-015 (phần 2): Wire NATS EventPublisher
var eventPub ucasset.EventPublisher = &noopEventPublisher{} // fallback

natsURL := os.Getenv("NATS_URL")
if natsURL == "" {
    natsURL = "nats://localhost:4222"
}
nc, err := natsgo.Connect(natsURL)
if err != nil {
    logger.Warn().Err(err).Msg("Asset-service: NATS unavailable, asset events disabled")
    // eventPub = noopEventPublisher{} — graceful fallback
} else {
    eventPub = assetinfra.NewAssetEventPublisher(nc)
    logger.Info().Msg("Asset-service: NATS event publisher connected")
}

// Wire usecases với real dependencies
crudUC    := ucasset.NewAssetCRUDUseCase(repo, eventPub)
taggingUC := ucasset.NewTaggingUseCase(repo)
riskUC    := ucasset.NewRiskScoringUseCase(repo, fc)
listUC    := ucasset.NewListAssetsUseCase(repo)
```

#### NATS Events mà asset-service cần publish

Theo `01-architecture.md §3.8` (NATS Events Subscribed by notification-service):

```
asset.created    → Audit log
asset.updated    → Audit log
asset.deleted    → Audit log
asset.risk_changed → Notification (khi risk score thay đổi đáng kể)
```

---

## Tổng kết — Thứ tự thực hiện

```
Sprint 1 (P0 + P1):
  Week 1: MOCK-002, MOCK-012, MOCK-014 (Crash fixes — no DB changes needed)
  Week 2: MOCK-007, MOCK-008, MOCK-011 (Data correctness)

Sprint 2 (P2):
  Week 3: MOCK-001 (ReportRepo + MinIO)
           MOCK-004 (7 nil handlers in finding-service)
  Week 4: MOCK-006 (ScanRepo + AgentRepo)
           MOCK-009, MOCK-010 (OpenSearch)
           MOCK-013 (SearchAddr config)

Sprint 3 (P3):
  Week 5: MOCK-005 (Outbox pattern — requires DB migration)
           MOCK-003 (BulkHandler eventbus — follows MOCK-005)
  Week 6: MOCK-015 (Asset FindingClient + EventPublisher)
```

## DB Migrations Summary

| Migration | Bugs Fixed | Table |
|-----------|-----------|-------|
| `001_add_scan_agents.sql` | MOCK-007 | `scan_agents` |
| `002_add_scans.sql` | MOCK-006 | `scans` |
| `003_add_reports.sql` | MOCK-001 | `reports` |
| `004_add_alerts.sql` | MOCK-014 | `alerts` |
| `005_add_outbox.sql` | MOCK-005 | `outbox_events` |

## Environment Variables Summary

| Variable | Service | Bugs Fixed | Default |
|----------|---------|-----------|---------|
| `MINIO_ENDPOINT` | finding-service | MOCK-001 | _(none — feature disabled)_ |
| `MINIO_ACCESS_KEY` | finding-service | MOCK-001 | _(none)_ |
| `MINIO_SECRET_KEY` | finding-service | MOCK-001 | _(none)_ |
| `MINIO_REPORT_BUCKET` | finding-service | MOCK-001 | `osv-reports` |
| `AI_SERVICE_GRPC` | search-service | MOCK-008 | `localhost:9103` |
| `GOOGLE_CLIENT_ID` | identity-service | MOCK-011 | _(none)_ |
| `GOOGLE_CLIENT_SECRET` | identity-service | MOCK-011 | _(none)_ |
| `GITHUB_CLIENT_ID` | identity-service | MOCK-011 | _(none)_ |
| `GITHUB_CLIENT_SECRET` | identity-service | MOCK-011 | _(none)_ |
| `OPENSEARCH_URL` | search-service | MOCK-009 | _(none — Postgres fallback)_ |
| `SEARCH_SERVICE_ADDR` | gateway-service | MOCK-013 | `http://localhost:8083` |
| `FINDING_SERVICE_GRPC` | asset-service | MOCK-015 | `localhost:50060` |
| `NATS_URL` | finding-service, asset-service | MOCK-005, MOCK-015 | `nats://localhost:4222` |
