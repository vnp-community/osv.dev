# TASK-006 — apps/osv: Remove nil guard — Luôn đăng ký Admin Health/Settings routes

**Bug**: [BUG-015](../BUG-015-admin-health.md), [BUG-016](../BUG-016-admin-settings.md)  
**Solution**: [SOL-013](../solutions/SOL-013-admin-health.md), [SOL-014](../solutions/SOL-014-admin-settings.md)  
**Priority**: 🟠 P1  
**Effort**: ~10 phút  
**Status**: `[x] DONE`

---

## Mô tả

Trong `apps/osv/internal/gateway/router.go`, routes `GET /api/v1/admin/health` và `GET /api/v1/admin/settings` chỉ được đăng ký khi `healthBFF != nil && settingsBFF != nil`. Nếu NATS hoặc DB chưa kết nối tại thời điểm startup → `404` thay vì response lỗi có nghĩa.

---

## File cần sửa

**File**: [`apps/osv/internal/gateway/router.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go)

---

## Thay đổi — router.go (dòng ~44–260)

**Tìm** đoạn code (dòng ~44):

```go
    // Initialize Admin BFFs
    var healthBFF *bff.HealthBFF
    var settingsBFF *bff.SettingsBFF
    if natsConn != nil && db != nil {
        healthBFF = bff.NewHealthBFF(redisClient, natsConn, func(ctx context.Context) error { return db.Ping(ctx) })
        settingsRepo := postgres.NewSettingsRepo(db)
        settingsBFF = bff.NewSettingsBFF(settingsRepo)
    }
```

**Thay bằng**:

```go
    // Initialize Admin BFFs — luôn khởi tạo, handle nil dependencies internally
    pgPing := func(ctx context.Context) error {
        if db != nil {
            return db.Ping(ctx)
        }
        return fmt.Errorf("database not connected")
    }
    healthBFF = bff.NewHealthBFF(redisClient, natsConn, pgPing)

    var settingsBFF *bff.SettingsBFF
    if db != nil {
        settingsRepo := postgres.NewSettingsRepo(db)
        settingsBFF = bff.NewSettingsBFF(settingsRepo)
    } else {
        // Fallback: trả default settings khi DB chưa sẵn sàng
        settingsBFF = bff.NewSettingsBFF(nil)
    }
```

**Tìm** đoạn đăng ký route (dòng ~256):

```go
    if healthBFF != nil && settingsBFF != nil {
        mux.Handle("GET /api/v1/admin/health",    adminOnly(http.HandlerFunc(healthBFF.HandleAdminHealth)))
        mux.Handle("GET /api/v1/admin/settings",  adminOnly(http.HandlerFunc(settingsBFF.GetSettings)))
        mux.Handle("PATCH /api/v1/admin/settings", adminOnly(http.HandlerFunc(settingsBFF.UpdateSettings)))
    }
```

**Thay bằng** (bỏ if guard):

```go
    mux.Handle("GET /api/v1/admin/health",    adminOnly(http.HandlerFunc(healthBFF.HandleAdminHealth)))
    mux.Handle("GET /api/v1/admin/settings",  adminOnly(http.HandlerFunc(settingsBFF.GetSettings)))
    mux.Handle("PATCH /api/v1/admin/settings", adminOnly(http.HandlerFunc(settingsBFF.UpdateSettings)))
```

---

## Thay đổi — bff/health.go: Nil guard cho natsConn

**File**: [`apps/osv/internal/gateway/bff/health.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/health.go)

**Tìm** hàm `checkNATS` (dòng ~215):

```go
func (bff *HealthBFF) checkNATS(ctx context.Context) InfraHealth {
    start := time.Now()
    status := "healthy"
    if !bff.natsConn.IsConnected() {
        status = "down"
    }
```

**Thay bằng**:

```go
func (bff *HealthBFF) checkNATS(ctx context.Context) InfraHealth {
    start := time.Now()
    status := "healthy"
    if bff.natsConn == nil || !bff.natsConn.IsConnected() {  // nil guard
        status = "down"
    }
```

---

## Thay đổi — bff/settings.go: Handle nil repo

**File**: [`apps/osv/internal/gateway/bff/settings.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/settings.go)

**Tìm** `GetSettings`:

```go
func (bff *SettingsBFF) GetSettings(w http.ResponseWriter, r *http.Request) {
    settings, err := bff.settingsRepo.Get(r.Context())
```

**Thêm nil guard**:

```go
func (bff *SettingsBFF) GetSettings(w http.ResponseWriter, r *http.Request) {
    if bff.settingsRepo == nil {
        respondJSON(w, 200, defaultSettings())
        return
    }
    settings, err := bff.settingsRepo.Get(r.Context())
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/admin/health` trả `200` kể cả khi NATS chưa kết nối
- [ ] `GET /api/v1/admin/settings` trả `200` với default values khi DB chưa sẵn sàng
- [ ] Không còn `404` cho admin routes
- [ ] `go build ./...` trong apps/osv không có lỗi

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/apps/osv
go build ./...

# Test health (cần admin token)
curl -s -H "Authorization: Bearer <admin_token>" \
  "https://c12.openledger.vn/api/v1/admin/health" | jq '.overall_status'
# Expected: "healthy" hoặc "degraded" (không phải 404)

# Test settings
curl -s -H "Authorization: Bearer <admin_token>" \
  "https://c12.openledger.vn/api/v1/admin/settings" | jq '.general.platform_name'
# Expected: "OSV Platform"
```
