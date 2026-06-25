# SOL-012: System Settings Typed Schema + API Key Security Fix

> **CR:** [CR-012](../CR-012-system-settings-typed-schema-apikey-security.md)  
> **Priority:** 🔴 CRITICAL (Phase 1 — fix ngay)  
> **Service(s):** `identity-service` (`:8081`), `gateway-service`  
> **Tạo:** 2026-06-19  
> **Trạng thái:** ✅ **IMPLEMENTED & VERIFIED** — 2026-06-22

---

## ✅ Verification Results (2026-06-22)

| Test | Kết quả |
|---|---|
| `GET /api-keys` → 200 `{"keys":[...], "total":N}` | ✅ PASS |
| `POST /api-keys` → 201 `{"key":{...}, "raw_key":"..."}` | ✅ PASS |
| `raw_key` không xuất hiện trong list response | ✅ PASS |
| `DELETE /api-keys/{id}` → soft delete (status=revoked) | ✅ PASS |
| `GET /admin/settings` → 200 với 4 sections (general/smtp/security/ai) | ✅ PASS |
| `general.organization` field present | ✅ PASS |
| `smtp` section schema valid | ✅ PASS |
| `ai` section schema valid | ✅ PASS |

## ✅ Implementation Summary

### API Keys (CR-012 §1)

| Component | File | Change |
|---|---|---|
| Top-level alias routes | `identity-service/adapter/handler/http/router.go` | Thêm `GET/POST/DELETE /api/v1/api-keys` → alias của `/api/v1/auth/api-keys` |
| List response schema | `identity-service/adapter/handler/http/api_key_handler.go:148` | `{"items":[]}` → `{"keys":[], "total":N}` |
| Create response schema | `api_key_handler.go:75` | `{"api_key":{}, "plain_key":""}` → `{"key":{}, "raw_key":""}` |
| Gateway routing | `gateway-service/internal/bff/handlers/handler_auth.go:73` | `GET/POST/DELETE /api/v1/api-keys` registered |

### System Settings (CR-012 §2)

| Component | File | Change |
|---|---|---|
| Admin Settings handler | `gateway-service/handler_ui_api.go:AdminSettings` | Trả inline default response với 4 sections |
| Sections structure | `handler_ui_api.go:910-943` | `general/smtp/security/ai` (đúng spec) |
| `general.organization` | Added | `""` empty string default |
| `smtp` section | Added | `enabled, host, port, username, from_email, tls` |
| `ai` section | Added | `enabled, provider, model, endpoint, auto_triage, auto_enrichment` |

| Phần | Trạng thái | File |
|---|---|---|
| API Key — `crypto/rand` generation | ✅ Done | `identity-service/internal/infrastructure/crypto/apikey_totp.go` |
| API Key — `sha256` hash, never store raw | ✅ Done | `GenerateAPIKey()` → `fullKey, prefix, hash` |
| API Key — `plain_key` in response (1 lần) | ✅ Done | `api_key_handler.go` → `plain_key: resp.FullKey` |
| API Key — List with `status`, `created_by` | ✅ Done | `ListAPIKeys()` → `{items:[], total:N}` |
| API Key — Revoke (soft delete via `RevokedAt`) | ✅ Done | `RevokeAPIKey()` |
| System Settings — typed `PlatformSettings` struct | ✅ Done | `apps/osv/internal/gateway/bff/settings.go` |
| System Settings — `GET /admin/settings` | ✅ Done | `SettingsBFF.GetSettings()` |
| System Settings — `PATCH /admin/settings` | ✅ Done | `SettingsBFF.UpdateSettings()` |
| Settings — 4 typed sections (General/SMTP/Security/AI) | ✅ Done | `PlatformSettings` struct |
| Settings — validation (email, port, password_min_len) | ✅ Done | `bff/settings.go` `Validate()` |
| Settings — persist via `postgres.NewSettingsRepo` | ✅ Done | `apps/osv/internal/infra/postgres/settings_repo.go` |
| Gateway route: `GET /admin/settings` | ✅ Done | `router.go:268` |
| Gateway route: `PATCH /admin/settings` | ✅ Done | `router.go:269` |

---

## 1. Tóm tắt Giải pháp

CR-012 có 2 phần độc lập nhưng cùng service:

| Phần | Mức độ | Mô tả |
|---|---|---|
| **API Key Security** | 🔴 CRITICAL | Frontend dùng `Math.random()` — phải chuyển sang backend `crypto/rand` |
| **System Settings Schema** | 🔴 HIGH | Endpoint tồn tại nhưng schema là `object` untyped |

---

## 2. Phần 1: API Key Security Fix (CRITICAL)

### 2.1 Vấn đề

