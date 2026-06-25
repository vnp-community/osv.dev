# BUG-002 — Gateway: Hardcoded ProductTypes và AdminRoles responses

## Metadata
- **ID**: BUG-002
- **Service**: `gateway-service`
- **File**: [`internal/bff/handlers/handler_ui_api.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/bff/handlers/handler_ui_api.go)
- **Lines**: 608–618 (ProductTypes), 809–819 (AdminRoles)
- **Severity**: Medium
- **Category**: Hardcode / Data
- **Status**: Open

## Mô tả

Hai endpoints bị hardcode response thay vì lấy từ service:

### 1. `ProductTypes` — GET /api/v1/products/types

```go
// Line 608-618
// ProductTypes handles GET /api/v1/products/types — returns hardcoded enum for now.
func (h *UIAPIHandler) ProductTypes(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "types": []map[string]string{
            {"value": "web_app", "label": "Web Application"},
            {"value": "api", "label": "API"},
            {"value": "infrastructure", "label": "Infrastructure"},
            {"value": "mobile", "label": "Mobile"},
        },
    })
}
```

### 2. `AdminRoles` — GET /api/v1/admin/roles

```go
// Line 809-819
// AdminRoles returns the hardcoded role definitions (CR-UI-010 §2.7).
func (h *UIAPIHandler) AdminRoles(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "roles": []map[string]interface{}{
            {"value": "admin",    "label": "Administrator",   "description": "Full system access"},
            {"value": "user",     "label": "Security Analyst","description": "Can scan and manage findings"},
            {"value": "readonly", "label": "Read-Only",       "description": "View-only access"},
            {"value": "agent",    "label": "Agent",           "description": "Automated scanning agent"},
        },
    })
}
```

## Tác động

1. **ProductTypes**: Khi admin tạo thêm loại product mới (e.g., `iot`, `desktop`),
   UI sẽ không hiển thị loại mới vì response bị hardcode.
2. **AdminRoles**: Các role được định nghĩa tại nhiều nơi (identity-service, gateway,
   frontend constants) → thiếu single source of truth → dễ lệch nhau khi thêm role mới.
3. Cả hai endpoint đều bỏ qua auth context — không phân biệt data theo tenant.

## Fix Proposal

### ProductTypes → proxy đến product-service

```go
func (h *UIAPIHandler) ProductTypes(w http.ResponseWriter, r *http.Request) {
    h.proxyRequest(w, r, h.productServiceURL+"/api/v1/products/types")
}
```

Trong `product-service`, thêm endpoint `GET /api/v1/products/types` trả về
enum từ database hoặc từ config (không hardcode trong gateway).

### AdminRoles → proxy đến identity-service

```go
func (h *UIAPIHandler) AdminRoles(w http.ResponseWriter, r *http.Request) {
    h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/roles")
}
```

`identity-service` đã có `admin_handler.go` với logic roles tại L262–L282.
Endpoint `/api/v1/admin/roles` nên được thêm vào identity-service để làm
source of truth duy nhất.

## References

- [handler_ui_api.go L608-618](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/bff/handlers/handler_ui_api.go#L608-L618)
- [handler_ui_api.go L809-819](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/bff/handlers/handler_ui_api.go#L809-L819)
- [identity-service admin_handler.go L262-282](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service/adapter/handler/http/admin_handler.go#L262-L282)
