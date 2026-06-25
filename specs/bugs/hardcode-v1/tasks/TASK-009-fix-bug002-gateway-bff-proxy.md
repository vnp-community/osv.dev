# TASK-009 — Fix BUG-002: Gateway BFF Proxy ProductTypes & AdminRoles

> **Bug**: BUG-002  
> **Priority**: 🟡 Medium — Không thể extend roles/types mà không deploy gateway  
> **Depends on**: không có dependency  
> **Solution ref**: [SOL-GROUP-B](../solutions/SOL-GROUP-B-gateway-bff-data.md#bug-002)  
> **Trạng thái**: ✅ DONE — 2026-06-22  
> **Ghi chú**: `AdminSettings` default thay `"http://localhost:11434"` → đọc từ `AI_BASE_URL`/`OLLAMA_BASE_URL` env. `"llama3"` → `AI_MODEL` env. Thêm `_meta.is_default`. `ProductTypes` và `AdminRoles` đã proxy qua `h.proxyRequest` từ trước — không thay đổi thêm (đúng spec). Build pass.

## Files Cần Đọc Trước

```
services/gateway-service/internal/bff/handlers/handler_ui_api.go     (lines 600-625, 800-825)
services/product-service/internal/delivery/http/handlers.go            (xem route setup)
services/product-service/internal/delivery/http/                       (xem toàn bộ)
services/identity-service/adapter/handler/http/admin_handler.go        (lines 255-285)
services/gateway-service/internal/bff/handlers/                        (xem UIAPIHandlerConfig)
```

## Files Sẽ Bị Sửa / Tạo

```
services/gateway-service/internal/bff/handlers/handler_ui_api.go  [MODIFY]
services/product-service/internal/delivery/http/handlers.go        [MODIFY — thêm endpoint]
services/identity-service/adapter/handler/http/admin_handler.go    [MODIFY — thêm endpoint]
services/identity-service/adapter/handler/http/routes.go           [MODIFY — đăng ký route, nếu có]
```

## Thay Đổi Chi Tiết

### Bước 1: Sửa `handler_ui_api.go` — Proxy thay vì hardcode

**Đọc file** để xác định:
- Tên chính xác của `UIAPIHandler` struct
- Field nào đang lưu product service URL và identity service URL
- Tên hàm proxy helper (có thể là `h.proxyRequest`, `h.forward`, hay tương đương)

```bash
grep -n "ProductTypes\|AdminRoles\|proxyRequest\|forward\|productService\|identityService" \
    services/gateway-service/internal/bff/handlers/handler_ui_api.go | head -30
```

**Sửa `ProductTypes`** (khoảng line 608):
```go
// [FIX] Thay hardcoded JSON response bằng proxy đến product-service
func (h *UIAPIHandler) ProductTypes(w http.ResponseWriter, r *http.Request) {
    // h.proxyRequest — đọc tên helper thực tế từ file
    target := h.productServiceURL + "/api/v1/products/types"
    h.proxyRequest(w, r, target)  // dùng helper method của UIAPIHandler
}
```

**Sửa `AdminRoles`** (khoảng line 809):
```go
// [FIX] Thay hardcoded JSON response bằng proxy đến identity-service
func (h *UIAPIHandler) AdminRoles(w http.ResponseWriter, r *http.Request) {
    target := h.identityServiceURL + "/api/v1/admin/roles"
    h.proxyRequest(w, r, target)
}
```

> **Nếu không có `proxyRequest` helper**: Dùng `http.Client` trực tiếp hoặc tìm helper
> tương đương trong cùng file/package.

**Kiểm tra config struct** có field cho product/identity URL chưa:
```bash
grep -n "productServiceURL\|ProductServiceURL\|identityServiceURL\|IdentityServiceURL" \
    services/gateway-service/internal/bff/handlers/handler_ui_api.go
```

Nếu chưa có, thêm vào `UIAPIHandler` struct và `UIAPIHandlerConfig`, cũng như constructor.

### Bước 2: Thêm `GET /api/v1/products/types` vào `product-service`

Đọc `product-service/internal/delivery/http/handlers.go` để hiểu pattern đăng ký route.

**Thêm handler method**:
```go
// GetProductTypes trả về danh sách product types được hỗ trợ.
// Source of truth cho product type enum — gateway proxy về đây thay vì hardcode.
func (h *ProductHandler) GetProductTypes(w http.ResponseWriter, r *http.Request) {
    types := []map[string]string{
        {"value": "web_app",        "label": "Web Application"},
        {"value": "api",            "label": "API"},
        {"value": "infrastructure", "label": "Infrastructure"},
        {"value": "mobile",         "label": "Mobile"},
    }
    // Dùng respondJSON helper theo pattern của service
    respondJSON(w, http.StatusOK, map[string]interface{}{"types": types})
}
```

**Đăng ký route**:
```go
// Trong route setup (tìm nơi các routes được đăng ký):
mux.HandleFunc("GET /api/v1/products/types", productHandler.GetProductTypes)
// hoặc theo pattern thực tế của codebase
```

### Bước 3: Thêm `GET /api/v1/admin/roles` vào `identity-service`

Đọc `admin_handler.go` — tại lines 262-282 đã có logic roles. Chỉ cần expose ra endpoint.

**Thêm handler method** (hoặc nếu đã có, kiểm tra route đã được đăng ký chưa):
```go
// GetRoles trả về role definitions — single source of truth.
// Gateway và frontend đều proxy về đây thay vì hardcode.
func (h *AdminHandler) GetRoles(w http.ResponseWriter, r *http.Request) {
    roles := []map[string]interface{}{
        {"value": "admin",    "label": "Administrator",    "description": "Full system access"},
        {"value": "user",     "label": "Security Analyst", "description": "Can scan and manage findings"},
        {"value": "readonly", "label": "Read-Only",        "description": "View-only access"},
        {"value": "agent",    "label": "Agent",            "description": "Automated scanning agent"},
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{"roles": roles})
}
```

**Đăng ký route** trong identity-service router (đọc file route setup):
```go
mux.HandleFunc("GET /api/v1/admin/roles", adminHandler.GetRoles)
```

## Quy Tắc Thực Thi

1. **Đọc helper method** của UIAPIHandler trước — mỗi service có pattern proxy khác nhau
2. **Không xóa old function body** nếu chưa test được — có thể comment out trước
3. **Kiểm tra auth** của endpoint mới: `/api/v1/products/types` và `/api/v1/admin/roles`
   phải được bảo vệ tương đương với các endpoints hiện tại

## Verification

```bash
# Build tất cả services
go build ./services/gateway-service/...
go build ./services/product-service/...
go build ./services/identity-service/...

# Test: product-service có endpoint mới
curl http://localhost:8089/api/v1/products/types
# → {"types": [{"value":"web_app","label":"Web Application"}, ...]}

# Test: identity-service có endpoint mới
curl -H "Authorization: Bearer $TOKEN" http://localhost:8081/api/v1/admin/roles
# → {"roles": [...]}

# Test: gateway proxy đúng (không còn hardcoded)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/products/types
# → response từ product-service (verify bằng cách thay đổi product-service response)
```

## Acceptance Criteria

- [ ] `ProductTypes` handler không còn chứa hardcoded `[]map[string]string{...}`
- [ ] `AdminRoles` handler không còn chứa hardcoded `[]map[string]interface{}{...}`
- [ ] `product-service` có `GET /api/v1/products/types` endpoint
- [ ] `identity-service` có `GET /api/v1/admin/roles` endpoint  
- [ ] Cả 3 services build thành công
- [ ] Gateway proxy đến đúng upstream (test với curl)