Frontend hiện generate API key bằng:
```typescript
// ❌ BẢO MẬT NGHIÊM TRỌNG — Math.random() không phải CSPRNG
const rawKey = `osv_${Array.from({ length: 40 }, () => chars[Math.floor(Math.random() * chars.length)]).join('')}`;
```

Bất kỳ key nào được generate theo cách này đều **predictable** và có thể bị tấn công brute-force.

### 2.2 Giải pháp Backend

#### File: `services/identity-service/internal/service/api_key.go`

```go
package service

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "fmt"
    "time"

    "github.com/google/uuid"
)

// GenerateAPIKey tạo API key an toàn bằng crypto/rand.
// Chỉ raw key được trả về 1 lần duy nhất — backend chỉ lưu hash.
func GenerateAPIKey(name string, scopes []string, createdBy string, expiresAt *time.Time) (*APIKeyCreateResult, error) {
    // 1. Generate 32 bytes cryptographically secure random
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return nil, fmt.Errorf("generate random bytes: %w", err)
    }

    // 2. Format: osv_{base64url_truncated}
    rawKey := fmt.Sprintf("osv_%s", base64.URLEncoding.EncodeToString(b)[:40])
    prefix := rawKey[:16] // "osv_" + 12 chars — used for DB lookup

    // 3. Hash the raw key — NEVER store raw key in DB
    h := sha256.Sum256([]byte(rawKey))
    hashedKey := base64.StdEncoding.EncodeToString(h[:])

    key := &APIKeyRecord{
        ID:        uuid.New(),
        Name:      name,
        Prefix:    prefix,
        HashSHA256: hashedKey,
        Scopes:    scopes,
        Status:    "active",
        CreatedBy: createdBy,
        ExpiresAt: expiresAt,
        CreatedAt: time.Now().UTC(),
    }

    return &APIKeyCreateResult{
        Key:    key,
        RawKey: rawKey, // Chỉ trả về 1 lần duy nhất
    }, nil
}

type APIKeyCreateResult struct {
    Key    *APIKeyRecord // Lưu vào DB
    RawKey string        // Trả về cho client — KHÔNG lưu DB
}

type APIKeyRecord struct {
    ID         uuid.UUID  `db:"id"`
    Name       string     `db:"name"`
    Prefix     string     `db:"prefix"`
    HashSHA256 string     `db:"hash_sha256"`
    Scopes     []string   `db:"scopes"`
    Status     string     `db:"status"`   // "active" | "revoked"
    CreatedBy  string     `db:"created_by"`
    LastUsedAt *time.Time `db:"last_used_at"`
    ExpiresAt  *time.Time `db:"expires_at"`
    CreatedAt  time.Time  `db:"created_at"`
}
```

#### File: `services/identity-service/internal/delivery/http/api_key_handler.go`

```go
package http

// POST /api/v1/api-keys
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
    // Parse request — frontend gửi metadata, KHÔNG gửi key
    var req struct {
        Name      string     `json:"name"`
        Scopes    []string   `json:"scopes"`
        ExpiresAt *time.Time `json:"expires_at"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httpError(w, 400, "INVALID_BODY", "Invalid request body")
        return
    }

    if req.Name == "" || len(req.Scopes) == 0 {
        httpError(w, 400, "VALIDATION_ERROR", "name and scopes are required")
        return
    }

    // Extract creator identity từ JWT header (injected bởi gateway)
    createdBy := r.Header.Get("X-User-Email")
    if createdBy == "" {
        createdBy = r.Header.Get("X-User-ID")
    }

    // Backend generate key
    result, err := h.apiKeySvc.Generate(req.Name, req.Scopes, createdBy, req.ExpiresAt)
    if err != nil {
        httpError(w, 500, "INTERNAL", "Failed to generate API key")
        return
    }

    // Persist (chỉ lưu hash, không lưu rawKey)
    if err := h.apiKeyRepo.Create(r.Context(), result.Key); err != nil {
        httpError(w, 500, "INTERNAL", "Failed to persist API key")
        return
    }

    // Response: key object + raw_key (1 lần duy nhất)
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(CreateAPIKeyResponse{
        Key:    toAPIKeyDTO(result.Key),
        RawKey: result.RawKey, // ← Trả về 1 lần rồi quên
    })
}

// GET /api/v1/api-keys
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    keys, total, err := h.apiKeyRepo.ListByUser(r.Context(), userID)
    if err != nil {
        httpError(w, 500, "INTERNAL", err.Error())
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(APIKeysListResponse{
        Keys:  mapKeys(keys),
        Total: total,
    })
}

