# SOL-H2-C — notification-service: Xóa Jira stubs + Fix AlertsHandler wire

> Bugs: BUG-H2-005, BUG-H2-006
> Service: `notification-service`

---

## Fix 1: Xóa 6 package-level Jira stubs (BUG-H2-005)

Xóa đoạn L243-251 trong `integration_handler.go`:

```go
// XÓA TOÀN BỘ ĐOẠN NÀY
// Package-level functions for backward compatibility ...
func ListJiraIntegrationsHandler(w http.ResponseWriter, r *http.Request)  { w.WriteHeader(http.StatusNotImplemented) }
func CreateJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotImplemented) }
func UpdateJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotImplemented) }
func DeleteJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotImplemented) }
func SyncJiraHandler(w http.ResponseWriter, r *http.Request)              { w.WriteHeader(http.StatusNotImplemented) }
func JiraWebhookHandler(w http.ResponseWriter, r *http.Request)           { w.WriteHeader(http.StatusNotImplemented) }
```

`IntegrationHandler` methods đã đủ, không cần backward compat stubs.

---

## Fix 2: Kiểm tra AlertsHandler wiring (BUG-H2-006)

Xem `notification-service/embedded.go` để xác định `AlertsHandler` được create ở đâu.
Đảm bảo `AlertsHandler` luôn được khởi tạo khi `pool != nil`.

Nếu `embedded.go` không truyền `AlertsHandler` vào `SetupRouter`, cần wire:

```go
// embedded.go — nếu ah không được tạo
alertsRepo := postgres.NewAlertsRepo(pool)
alertsUC := alerts.NewUseCase(alertsRepo)
ah := httpdelivery.NewAlertsHandler(alertsUC, logger)

// Truyền vào SetupRouter
router := httpdelivery.SetupRouter(wh, sh, ih, ah, sse, rh, dh)
```

---

## Files cần modify

| File | Thay đổi |
|------|----------|
| `integration_handler.go` | Xóa 6 package-level stubs L243-251 |
| `embedded.go` | Đảm bảo `AlertsHandler` được wire khi pool != nil |
