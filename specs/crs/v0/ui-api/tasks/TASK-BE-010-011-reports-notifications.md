# TASK-BE-010 — finding-service: Reports Download (MinIO Presigned URL)

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-010 |
| **Service** | `services/finding-service` |
| **Solution Ref** | [SOL-UI-004 §1.5](../solutions/SOL-UI-004-finding-product-reports-admin.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | — |
| **Estimated** | 2h |

---

## Context

UI Reports page cần download report files. Files được lưu trên MinIO theo path `reports/{run_id}/{run_id}.{format}`. Hiện không có HTTP endpoint để tải xuống.

---

## Target Files

| Action | File Path |
|--------|-----------|
| MODIFY | `services/finding-service/internal/adapter/http/report_handler.go` |

---

## Implementation

```go
// services/finding-service/internal/adapter/http/report_handler.go

// GET /v2/reports/{id}/download
func (h *ReportHandler) DownloadReport(w http.ResponseWriter, r *http.Request) {
    reportID, err := uuid.Parse(r.PathValue("id"))
    if err != nil {
        respondError(w, 400, "VALIDATION_ERROR", "Invalid report ID")
        return
    }

    run, err := h.reportRepo.FindByID(r.Context(), reportID)
    if err != nil {
        respondError(w, 404, "NOT_FOUND", "Report not found")
        return
    }

    // Check ownership
    userID := r.Header.Get("X-User-ID")
    if run.RequestedByID != userID {
        // Admin can download any report — check role
        role := r.Header.Get("X-User-Role")
        if role != "admin" {
            respondError(w, 403, "FORBIDDEN", "Cannot download this report")
            return
        }
    }

    // Check status
    if run.Status != "completed" {
        respondError(w, 409, "REPORT_NOT_READY",
            fmt.Sprintf("Report is not ready (status: %s)", run.Status))
        return
    }

    // run.ArtifactPath = "reports/{run_id}/{run_id}.pdf"
    presignedURL, err := h.minioClient.PresignGetObject(r.Context(), run.ArtifactPath, 5*time.Minute)
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", "Failed to generate download URL")
        return
    }

    // Option A: 302 redirect to presigned URL (preferred — avoids streaming through service)
    http.Redirect(w, r, presignedURL, http.StatusFound)

    // Option B: Return presigned URL as JSON (if frontend needs the URL directly)
    // respondJSON(w, 200, map[string]interface{}{
    //     "download_url":  presignedURL,
    //     "expires_in":    300, // seconds
    //     "filename":      filepath.Base(run.ArtifactPath),
    // })
}
```

### MinIO Client interface (add to infra if not exists):

```go
// services/finding-service/internal/infra/minio/client.go

package minio

import (
    "context"
    "time"

    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
    client     *minio.Client
    bucketName string
}

func NewClient(endpoint, accessKey, secretKey, bucketName string, useSSL bool) (*Client, error) {
    mc, err := minio.New(endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure: useSSL,
    })
    if err != nil {
        return nil, err
    }
    return &Client{client: mc, bucketName: bucketName}, nil
}

// PresignGetObject generates a presigned download URL for object
func (c *Client) PresignGetObject(ctx context.Context, objectPath string, expiry time.Duration) (string, error) {
    u, err := c.client.PresignedGetObject(ctx, c.bucketName, objectPath, expiry, nil)
    if err != nil {
        return "", err
    }
    return u.String(), nil
}
```

### Router addition:

```go
// services/finding-service/internal/adapter/http/router.go
mux.HandleFunc("GET /v2/reports/{id}/download", h.Report.DownloadReport)
// v1 alias:
mux.HandleFunc("GET /reports/{id}/download",    h.Report.DownloadReport)
```

---

## Verification

```bash
cd services/finding-service
go build ./...

# Download completed report
curl -L -H "X-User-ID: $USER_ID" \
  "http://localhost:8085/v2/reports/$REPORT_ID/download" \
  -o report.pdf

# Should save PDF file
ls -la report.pdf
```

---

## Checklist

- [x] `DownloadReport` kiểm tra report status == "completed" trước (409 nếu không)
- [x] Ownership check: chỉ report owner hoặc admin mới download được
- [x] Generate presigned URL với TTL 5 phút
- [x] 302 redirect đến presigned URL (không stream file qua service)
- [x] MinIO client dùng `github.com/minio/minio-go/v7`
- [x] `go build ./...` thành công

---

# TASK-BE-011 — notification-service: Notifications REST API

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-011 |
| **Service** | `services/notification-service` |
| **Solution Ref** | [SOL-UI-002 §2.6](../solutions/SOL-UI-002-dashboard-bff-sse.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | — |
| **Estimated** | 3h |

---

## Context

notification-service có `in_app_alerts` table nhưng chưa expose HTTP API cho UI. Frontend cần notification bell với unread count, list, và mark-read.

---

## Target Files

| Action | File Path |
|--------|-----------|
| MODIFY | `services/notification-service/internal/adapter/http/alerts_handler.go` |
| MODIFY | `services/notification-service/internal/adapter/http/router.go` |

---

## Implementation

```go
// services/notification-service/internal/adapter/http/alerts_handler.go

type AlertsHandler struct {
    alertRepo AlertRepository
}

// GET /v2/notification-alerts → alias /notifications
func (h *AlertsHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
    userID  := r.Header.Get("X-User-ID")
    isRead  := r.URL.Query().Get("is_read") // "true" | "false" | ""
    page, ps := parsePagination(r)

    alerts, total, unread, err := h.alertRepo.ListByUser(r.Context(), userID, isRead, page, ps)
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }

    respondJSON(w, 200, map[string]interface{}{
        "notifications": mapAlerts(alerts),
        "total":         total,
        "unread_count":  unread,
        "page":          page,
        "page_size":     ps,
    })
}

// PATCH /notifications/{id}/read
func (h *AlertsHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
    alertID := r.PathValue("id")
    userID  := r.Header.Get("X-User-ID")

    if err := h.alertRepo.MarkRead(r.Context(), alertID, userID); err != nil {
        respondError(w, 404, "NOT_FOUND", "Notification not found")
        return
    }
    respondJSON(w, 200, map[string]interface{}{"id": alertID, "is_read": true})
}

// POST /notifications/mark-all-read
func (h *AlertsHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    count, err := h.alertRepo.MarkAllRead(r.Context(), userID)
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }
    respondJSON(w, 200, map[string]interface{}{"marked_count": count})
}

// GET /notifications/unread-count
func (h *AlertsHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    count, err := h.alertRepo.CountUnread(r.Context(), userID)
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }
    respondJSON(w, 200, map[string]interface{}{"unread_count": count})
}

// NotificationDTO — response shape
type NotificationDTO struct {
    ID         string  `json:"id"`
    Type       string  `json:"type"`       // "kev.new" | "finding.sla.breached" | ...
    Title      string  `json:"title"`
    Message    string  `json:"message"`
    Severity   string  `json:"severity"`   // "Critical" | "High" | "Info"
    IsRead     bool    `json:"is_read"`
    EntityType string  `json:"entity_type"` // "cve" | "finding" | "scan"
    EntityID   string  `json:"entity_id"`
    CreatedAt  string  `json:"created_at"`
    ReadAt     *string `json:"read_at"`
}

func mapAlerts(alerts []*Alert) []NotificationDTO {
    dtos := make([]NotificationDTO, len(alerts))
    for i, a := range alerts {
        var readAt *string
        if a.ReadAt != nil {
            s := a.ReadAt.Format(time.RFC3339)
            readAt = &s
        }
        dtos[i] = NotificationDTO{
            ID:         a.ID.String(),
            Type:       a.Type,
            Title:      a.Title,
            Message:    a.Message,
            Severity:   a.Severity,
            IsRead:     a.IsRead,
            EntityType: a.EntityType,
            EntityID:   a.EntityID,
            CreatedAt:  a.CreatedAt.Format(time.RFC3339),
            ReadAt:     readAt,
        }
    }
    return dtos
}
```

### SQL for AlertRepository:

```sql
-- ListByUser
SELECT id, type, title, message, severity, is_read, entity_type, entity_id, created_at, read_at
FROM in_app_alerts
WHERE user_id = $1
  AND ($2::text IS NULL OR (is_read = ($2 = 'true')))
ORDER BY created_at DESC
LIMIT $3 OFFSET ($4-1)*$3;

-- CountUnread (for unread_count in list AND unread-count endpoint)
SELECT COUNT(*) FROM in_app_alerts WHERE user_id = $1 AND is_read = false;

-- MarkRead
UPDATE in_app_alerts SET is_read = true, read_at = NOW()
WHERE id = $1 AND user_id = $2;

-- MarkAllRead
UPDATE in_app_alerts SET is_read = true, read_at = NOW()
WHERE user_id = $1 AND is_read = false
RETURNING id;
-- count = RowsAffected
```

### Router:

```go
// services/notification-service/internal/adapter/http/router.go
mux.HandleFunc("GET  /notifications",              h.Alerts.ListNotifications)
mux.HandleFunc("PATCH /notifications/{id}/read",   h.Alerts.MarkRead)
mux.HandleFunc("POST  /notifications/mark-all-read", h.Alerts.MarkAllRead)
mux.HandleFunc("GET   /notifications/unread-count", h.Alerts.UnreadCount)
```

---

## Verification

```bash
cd services/notification-service
go build ./...

curl -H "X-User-ID: $USER_ID" http://localhost:8087/notifications | \
  jq '{total, unread_count, count: (.notifications | length)}'

curl -H "X-User-ID: $USER_ID" \
  http://localhost:8087/notifications/unread-count | jq .unread_count
# Expected: number >= 0

curl -X POST -H "X-User-ID: $USER_ID" \
  http://localhost:8087/notifications/mark-all-read | jq .marked_count
```

---

## Checklist

- [x] `ListNotifications` trả về `notifications[]`, `total`, `unread_count`, pagination
- [x] Filter `is_read=true/false` hoạt động đúng
- [x] `MarkRead` verify user ownership (user_id = X-User-ID)
- [x] `MarkAllRead` trả về `marked_count` = số records updated
- [x] `UnreadCount` trả về `{"unread_count": N}`
- [x] SQL queries parameterized
- [x] `go build ./...` thành công
