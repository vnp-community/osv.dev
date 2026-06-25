# SOL-012 — Audit Log: Kiểm tra Handler và Response (P2)

**Bug**: [BUG-014](../BUG-014-admin-audit.md)  
**Service**: `audit-service`  
**Endpoint**: `GET /api/v1/audit-log`  
**HTTP Error**: `404 Not Found`

**Status**: `✅ Implemented` — via [TASK-013](../../tasks/TASK-013-*.md)

---

## Root Cause

Route đã đăng ký trong gateway tại [`router.go:275`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L275):

```go
mux.Handle("GET /api/v1/audit-log", adminOnly(proxy.Forward("audit-service:8090")))
```

**Vấn đề**: `audit-service:8090` có thể chưa implement handler cho `GET /api/v1/audit-log`, hoặc service chưa start.

---

## Giải pháp

### Bước 1: Kiểm tra audit-service

```bash
# Kiểm tra service đang chạy
docker ps | grep audit-service
curl http://audit-service:8090/health

# Test endpoint trực tiếp
curl -H "X-User-Role: Admin" \
  "http://audit-service:8090/api/v1/audit-log"
```

### Bước 2: Implement handler trong audit-service

```go
// services/audit-service/internal/delivery/http/audit_handler.go

// List — GET /api/v1/audit-log
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
    filter := AuditFilter{
        Limit:  parseIntParam(r, "limit", 50),
        Offset: parseIntParam(r, "offset", 0),
    }
    
    // Optional filters
    if actor := r.URL.Query().Get("actor"); actor != "" {
        filter.ActorEmail = actor
    }
    if action := r.URL.Query().Get("action"); action != "" {
        filter.Action = action
    }
    if from := r.URL.Query().Get("from"); from != "" {
        filter.From = parseTime(from)
    }
    if to := r.URL.Query().Get("to"); to != "" {
        filter.To = parseTime(to)
    }
    
    events, total, err := h.repo.List(r.Context(), filter)
    if err != nil {
        h.log.Error().Err(err).Msg("AuditHandler.List")
        respondError(w, http.StatusInternalServerError, "failed to list audit log")
        return
    }
    
    // Defensive: never nil
    if events == nil {
        events = make([]*AuditEvent, 0)
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

### Bước 3: Repository Layer

```go
// services/audit-service/internal/infra/postgres/audit_repo.go

func (r *AuditRepo) List(ctx context.Context, filter AuditFilter) ([]*AuditEvent, int, error) {
    // Count query
    var total int
    err := r.pool.QueryRow(ctx, `
        SELECT COUNT(*) FROM osv_audit.audit_events
        WHERE ($1::text IS NULL OR actor_email = $1)
          AND ($2::text IS NULL OR action = $2)
          AND ($3::timestamptz IS NULL OR created_at >= $3)
          AND ($4::timestamptz IS NULL OR created_at <= $4)
    `, filter.ActorEmail, filter.Action, filter.From, filter.To).Scan(&total)
    if err != nil {
        return nil, 0, err
    }
    
    // Data query
    rows, err := r.pool.Query(ctx, `
        SELECT id, actor_id, actor_email, action, resource_type, resource_id,
               before_json, after_json, created_at
        FROM osv_audit.audit_events
        WHERE ($1::text IS NULL OR actor_email = $1)
          AND ($2::text IS NULL OR action = $2)
          AND ($3::timestamptz IS NULL OR created_at >= $3)
          AND ($4::timestamptz IS NULL OR created_at <= $4)
        ORDER BY created_at DESC
        LIMIT $5 OFFSET $6
    `, filter.ActorEmail, filter.Action, filter.From, filter.To, filter.Limit, filter.Offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    events := make([]*AuditEvent, 0)  // never nil
    for rows.Next() {
        e := &AuditEvent{}
        if err := rows.Scan(
            &e.ID, &e.ActorID, &e.ActorEmail, &e.Action,
            &e.ResourceType, &e.ResourceID,
            &e.BeforeJSON, &e.AfterJSON, &e.CreatedAt,
        ); err != nil {
            return nil, 0, err
        }
        events = append(events, e)
    }
    
    return events, total, rows.Err()
}
```

### Bước 4: Register route

```go
// services/audit-service/internal/delivery/http/router.go

r.Get("/api/v1/audit-log", auditHandler.List)
r.Get("/api/v2/audit-log", auditHandler.List)           // v2 alias
r.Get("/api/v2/audit-log/{id}", auditHandler.Get)
r.Get("/api/v2/audit-log/export", auditHandler.Export)
```

---

## Response Schema

```json
{
  "data": [
    {
      "id": "uuid",
      "actor_email": "admin@example.com",
      "action": "finding.risk_accepted",
      "resource_type": "finding",
      "resource_id": "uuid",
      "created_at": "2026-06-20T00:00:00Z"
    }
  ],
  "total": 100,
  "pagination": { "limit": 50, "offset": 0, "total": 100 }
}
```
