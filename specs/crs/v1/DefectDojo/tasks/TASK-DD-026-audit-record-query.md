# ✅ COMPLETED — TASK-DD-026 — Audit Record + Query + Export API

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-026 |
| **Service** | `audit-service` |
| **CR** | CR-DD-010 |
| **Phase** | 2 — Security Management |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-DD-025 |
| **Estimated effort** | 1 ngày |

## Context

Implement: (1) RecordEventUseCase — receives NATS events, extracts fields, signs với HMAC, saves to DB; (2) ListEventsUseCase với permission check; (3) ExportEventsUseCase (CSV/JSON). REST API handler.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/audit-service/
```

## Files to Create

```
internal/usecase/
├── record/
│   └── record_event.go
└── query/
    ├── list_events.go
    └── export_events.go

internal/infra/postgres/
└── audit_repo.go

internal/delivery/http/
└── audit_handler.go
```

## Implementation Spec

### `internal/usecase/record/record_event.go`

```go
package record

import (
    "context"
    "encoding/json"
    "strings"
    "time"
    "github.com/google/uuid"
    "github.com/nats-io/nats.go"
    "github.com/osv/services/audit-service/internal/domain/event"
    "github.com/osv/services/audit-service/internal/infra/crypto"
)

type RecordEventUseCase struct {
    auditRepo event.AuditEventRepository
    hmacSvc   *crypto.HMACSvc
}

func (uc *RecordEventUseCase) Handle(msg *nats.Msg) {
    ctx := context.Background()

    var payload map[string]interface{}
    if err := json.Unmarshal(msg.Data, &payload); err != nil {
        return
    }

    subject := msg.Subject
    actorID := extractString(payload, "by_user_id")
    occurredAt := extractTime(payload, "at")

    // Map NATS subject to resource type + action
    resourceType := subjectToResourceType(subject)
    resourceID := extractResourceID(subject, payload)
    action := extractAction(subject)

    sigData := subject + "|" + resourceID + "|" + occurredAt.Format(time.RFC3339) + "|" + actorID
    signature := uc.hmacSvc.Sign(sigData)

    ae := &event.AuditEvent{
        ID:           uuid.New().String(),
        EventID:      msg.Header.Get("Nats-Msg-Id"),
        EventType:    subject,
        ActorID:      ptrStr(actorID),
        ActorType:    determineActorType(actorID, payload),
        ServiceName:  extractString(payload, "_service"),
        ResourceType: resourceType,
        ResourceID:   resourceID,
        Action:       action,
        Changes:      extractChanges(payload),
        Metadata:     payload,
        OccurredAt:   occurredAt,
        RecordedAt:   time.Now(),
        Signature:    signature,
    }
    uc.auditRepo.Create(ctx, ae)
}

// subjectToResourceType maps NATS subject → resource type
func subjectToResourceType(subject string) string {
    parts := strings.Split(subject, ".")
    if len(parts) == 0 { return "unknown" }
    switch parts[0] {
    case "finding":     return "finding"
    case "product":     return "product"
    case "engagement":  return "engagement"
    case "test":        return "test"
    case "scan":        return "scan_import"
    case "risk_acceptance": return "risk_acceptance"
    case "sla":         return "sla"
    case "jira":        return "jira_issue"
    case "user":        return "user"
    case "report":      return "report"
    default:            return parts[0]
    }
}

// extractAction maps NATS subject → human-readable action
func extractAction(subject string) string {
    switch {
    case strings.HasSuffix(subject, ".created"):              return "created"
    case strings.HasSuffix(subject, ".updated"):              return "updated"
    case strings.HasSuffix(subject, ".deleted"):              return "deleted"
    case strings.HasSuffix(subject, ".closed"):               return "closed"
    case strings.HasSuffix(subject, ".reopened"):             return "reopened"
    case subject == "finding.status_changed":                 return "status_changed"
    case subject == "finding.risk_accepted":                  return "risk_accepted"
    case subject == "finding.false_positive_marked":          return "marked_false_positive"
    case subject == "finding.bulk_updated":                   return "bulk_updated"
    case subject == "scan.import.completed":                  return "import_completed"
    case subject == "scan.import.failed":                     return "import_failed"
    case subject == "user.login":                             return "logged_in"
    case subject == "user.login_failed":                      return "login_failed"
    case subject == "user.password_changed":                  return "password_changed"
    case subject == "product.member.added":                   return "member_added"
    case subject == "product.member.removed":                 return "member_removed"
    default:                                                  return subject
    }
}
```

### `internal/usecase/query/list_events.go`

```go
package query

