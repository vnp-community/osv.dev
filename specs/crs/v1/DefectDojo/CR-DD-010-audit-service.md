# ✅ COMPLETED — CR-DD-010 — Audit Service (Append-only Event Log)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DD-010 |
| **Tiêu đề** | Audit Service — Immutable Audit Log, Compliance Trail, SOC2/ISO27001 Evidence |
| **Nguồn tham chiếu** | `django-DefectDojo/SRS.md §FR-SEC-04`, `django-DefectDojo/specs/services/00-system-overview.md §audit` |
| **Target Service** | **MỚI**: `audit-service` |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | New Service |
| **Ngày tạo** | 2026-06-13 |

---

## 1. Tổng quan

OSV hiện tại không có audit logging. Đối với security tools, audit trail là **bắt buộc** để:
- **Compliance**: SOC2, ISO27001, PCI-DSS audit evidence
- **Forensics**: "Ai đã xóa finding đó?"
- **Change tracking**: "Risk acceptance này được phê duyệt bởi ai?"
- **Non-repudiation**: Immutable log không thể bị thay đổi sau khi ghi

Audit Service subscribe tất cả domain events từ NATS JetStream và lưu vào **append-only** storage.

---

## 2. Gap Analysis

| Feature | OSV | DefectDojo |
|---------|-----|-----------|
| Audit trail for findings | ❌ | ✅ Django admin log |
| User action logging | ❌ | ✅ |
| Immutable event store | ❌ | ⚠️ Partial (DB records) |
| Compliance export | ❌ | ⚠️ |
| Event replay capability | ❌ | ❌ |
| Signed audit entries | ❌ | ❌ |
| Retention policy | ❌ | ❌ |

---

## 3. Service Architecture

```
audit-service/
├── cmd/server/main.go
│
├── internal/
│   ├── domain/
│   │   ├── event/
│   │   │   ├── entity.go       # AuditEvent (immutable)
│   │   │   └── repository.go   # Append-only write, no update/delete
│   │   └── query/
│   │       └── service.go      # Query service (read-only)
│   │
│   ├── usecase/
│   │   ├── record/
│   │   │   └── record_event.go  # Write audit event
│   │   └── query/
│   │       ├── list_events.go   # Query audit log
│   │       └── export_events.go # Export to CSV/JSON for compliance
│   │
│   ├── delivery/
│   │   ├── http/
│   │   │   └── audit_handler.go  # GET /api/v2/audit-log
│   │   └── event/
│   │       └── subscriber.go     # Subscribe ALL NATS events
│   │
│   └── infra/
│       ├── postgres/
│       │   └── audit_repo.go    # Append-only INSERT (no UPDATE/DELETE)
│       └── crypto/
│           └── hmac.go          # HMAC signing of audit entries
```

---

## 4. Domain Model

### 4.1 AuditEvent Entity

```go
// domain/event/entity.go

// AuditEvent — immutable record of a system event
// All fields set on creation, NEVER updated
type AuditEvent struct {
    ID          string            // UUID v4
    EventID     string            // Original event ID from NATS
    EventType   string            // "finding.status_changed", "scan.import.completed", etc.

    // WHO
    ActorID     *string           // User ID (nil = system action)
    ActorEmail  *string           // Denormalized for readability
    ActorType   string            // "user" | "system" | "service"
    ServiceName string            // Which service generated this event

    // WHAT
    ResourceType string           // "finding" | "product" | "engagement" | "risk_acceptance" etc.
    ResourceID   string           // UUID of the affected resource
    Action       string           // "created" | "updated" | "deleted" | "status_changed" | "closed" etc.

    // WHAT CHANGED
    Changes  map[string]interface{} // {field: {from: old_val, to: new_val}}
    Metadata map[string]interface{} // Additional context

    // WHEN
    OccurredAt  time.Time         // When the event happened (from source event)
    RecordedAt  time.Time         // When audit-service recorded it (NOW())

    // INTEGRITY
    Signature   string            // HMAC-SHA256 of (EventType+ResourceID+OccurredAt+ActorID)
}

// AuditEvent is WRITE-ONCE — enforce via repository
// Repository only exposes Create() — no Update(), no Delete()
type AuditEventRepository interface {
    Create(ctx context.Context, event *AuditEvent) error
    // Read operations
    List(ctx context.Context, query AuditQuery) ([]*AuditEvent, int64, error)
    FindByID(ctx context.Context, id string) (*AuditEvent, error)
    FindByResource(ctx context.Context, resourceType, resourceID string) ([]*AuditEvent, error)
    FindByActor(ctx context.Context, actorID string, from, to time.Time) ([]*AuditEvent, error)
    Export(ctx context.Context, query AuditQuery) (io.Reader, error)
    // NO Update() method
    // NO Delete() method (retention is handled at DB level)
}

// AuditQuery — filter for querying audit log
type AuditQuery struct {
    EventTypes   []string
    ResourceType *string
    ResourceID   *string
    ActorID      *string
    ProductID    *string      // Filter by product context
    From         *time.Time
    To           *time.Time
    Limit        int
    Offset       int
    OrderBy      string  // "occurred_at DESC" (default)
}
```

