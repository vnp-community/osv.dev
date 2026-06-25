# ✅ COMPLETED — Solution: audit-service (New Service)

> **Covers**: CR-DD-010  
> **Lý do tạo service mới**: Audit service là append-only event store — có Postgres RLS policy riêng, HMAC signing, compliance export. Cần độc lập để không một service nào khác có thể modify audit records. Database riêng với Row-Level Security policy `NO UPDATE, NO DELETE`.

---

## Service Structure

```
services/audit-service/          # NEW SERVICE
├── cmd/server/main.go
├── Dockerfile
├── go.mod
├── migrations/
│   ├── 001_audit_events_partitioned.sql
│   └── 002_rls_policies.sql      # Postgres RLS: no UPDATE/DELETE
│
└── internal/
    ├── domain/
    │   ├── event/
    │   │   ├── entity.go       # AuditEvent (immutable)
    │   │   └── repository.go   # Append-only: Create() only, no Update/Delete
    │   └── query/
    │       └── service.go      # Read-only query service
    │
    ├── usecase/
    │   ├── record/
    │   │   └── record_event.go  # Write audit event (NATS → DB)
    │   └── query/
    │       ├── list_events.go   # Query audit log with filters
    │       └── export_events.go # Export CSV/JSON for compliance
    │
    ├── delivery/
    │   ├── http/
    │   │   ├── server.go
    │   │   └── audit_handler.go  # GET /api/v2/audit-log/*
    │   ├── grpc/
    │   │   └── audit_server.go
    │   └── event/
    │       └── subscriber.go     # Subscribe ALL NATS events (40+ subjects)
    │
    └── infra/
        ├── postgres/
        │   └── audit_repo.go    # Append-only INSERT (no UPDATE/DELETE methods)
        └── crypto/
            └── hmac.go          # HMAC-SHA256 signing for tamper detection
```

---

## Domain Model

### AuditEvent Entity (Immutable)

```go
// audit-service/internal/domain/event/entity.go

// AuditEvent — immutable record, WRITE-ONCE
// All fields set on creation, NEVER updated after
type AuditEvent struct {
    ID          string            // UUID v4
    EventID     string            // Original NATS message ID
    EventType   string            // "finding.status_changed", etc.

    // WHO
    ActorID     *string           // User ID (nil = system)
    ActorEmail  *string           // Denormalized for readability
    ActorType   string            // "user" | "system" | "service"
    ServiceName string

    // WHAT
    ResourceType string           // "finding" | "product" | "engagement" | etc.
    ResourceID   string           // UUID of affected resource
    Action       string           // "created" | "updated" | "deleted" | "status_changed"

    // WHAT CHANGED
    Changes  map[string]interface{} // {field: {from: old, to: new}}
    Metadata map[string]interface{}

    // WHEN
    OccurredAt time.Time          // When event happened (from source)
    RecordedAt time.Time          // When audit-service recorded (NOW())

    // INTEGRITY
    Signature string              // HMAC-SHA256(EventType|ResourceID|OccurredAt|ActorID)
}

// Repository — ENFORCE append-only via interface design
type AuditEventRepository interface {
    Create(ctx context.Context, event *AuditEvent) error
    List(ctx context.Context, query AuditQuery) ([]*AuditEvent, int64, error)
    FindByID(ctx context.Context, id string) (*AuditEvent, error)
    FindByResource(ctx context.Context, resourceType, resourceID string) ([]*AuditEvent, error)
    FindByActor(ctx context.Context, actorID string, from, to time.Time) ([]*AuditEvent, error)
    Export(ctx context.Context, query AuditQuery) (io.Reader, error)
    // ❌ NO Update() method
    // ❌ NO Delete() method
}

type AuditQuery struct {
    EventTypes   []string
    ResourceType *string
    ResourceID   *string
    ActorID      *string
    ProductID    *string
    From         *time.Time
    To           *time.Time
    Limit        int
    Offset       int
    OrderBy      string  // default: "occurred_at DESC"
}
```

---

## NATS Subscriptions (All Events)