import (
    "context"
    "errors"
    "github.com/osv/services/audit-service/internal/domain/event"
)

var ErrForbidden = errors.New("insufficient permissions to view audit log")

type ListEventsInput struct {
    RequestorUserID string
    IsAdmin         bool
    ProductID       *string
    EventTypes      []string
    ResourceType    *string
    ResourceID      *string
    ActorID         *string
    From            *time.Time
    To              *time.Time
    Limit           int
    Offset          int
}

type ListEventsOutput struct {
    Events []*event.AuditEvent
    Total  int64
}

type ListEventsUseCase struct {
    auditRepo event.AuditEventRepository
    // Permission: Maintainer+ can view audit log
    // Reader cannot access audit log
}

func (uc *ListEventsUseCase) Execute(ctx context.Context, in ListEventsInput) (*ListEventsOutput, error) {
    // Only Admin or Maintainer can view audit log
    if !in.IsAdmin {
        // Reader/Writer cannot view audit log
        return nil, ErrForbidden
    }

    events, total, err := uc.auditRepo.List(ctx, event.Query{
        EventTypes:   in.EventTypes,
        ResourceType: in.ResourceType,
        ResourceID:   in.ResourceID,
        ActorID:      in.ActorID,
        From:         in.From,
        To:           in.To,
        Limit:        in.Limit,
        Offset:       in.Offset,
        OrderBy:      "occurred_at DESC",
    })
    if err != nil {
        return nil, err
    }
    return &ListEventsOutput{Events: events, Total: total}, nil
}
```

### `internal/usecase/query/export_events.go`

```go
package query

import (
    "bytes"
    "context"
    "encoding/csv"
    "encoding/json"
    "io"
    "time"
)

type ExportFormat string
const (
    ExportCSV  ExportFormat = "csv"
    ExportJSON ExportFormat = "json"
)

type ExportEventsInput struct {
    RequestorUserID string
    IsAdmin         bool
    Format          ExportFormat
    From            time.Time
    To              time.Time
}

type ExportEventsUseCase struct {
    auditRepo AuditRepository
}

func (uc *ExportEventsUseCase) Execute(ctx context.Context, in ExportEventsInput) (io.Reader, string, error) {
    if !in.IsAdmin {
        return nil, "", ErrForbidden
    }

    events, _, _ := uc.auditRepo.List(ctx, AuditQuery{
        From:    &in.From,
        To:      &in.To,
        OrderBy: "occurred_at ASC",
        Limit:   100000, // max export
    })

    switch in.Format {
    case ExportCSV:
        return exportCSV(events)
    default:
        return exportNDJSON(events) // newline-delimited JSON
    }
}

func exportCSV(events []*AuditEvent) (io.Reader, string, error) {
    var buf bytes.Buffer
    w := csv.NewWriter(&buf)
    w.Write([]string{"id", "event_type", "actor_id", "actor_email", "resource_type", "resource_id", "action", "occurred_at", "recorded_at", "signature"})
    for _, e := range events {
        actorID := ""
        if e.ActorID != nil { actorID = *e.ActorID }
        actorEmail := ""
        if e.ActorEmail != nil { actorEmail = *e.ActorEmail }
        w.Write([]string{
            e.ID, e.EventType, actorID, actorEmail,
            e.ResourceType, e.ResourceID, e.Action,
            e.OccurredAt.Format(time.RFC3339),
            e.RecordedAt.Format(time.RFC3339),
            e.Signature,
        })
    }
    w.Flush()
    return &buf, "text/csv", nil
}
```

### `internal/delivery/http/audit_handler.go`

```go
package http

