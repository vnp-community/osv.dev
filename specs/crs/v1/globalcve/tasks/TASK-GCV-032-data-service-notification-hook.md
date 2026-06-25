# TASK-GCV-032 — data-service Post-Sync Notification Hook

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-032 |
| **Service** | `data-service` |
| **CR** | CR-GCV-006 |
| **Phase** | 3 — Notifications |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-GCV-031 |

## Context

Sau khi CVE sync hoàn thành, `data-service` cần notify `notification-service` về các CVEs mới/cập nhật có severity CRITICAL/HIGH hoặc EPSS cao. Implement via gRPC hoặc HTTP call với retry logic. Thêm `NotificationHook` vào sync pipeline.

## Reference

- Solution: [SOL-GCV-006](../solutions/SOL-GCV-006-notification-webhook.md) §4.1

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/notification/http_hook.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/usecase/sync/cve_sync.go
        (thêm notification hook call sau sync)
```

## Implementation Spec

### notification/http_hook.go

```go
// Package notification — HTTP client to call notification-service after CVE sync.
package notification

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "time"

    "github.com/rs/zerolog"
    entity "github.com/osv/data-service/internal/domain/entity"
)

// Hook calls notification-service to dispatch CVE alerts.
// Implements fire-and-forget pattern — non-blocking, non-fatal.
type Hook struct {
    baseURL string // notification-service HTTP base URL
    client  *http.Client
    logger  zerolog.Logger
}

// NewHook creates a notification hook.
// notificationServiceURL: e.g. "http://notification-service:8084"
func NewHook(notificationServiceURL string, log zerolog.Logger) *Hook {
    return &Hook{
        baseURL: notificationServiceURL,
        client:  &http.Client{Timeout: 10 * time.Second},
        logger:  log.With().Str("component", "notification-hook").Logger(),
    }
}

// IsEnabled returns true if NOTIFICATION_SERVICE_URL env is set.
func IsEnabled() bool {
    return os.Getenv("NOTIFICATION_SERVICE_URL") != ""
}

// CVENotification is the payload sent to notification-service.
type CVENotification struct {
    CVEID       string   `json:"cve_id"`
    Severity    string   `json:"severity"`
    EPSS        float64  `json:"epss"`
    Vendors     []string `json:"vendors"`
    Products    []string `json:"products"`
    IsKEV       bool     `json:"is_kev"`
    IsExploit   bool     `json:"is_exploit"`
    Description string   `json:"description"`
}

// NotifyBatch sends a batch of CVE notifications to notification-service.
// Non-blocking: runs in goroutine. Non-fatal: errors are only logged.
func (h *Hook) NotifyBatch(cves []*entity.CVE) {
    if len(cves) == 0 { return }

    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        notifications := make([]*CVENotification, 0, len(cves))
        for _, cve := range cves {
            // Only notify for high-priority CVEs
            if !isHighPriority(cve) { continue }
            notifications = append(notifications, &CVENotification{
                CVEID:       cve.ID,
                Severity:    cve.Severity,
                EPSS:        cve.EPSS,
                Vendors:     cve.Vendors,
                Products:    cve.Products,
                IsKEV:       cve.IsKEV,
                IsExploit:   cve.IsExploit,
                Description: cve.Description,
            })
        }
        if len(notifications) == 0 { return }

        if err := h.sendNotifications(ctx, notifications); err != nil {
            h.logger.Warn().Err(err).Int("count", len(notifications)).
                Msg("notification hook failed (non-fatal)")
        } else {
            h.logger.Info().Int("count", len(notifications)).
                Msg("notifications dispatched")
        }
    }()
}

func (h *Hook) sendNotifications(ctx context.Context, notifications []*CVENotification) error {
    body, err := json.Marshal(map[string]interface{}{
        "events": notifications,
    })
    if err != nil { return err }

    req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
        h.baseURL+"/internal/events/cve", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Internal-Service", "data-service")

    resp, err := h.client.Do(req)
    if err != nil { return fmt.Errorf("notification hook: request: %w", err) }
    defer resp.Body.Close()

    if resp.StatusCode >= 300 {
        return fmt.Errorf("notification hook: status %d", resp.StatusCode)
    }
    return nil
}

// isHighPriority returns true if CVE warrants notification.
func isHighPriority(cve *entity.CVE) bool {
    return cve.Severity == "CRITICAL" ||
        cve.Severity == "HIGH" ||
        cve.EPSS >= 0.7 ||
        cve.IsKEV ||
        cve.IsExploit
}
```

### cve_sync.go — ADD hook call after sync

Tìm CVE sync use case, thêm notification hook call:

```go
// Trong UseCase struct:
type UseCase struct {
    // existing fields...
    notifHook *notification.Hook // NEW: nil if not configured
}

// Trong Execute/Sync method, thêm ở cuối:

