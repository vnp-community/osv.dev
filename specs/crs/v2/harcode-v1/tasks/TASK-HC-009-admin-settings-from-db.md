# TASK-HC-009: Admin Settings từ DB

**Status:** ✅ DONE  
**Sprint:** 2 | **Ước lượng:** 4 giờ  
**Solution:** [SOL-004](../solutions/SOL-004-gateway-admin-settings.md)  
**Service:** `services/identity-service`, `services/gateway-service`

---

## Mô tả

`GET /api/v1/admin/settings` trả hardcode values với `"_meta":{"is_default":true}`. Cần tạo `platform_settings` table trong identity-service và gateway proxy đến đó.

---

## Acceptance Criteria

- [x] Table `platform_settings` tồn tại với 16 default settings
- [x] `GET /api/v1/admin/settings` trả data từ DB (không còn `"_meta":{"is_default":true}`)
- [x] `PUT /api/v1/admin/settings` với body `{"security":{"password_policy":"high"}}` persist vào DB
- [x] Sau restart, setting được giữ nguyên
- [x] `go build ./...` pass trong cả hai services

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| NEW | `services/identity-service/migrations/005_platform_settings.sql` | Schema + seed |
| NEW | `services/identity-service/internal/domain/repository/system_settings.go` | Interface |
| NEW | `services/identity-service/internal/infra/postgres/system_settings_repo.go` | PostgreSQL impl |
| MODIFY | `services/identity-service/adapter/handler/http/admin_handler.go` | GetAdminSettings + UpdateAdminSettings |
| MODIFY | `services/identity-service/embedded.go` | Wire settingsRepo |
| MODIFY | `services/gateway-service/internal/bff/handlers/handler_ui_api.go` | Proxy thay hardcode |

---

## Bước thực thi

### 1. Tạo migration

**File:** `services/identity-service/migrations/005_platform_settings.sql`

```sql
CREATE TABLE IF NOT EXISTS platform_settings (
    key         VARCHAR(100) PRIMARY KEY,
    value       JSONB        NOT NULL,
    description TEXT,
    category    VARCHAR(50)  NOT NULL DEFAULT 'general',
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_by  UUID
);

CREATE INDEX IF NOT EXISTS idx_platform_settings_category ON platform_settings(category);

INSERT INTO platform_settings (key, value, description, category) VALUES
    ('smtp.enabled',                 'false',    'Enable SMTP',                       'smtp'),
    ('smtp.host',                    '""',       'SMTP server hostname',              'smtp'),
    ('smtp.port',                    '587',      'SMTP server port',                  'smtp'),
    ('smtp.username',                '""',       'SMTP username',                     'smtp'),
    ('smtp.from_email',              '""',       'Sender email',                      'smtp'),
    ('smtp.tls',                     'true',     'Enable TLS',                        'smtp'),
    ('security.mfa_required',        'false',    'Require MFA for all users',         'security'),
    ('security.password_policy',     '"medium"', 'Password policy: low|medium|high',  'security'),
    ('security.max_login_attempts',  '5',        'Max failed login attempts',         'security'),
    ('security.lockout_duration_min','30',       'Account lockout duration',          'security'),
    ('security.api_key_expiry_days', '90',       'API key expiry in days',            'security'),
    ('security.jwt_expiry_min',      '15',       'JWT token expiry in minutes',       'security'),
    ('ai.enabled',                   'false',    'Enable AI features',                'ai'),
    ('ai.provider',                  '"ollama"', 'AI provider: ollama|openai|vertex', 'ai'),
    ('ai.auto_triage',               'false',    'Enable automatic finding triage',   'ai'),
    ('ai.auto_enrichment',           'false',    'Enable automatic CVE enrichment',   'ai')
ON CONFLICT (key) DO NOTHING;
```

```bash
psql $DATABASE_URL -f services/identity-service/migrations/005_platform_settings.sql
psql $DATABASE_URL -c "SELECT key, value FROM platform_settings LIMIT 5;"
```

### 2. Tạo domain interface

**File:** `services/identity-service/internal/domain/repository/system_settings.go`

```go
package repository

import (
    "context"
    "encoding/json"
    "github.com/google/uuid"
)

type SettingEntry struct {
    Key      string          `json:"key"`
    Value    json.RawMessage `json:"value"`
    Category string          `json:"category"`
}

type SystemSettingsRepository interface {
    GetAll(ctx context.Context) ([]*SettingEntry, error)
    Set(ctx context.Context, key string, value interface{}, updatedBy uuid.UUID) error
    SetBulk(ctx context.Context, settings map[string]interface{}, updatedBy uuid.UUID) error
}
```

### 3. Tạo PostgreSQL implementation

**File:** `services/identity-service/internal/infra/postgres/system_settings_repo.go`

```go
package postgres

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/osv/identity-service/internal/domain/repository"
)

type SystemSettingsRepo struct {
    pool *pgxpool.Pool
}

func NewSystemSettingsRepo(pool *pgxpool.Pool) *SystemSettingsRepo {
    return &SystemSettingsRepo{pool: pool}
}

func (r *SystemSettingsRepo) GetAll(ctx context.Context) ([]*repository.SettingEntry, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT key, value, COALESCE(category,'general')
        FROM platform_settings ORDER BY category, key
    `)
    if err != nil {
        return nil, fmt.Errorf("system_settings.GetAll: %w", err)
    }
    defer rows.Close()

    var entries []*repository.SettingEntry
    for rows.Next() {
        e := &repository.SettingEntry{}
        if err := rows.Scan(&e.Key, &e.Value, &e.Category); err != nil {
            return nil, fmt.Errorf("system_settings.GetAll scan: %w", err)
        }
        entries = append(entries, e)
    }
    return entries, rows.Err()
}