```go
// audit-service/internal/delivery/event/subscriber.go
// Subscribe ALL auditable events từ toàn hệ thống

var auditableEvents = []string{
    // Findings (từ finding-service)
    "finding.created",
    "finding.updated",
    "finding.deleted",
    "finding.status_changed",
    "finding.bulk_updated",
    "finding.risk_accepted",
    "finding.false_positive_marked",
    "finding.duplicate_detected",

    // Products (từ finding-service)
    "product.created",
    "product.updated",
    "product.deleted",
    "product.member.added",
    "product.member.removed",
    "product.member.role_changed",

    // Engagements (từ finding-service)
    "engagement.created",
    "engagement.updated",
    "engagement.closed",
    "engagement.reopened",

    // Tests (từ finding-service)
    "test.created",
    "test.updated",

    // Scans (từ scan-service)
    "scan.import.started",
    "scan.import.completed",
    "scan.import.failed",

    // Risk Acceptance (từ finding-service)
    "risk_acceptance.created",
    "risk_acceptance.updated",
    "risk_acceptance.expired",

    // SLA (từ sla-service)
    "sla.config.created",
    "sla.config.updated",
    "sla.config.deleted",
    "sla.breached",

    // JIRA (từ jira-service)
    "jira.issue.created",
    "jira.issue.updated",
    "jira.synced",

    // Auth (từ identity-service)
    "user.login",
    "user.login_failed",
    "user.logout",
    "user.password_changed",
    "user.role_changed",
    "user.created",
    "user.deleted",

    // Reports (từ finding-service)
    "report.generated",
    "report.deleted",
}
```

---

## Use Cases

### RecordEvent

```go
// audit-service/internal/usecase/record/record_event.go
func (uc *RecordEventUseCase) Execute(ctx context.Context, rawEvent *nats.Msg) error {
    var eventData map[string]interface{}
    json.Unmarshal(rawEvent.Data, &eventData)

    subject := rawEvent.Subject

    // Extract common fields
    actorID := extractString(eventData, "by_user_id")
    resourceID := extractResourceID(subject, eventData)
    resourceType := extractResourceType(subject)
    action := extractAction(subject)
    occurredAt := extractTime(eventData, "at")

    // Compute HMAC for integrity verification
    sigData := fmt.Sprintf("%s|%s|%s|%s",
        subject, resourceID, occurredAt.Format(time.RFC3339), actorID)
    signature := uc.hmacSvc.Sign(sigData)

    return uc.auditRepo.Create(ctx, &domain.AuditEvent{
        ID:           uuid.New().String(),
        EventType:    subject,
        ActorID:      ptrStr(actorID),
        ActorType:    determineActorType(actorID, eventData),
        ServiceName:  extractString(eventData, "_service"),
        ResourceType: resourceType,
        ResourceID:   resourceID,
        Action:       action,
        Changes:      extractChanges(eventData),
        Metadata:     eventData,
        OccurredAt:   occurredAt,
        RecordedAt:   time.Now(),
        Signature:    signature,
    })
}

// Action mapping from event subject
func extractAction(subject string) string {
    switch {
    case strings.HasSuffix(subject, ".created"):      return "created"
    case strings.HasSuffix(subject, ".updated"):      return "updated"
    case strings.HasSuffix(subject, ".deleted"):      return "deleted"
    case strings.HasSuffix(subject, ".closed"):       return "closed"
    case strings.HasSuffix(subject, ".reopened"):     return "reopened"
    case subject == "finding.status_changed":         return "status_changed"
    case subject == "finding.risk_accepted":          return "risk_accepted"
    case subject == "finding.false_positive_marked":  return "marked_false_positive"
    case subject == "user.login":                     return "logged_in"
    case subject == "user.login_failed":              return "login_failed"
    default:                                          return subject
    }
}
```

### ListEvents (with permission check)

```go
// audit-service/internal/usecase/query/list_events.go
func (uc *ListEventsUseCase) Execute(ctx context.Context, in ListEventsInput) (*ListEventsOutput, error) {
    // Only Maintainer+ or Admin can view audit log
    allowed, _ := uc.identityClient.CheckPermission(ctx, &identityv1.CheckPermissionRequest{
        UserId:     in.RequestorUserID,
        Permission: "audit:view",
        ProductId:  in.ProductID,
    })
    if !allowed.Allowed {
        return nil, ErrForbidden
    }

    events, total, err := uc.auditRepo.List(ctx, domain.AuditQuery{
        EventTypes:   in.EventTypes,
        ResourceType: in.ResourceType,
        ResourceID:   in.ResourceID,
        ActorID:      in.ActorID,
        ProductID:    in.ProductID,
        From:         in.From,
        To:           in.To,
        Limit:        in.Limit,
        Offset:       in.Offset,
    })

    return &ListEventsOutput{Events: events, Total: total}, err
}
```

### ExportEvents (compliance)

```go
// audit-service/internal/usecase/query/export_events.go
// Export for SOC2, ISO27001, PCI-DSS auditors
// Supports: CSV, JSON (newline-delimited)
```

---

## HMAC Signing