// Notify notification-service about high-priority new/updated CVEs (non-blocking)
if uc.notifHook != nil {
    uc.notifHook.NotifyBatch(upsertedCVEs)
}
```

### notification-service — ADD /internal/events/cve endpoint

Trong `notification-service`, thêm internal endpoint:

```go
// router.go (trong notification-service):
r.Post("/internal/events/cve", internalHandler.ReceiveCVEEvents)

// handler:
func (h *InternalHandler) ReceiveCVEEvents(w http.ResponseWriter, r *http.Request) {
    // Verify X-Internal-Service header
    if r.Header.Get("X-Internal-Service") != "data-service" {
        respondError(w, 403, "internal only")
        return
    }

    var payload struct {
        Events []usecase.CVENotification `json:"events"`
    }
    json.NewDecoder(r.Body).Decode(&payload)

    for _, ev := range payload.Events {
        h.dispatcher.Dispatch(r.Context(), usecase.CVEEvent{
            CVEID:       ev.CVEID,
            Severity:    ev.Severity,
            EPSS:        ev.EPSS,
            Vendors:     ev.Vendors,
            Products:    ev.Products,
            IsKEV:       ev.IsKEV,
            IsExploit:   ev.IsExploit,
            Description: ev.Description,
        })
    }
    respondJSON(w, 200, map[string]string{"status": "dispatched"})
}
```

## Acceptance Criteria

- [x] `NOTIFICATION_SERVICE_URL` not set → hook disabled, sync proceeds normally
- [x] `NOTIFICATION_SERVICE_URL` set → hook fires after sync with high-priority CVEs
- [x] Hook is non-blocking (goroutine) — sync does not wait for notification-service
- [x] Hook failure → logged warning, sync result unaffected (non-fatal)
- [x] Only CRITICAL/HIGH severity OR EPSS >= 0.7 OR IsKEV OR IsExploit CVEs are notified
- [x] MEDIUM/LOW CVEs without EPSS → not included in notification batch
- [x] `notification-service /internal/events/cve` without `X-Internal-Service: data-service` header → 403
- [x] `go build ./...` pass cho cả `data-service` và `notification-service`


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Verified directly from codebase.

---

# TASK-GCV-033 — CVE Export (CSV + JSON) Endpoint

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-033 |
| **Service** | `search-service` |
| **CR** | CR-GCV-010 |
| **Phase** | 4 — Export & UI Support |
| **Priority** | 🟢 Low |
| **Prerequisites** | — |

## Context

Thêm `GET /api/v2/cves/export` với `?format=csv` hoặc `?format=json`. Áp dụng cùng filters như GET /cves nhưng stream kết quả. Giới hạn 10,000 records per export, require authentication.

## Reference

- Solution: [SOL-GCV-010](../solutions/SOL-GCV-010-export-frontend-support.md) §2.2

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/delivery/http/search_handler.go
        (ADD ExportCVEs handler)
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/domain/repository/
        (ADD StreamAll method to CVERepository)
```

## Implementation Spec

```go
const maxExportRows = 10000

// GET /api/v2/cves/export?format=csv&severity=CRITICAL&min_epss=0.8
func (h *Handler) ExportCVEs(w http.ResponseWriter, r *http.Request) {
    format := r.URL.Query().Get("format")
    if format == "" { format = "json" }
    if format != "csv" && format != "json" {
        respondError(w, 400, "format must be 'csv' or 'json'")
        return
    }

    // Parse same filters as GET /api/v2/cves
    req := parseSearchRequest(r)
    req.Limit = maxExportRows

    cves, _, err := h.cveRepo.Search(r.Context(), req)
    if err != nil {
        respondError(w, 500, "export failed")
        return
    }

    switch format {
    case "csv":
        w.Header().Set("Content-Type", "text/csv")
        w.Header().Set("Content-Disposition", `attachment; filename="cves-export.csv"`)
        enc := csv.NewWriter(w)
        enc.Write([]string{"ID","Severity","CVSS3","EPSS","Published","Description","IsKEV","Source"})
        for _, cve := range cves {
            enc.Write([]string{
                cve.ID, cve.Severity,
                fmt.Sprintf("%.1f", cve.CVSS3),
                fmt.Sprintf("%.6f", cve.EPSS),
                cve.Published.Format("2006-01-02"),
                cve.Description, boolStr(cve.IsKEV), cve.Source,
            })
        }
        enc.Flush()

    case "json":
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("Content-Disposition", `attachment; filename="cves-export.json"`)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "data":       cves,
            "count":      len(cves),
            "exported_at": time.Now().UTC().Format(time.RFC3339),
        })
    }
}
```

## Acceptance Criteria

- [x] `GET /api/v2/cves/export?format=csv` → CSV file downloaded
- [x] `GET /api/v2/cves/export?format=json` → JSON file downloaded
- [x] CSV có header row: `ID,Severity,CVSS3,EPSS,Published,Description,IsKEV,Source`
- [x] `?format=xml` → 400 error
- [x] Max 10,000 records per export
- [x] Same filters (severity, min_epss, vendor, cwe) as GET /cves apply
- [x] `go build ./...` pass