func (r *SystemSettingsRepo) Set(ctx context.Context, key string, value interface{}, updatedBy uuid.UUID) error {
    raw, err := json.Marshal(value)
    if err != nil {
        return fmt.Errorf("system_settings.Set marshal: %w", err)
    }
    _, err = r.pool.Exec(ctx, `
        INSERT INTO platform_settings (key, value, updated_at, updated_by)
        VALUES ($1, $2, NOW(), $3)
        ON CONFLICT (key) DO UPDATE SET
            value = EXCLUDED.value,
            updated_at = NOW(),
            updated_by = EXCLUDED.updated_by
    `, key, raw, updatedBy)
    return err
}

func (r *SystemSettingsRepo) SetBulk(ctx context.Context, settings map[string]interface{}, updatedBy uuid.UUID) error {
    for key, value := range settings {
        if err := r.Set(ctx, key, value, updatedBy); err != nil {
            return fmt.Errorf("system_settings.SetBulk key=%s: %w", key, err)
        }
    }
    return nil
}
```

### 4. Sửa GetAdminSettings trong admin_handler.go

```bash
grep -n "GetAdminSettings\|AdminSettings\|func.*admin.*settings\|_meta" \
  services/identity-service/adapter/handler/http/admin_handler.go | head -15
```

```go
// [FIX CR-HC-004] GetAdminSettings reads from platform_settings table
func (h *AdminHandler) GetAdminSettings(w http.ResponseWriter, r *http.Request) {
    if h.settingsRepo == nil {
        writeError(w, http.StatusServiceUnavailable, "settings repository not configured")
        return
    }
    entries, err := h.settingsRepo.GetAll(r.Context())
    if err != nil {
        h.log.Error().Err(err).Msg("GetAdminSettings: failed")
        writeError(w, http.StatusInternalServerError, "failed to load settings")
        return
    }
    
    // Build nested response: "category.key" → {category: {key: value}}
    result := make(map[string]map[string]interface{})
    for _, e := range entries {
        var val interface{}
        _ = json.Unmarshal(e.Value, &val)
        parts := strings.SplitN(e.Key, ".", 2)
        if len(parts) == 2 {
            if result[parts[0]] == nil {
                result[parts[0]] = make(map[string]interface{})
            }
            result[parts[0]][parts[1]] = val
        }
    }
    writeJSON(w, http.StatusOK, result)
}

// [FIX CR-HC-004] AdminSettingsUpdate persists to DB
func (h *AdminHandler) UpdateAdminSettings(w http.ResponseWriter, r *http.Request) {
    if h.settingsRepo == nil {
        writeError(w, http.StatusServiceUnavailable, "settings repository not configured")
        return
    }
    userID, _ := uuid.Parse(r.Header.Get("X-User-ID"))

    var body map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    
    // Flatten nested map to dot-notation
    flat := flattenMap(body, "")
    if err := h.settingsRepo.SetBulk(r.Context(), flat, userID); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to save settings")
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func flattenMap(m map[string]interface{}, prefix string) map[string]interface{} {
    result := make(map[string]interface{})
    for k, v := range m {
        key := k
        if prefix != "" { key = prefix + "." + k }
        if nested, ok := v.(map[string]interface{}); ok {
            for nk, nv := range flattenMap(nested, key) {
                result[nk] = nv
            }
        } else {
            result[key] = v
        }
    }
    return result
}
```

### 5. Wire settingsRepo trong identity-service embedded.go

```bash
grep -n "AdminHandler\|NewAdmin\|embedded\|Wire" services/identity-service/embedded.go | head -10
```

```go
settingsRepo := pginfra.NewSystemSettingsRepo(pool)
adminHandler := handler.NewAdminHandler(..., settingsRepo)
```

### 6. Gateway proxy thay hardcode

```bash
grep -n "GetAdminSettings\|AdminSettings\|identityServiceURL" \
  services/gateway-service/internal/bff/handlers/handler_ui_api.go | head -10
```

```go
// [FIX CR-HC-004]
func (h *UIAPIHandler) GetAdminSettings(w http.ResponseWriter, r *http.Request) {
    h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/settings")
}
func (h *UIAPIHandler) AdminSettingsUpdate(w http.ResponseWriter, r *http.Request) {
    h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/settings")
}
```

### 7. Build check
```bash
cd services/identity-service && go build ./...
cd services/gateway-service && go build ./...
```

---

## Verification

```bash
# GET settings từ DB
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  "https://c12.openledger.vn/api/v1/admin/settings" | jq 'has("_meta")'
# PASS nếu = false (không còn _meta hardcode)

# PUT settings và verify persist
curl -s -X PUT -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"security":{"password_policy":"high"}}' \
  "https://c12.openledger.vn/api/v1/admin/settings"

psql $DATABASE_URL -c "SELECT value FROM platform_settings WHERE key = 'security.password_policy';"
# PASS nếu = "high"
```
