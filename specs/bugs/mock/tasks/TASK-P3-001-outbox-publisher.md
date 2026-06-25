# TASK-P3-001 — Implement Outbox Pattern cho NATS Publisher

**Bug:** MOCK-005  
**Priority:** 🟡 P3 — Resilience  
**Effort:** ~3 giờ  
**Service:** `finding-service`  
**Loại thay đổi:** New files + DB migration + Wire embedded.go

---

## Mục tiêu

Khi NATS không available lúc startup, `NoopPublisher` được dùng và events bị mất vĩnh viễn. Implement `OutboxPublisher` dùng PostgreSQL outbox để đảm bảo **at-least-once delivery** theo kiến trúc spec.

---

## Preconditions

- [ ] Đọc `services/finding-service/embedded.go` — xem cách NATS publisher được khởi tạo
- [ ] Xác định `EventPublisher` interface:
  ```bash
  grep -rn "type EventPublisher interface\|type.*Publisher.*interface" \
    services/finding-service/internal/domain/
  ```
- [ ] Xác định module name: `grep "^module" services/finding-service/go.mod`

---

## Steps

### Step 1 — Tạo DB migration cho outbox_events

**File mới** (đặt theo convention migrations hiện có):
```
services/finding-service/internal/infra/postgres/migrations/XXX_add_outbox_events.sql
```

```sql
CREATE TABLE IF NOT EXISTS outbox_events (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject      VARCHAR(200) NOT NULL,
    payload      JSONB NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','published','failed')),
    attempts     INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    last_error   TEXT
);

-- Partial index cho pending events (polling query)
CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON outbox_events(created_at)
    WHERE status = 'pending' AND attempts < 10;
```

### Step 2 — Tạo OutboxPublisher

**File mới**: `services/finding-service/internal/infra/nats/outbox_publisher.go`

```go
package nats

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    natsgo "github.com/nats-io/nats.go"
    "github.com/rs/zerolog"
)

// OutboxPublisher implements domain.EventPublisher using PostgreSQL transactional outbox.
// Guarantees at-least-once delivery even when NATS is temporarily unavailable.
type OutboxPublisher struct {
    db     *pgxpool.Pool
    nc     *natsgo.Conn  // may be nil on startup
    logger zerolog.Logger
}

// NewOutboxPublisher creates an OutboxPublisher.
// nc may be nil — outbox will buffer events until NATS reconnects.
func NewOutboxPublisher(db *pgxpool.Pool, nc *natsgo.Conn, logger zerolog.Logger) *OutboxPublisher {
    return &OutboxPublisher{db: db, nc: nc, logger: logger}
}

// Publish writes the event to the outbox table.
// Must be called within the same transaction as the business operation.
func (p *OutboxPublisher) Publish(ctx context.Context, subject string, data interface{}) error {
    payload, err := json.Marshal(data)
    if err != nil {
        return fmt.Errorf("outbox marshal: %w", err)
    }
    _, err = p.db.Exec(ctx,
        `INSERT INTO outbox_events (subject, payload) VALUES ($1, $2)`,
        subject, payload)
    return err
}

// Run polls the outbox and publishes pending events to NATS.
// Blocks until ctx is cancelled. Call as a goroutine.
func (p *OutboxPublisher) Run(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    p.logger.Info().Msg("OutboxPublisher: polling goroutine started")
    for {
        select {
        case <-ctx.Done():
            p.logger.Info().Msg("OutboxPublisher: polling goroutine stopped")
            return
        case <-ticker.C:
            p.flush(ctx)
        }
    }
}

func (p *OutboxPublisher) flush(ctx context.Context) {
    if p.nc == nil || !p.nc.IsConnected() {
        return  // NATS not ready yet, skip silently
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
        p.logger.Error().Err(err).Msg("OutboxPublisher: query error")
        return
    }
    defer rows.Close()

    for rows.Next() {
        var id, subject string
        var payload []byte
        if err := rows.Scan(&id, &subject, &payload); err != nil {
            continue
        }

        if pubErr := p.nc.Publish(subject, payload); pubErr != nil {
            p.logger.Warn().Err(pubErr).Str("event_id", id).
                Msg("OutboxPublisher: publish failed, will retry")
            p.db.Exec(ctx,
                `UPDATE outbox_events SET attempts = attempts+1, last_error = $2 WHERE id = $1`,
                id, pubErr.Error())
            continue
        }

        p.db.Exec(ctx,
            `UPDATE outbox_events SET status='published', published_at=NOW(), attempts=attempts+1 WHERE id=$1`,
            id)
    }
}
```

### Step 3 — Wire trong embedded.go

Mở `services/finding-service/embedded.go`.

Tìm đoạn NATS connection hiện tại:
```bash
grep -n "nats\|NoopPublisher\|NewNoopPublisher" services/finding-service/embedded.go
```

Thay đoạn NATS init bằng OutboxPublisher:

```go
// FIX MOCK-005: Dùng OutboxPublisher thay vì NoopPublisher
natsURL := os.Getenv("NATS_URL")
if natsURL == "" {
    natsURL = "nats://localhost:4222"
}
var nc *natsgo.Conn
nc, err := natsgo.Connect(natsURL,
    natsgo.RetryOnFailedConnect(true),
    natsgo.MaxReconnects(-1),  // reconnect forever
)
if err != nil {
    logger.Warn().Err(err).Str("url", natsURL).
        Msg("NATS unreachable at startup — OutboxPublisher will buffer events")
    nc = nil
}

outboxPub := mynats.NewOutboxPublisher(pool, nc, logger)
go outboxPub.Run(ctx)  // start background poller

// Dùng outboxPub thay vì noop/real publisher
pub := outboxPub  // implements domain.EventPublisher
```

---

## Acceptance Criteria

- [ ] Bảng `outbox_events` tồn tại trong DB
- [ ] Khi NATS unreachable lúc startup → `Publish(...)` lưu vào DB, không panic
- [ ] Khi NATS reconnect → outbox poller tự động deliver các pending events
- [ ] Events không bị retry > 10 lần (attempts < 10 guard)
- [ ] `go build ./services/finding-service/...` — thành công

---

## Test Commands

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/finding-service/...
go vet ./services/finding-service/...

# Verify NoopPublisher removed
grep -n "NoopPublisher\|NewNoopPublisher" services/finding-service/embedded.go
# Expected: no output

go test ./services/finding-service/internal/infra/nats/... -v
```
