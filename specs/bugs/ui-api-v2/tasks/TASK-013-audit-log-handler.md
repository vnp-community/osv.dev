# TASK-013 — audit-service: Implement Audit Log List Handler

**Bug**: [BUG-014](../BUG-014-admin-audit.md)  
**Solution**: [SOL-012](../solutions/SOL-012-audit-log.md)  
**Priority**: 🟡 P2  
**Effort**: ~25 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/audit-log` trả `404`. Gateway đã đăng ký route → `audit-service:8090` nhưng audit-service chưa implement handler.

---

## File cần sửa / tạo

Tìm structure của audit-service:

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services/audit-service/internal -name "*.go" | head -20
```

---

## Thay đổi 1 — Tạo AuditHandler (nếu chưa có)

**Tạo hoặc cập nhật** `services/audit-service/internal/delivery/http/audit_handler.go`:

```go
package http

import (
    "net/http"
    "time"

    "github.com/rs/zerolog"
)

type AuditFilter struct {
    ActorEmail string
    Action     string
    From       *time.Time
    To         *time.Time
    Limit      int
    Offset     int
}

type AuditEvent struct {
    ID           string      `json:"id"`
    ActorID      string      `json:"actor_id"`
    ActorEmail   string      `json:"actor_email"`
    Action       string      `json:"action"`
    ResourceType string      `json:"resource_type"`
    ResourceID   string      `json:"resource_id"`
    Before       interface{} `json:"before,omitempty"`
    After        interface{} `json:"after,omitempty"`
    CreatedAt    string      `json:"created_at"`
}

type AuditRepository interface {
    List(ctx context.Context, filter AuditFilter) ([]*AuditEvent, int, error)
    GetByID(ctx context.Context, id string) (*AuditEvent, error)
}

type AuditHandler struct {
    repo AuditRepository
    log  zerolog.Logger
}

func NewAuditHandler(repo AuditRepository, log zerolog.Logger) *AuditHandler {
    return &AuditHandler{repo: repo, log: log}
}

// List — GET /api/v1/audit-log
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
    filter := AuditFilter{
        ActorEmail: r.URL.Query().Get("actor"),
        Action:     r.URL.Query().Get("action"),
        Limit:      parseIntParam(r, "limit", 50),
        Offset:     parseIntParam(r, "offset", 0),
    }

    if fromStr := r.URL.Query().Get("from"); fromStr != "" {
        if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
            filter.From = &t
        }
    }
    if toStr := r.URL.Query().Get("to"); toStr != "" {
        if t, err := time.Parse(time.RFC3339, toStr); err == nil {
            filter.To = &t
        }
    }

    events, total, err := h.repo.List(r.Context(), filter)
    if err != nil {
        h.log.Error().Err(err).Msg("AuditHandler.List")
        respondError(w, http.StatusInternalServerError, "failed to list audit log")
        return
    }

    if events == nil {
        events = make([]*AuditEvent, 0)  // never nil
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":  events,
        "total": total,
        "pagination": map[string]interface{}{
            "limit":  filter.Limit,
            "offset": filter.Offset,
            "total":  total,
        },
    })
}
```

---

## Thay đổi 2 — Implement Repository

```go
func (r *AuditRepo) List(ctx context.Context, filter AuditFilter) ([]*AuditEvent, int, error) {
    // Count
    var total int
    err := r.pool.QueryRow(ctx, `
        SELECT COUNT(*) FROM osv_audit.audit_events
        WHERE ($1::text IS NULL OR actor_email = $1)
          AND ($2::text IS NULL OR action = $2)
          AND ($3::timestamptz IS NULL OR created_at >= $3)
          AND ($4::timestamptz IS NULL OR created_at <= $4)
    `, nullStr(filter.ActorEmail), nullStr(filter.Action),
       filter.From, filter.To).Scan(&total)
    if err != nil {
        return nil, 0, err
    }

    // Data
    rows, err := r.pool.Query(ctx, `
        SELECT id, actor_id, actor_email, action, resource_type, resource_id, created_at
        FROM osv_audit.audit_events
        WHERE ($1::text IS NULL OR actor_email = $1)
          AND ($2::text IS NULL OR action = $2)
          AND ($3::timestamptz IS NULL OR created_at >= $3)
          AND ($4::timestamptz IS NULL OR created_at <= $4)
        ORDER BY created_at DESC
        LIMIT $5 OFFSET $6
    `, nullStr(filter.ActorEmail), nullStr(filter.Action),
       filter.From, filter.To, filter.Limit, filter.Offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    events := make([]*AuditEvent, 0)
    for rows.Next() {
        e := &AuditEvent{}
        var createdAt time.Time
        if err := rows.Scan(&e.ID, &e.ActorID, &e.ActorEmail,
            &e.Action, &e.ResourceType, &e.ResourceID, &createdAt); err != nil {
            return nil, 0, err
        }
        e.CreatedAt = createdAt.Format(time.RFC3339)
        events = append(events, e)
    }
    return events, total, rows.Err()
}

// nullStr converts empty string to nil for nullable SQL params
func nullStr(s string) interface{} {
    if s == "" {
        return nil
    }
    return s
}
```

---

## Thay đổi 3 — Register routes

```go
// audit-service router
r.Get("/api/v1/audit-log", auditHandler.List)
r.Get("/api/v2/audit-log", auditHandler.List)
r.Get("/api/v2/audit-log/{id}", auditHandler.GetByID)
r.Get("/api/v2/audit-log/export", auditHandler.Export)
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/audit-log` trả `200` với `{"data": [], "total": 0}`
- [ ] Pagination fields có mặt trong response
- [ ] Filter `?actor=email&action=action` hoạt động
- [ ] `go build ./...` trong audit-service không có lỗi

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/audit-service
go build ./...

curl -s -H "Authorization: Bearer <admin_token>" \
  "https://c12.openledger.vn/api/v1/audit-log" | jq '{total, data_type: (.data | type)}'
# Expected: {"total": N, "data_type": "array"}
```
