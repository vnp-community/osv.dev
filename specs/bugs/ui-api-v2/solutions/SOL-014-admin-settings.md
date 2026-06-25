# SOL-014 — Admin Settings: Đảm bảo BFF hoạt động (P2)

**Bug**: [BUG-016](../BUG-016-admin-settings.md)  
**Service**: `apps/osv` — `SettingsBFF` + `postgres.SettingsRepo` (in-gateway)  
**Endpoint**: `GET /api/v1/admin/settings`, `PATCH /api/v1/admin/settings`

**Status**: `✅ Implemented` — via [TASK-014](../../tasks/TASK-014-*.md)

---

## Root Cause Analysis

### Code hiện tại

`SettingsBFF` đã implement trong [`bff/settings.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/settings.go).

Routes đã đăng ký trong [`router.go:258`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L258):

```go
if healthBFF != nil && settingsBFF != nil {
    mux.Handle("GET /api/v1/admin/health",    adminOnly(http.HandlerFunc(healthBFF.HandleAdminHealth)))
    mux.Handle("GET /api/v1/admin/settings",  adminOnly(http.HandlerFunc(settingsBFF.GetSettings)))
    mux.Handle("PATCH /api/v1/admin/settings", adminOnly(http.HandlerFunc(settingsBFF.UpdateSettings)))
}
```

**Vấn đề**: Route chỉ được đăng ký khi `healthBFF != nil && settingsBFF != nil`. Nếu NATS/DB chưa ready → 404.

### Settings Repository

`SettingsRepo` được inject tại [`router.go:48`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L48):

```go
settingsRepo := postgres.NewSettingsRepo(db)
```

Repository này cần table `platform_settings` trong database gateway (`osv_gateway` schema).

---

## Giải pháp

### Bước 1: Tạo migration cho `platform_settings` table

File cần tạo: `apps/osv/migrations/001_create_settings.sql`

```sql
-- apps/osv/migrations/001_create_settings.sql
CREATE SCHEMA IF NOT EXISTS osv_gateway;

CREATE TABLE IF NOT EXISTS osv_gateway.platform_settings (
    id          SERIAL PRIMARY KEY,
    key         VARCHAR(100) NOT NULL UNIQUE,
    value       JSONB NOT NULL DEFAULT '{}',
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Insert defaults
INSERT INTO osv_gateway.platform_settings (key, value)
VALUES ('general', '{"platform_name": "OSV Platform", "timezone": "UTC", "date_format": "YYYY-MM-DD"}')
ON CONFLICT (key) DO NOTHING;

INSERT INTO osv_gateway.platform_settings (key, value)
VALUES ('security', '{"password_min_length": 12, "password_max_age_days": 90, "session_timeout_minutes": 60, "max_concurrent_sessions": 3, "mfa_required": false}')
ON CONFLICT (key) DO NOTHING;

INSERT INTO osv_gateway.platform_settings (key, value)
VALUES ('smtp', '{"host": "", "port": 587, "use_tls": true}')
ON CONFLICT (key) DO NOTHING;

INSERT INTO osv_gateway.platform_settings (key, value)
VALUES ('ai', '{"active_provider_id": "ollama", "providers": []}')
ON CONFLICT (key) DO NOTHING;
```

### Bước 2: Kiểm tra `postgres.NewSettingsRepo`

```bash
# Tìm file SettingsRepo trong gateway
find apps/osv/internal/infra/postgres -name "*.go" | xargs grep -l "SettingsRepo\|settings"
```

Nếu chưa có → tạo:

```go
// apps/osv/internal/infra/postgres/settings_repo.go

package postgres

import (
    "context"
    "encoding/json"
    
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/osv/apps/osv/internal/gateway/bff"
)

type SettingsRepo struct {
    pool *pgxpool.Pool
}

func NewSettingsRepo(pool *pgxpool.Pool) *SettingsRepo {
    return &SettingsRepo{pool: pool}
}

func (r *SettingsRepo) Get(ctx context.Context) (*bff.PlatformSettings, error) {
    settings := &bff.PlatformSettings{}
    
    rows, err := r.pool.Query(ctx, 
        `SELECT key, value FROM osv_gateway.platform_settings`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    for rows.Next() {
        var key string
        var value []byte
        if err := rows.Scan(&key, &value); err != nil {
            return nil, err
        }
        switch key {
        case "general":
            json.Unmarshal(value, &settings.General)
        case "smtp":
            json.Unmarshal(value, &settings.SMTP)
        case "security":
            json.Unmarshal(value, &settings.Security)
        case "ai":
            json.Unmarshal(value, &settings.AI)
        }
    }
    return settings, rows.Err()
}

func (r *SettingsRepo) Patch(ctx context.Context, patch map[string]interface{}) error {
    for key, val := range patch {
        b, _ := json.Marshal(val)
        _, err := r.pool.Exec(ctx, `
            INSERT INTO osv_gateway.platform_settings (key, value, updated_at)
            VALUES ($1, $2, NOW())
            ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`,
            key, b)
        if err != nil {
            return err
        }
    }
    return nil
}
```

### Bước 3: Fix `router.go` — Bỏ guard nil

```go
// router.go — TRƯỚC
if natsConn != nil && db != nil {
    healthBFF = bff.NewHealthBFF(...)
    settingsRepo := postgres.NewSettingsRepo(db)
    settingsBFF = bff.NewSettingsBFF(settingsRepo)
}

// router.go — SAU
healthBFF = bff.NewHealthBFF(redisClient, natsConn, func(ctx context.Context) error {
    if db != nil { return db.Ping(ctx) }
    return fmt.Errorf("db not connected")
})
settingsBFF = bff.NewSettingsBFF(postgres.NewSettingsRepo(db))

// Luôn đăng ký routes
mux.Handle("GET /api/v1/admin/health",    adminOnly(http.HandlerFunc(healthBFF.HandleAdminHealth)))
mux.Handle("GET /api/v1/admin/settings",  adminOnly(http.HandlerFunc(settingsBFF.GetSettings)))
mux.Handle("PATCH /api/v1/admin/settings", adminOnly(http.HandlerFunc(settingsBFF.UpdateSettings)))
```

---

## Response Schema (đã đúng trong settings.go)

```json
{
  "general": {
    "platform_name": "OSV Platform",
    "organization": "",
    "support_email": "",
    "timezone": "UTC",
    "date_format": "YYYY-MM-DD"
  },
  "smtp": {
    "host": "",
    "port": 587,
    "username": "",
    "from_name": "",
    "use_tls": true
  },
  "security": {
    "password_min_length": 12,
    "password_max_age_days": 90,
    "session_timeout_minutes": 60,
    "max_concurrent_sessions": 3,
    "mfa_required": false,
    "allow_sms_otp": false
  },
  "ai": {
    "active_provider_id": "ollama",
    "providers": []
  }
}
```

---

## Files cần sửa

| File | Trạng thái | Thay đổi |
|------|------------|----------|
| [`bff/settings.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/settings.go) | ✅ Đã implement | Không cần sửa |
| [`gateway/router.go:46`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L46) | ⚠️ Cần sửa guard | Remove nil guard, always register |
| `apps/osv/internal/infra/postgres/settings_repo.go` | ❓ Cần kiểm tra | Tạo nếu chưa có |
| `apps/osv/migrations/001_create_settings.sql` | ❓ Cần kiểm tra | Tạo nếu chưa có |

---

## Verification

```bash
# Test GET settings
curl -H "Authorization: Bearer <admin_token>" \
  "https://c12.openledger.vn/api/v1/admin/settings" | jq '.general.platform_name'
# Expected: "OSV Platform"

# Test PATCH settings
curl -X PATCH \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{"general": {"platform_name": "My OSV Platform"}}' \
  "https://c12.openledger.vn/api/v1/admin/settings"
# Expected: 200 với settings đã cập nhật
```
