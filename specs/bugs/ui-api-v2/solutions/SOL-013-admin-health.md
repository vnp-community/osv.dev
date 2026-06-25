# SOL-013 — Admin Health: Fix Response Schema (P1)

**Bug**: [BUG-015](../BUG-015-admin-health.md)  
**Service**: `apps/osv` — `HealthBFF` (in-gateway)  
**Endpoint**: `GET /api/v1/admin/health`  
**Lỗi frontend**: `TypeError: Cannot read properties of undefined (reading 'includes')` — `service.status` là `undefined`

**Status**: `✅ Implemented` — via [TASK-006](../../tasks/TASK-006-*.md)

---

## Root Cause Analysis

### Code hiện tại — HealthBFF đã có

File [`apps/osv/internal/gateway/bff/health.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/health.go) **đã implement** `HandleAdminHealth`.

**Vấn đề**: Response cũ trả `services` là **array** (`[]ServiceHealth`). Frontend đọc `response.services` như một object key-value (map), gọi `.includes()` trên `service.status` của từng service.

### Response cũ (sai schema)

```json
{
  "overall_status": "healthy",
  "services": [
    { "name": "identity-service", "status": "healthy", "response_time_ms": 12 }
  ],
  "infrastructure": [
    { "name": "postgres", "status": "healthy" }
  ]
}
```

### Response mới (đúng schema — map keyed by name)

Code hiện tại trong [`health.go:134`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/health.go#L134) **đã sửa** để trả map:

```go
// FIX: Build services as MAP (keyed by name) per spec
servicesMap := make(map[string]map[string]interface{}, len(serviceResults)+len(infraResults))
for _, s := range serviceResults {
    statusStr := "up"
    if s.Status != "healthy" {
        statusStr = "down"
    }
    servicesMap[s.Name] = map[string]interface{}{
        "status":       statusStr,
        "latency_ms":   s.ResponseTimeMs,
        "version":      s.Version,
        "last_checked": s.LastCheckedAt,
    }
}
```

**Response mới:**
```json
{
  "overall_status": "healthy",
  "status": "healthy",
  "services": {
    "identity-service": { "status": "up", "latency_ms": 12 },
    "postgres": { "status": "up", "latency_ms": 1 },
    "redis": { "status": "up", "latency_ms": 0 }
  },
  "checked_at": "2026-06-20T00:00:00Z"
}
```

---

## Hành động cần làm

### Kiểm tra deploy hiện tại

Gateway đã có code fix (xem `health.go` dòng 134). **Vấn đề chính** là gateway có thể chưa được deploy lại sau khi fix.

```bash
# Kiểm tra response thực tế
curl -H "Authorization: Bearer <admin_token>" \
  "https://c12.openledger.vn/api/v1/admin/health" | jq '.services | type'
# Nếu "array" → cần redeploy
# Nếu "object" → fix đã apply, vấn đề khác
```

### Kiểm tra guard cho NATS/DB null

Trong [`router.go:46`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L46):

```go
if natsConn != nil && db != nil {
    healthBFF = bff.NewHealthBFF(redisClient, natsConn, ...)
    settingsBFF = bff.NewSettingsBFF(settingsRepo)
}
```

**Nguy cơ**: Nếu NATS hoặc DB chưa kết nối được → `healthBFF` là `nil` → route không được đăng ký → 404. Cần thêm fallback:

```go
// FIX: Đăng ký route kể cả khi NATS/DB nil — trả error message rõ ràng
healthBFF = bff.NewHealthBFF(redisClient, natsConn, pgPingFunc)
settingsBFF = bff.NewSettingsBFF(settingsRepo)

// Luôn đăng ký route (không check nil)
mux.Handle("GET /api/v1/admin/health", adminOnly(http.HandlerFunc(healthBFF.HandleAdminHealth)))
```

Trong `HealthBFF.HandleAdminHealth`, xử lý nil NATS gracefully:

```go
// health.go — trong checkNATS
func (bff *HealthBFF) checkNATS(ctx context.Context) InfraHealth {
    start := time.Now()
    status := "healthy"
    if bff.natsConn == nil || !bff.natsConn.IsConnected() {
        status = "down"
    }
    return InfraHealth{
        Name:           "nats",
        Status:         status,
        ResponseTimeMs: time.Since(start).Milliseconds(),
    }
}
```

---

## Files cần kiểm tra/sửa

| File | Trạng thái | Thay đổi cần làm |
|------|------------|------------------|
| [`bff/health.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/health.go) | ✅ Đã có fix schema (map) | Thêm nil guard cho natsConn |
| [`gateway/router.go:46`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L46) | ⚠️ Guard nil block | Bỏ condition, always register route |

---

## Verification

```bash
# Response phải là object (map), không phải array
curl -H "Authorization: Bearer <admin_token>" \
  "https://c12.openledger.vn/api/v1/admin/health" | jq '
{
  services_type: (.services | type),    
  has_postgres: (.services | has("postgres")),
  postgres_status: .services.postgres.status
}'
# Expected:
# {
#   "services_type": "object",
#   "has_postgres": true,
#   "postgres_status": "up"
# }
```
