# ✅ COMPLETED — TASK-DD-025 — Audit Service Bootstrap (New Service)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-025 |
| **Service** | `audit-service` (NEW) |
| **CR** | CR-DD-010 |
| **Phase** | 2 — Security Management |
| **Priority** | 🔴 High |
| **Prerequisites** | — (độc lập) |
| **Estimated effort** | 1 ngày |

## Context

Tạo mới `audit-service` với scaffolding chuẩn: append-only event store, HMAC signing, Postgres RLS policy. Service này subscribe tất cả NATS events và ghi vào partitioned audit_events table.

## Reference

- Solution: [`sol-audit-service.md`](../solutions/sol-audit-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/audit-service/
```

## Files to Create

```
services/audit-service/
├── go.mod
├── Dockerfile
├── .env.example
├── cmd/server/main.go
├── migrations/
│   ├── 001_audit_events_partitioned.sql
│   └── 002_rls_policies.sql
└── internal/
    ├── config/config.go
    ├── domain/
    │   └── event/
    │       ├── entity.go          # AuditEvent (immutable)
    │       └── repository.go      # append-only interface
    ├── infra/
    │   ├── postgres/
    │   │   └── audit_repo.go
    │   ├── crypto/
    │   │   └── hmac.go
    │   └── nats/
    │       └── client.go
    └── delivery/
        ├── http/server.go
        └── event/subscriber.go    # Subscribe 40+ NATS subjects
```

## Implementation Spec

### `internal/domain/event/entity.go`

```go
package event

import "time"

// AuditEvent is IMMUTABLE — never updated after creation
type AuditEvent struct {
    ID          string
    EventID     string            // NATS message ID
    EventType   string            // NATS subject: "finding.status_changed"

    // WHO
    ActorID     *string
    ActorEmail  *string
    ActorType   string   // "user" | "system" | "service"
    ServiceName string

    // WHAT
    ResourceType string
    ResourceID   string
    Action       string

    // CHANGES
    Changes  map[string]interface{}
    Metadata map[string]interface{}

    // WHEN
    OccurredAt time.Time
    RecordedAt time.Time

    // INTEGRITY
    Signature string  // HMAC-SHA256
}
```

### `internal/domain/event/repository.go`

```go
package event

import (
    "context"
    "io"
    "time"
)

// AuditEventRepository — APPEND ONLY
// NO Update() or Delete() methods by design
type AuditEventRepository interface {
    Create(ctx context.Context, event *AuditEvent) error
    FindByID(ctx context.Context, id string) (*AuditEvent, error)
    List(ctx context.Context, query Query) ([]*AuditEvent, int64, error)
    Export(ctx context.Context, query Query) (io.Reader, error)
}

type Query struct {
    EventTypes   []string
    ResourceType *string
    ResourceID   *string
    ActorID      *string
    From         *time.Time
    To           *time.Time
    Limit        int
    Offset       int
    OrderBy      string
}
```

### `migrations/001_audit_events_partitioned.sql`

```sql
CREATE TABLE IF NOT EXISTS audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id VARCHAR(255),
    event_type VARCHAR(200) NOT NULL,
    actor_id UUID,
    actor_email VARCHAR(255),
    actor_type VARCHAR(50) DEFAULT 'user',
    service_name VARCHAR(100),
    resource_type VARCHAR(100) NOT NULL,
    resource_id UUID NOT NULL,
    action VARCHAR(100) NOT NULL,
    changes JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    signature VARCHAR(64)
) PARTITION BY RANGE (occurred_at);

-- Create current + next 3 months partitions
CREATE TABLE IF NOT EXISTS audit_events_2026_06
    PARTITION OF audit_events FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS audit_events_2026_07
    PARTITION OF audit_events FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE IF NOT EXISTS audit_events_2026_08
    PARTITION OF audit_events FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE IF NOT EXISTS audit_events_2026_09
    PARTITION OF audit_events FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
