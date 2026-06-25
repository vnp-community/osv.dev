# TASK-011 — notification-service: Implement Notifications List Handler

**Bug**: [BUG-010](../BUG-010-notifications.md)  
**Solution**: [SOL-009](../solutions/SOL-009-notifications.md)  
**Priority**: 🟡 P2  
**Effort**: ~20 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/notifications` trả `404`. notification-service chưa có handler cho endpoint này. Gateway forward tới `notification-service:8087` nhưng service không handle path `/api/v1/notifications`.

---

## File cần sửa

Tìm structure của notification-service:

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/delivery -name "*.go"
```

**File sửa/tạo**: `services/notification-service/internal/delivery/http/notification_handler.go`  
**File sửa**: `services/notification-service/internal/delivery/http/router.go`

---

## Thay đổi 1 — notification_handler.go: Thêm List

```go
// List — GET /api/v1/notifications
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    if userID == "" {
        respondError(w, http.StatusUnauthorized, "user id required")
        return
    }

    limit := parseIntParam(r, "limit", 20)
    offset := parseIntParam(r, "offset", 0)

    notifications, err := h.alertRepo.ListByUser(r.Context(), userID, limit, offset)
    if err != nil {
        h.log.Error().Err(err).Str("user_id", userID).Msg("NotificationHandler.List")
        respondError(w, http.StatusInternalServerError, "failed to list notifications")
        return
    }

    // Defensive: never nil
    if notifications == nil {
        notifications = make([]*Alert, 0)
    }

    unread := 0
    for _, n := range notifications {
        if !n.Read {
            unread++
        }
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":   notifications,
        "total":  len(notifications),
        "unread": unread,
    })
}

// UnreadCount — GET /api/v1/notifications/unread-count
func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")

    count, err := h.alertRepo.CountUnread(r.Context(), userID)
    if err != nil {
        count = 0  // graceful fallback
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "count": count,
    })
}

// MarkRead — PATCH /api/v1/notifications/{id}/read
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if err := h.alertRepo.MarkRead(r.Context(), id); err != nil {
        respondError(w, http.StatusInternalServerError, "failed to mark read")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

// MarkAllRead — POST /api/v1/notifications/mark-all-read
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    if err := h.alertRepo.MarkAllRead(r.Context(), userID); err != nil {
        respondError(w, http.StatusInternalServerError, "failed to mark all read")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
```

---

## Thay đổi 2 — repo: Implement ListByUser

**Tìm** alert repository và **thêm**:

```go
func (r *AlertRepo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*Alert, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT id, user_id, type, title, message, read, created_at
        FROM osv_notif.alerts
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `, userID, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    result := make([]*Alert, 0)  // never nil
    for rows.Next() {
        a := &Alert{}
        if err := rows.Scan(&a.ID, &a.UserID, &a.Type,
            &a.Title, &a.Message, &a.Read, &a.CreatedAt); err != nil {
            return nil, err
        }
        result = append(result, a)
    }
    return result, rows.Err()
}

func (r *AlertRepo) CountUnread(ctx context.Context, userID string) (int, error) {
    var count int
    err := r.pool.QueryRow(ctx,
        `SELECT COUNT(*) FROM osv_notif.alerts WHERE user_id = $1 AND read = false`,
        userID).Scan(&count)
    return count, err
}
```

---

## Thay đổi 3 — router.go: Register v1 notification routes

```go
// v1 notification routes (gateway forwards to notification-service)
// QUAN TRỌNG: literal paths TRƯỚC /{id}
r.Post("/api/v1/notifications/mark-all-read", notifHandler.MarkAllRead)   // literal TRƯỚC
r.Get("/api/v1/notifications/unread-count", notifHandler.UnreadCount)     // literal TRƯỚC
r.Get("/api/v1/notifications", notifHandler.List)
r.Patch("/api/v1/notifications/{id}/read", notifHandler.MarkRead)
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/notifications` trả `200` với `{"data": [], "total": 0}`
- [ ] `GET /api/v1/notifications/unread-count` trả `{"count": N}`
- [ ] Response `data` là array (kể cả empty)
- [ ] `go build ./...` không có lỗi

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service
go build ./...

curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/notifications" | jq '{total, data_type: (.data | type)}'
# Expected: {"total": 0, "data_type": "array"}
```
