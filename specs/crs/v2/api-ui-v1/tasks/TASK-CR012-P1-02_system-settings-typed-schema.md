# TASK-CR012-P1-02 — System Settings: Typed Schema (GET/PUT)

**Phase:** Phase 1 — Settings Schema Upgrade  
**Nguồn giải pháp:** [`solutions/SOL-012 §3`](../solutions/SOL-012-apikey-security-system-settings.md)  
**Ưu tiên:** 🔴 P1 — Untyped schema, không validation  
**Phụ thuộc:** Không có (độc lập với TASK-CR012-P0-01)  
**Status:** ✅ **DONE** — 2026-06-19  

---

## Mục tiêu

`GET/PUT /api/v1/admin/settings` hiện trả `object` untyped. Nâng cấp lên `SystemSettings` typed schema với 4 sections: `general`, `smtp`, `security`, `ai`. Thêm validation khi PUT.

---

## Điều tra trước khi code

```bash
# 1. Tìm settings handler hiện tại
grep -r "admin/settings\|GetSettings\|UpdateSettings\|SettingsBFF" \
  apps/osv/ services/identity-service/ --include="*.go" -l

# 2. Xem implementation
grep -rn "admin/settings\|GetSettings\|UpdateSettings" \
  apps/osv/ services/identity-service/ --include="*.go"

# 3. Kiểm tra table settings trong DB
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "\d system_settings" 2>/dev/null || echo "TABLE NOT FOUND"

# 4. Xem BFF hiện tại nếu có
cat apps/osv/internal/gateway/bff/settings.go 2>/dev/null || echo "NOT FOUND"
```

---

## Bước 1: Tạo DB Table (nếu chưa có)

```bash
# Check xem có table chưa
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "SELECT * FROM system_settings LIMIT 1;" 2>&1
```

**File mới:** `services/identity-service/migrations/20260619_002_system_settings.sql`

```sql
CREATE TABLE IF NOT EXISTS system_settings (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    settings   JSONB NOT NULL DEFAULT '{}',
    updated_by VARCHAR(255),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default nếu chưa có
INSERT INTO system_settings (settings)
SELECT '{
    "general": {
        "platform_name": "OSV Platform",
        "organization": "",
        "support_email": "",
        "timezone": "UTC",
        "logo_url": null
    },
    "smtp": {"host": "", "port": 587, "username": "", "use_tls": true, "from_email": ""},
    "security": {
        "password_min_length": 12,
        "password_max_age_days": 90,
        "session_timeout_minutes": 60,
        "max_concurrent_sessions": 3,
        "mfa_required": false,
        "allow_oauth": true
    },
    "ai": {"active_provider_id": "ollama", "providers": []}
}'::jsonb
WHERE NOT EXISTS (SELECT 1 FROM system_settings);
```

```bash
# Chạy migration
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -f /migrations/20260619_002_system_settings.sql
```

---

## Bước 2: Domain Model với Validation

**Tìm domain folder:**
```bash
ls services/identity-service/internal/domain/
```

**File:** `services/identity-service/internal/domain/settings.go`

```go
package domain

import (
    "fmt"
    "net/mail"
)

type SystemSettings struct {
    General  GeneralSettings  `json:"general"`
    SMTP     SMTPSettings     `json:"smtp"`
    Security SecuritySettings `json:"security"`
    AI       AISettings       `json:"ai"`
}

type GeneralSettings struct {
    PlatformName string  `json:"platform_name"`
    Organization string  `json:"organization"`
    SupportEmail string  `json:"support_email"`
    Timezone     string  `json:"timezone"`
    LogoURL      *string `json:"logo_url"`
}

type SMTPSettings struct {
    Host      string `json:"host"`
    Port      int    `json:"port"`
    Username  string `json:"username"`
    UseTLS    bool   `json:"use_tls"`
    FromEmail string `json:"from_email"`
}

type SecuritySettings struct {
    PasswordMinLength      int  `json:"password_min_length"`
    PasswordMaxAgeDays     int  `json:"password_max_age_days"`
    SessionTimeoutMinutes  int  `json:"session_timeout_minutes"`
    MaxConcurrentSessions  int  `json:"max_concurrent_sessions"`
    MFARequired            bool `json:"mfa_required"`
    AllowOAuth             bool `json:"allow_oauth"`
}

type AISettings struct {
    ActiveProviderID string       `json:"active_provider_id"`
    Providers        []AIProvider `json:"providers"`
}

type AIProvider struct {
    ID             string   `json:"id"`
    Name           string   `json:"name"`
    Model          string   `json:"model"`
    Status         string   `json:"status"`
    LatencyMs      *int     `json:"latency_ms"`
    RequestsPerDay *int     `json:"requests_per_day"`
    CostPerDay     float64  `json:"cost_per_day"`
}

func (s *SystemSettings) Validate() error {
    if s.General.SupportEmail != "" {
        if _, err := mail.ParseAddress(s.General.SupportEmail); err != nil {
            return fmt.Errorf("general.support_email: invalid email format")
        }
    }
    if s.SMTP.Port != 0 && (s.SMTP.Port < 1 || s.SMTP.Port > 65535) {
        return fmt.Errorf("smtp.port: must be 1-65535")
    }
    if s.Security.PasswordMinLength != 0 && s.Security.PasswordMinLength < 8 {
        return fmt.Errorf("security.password_min_length: must be >= 8")
    }
    if s.Security.SessionTimeoutMinutes != 0 &&
        (s.Security.SessionTimeoutMinutes < 5 || s.Security.SessionTimeoutMinutes > 480) {
        return fmt.Errorf("security.session_timeout_minutes: must be 5-480")
    }
    return nil
}
```