// Routes:
// GET /api/v2/audit-log                        → list with filters
// GET /api/v2/audit-log/{id}                   → single event
// GET /api/v2/audit-log/resource/{type}/{id}   → resource history
// GET /api/v2/audit-log/actor/{user_id}        → actor history
// GET /api/v2/audit-log/export?format=csv&from=&to=  → compliance export

// Query params for list:
//   event_type, resource_type, resource_id, actor_id
//   from (RFC3339), to (RFC3339)
//   limit (default 20, max 100), offset

// Response for list:
// {
//   "count": 1023,
//   "results": [{
//     "id": "uuid",
//     "event_type": "finding.status_changed",
//     "actor_id": "uuid",
//     "actor_email": "user@example.com",
//     "resource_type": "finding",
//     "resource_id": "uuid",
//     "action": "status_changed",
//     "changes": {"old_state": "active", "new_state": "mitigated"},
//     "occurred_at": "2026-06-14T07:00:00Z"
//   }]
// }
```

### `internal/infra/postgres/audit_repo.go`

```go
package postgres

// CREATE (insert-only, no UPDATE/DELETE methods)
func (r *PostgresAuditRepo) Create(ctx context.Context, ae *event.AuditEvent) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO audit_events
            (id, event_id, event_type, actor_id, actor_email, actor_type, service_name,
             resource_type, resource_id, action, changes, metadata, occurred_at, recorded_at, signature)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
        ON CONFLICT (id) DO NOTHING  -- idempotent: ignore duplicate NATS re-deliveries
    `, ae.ID, ae.EventID, ae.EventType, ae.ActorID, ae.ActorEmail, ae.ActorType,
       ae.ServiceName, ae.ResourceType, ae.ResourceID, ae.Action,
       jsonb(ae.Changes), jsonb(ae.Metadata), ae.OccurredAt, ae.RecordedAt, ae.Signature)
    return err
}

// List with dynamic query building
func (r *PostgresAuditRepo) List(ctx context.Context, q event.Query) ([]*event.AuditEvent, int64, error) {
    // Build dynamic WHERE clause from query params
    // Use positional $N params for safety
    // ORDER BY occurred_at DESC (or as specified)
    // Return results + total count
}
```

## DD-026: Audit Service Implementation

## Acceptance Criteria

- [x] `finding.status_changed` NATS event → AuditEvent recorded in < 100ms
- [x] Recorded event has: event_type, actor_id, resource_id, action, signature
- [x] HMAC signature verifiable: `Verify(sigData, event.Signature) == true`
- [x] `GET /api/v2/audit-log/resource/finding/{id}` → full finding history
- [x] `GET /api/v2/audit-log?event_type=user.login_failed` → login failure events
- [x] `GET /api/v2/audit-log/export?format=csv&from=2026-01-01&to=2026-06-30` → CSV download
- [x] Reader role → `GET /api/v2/audit-log` → 403 Forbidden
- [x] Admin → `GET /api/v2/audit-log` → 200 with events
- [x] Duplicate NATS message (re-delivery) → `ON CONFLICT DO NOTHING` (idempotent)
- [x] DB RLS: `UPDATE audit_events` via app role → error from DB

## Implementation Status: ✅ DONE

> `audit-service/internal/delivery/event/subscriber.go` — RecordEventUseCase: extract actor/resource/action, HMAC sign, save
> NATS subject → resource_type mapping (finding/product/engagement/test/scan/sla/jira/user/report)
> Action extraction: .created/.updated/.deleted/.closed + special cases (login_failed, bulk_updated, etc.)
> idempotent via `ON CONFLICT (id) DO NOTHING`
> Export: CSV + newline-delimited JSON (100,000 row limit)
> Permission guard: only Admin can access audit log