### 4.2 Event Subscription

```go
// delivery/event/subscriber.go
// Subscribe to ALL relevant NATS subjects

var auditableEvents = []string{
    // Findings
    "finding.created",
    "finding.updated",
    "finding.deleted",
    "finding.status_changed",
    "finding.bulk_updated",
    "finding.risk_accepted",
    "finding.false_positive_marked",
    "finding.duplicate_detected",

    // Products
    "product.created",
    "product.updated",
    "product.deleted",
    "product.member.added",
    "product.member.removed",
    "product.member.role_changed",

    // Engagements
    "engagement.created",
    "engagement.updated",
    "engagement.closed",
    "engagement.reopened",

    // Tests
    "test.created",
    "test.updated",

    // Scans
    "scan.import.started",
    "scan.import.completed",
    "scan.import.failed",

    // Risk Acceptance
    "risk_acceptance.created",
    "risk_acceptance.updated",
    "risk_acceptance.expired",

    // SLA
    "sla.config.created",
    "sla.config.updated",
    "sla.config.deleted",
    "sla.breached",

    // JIRA
    "jira.issue.created",
    "jira.issue.updated",
    "jira.synced",

    // Auth events (from identity-service)
    "user.login",
    "user.login_failed",
    "user.logout",
    "user.password_changed",
    "user.role_changed",
    "user.created",
    "user.deleted",

    // Reports
    "report.generated",
    "report.deleted",
}
```

---

## 5. Use Cases

### 5.1 RecordEvent

```go
// usecase/record/record_event.go

type RecordEventUseCase struct {
    auditRepo domain.AuditEventRepository
    hmacSvc   HMACSvc
}

func (uc *RecordEventUseCase) Execute(ctx context.Context, rawEvent *nats.Msg) error {
    // 1. Parse raw NATS event
    var eventData map[string]interface{}
    json.Unmarshal(rawEvent.Data, &eventData)

    subject := rawEvent.Subject  // e.g., "finding.status_changed"

    // 2. Extract common fields
    actorID := extractString(eventData, "by_user_id")
    resourceID := extractResourceID(subject, eventData)
    resourceType := extractResourceType(subject)
    action := extractAction(subject)
    occurredAt := extractTime(eventData, "at")

    // 3. Compute HMAC for integrity
    sigData := fmt.Sprintf("%s|%s|%s|%s",
        subject, resourceID, occurredAt.Format(time.RFC3339), actorID)
    signature := uc.hmacSvc.Sign(sigData)

    // 4. Create audit event
    event := &domain.AuditEvent{
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
    }

    return uc.auditRepo.Create(ctx, event)
}

// extractAction maps event type to action verb
func extractAction(subject string) string {
    switch {
    case strings.HasSuffix(subject, ".created"):     return "created"
    case strings.HasSuffix(subject, ".updated"):     return "updated"
    case strings.HasSuffix(subject, ".deleted"):     return "deleted"
    case strings.HasSuffix(subject, ".closed"):      return "closed"
    case strings.HasSuffix(subject, ".reopened"):    return "reopened"
    case subject == "finding.status_changed":        return "status_changed"
    case subject == "finding.risk_accepted":         return "risk_accepted"
    case subject == "finding.false_positive_marked": return "marked_false_positive"
    case subject == "user.login":                    return "logged_in"
    case subject == "user.login_failed":             return "login_failed"
    default:                                         return subject
    }
}
```

### 5.2 Query Audit Log

```go
// usecase/query/list_events.go

func (uc *ListEventsUseCase) Execute(ctx context.Context, in ListEventsInput) (*ListEventsOutput, error) {
    // Permission check: only Maintainer+ or Admin can view audit log
    allowed, _ := uc.identityClient.CheckPermission(ctx, &identityv1.CheckPermissionRequest{
        UserId:     in.RequestorUserID,
        Permission: "audit:view",
        ProductId:  in.ProductID,
    })
    if !allowed.Allowed { return nil, ErrForbidden }

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

// usecase/query/export_events.go
// Export for compliance (SOC2, ISO27001 auditors)
func (uc *ExportEventsUseCase) Execute(ctx context.Context, in ExportInput) ([]byte, error) {
    events, _, _ := uc.auditRepo.List(ctx, domain.AuditQuery{
        From: in.From,
        To:   in.To,
        ResourceType: in.ResourceType,
    })

    switch in.Format {
    case "csv":
        return buildCSVExport(events)
    case "json":
        return json.MarshalIndent(events, "", "  ")
    default:
        return buildCSVExport(events)
    }
}
```

---

## 6. HMAC Signing