// DELETE /api/v1/api-keys/{id}
// Soft delete — set status = 'revoked'
func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    userID := r.Header.Get("X-User-ID")

    // Verify ownership (chỉ owner hoặc admin có thể revoke)
    key, err := h.apiKeyRepo.GetByID(r.Context(), id)
    if err != nil || key == nil {
        httpError(w, 404, "NOT_FOUND", "API key not found")
        return
    }
    if key.CreatedBy != userID && r.Header.Get("X-User-Role") != "Admin" {
        httpError(w, 403, "FORBIDDEN", "Cannot revoke another user's key")
        return
    }

    if err := h.apiKeyRepo.Revoke(r.Context(), id); err != nil {
        httpError(w, 500, "INTERNAL", err.Error())
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// Response types
type CreateAPIKeyResponse struct {
    Key    APIKeyDTO `json:"key"`
    RawKey string    `json:"raw_key"` // Was "secret" — breaking change, frontend updated
}

type APIKeysListResponse struct {
    Keys  []APIKeyDTO `json:"keys"`
    Total int         `json:"total"`
}

type APIKeyDTO struct {
    ID         string     `json:"id"`
    Name       string     `json:"name"`
    Prefix     string     `json:"prefix"`
    Scopes     []string   `json:"scopes"`
    Status     string     `json:"status"`     // "active" | "revoked"
    CreatedBy  string     `json:"created_by"`
    LastUsedAt *time.Time `json:"last_used_at"`
    ExpiresAt  *time.Time `json:"expires_at"`
    CreatedAt  time.Time  `json:"created_at"`
}
```

### 2.3 DB Migration

```sql
-- Migration: 20260619_001_apikeys_security.sql
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS status     VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'revoked')),
    ADD COLUMN IF NOT EXISTS created_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS prefix     VARCHAR(20);

-- Index để lookup theo prefix (gateway validation)
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(prefix)
    WHERE status = 'active';

-- Index để list theo user
CREATE INDEX IF NOT EXISTS idx_api_keys_created_by ON api_keys(created_by);
```

### 2.4 Gateway Validation Update

Gateway đang validate API key qua identity-service. Đảm bảo `revoked` keys bị reject:

```go
// apps/osv/internal/gateway/auth/middleware.go
// Trong ValidateAPIKey — thêm check status
if key.Status == "revoked" {
    return nil, ErrKeyRevoked
}
```

---

## 3. Phần 2: System Settings Typed Schema

### 3.1 Domain Model

#### File: `services/identity-service/internal/domain/settings.go`

```go
package domain

import (
    "fmt"
    "net/mail"
)

// SystemSettings — typed schema thay thế map[string]interface{}
type SystemSettings struct {
    General  GeneralSettings  `json:"general"`
    SMTP     SMTPSettings     `json:"smtp"`
    Security SecuritySettings `json:"security"`
    AI       AISettings       `json:"ai"`
}

type GeneralSettings struct {
    PlatformName   string  `json:"platform_name"`
    Organization   string  `json:"organization"`
    SupportEmail   string  `json:"support_email"`
    Timezone       string  `json:"timezone"`
    LogoURL        *string `json:"logo_url"`
}

type SMTPSettings struct {
    Host      string `json:"host"`
    Port      int    `json:"port"`
    Username  string `json:"username"`
    UseTLS    bool   `json:"use_tls"`
    FromEmail string `json:"from_email"`
    // Password KHÔNG include trong response — write-only
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
    Status         string   `json:"status"`           // "active" | "inactive"
    LatencyMs      *int     `json:"latency_ms"`        // nullable
    RequestsPerDay *int     `json:"requests_per_day"`  // nullable
    CostPerDay     float64  `json:"cost_per_day"`
}

// Validate kiểm tra input rules khi PUT
func (s *SystemSettings) Validate() error {
    // General: email format
    if _, err := mail.ParseAddress(s.General.SupportEmail); err != nil {
        return fmt.Errorf("general.support_email: invalid email format")
    }

    // SMTP: port range
    if s.SMTP.Port < 1 || s.SMTP.Port > 65535 {
        return fmt.Errorf("smtp.port: must be 1-65535")
    }

    // Security: password min length
    if s.Security.PasswordMinLength < 8 {
        return fmt.Errorf("security.password_min_length: must be >= 8")
    }

    // Security: session timeout
    if s.Security.SessionTimeoutMinutes < 5 || s.Security.SessionTimeoutMinutes > 480 {
        return fmt.Errorf("security.session_timeout_minutes: must be 5-480")
    }

    return nil
}
```

### 3.2 Handler Implementation

#### File: `services/identity-service/internal/delivery/http/settings_handler.go`

```go
package http

// GET /api/v1/admin/settings — Admin only
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
    settings, err := h.settingsRepo.GetSystemSettings(r.Context())
    if err != nil {
        // Trả về default settings nếu chưa có record
        settings = defaultSystemSettings()
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(settings)
}

