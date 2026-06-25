# BUG-H2-005 — notification-service: 6 package-level Jira stubs trả 501

## Metadata
- **ID**: BUG-H2-005
- **Service**: `notification-service`
- **File**: [`integration_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/delivery/http/integration_handler.go)
- **Lines**: 243–251
- **Severity**: 🟠 Medium
- **Category**: Orphan Stub
- **Status**: ✅ Fixed

## Mô tả

6 package-level functions (non-method) tại cuối `integration_handler.go` trả về `501 Not Implemented`. Chúng được comment là "Package-level functions for backward compatibility" nhưng thực ra là dead code nguy hiểm — nếu vô tình được mount lên router sẽ trả 501.

```go
// integration_handler.go L245-250 — BUG
func ListJiraIntegrationsHandler(w http.ResponseWriter, r *http.Request)  { w.WriteHeader(http.StatusNotImplemented) }
func CreateJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotImplemented) }
func UpdateJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotImplemented) }
func DeleteJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotImplemented) }
func SyncJiraHandler(w http.ResponseWriter, r *http.Request)              { w.WriteHeader(http.StatusNotImplemented) }
func JiraWebhookHandler(w http.ResponseWriter, r *http.Request)           { w.WriteHeader(http.StatusNotImplemented) }
```

## Root Cause

`IntegrationHandler` struct (methods) đã implement đầy đủ tất cả chức năng. 6 hàm package-level này là legacy stubs từ thời refactor chưa được dọn dẹp.

## Tác động

- Nếu có bất kỳ code nào dùng `http.ListJiraIntegrationsHandler` thay vì `handler.ListJiraIntegrationsHandler`, route sẽ trả 501
- Code dead gây confusion khi review

## Fix

Xóa 6 package-level stubs. Chỉ giữ method implementations trên `*IntegrationHandler`.

## References
- [integration_handler.go:L243-251](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/delivery/http/integration_handler.go#L243-L251)
