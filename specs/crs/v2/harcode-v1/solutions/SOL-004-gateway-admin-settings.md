# SOL-004: Admin Settings từ DB — gateway + identity-service

**CR:** CR-HC-004 | **Priority:** 🟠 High | **Sprint:** 2  
**Services:** `services/gateway-service`, `services/identity-service`

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-009
**Note:** Admin settings đọc/ghi từ bảng platform_settings (PostgreSQL)
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**Gateway GetAdminSettings hiện tại** (`handler_ui_api.go:960`):
```go
"security": map[string]interface{}{
    "mfa_required":   false,     // hardcode
    "password_policy": "medium", // hardcode
    // ...
},
"_meta": {"is_default": true},  // tự thú nhận là không đọc từ DB
```

**Identity-service** đã có: `internal/infra/postgres/` — nhiều repos nhưng chưa có `system_settings_repo`.

**Strategy:**
- `platform_settings` table lưu trong **identity-service** PostgreSQL (authentication authority)
- Identity-service expose endpoint `GET/PUT /api/v1/admin/settings`
- Gateway proxy đến identity-service (thay vì hardcode trả response)

---

## Solution

### Bước 1: Migration SQL trong identity-service

**File mới:** `identity-service/migrations/005_platform_settings.sql`

```sql
CREATE TABLE IF NOT EXISTS platform_settings (
    key         VARCHAR(100) PRIMARY KEY,
    value       JSONB        NOT NULL,
    description TEXT,
    category    VARCHAR(50)  NOT NULL DEFAULT 'general',
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_by  UUID
);

CREATE INDEX IF NOT EXISTS idx_platform_settings_category
    ON platform_settings(category);

-- Default values — mirrors current hardcoded values
INSERT INTO platform_settings (key, value, description, category) VALUES
    ('smtp.enabled',            'false',        'Enable SMTP email sending',                 'smtp'),
    ('smtp.host',               '""',           'SMTP server hostname',                       'smtp'),
    ('smtp.port',               '587',          'SMTP server port',                           'smtp'),
    ('smtp.username',           '""',           'SMTP authentication username',               'smtp'),
    ('smtp.from_email',         '""',           'Sender email address',                       'smtp'),
    ('smtp.tls',                'true',         'Enable TLS for SMTP',                        'smtp'),
    ('security.mfa_required',   'false',        'Require MFA for all users',                 'security'),
    ('security.password_policy','\"medium\"',   'Password strength policy: low|medium|high', 'security'),
    ('security.max_login_attempts', '5',        'Max failed login attempts',                 'security'),
    ('security.lockout_duration_min', '30',     'Account lockout duration in minutes',       'security'),
    ('security.api_key_expiry_days', '90',      'API key expiry in days (0 = never)',        'security'),
    ('security.jwt_expiry_min', '15',           'JWT token expiry in minutes',               'security'),
    ('ai.enabled',              'false',        'Enable AI features',                         'ai'),
    ('ai.provider',             '\"ollama\"',   'AI provider: ollama|openai|vertex',          'ai'),
    ('ai.auto_triage',          'false',        'Enable automatic finding triage',            'ai'),
    ('ai.auto_enrichment',      'false',        'Enable automatic CVE enrichment',            'ai')
ON CONFLICT (key) DO NOTHING;
```

### Bước 2: Domain interface trong identity-service

**File mới:** `identity-service/internal/domain/repository/system_settings.go`

```go
package repository

import (
    "context"
    "encoding/json"
    "github.com/google/uuid"
)

// SettingEntry represents one platform setting.
type SettingEntry struct {
    Key         string          `json:"key"`
    Value       json.RawMessage `json:"value"`
    Description string          `json:"description"`
    Category    string          `json:"category"`
    UpdatedBy   *uuid.UUID      `json:"updated_by,omitempty"`
}

// SystemSettingsRepository provides read/write access to platform settings.
type SystemSettingsRepository interface {
    GetAll(ctx context.Context) ([]*SettingEntry, error)
    GetByCategory(ctx context.Context, category string) ([]*SettingEntry, error)
    Get(ctx context.Context, key string) (*SettingEntry, error)
    Set(ctx context.Context, key string, value interface{}, updatedBy uuid.UUID) error
    SetBulk(ctx context.Context, settings map[string]interface{}, updatedBy uuid.UUID) error
}
```