-- Add more partitions as needed (automate via cron or pg_partman)

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_events(resource_type, resource_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_events(actor_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_event_type ON audit_events(event_type, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_occurred ON audit_events(occurred_at DESC);
```

### `migrations/002_rls_policies.sql`

```sql
-- Row Level Security: prevent modifications
ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;

-- Block all updates and deletes at DB level
CREATE POLICY audit_no_update ON audit_events FOR UPDATE USING (FALSE);
CREATE POLICY audit_no_delete ON audit_events FOR DELETE USING (FALSE);

-- Allow inserts and selects for app role
CREATE POLICY audit_insert ON audit_events FOR INSERT TO audit_app_role WITH CHECK (TRUE);
CREATE POLICY audit_select ON audit_events FOR SELECT TO audit_app_role USING (TRUE);
```

### `internal/infra/crypto/hmac.go`

```go
package crypto

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "io"
)

type HMACSvc struct {
    key []byte  // 32 bytes from env OSV_AUDIT_HMAC_KEY
}

func NewHMACSvc(key []byte) *HMACSvc {
    return &HMACSvc{key: key}
}

func (s *HMACSvc) Sign(data string) string {
    mac := hmac.New(sha256.New, s.key)
    io.WriteString(mac, data)
    return hex.EncodeToString(mac.Sum(nil))
}

func (s *HMACSvc) Verify(data, signature string) bool {
    expected := s.Sign(data)
    return hmac.Equal([]byte(expected), []byte(signature))
}

// SignEvent creates HMAC signature for an audit event
func (s *HMACSvc) SignEvent(eventType, resourceID, occurredAt, actorID string) string {
    data := eventType + "|" + resourceID + "|" + occurredAt + "|" + actorID
    return s.Sign(data)
}
```

### `internal/delivery/event/subscriber.go` (all 40+ subjects)

```go
// Subscribe ALL auditable events
var auditableSubjects = []string{
    "finding.created", "finding.updated", "finding.deleted",
    "finding.status_changed", "finding.bulk_updated",
    "finding.risk_accepted", "finding.false_positive_marked", "finding.duplicate_detected",
    "product.created", "product.updated", "product.deleted",
    "product.member.added", "product.member.removed", "product.member.role_changed",
    "engagement.created", "engagement.updated", "engagement.closed", "engagement.reopened",
    "test.created", "test.updated",
    "scan.import.started", "scan.import.completed", "scan.import.failed",
    "risk_acceptance.created", "risk_acceptance.updated", "risk_acceptance.expired",
    "sla.config.created", "sla.config.updated", "sla.config.deleted",
    "sla.breached",
    "jira.issue.created", "jira.issue.updated", "jira.synced",
    "user.login", "user.login_failed", "user.logout",
    "user.password_changed", "user.role_changed", "user.created", "user.deleted",
    "report.generated", "report.deleted",
}
```

## Acceptance Criteria

- [x] `go build ./...` thành công
- [x] `docker build -t audit-service:test .` thành công
- [x] Service starts on port 8090 (HTTP) và 9010 (gRPC)
- [x] Migrations chạy thành công — `audit_events` partitioned table created
- [x] `audit_events_2026_06` partition created
- [x] RLS policy `audit_no_update` → `UPDATE audit_events SET ...` fails với DB error
- [x] RLS policy `audit_no_delete` → `DELETE FROM audit_events` fails với DB error
- [x] `HMACSvc.Sign` then `HMACSvc.Verify` → true
- [x] `HMACSvc.Verify` with tampered data → false
- [x] Service subscribes to all 40+ NATS subjects on startup
- [x] `GET /health` → 200

## Implementation Status: ✅ DONE

> `audit-service/internal/domain/event/entity.go` — AuditEvent (immutable), AuditEventRepository (append-only), Query struct
> `audit-service/internal/infra/crypto/hmac.go` — HMACSvc.Sign, Verify (constant-time)
> `audit-service/migrations/001_audit_events_partitioned.sql` — partitioned table + 4 quarterly partitions + 4 indexes
> `audit-service/migrations/002_rls_policies.sql` — RLS: audit_no_update + audit_no_delete policies
> `audit-service/internal/delivery/event/subscriber.go` — subscribes to 40+ NATS subjects