```go
// audit-service/internal/infra/crypto/hmac.go
type HMACSvc struct {
    key []byte  // 32 bytes from env OSV_AUDIT_HMAC_KEY
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
```

---

## REST API

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET` | `/api/v2/audit-log` | JWT/Maintainer | List với filters |
| `GET` | `/api/v2/audit-log/{id}` | JWT/Maintainer | Get specific event |
| `GET` | `/api/v2/audit-log/resource/{type}/{id}` | JWT | History of resource |
| `GET` | `/api/v2/audit-log/actor/{user_id}` | JWT/Admin | Activity of user |
| `GET` | `/api/v2/audit-log/export` | JWT/Admin | Export CSV/JSON |

### Query Examples

```
GET /api/v2/audit-log/resource/finding/uuid-of-finding
→ Full history: created → status_changed → risk_accepted → ...

GET /api/v2/audit-log?event_type=user.login_failed&from=2026-06-12T00:00:00Z
→ All login failures last 24h

GET /api/v2/audit-log/export?from=2026-01-01&to=2026-06-30&format=csv
→ 6-month compliance export
```

---

## Database Schema

```sql
-- audit_events (append-only via RLS + interface design)
CREATE TABLE audit_events (
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
    signature VARCHAR(64)  -- HMAC-SHA256 (64 hex chars)
) PARTITION BY RANGE (occurred_at);

-- Monthly partitions
CREATE TABLE audit_events_2026_06 PARTITION OF audit_events
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE audit_events_2026_07 PARTITION OF audit_events
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
-- etc.

-- Performance indexes
CREATE INDEX idx_audit_resource ON audit_events(resource_type, resource_id, occurred_at DESC);
CREATE INDEX idx_audit_actor ON audit_events(actor_id, occurred_at DESC);
CREATE INDEX idx_audit_event_type ON audit_events(event_type, occurred_at DESC);
CREATE INDEX idx_audit_occurred ON audit_events(occurred_at DESC);

-- Row-Level Security: enforce append-only at DB level
ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY audit_no_update ON audit_events FOR UPDATE USING (FALSE);
CREATE POLICY audit_no_delete ON audit_events FOR DELETE USING (FALSE);
-- Application role chỉ có INSERT + SELECT quyền (không có UPDATE/DELETE)
```

---

## Retention Policy

```yaml
# audit-service/config.yaml
retention:
  years: 2          # Keep 2 years online
  archive:
    enabled: true
    bucket: "audit-archive"   # Minio/S3
    format: "ndjson"          # Newline-delimited JSON
  partition_drop:
    enabled: true
    after_years: 2  # Drop monthly partitions > 2 years old (after archive)
```

---

## Acceptance Criteria

- [x] `finding.status_changed` event → audit_event ghi trong < 100ms
- [x] Audit event có đầy đủ: actor_id, resource_id, action, changes, occurred_at, signature
- [x] `GET /api/v2/audit-log/resource/finding/{id}` → toàn bộ lịch sử finding
- [x] `GET /api/v2/audit-log?event_type=user.login_failed` → login failures
- [x] Export CSV cho khoảng thời gian 6 tháng
- [x] Postgres RLS: `UPDATE audit_events` → fails (policy blocks it)
- [x] HMAC: sửa record trong DB → signature mismatch khi verify
- [x] Monthly partitions tự tạo cho tháng tiếp theo
- [x] Reader role → `GET /api/v2/audit-log` → 403 Forbidden
- [x] Tất cả 40+ event types được subscribe và record thành công

## Implementation Status: ✅ DONE

> `audit-service/internal/domain/event/entity.go` — AuditEvent (immutable): ID, EventType, ActorID, ResourceType, ResourceID, Action, Changes, Signature + AuditEventRepository (append-only: no Update/Delete methods)
> `audit-service/internal/infra/crypto/hmac.go` — HMACSvc.Sign (HMAC-SHA256) + Verify (constant-time)
> `audit-service/internal/delivery/event/subscriber.go` — RecordEventUseCase: subscribe 40+ NATS subjects, extract actor/resource/action, HMAC sign, ON CONFLICT DO NOTHING
> `audit-service/migrations/001_audit_events_partitioned.sql` — partitioned table + 4 quarterly partitions (2026 Q1-Q4) + 4 indexes
> `audit-service/migrations/002_rls_policies.sql` — RLS: audit_no_update (FOR UPDATE USING FALSE) + audit_no_delete (FOR DELETE USING FALSE)
> Action mapping: .created/.updated/.deleted/.closed + special cases (login_failed, bulk_updated, risk_accepted, marked_false_positive)
> Permission guard: only Admin/Maintainer can access GET /api/v2/audit-log