### Bước 3: Repository implementation

**File mới:** `identity-service/internal/infra/postgres/system_settings_repo.go`

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
        SELECT key, value, COALESCE(description,''), COALESCE(category,'general'), updated_by
        FROM platform_settings
        ORDER BY category, key
    `)
    if err != nil {
        return nil, fmt.Errorf("system_settings.GetAll: %w", err)
    }
    defer rows.Close()

    var entries []*repository.SettingEntry
    for rows.Next() {
        e := &repository.SettingEntry{}
        if err := rows.Scan(&e.Key, &e.Value, &e.Description, &e.Category, &e.UpdatedBy); err != nil {
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
    if err != nil {
        return fmt.Errorf("system_settings.Set: %w", err)
    }
    return nil
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

### Bước 4: Admin Settings handler trong identity-service

**File sửa:** `identity-service/adapter/handler/http/admin_handler.go`

```go
// [FIX CR-HC-004] GetAdminSettings reads from platform_settings table
func (h *AdminHandler) GetAdminSettings(w http.ResponseWriter, r *http.Request) {
    entries, err := h.settingsRepo.GetAll(r.Context())
    if err != nil {
        h.log.Error().Err(err).Msg("GetAdminSettings: failed to read")
        writeError(w, http.StatusInternalServerError, "failed to load settings")
        return
    }

    // Build structured response grouped by category
    response := buildSettingsResponse(entries)
    writeJSON(w, http.StatusOK, response)
}

// buildSettingsResponse converts flat key-value pairs to nested category structure
func buildSettingsResponse(entries []*repository.SettingEntry) map[string]interface{} {
    result := map[string]interface{}{}
    for _, e := range entries {
        var val interface{}
        _ = json.Unmarshal(e.Value, &val)
        // Parse category.key → nested map
        parts := strings.SplitN(e.Key, ".", 2)
        if len(parts) == 2 {
            cat, key := parts[0], parts[1]
            if _, ok := result[cat]; !ok {
                result[cat] = map[string]interface{}{}
            }
            result[cat].(map[string]interface{})[key] = val
        }
    }
    return result
}

// [FIX CR-HC-004] UpdateAdminSettings persists to DB
func (h *AdminHandler) UpdateAdminSettings(w http.ResponseWriter, r *http.Request) {
    userID, _ := uuid.Parse(r.Header.Get("X-User-ID"))

    var body map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    // Flatten nested map to dot-notation keys
    flat := flattenSettings(body, "")
    if err := h.settingsRepo.SetBulk(r.Context(), flat, userID); err != nil {
        h.log.Error().Err(err).Msg("UpdateAdminSettings: failed to save")
        writeError(w, http.StatusInternalServerError, "failed to save settings")
        return
    }

    writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
```

### Bước 5: Gateway proxy đến identity-service

**File sửa:** `gateway-service/internal/bff/handlers/handler_ui_api.go`

```go
// [FIX CR-HC-004] Proxy to identity-service — không hardcode
func (h *UIAPIHandler) GetAdminSettings(w http.ResponseWriter, r *http.Request) {
    h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/settings")
}

func (h *UIAPIHandler) AdminSettingsUpdate(w http.ResponseWriter, r *http.Request) {
    h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/settings")
}
```

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| NEW | `identity-service/migrations/005_platform_settings.sql` |
| NEW | `identity-service/internal/domain/repository/system_settings.go` |
| NEW | `identity-service/internal/infra/postgres/system_settings_repo.go` |
| MODIFY | `identity-service/adapter/handler/http/admin_handler.go` |
| MODIFY | `identity-service/embedded.go` — wire settingsRepo |
| MODIFY | `gateway-service/internal/bff/handlers/handler_ui_api.go` — proxy |

---

## Verification

```bash
psql $DATABASE_URL -f identity-service/migrations/005_platform_settings.sql

# GET settings — từ DB
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  "https://c12.openledger.vn/api/v1/admin/settings"
# Expect: {"security":{"mfa_required":false,"password_policy":"medium",...},...}

# PUT settings — persists to DB
curl -X PUT -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"security":{"password_policy":"high","mfa_required":true}}' \
  "https://c12.openledger.vn/api/v1/admin/settings"

# Verify in DB
psql $DATABASE_URL -c "SELECT key, value FROM platform_settings WHERE key LIKE 'security%';"
```