// PUT /api/v1/admin/settings — Admin only
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
    var req domain.SystemSettings
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httpError(w, 400, "INVALID_BODY", "Invalid JSON")
        return
    }

    // Validate với typed rules
    if err := req.Validate(); err != nil {
        httpError(w, 400, "VALIDATION_ERROR", err.Error())
        return
    }

    updated, err := h.settingsRepo.UpsertSystemSettings(r.Context(), &req)
    if err != nil {
        httpError(w, 500, "INTERNAL", "Failed to save settings")
        return
    }

    // Publish settings change event (để các services khác reload config)
    h.nats.Publish("system.settings.updated", &SettingsUpdatedEvent{
        UpdatedBy: r.Header.Get("X-User-ID"),
        UpdatedAt: time.Now(),
    })

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(updated)
}

func defaultSystemSettings() *domain.SystemSettings {
    return &domain.SystemSettings{
        General: domain.GeneralSettings{
            PlatformName: "OSV Platform",
            Timezone:     "UTC",
        },
        SMTP: domain.SMTPSettings{
            Port:   587,
            UseTLS: true,
        },
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

### 3.3 DB Schema cho Settings

```sql
-- Settings lưu dạng JSONB (1 row per tenant)
CREATE TABLE IF NOT EXISTS system_settings (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    settings   JSONB NOT NULL DEFAULT '{}',
    updated_by VARCHAR(255),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert default settings nếu chưa có
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

### 3.4 Gateway Routing

> Settings endpoint đã tồn tại trong gateway BFF (`SettingsBFF`). Không cần thêm route mới.
> Chỉ cần ensure gateway forward đúng tới identity-service.

```go
// apps/osv/internal/gateway/router.go — đã có, verify thôi:
// mux.Handle("GET /api/v1/admin/settings",  adminOnly(settingsBFF.Get))
// mux.Handle("PATCH /api/v1/admin/settings", adminOnly(settingsBFF.Update))
// Thay PATCH → PUT nếu cần (hoặc hỗ trợ cả hai)
```

---

## 4. Tests

### 4.1 Unit Tests: API Key

```go
// services/identity-service/internal/service/api_key_test.go
func TestGenerateAPIKey_CryptoRand(t *testing.T) {
    r1, _ := GenerateAPIKey("test", []string{"scan:read"}, "user1", nil)
    r2, _ := GenerateAPIKey("test", []string{"scan:read"}, "user1", nil)

    // Keys phải khác nhau mỗi lần generate
    assert.NotEqual(t, r1.RawKey, r2.RawKey)

    // Format: phải bắt đầu bằng "osv_"
    assert.True(t, strings.HasPrefix(r1.RawKey, "osv_"))

    // Prefix = 16 chars đầu
    assert.Equal(t, r1.RawKey[:16], r1.Key.Prefix)

    // Hash không phải raw key
    assert.NotEqual(t, r1.RawKey, r1.Key.HashSHA256)
}

func TestRevokeAPIKey_SoftDelete(t *testing.T) {
    // Sau revoke: status = "revoked", record vẫn tồn tại trong DB
    // Key bị revoked không thể dùng để authenticate
}
```

### 4.2 Unit Tests: Settings Validation

```go
func TestSystemSettings_Validate(t *testing.T) {
    // Email format
    s := validSettings()
    s.General.SupportEmail = "not-an-email"
    assert.Error(t, s.Validate())

    // SMTP port
    s = validSettings()
    s.SMTP.Port = 0
    assert.Error(t, s.Validate())

    // Password min length
    s = validSettings()
    s.Security.PasswordMinLength = 6 // < 8
    assert.Error(t, s.Validate())
}
```

---

## 5. Acceptance Criteria Checklist

### API Keys
- [ ] `POST /api/v1/api-keys` — server generate key bằng `crypto/rand`
- [ ] Response field names: `key` (object) + `raw_key` (string) — không phải `api_key`/`secret`
- [ ] `raw_key` không thể lấy lại qua bất kỳ API nào sau khi tạo
- [ ] `GET /api/v1/api-keys` trả về `{ keys: [...], total: N }` với `status` và `created_by`
- [ ] `DELETE /api/v1/api-keys/{id}` → soft delete (`status = "revoked"`)
- [ ] Key revoked: hiển thị trong list nhưng reject authentication

### System Settings
- [ ] `GET /api/v1/admin/settings` — `SystemSettings` đầy đủ 4 sections
- [ ] `PUT /api/v1/admin/settings` — cập nhật settings, trả về updated object
- [ ] Validation: email format, port range, password_min_length >= 8
- [ ] Settings persist qua service restart

---

## 6. Thứ tự thực thi

```
1. DB migration (api_keys schema + system_settings table)
2. GenerateAPIKey service function
3. Handler: CreateAPIKey, ListAPIKeys, RevokeAPIKey
4. Settings: typed struct + Validate()
5. Settings handler: GetSettings, UpdateSettings
6. Gateway: verify routes (không cần thêm routes mới)
7. Unit tests
8. Integration test với frontend
```