---

## Bước 3: Handler Implementation

**Tìm handler:**
```bash
# Tìm settings handler hoặc BFF
grep -rn "settings\|Settings" \
  apps/osv/internal/gateway/bff/ \
  services/identity-service/internal/ \
  --include="*.go" -l
```

**Sửa hoặc tạo handler** (tùy vào kết quả điều tra — có thể là BFF hoặc identity-service):

```go
// GET /api/v1/admin/settings
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
    settings, err := h.settingsRepo.GetSystemSettings(r.Context())
    if err != nil {
        // Trả default nếu chưa có
        settings = defaultSystemSettings()
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(settings)
}

// PUT /api/v1/admin/settings
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
    var req domain.SystemSettings
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, 400, "invalid JSON body")
        return
    }

    if err := req.Validate(); err != nil {
        respondError(w, 400, err.Error())
        return
    }

    updated, err := h.settingsRepo.UpsertSystemSettings(r.Context(), &req)
    if err != nil {
        respondError(w, 500, "failed to save settings")
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(updated)
}

func defaultSystemSettings() *domain.SystemSettings {
    return &domain.SystemSettings{
        General: domain.GeneralSettings{
            PlatformName: "OSV Platform",
            Timezone:     "UTC",
        },
        SMTP: domain.SMTPSettings{Port: 587, UseTLS: true},
        Security: domain.SecuritySettings{
            PasswordMinLength:     12,
            PasswordMaxAgeDays:    90,
            SessionTimeoutMinutes: 60,
            MaxConcurrentSessions: 3,
        },
        AI: domain.AISettings{
            ActiveProviderID: "ollama",
            Providers:        []domain.AIProvider{},
        },
    }
}
```

---

## Bước 4: Repository

```go
// GetSystemSettings đọc từ DB
func (r *SettingsRepo) GetSystemSettings(ctx context.Context) (*domain.SystemSettings, error) {
    var raw []byte
    err := r.db.QueryRow(ctx, `SELECT settings FROM system_settings LIMIT 1`).Scan(&raw)
    if err != nil {
        return nil, err
    }
    var s domain.SystemSettings
    if err := json.Unmarshal(raw, &s); err != nil {
        return nil, err
    }
    return &s, nil
}

// UpsertSystemSettings ghi vào DB
func (r *SettingsRepo) UpsertSystemSettings(ctx context.Context, s *domain.SystemSettings) (*domain.SystemSettings, error) {
    data, _ := json.Marshal(s)
    _, err := r.db.Exec(ctx, `
        UPDATE system_settings SET settings = $1, updated_at = NOW()
        WHERE id = (SELECT id FROM system_settings LIMIT 1)
    `, data)
    if err != nil {
        return nil, err
    }
    return s, nil
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/admin/settings` → `SystemSettings` với 4 sections — HTTP 200
- [ ] `PUT /api/v1/admin/settings` với body hợp lệ → HTTP 200, settings updated
- [ ] `PUT` với email sai format → HTTP 400
- [ ] `PUT` với `password_min_length < 8` → HTTP 400
- [ ] `PUT` với `smtp.port = 0` → không báo lỗi (optional field)
- [ ] Settings persist sau service restart

## Verification

```bash
TOKEN="<admin-token>"

# GET settings
curl -s https://c12.openledger.vn/api/v1/admin/settings \
  -H "Authorization: Bearer $TOKEN" | jq 'keys'
# Expected: ["ai", "general", "security", "smtp"]

# PUT valid settings
curl -s -X PUT https://c12.openledger.vn/api/v1/admin/settings \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"general":{"platform_name":"Test","support_email":"test@example.com","timezone":"UTC"},"smtp":{"port":587,"use_tls":true},"security":{"password_min_length":12,"session_timeout_minutes":60},"ai":{"active_provider_id":"ollama","providers":[]}}' | jq .
# Expected: HTTP 200

# PUT với email sai
curl -s -X PUT https://c12.openledger.vn/api/v1/admin/settings \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"general":{"support_email":"not-an-email"},"smtp":{},"security":{},"ai":{}}' | jq .
# Expected: HTTP 400
```
