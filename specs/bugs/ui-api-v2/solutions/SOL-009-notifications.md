# SOL-009 — Notifications: 404 Fix (P2)

**Bug**: [BUG-010](../BUG-010-notifications.md)  
**Service**: `notification-service`  
**Endpoint**: `GET /api/v1/notifications`  
**HTTP Error**: `404 Not Found`

**Status**: `✅ Implemented` — via [TASK-011](../../tasks/TASK-011-*.md)

---

## Root Cause

Route đã đăng ký tại [`router.go:238`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L238):

```go
mux.Handle("GET /api/v1/notifications", protected(proxy.Forward("notification-service:8087")))
```

Nhưng notification-service có thể chưa có handler cho `GET /api/v1/notifications`.

---

## Giải pháp

### Kiểm tra notification-service router

```bash
find services/notification-service/internal/delivery/http -name "*.go" | xargs grep -l "notifications"
```

### Pattern — Handler trong notification-service

```go
// services/notification-service/internal/delivery/http/notification_handler.go

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
        notifications = make([]*Notification, 0)
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":    notifications,
        "total":   len(notifications),
        "unread":  countUnread(notifications),
    })
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

### Đăng ký routes trong notification-service

```go
// services/notification-service/internal/delivery/http/router.go

// v1 notification routes
r.Get("/api/v1/notifications", notifHandler.List)
r.Patch("/api/v1/notifications/{id}/read", notifHandler.MarkRead)
r.Post("/api/v1/notifications/mark-all-read", notifHandler.MarkAllRead)
r.Get("/api/v1/notifications/unread-count", notifHandler.UnreadCount)
```

---

## Response Schema

```json
// GET /api/v1/notifications
{
  "data": [
    {
      "id": "uuid",
      "type": "finding.sla.breached",
      "title": "SLA Breach Alert",
      "message": "Finding XYZ has breached SLA",
      "read": false,
      "created_at": "2026-06-20T00:00:00Z"
    }
  ],
  "total": 5,
  "unread": 3
}
```
