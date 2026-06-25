# TASK-014 — apps/osv: Admin Settings — DB Migration + SettingsRepo

**Bug**: [BUG-016](../BUG-016-admin-settings.md)  
**Solution**: [SOL-014](../solutions/SOL-014-admin-settings.md)  
**Priority**: 🟡 P2  
**Effort**: ~30 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/admin/settings` trả `404` (do nil guard trong router) hoặc `500` (do table `platform_settings` chưa tồn tại). `SettingsBFF` đã implement đúng — vấn đề là DB schema chưa có và route bị block.

> **Note**: Route guard đã được fix trong TASK-006. TASK-014 này chỉ xử lý phần DB migration + SettingsRepo.

---

## File cần sửa / tạo

**File mới 1**: `apps/osv/migrations/001_create_platform_settings.sql`  
**File mới 2**: `apps/osv/internal/infra/postgres/settings_repo.go` (nếu chưa có)  
**File check**: [`apps/osv/internal/gateway/bff/settings.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/settings.go)

---

## Thay đổi 1 — Tạo DB migration

**Tạo file**: `apps/osv/migrations/001_create_platform_settings.sql`

```sql
-- Migration: Create platform_settings table for Admin Settings BFF
-- Schema: osv_gateway

CREATE SCHEMA IF NOT EXISTS osv_gateway;

CREATE TABLE IF NOT EXISTS osv_gateway.platform_settings (
    id         SERIAL PRIMARY KEY,
    key        VARCHAR(100) NOT NULL UNIQUE,
    value      JSONB        NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Seed default values
INSERT INTO osv_gateway.platform_settings (key, value) VALUES
    ('general', '{
        "platform_name": "OSV Platform",
        "organization": "",
        "support_email": "",
        "timezone": "UTC",
        "date_format": "YYYY-MM-DD",
        "items_per_page": 25
    }'),
    ('smtp', '{
        "host": "",
        "port": 587,
        "username": "",
        "from_name": "OSV Platform",
        "use_tls": true,
        "use_ssl": false
    }'),
    ('security', '{
        "password_min_length": 12,
        "password_max_age_days": 90,
        "password_require_uppercase": true,
        "password_require_number": true,
        "password_require_symbol": true,
        "session_timeout_minutes": 60,
        "max_concurrent_sessions": 3,
        "mfa_required": false,
        "allow_sms_otp": false,
        "ip_whitelist_enabled": false,
        "ip_whitelist": []
    }'),
    ('ai', '{
        "active_provider_id": "ollama",
        "providers": [],
        "auto_triage_enabled": false,
        "auto_enrichment_enabled": true,
        "enrichment_interval_hours": 24
    }')
ON CONFLICT (key) DO NOTHING;
```

---

## Thay đổi 2 — Kiểm tra / Tạo SettingsRepo

**Kiểm tra file tồn tại**:

```bash
ls /Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/infra/postgres/
```

**Nếu `settings_repo.go` chưa có**, tạo:

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
    if pool == nil {
        return nil
    }
    return &SettingsRepo{pool: pool}
}

// Get retrieves all platform settings
func (r *SettingsRepo) Get(ctx context.Context) (*bff.PlatformSettings, error) {
    if r == nil || r.pool == nil {
        return bff.DefaultSettings(), nil
    }

    rows, err := r.pool.Query(ctx,
        `SELECT key, value FROM osv_gateway.platform_settings`)
    if err != nil {
        return bff.DefaultSettings(), nil   // fallback to defaults on error
    }
    defer rows.Close()

    settings := bff.DefaultSettings()
    for rows.Next() {
        var key string
        var value []byte
        if err := rows.Scan(&key, &value); err != nil {
            continue
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
    return settings, nil
}

// Patch updates specific settings sections
func (r *SettingsRepo) Patch(ctx context.Context, patch map[string]interface{}) error {
    if r == nil || r.pool == nil {
        return nil
    }
    for key, val := range patch {
        b, err := json.Marshal(val)
        if err != nil {
            continue
        }
        _, err = r.pool.Exec(ctx, `
            INSERT INTO osv_gateway.platform_settings (key, value, updated_at)
            VALUES ($1, $2, NOW())
            ON CONFLICT (key) DO UPDATE
                SET value = $2, updated_at = NOW()
        `, key, b)
        if err != nil {
            return err
        }
    }
    return nil
}
```

---

## Thay đổi 3 — Thêm DefaultSettings() vào bff/settings.go

**Tìm** file [`apps/osv/internal/gateway/bff/settings.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/bff/settings.go) và **thêm**:

```go
// DefaultSettings returns safe defaults when DB is not available
func DefaultSettings() *PlatformSettings {
    return &PlatformSettings{
        General: GeneralSettings{
            PlatformName:  "OSV Platform",
            Timezone:      "UTC",
            DateFormat:    "YYYY-MM-DD",
            ItemsPerPage:  25,
        },
        Security: SecuritySettings{
            PasswordMinLength:        12,
            PasswordMaxAgeDays:       90,
            SessionTimeoutMinutes:    60,
            MaxConcurrentSessions:    3,
            MFARequired:             false,
        },
        SMTP: SMTPSettings{
            Port:   587,
            UseTLS: true,
        },
        AI: AISettings{
            ActiveProviderID: "ollama",
            Providers:        []interface{}{},
        },
    }
}
```

---

## Thay đổi 4 — Chạy migration

**Tìm** migration runner của apps/osv:

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/apps/osv -name "*.go" | xargs grep -l "migrate\|migration" | head -5
```

Nếu dùng file-based migrations, thêm file SQL vào thư mục migrations và đảm bảo app chạy migration khi startup.

---

## Acceptance Criteria

- [ ] Table `osv_gateway.platform_settings` tồn tại trong DB
- [ ] `GET /api/v1/admin/settings` trả `200` với đầy đủ sections
- [ ] `PATCH /api/v1/admin/settings` cập nhật thành công
- [ ] Khi DB down, vẫn trả `200` với default values (không crash)
- [ ] `go build ./...` trong apps/osv không có lỗi

---

## Verify

```bash
# Chạy migration
docker exec -it postgres psql -U postgres -d osv \
  -f /migrations/001_create_platform_settings.sql

# Kiểm tra table
docker exec -it postgres psql -U postgres -d osv \
  -c "SELECT key, jsonb_pretty(value) FROM osv_gateway.platform_settings;"

# Test endpoint
curl -s -H "Authorization: Bearer <admin_token>" \
  "https://c12.openledger.vn/api/v1/admin/settings" | jq '.general.platform_name'
# Expected: "OSV Platform"

# Test PATCH
curl -s -X PATCH \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{"general": {"platform_name": "My Platform"}}' \
  "https://c12.openledger.vn/api/v1/admin/settings" | jq '.general.platform_name'
# Expected: "My Platform"
```