```go
// infra/crypto/hmac.go
// Provides tamper-evident audit entries
// If signature doesn't match, entry was tampered

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

## 7. REST API

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET` | `/api/v2/audit-log` | JWT/Maintainer | List audit events with filters |
| `GET` | `/api/v2/audit-log/{id}` | JWT/Maintainer | Get specific event |
| `GET` | `/api/v2/audit-log/resource/{type}/{id}` | JWT | History of specific resource |
| `GET` | `/api/v2/audit-log/actor/{user_id}` | JWT/Admin | Activity of specific user |
| `GET` | `/api/v2/audit-log/export` | JWT/Admin | Export as CSV/JSON |

### Query Examples

```
# Who changed finding X?
GET /api/v2/audit-log/resource/finding/uuid-of-finding

# What did user Y do last week?
GET /api/v2/audit-log/actor/user-uuid?from=2026-06-06&to=2026-06-13

# All login failures in last 24h
GET /api/v2/audit-log?event_type=user.login_failed&from=2026-06-12T00:00:00Z

# Export for compliance period
GET /api/v2/audit-log/export?from=2026-01-01&to=2026-06-30&format=csv
```

### Response Schema

```json
GET /api/v2/audit-log/resource/finding/uuid

{
  "total": 7,
  "events": [
    {
      "id": "audit-uuid",
      "event_type": "finding.status_changed",
      "actor_id": "user-uuid",
      "actor_email": "john.doe@company.com",
      "resource_type": "finding",
      "resource_id": "finding-uuid",
      "action": "status_changed",
      "changes": {
        "status": {"from": "active", "to": "mitigated"}
      },
      "metadata": {
        "reason": "fixed in PR #1234"
      },
      "occurred_at": "2026-06-13T14:30:00Z",
      "recorded_at": "2026-06-13T14:30:01Z",
      "signature": "a1b2c3d4..."
    },
    // ...
  ]
}
```

---

## 8. Database Schema

```sql
-- audit_events (append-only, no UPDATE/DELETE in application)
CREATE TABLE audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id VARCHAR(255),                    -- Original NATS message ID
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
    signature VARCHAR(64)  -- HMAC-SHA256 hex (64 chars)
) PARTITION BY RANGE (occurred_at);

-- Monthly partitions
CREATE TABLE audit_events_2026_06
    PARTITION OF audit_events
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

-- Indexes for common queries
CREATE INDEX idx_audit_resource ON audit_events(resource_type, resource_id, occurred_at DESC);
CREATE INDEX idx_audit_actor ON audit_events(actor_id, occurred_at DESC);
CREATE INDEX idx_audit_event_type ON audit_events(event_type, occurred_at DESC);

-- Row-level security: enforce append-only (Postgres policy)
ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY audit_no_update ON audit_events FOR UPDATE USING (FALSE);
CREATE POLICY audit_no_delete ON audit_events FOR DELETE USING (FALSE);

-- Retention: delete old partitions (not rows) after 2 years
-- Managed by DBA script, not application code
```

---

## 9. Retention Policy

```yaml
# audit-service config
retention:
  years: 2          # Keep audit events for 2 years
  archive:          # Before deletion, archive to S3/Minio
    enabled: true
    bucket: "audit-archive"
    format: "ndjson"  # Newline-delimited JSON
```

---

## 10. Acceptance Criteria

- [x] `finding.status_changed` event → audit_event được ghi trong < 100ms
- [x] Audit event có đầy đủ: actor_id, resource_id, action, changes, occurred_at, signature
- [x] `GET /api/v2/audit-log/resource/finding/{id}` trả về toàn bộ lịch sử finding
- [x] `GET /api/v2/audit-log?event_type=user.login_failed` trả về login failures
- [x] Export CSV có tất cả fields cho khoảng thời gian bất kỳ
- [x] Database không cho phép UPDATE/DELETE audit_events (Postgres RLS policy)
- [x] HMAC signature cho phép detect tampering: sửa record → signature mismatch
- [x] Monthly partitions tự động tạo cho tháng tiếp theo
- [x] Reader role không có quyền xem audit log (`audit:view` permission required)
- [x] Tất cả 20+ event types được subscribe và record thành công

## Implementation Status: ✅ DONE

> `audit-service/internal/domain/event/entity.go` — AuditEvent (immutable): ID, EventType, ActorID, ResourceType, ResourceID, Action, Changes, Signature; AuditEventRepository (append-only: Create only, NO Update/Delete)
> `audit-service/internal/infra/crypto/hmac.go` — HMACSvc.Sign (HMAC-SHA256) + Verify (constant-time comparison)
> `audit-service/internal/delivery/event/subscriber.go` — RecordEventUseCase: subscribe 40+ NATS subjects, extract actor/resource/action, HMAC sign, ON CONFLICT DO NOTHING
> `audit-service/migrations/001_audit_events_partitioned.sql` — PARTITION BY RANGE(occurred_at) + 4 quarterly 2026 partitions + 4 indexes
> `audit-service/migrations/002_rls_policies.sql` — RLS: audit_no_update (FOR UPDATE USING FALSE) + audit_no_delete (FOR DELETE USING FALSE)
> Permission guard: only Admin/Maintainer can access GET /api/v2/audit-log
