# Change Request 012: System Settings — Typed Schema & API Keys Schema Update

**Tạo:** 2026-06-19  
**Status:** New — Thay thế untyped `object` schema bằng `SystemSettings` typed schema + API key security fixes.  
**Nguồn:** openapi.yaml schemas `SystemSettings`, `APIKey`, `CreateAPIKeyResponse`  
**Target directory:** `specs/crs/v2/api-ui-v1/`

---

## 1. Bối cảnh

### 1.1 System Settings

`AdminSettings.tsx` sau khi fix hardcode gọi `GET /api/v1/admin/settings` để load cấu hình hiện tại và `PUT /api/v1/admin/settings` để save. Hiện tại endpoint tồn tại nhưng schema là `object` (untyped) — không có validation.

### 1.2 API Key Security Fix

`APIKeyManagement.tsx` hiện **generate key ở frontend bằng `Math.random()`** — đây là **lỗ hổng bảo mật nghiêm trọng**. Sau khi fix, frontend chỉ gửi metadata (name, scopes) và backend phải generate key bằng `crypto/rand`. Schema `CreateAPIKeyResponse` cũng thay đổi: `secret` → `raw_key`, `api_key` → `key`.

| Thay đổi | Endpoint | Service | Trạng thái |
|---|---|---|---|
| Schema upgrade | `GET /PUT /api/v1/admin/settings` | identity-service | ⚠️ Schema untyped |
| Security fix | `POST /api/v1/api-keys` backend generate | identity-service | 🔴 CRITICAL |
| Schema update | `APIKey` — thêm `status`, `created_by` | identity-service | ⚠️ Fields thiếu |
| Schema update | `CreateAPIKeyResponse` — rename fields | identity-service | ⚠️ Breaking change |
| New endpoint | `GET /api/v1/api-keys` với `APIKeysListResponse` | identity-service | ⚠️ Response wrapper |

---

## 2. Chi tiết Thay đổi

### 2.1 [CRITICAL 🔴] API Key Generation — Backend MUST Generate

**Vấn đề:** Frontend hiện dùng `Math.random()` để tạo key — không phải CSPRNG. Bất kỳ key nào được generate theo cách này đều có thể bị đoán.

**Yêu cầu bắt buộc:** Backend PHẢI generate API key bằng `crypto/rand`:

```go
// services/identity-service/internal/service/api_key.go
import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "fmt"
)

func GenerateAPIKey(name string, scopes []string) (prefix, rawKey, hashedKey string, err error) {
    // Generate 32 bytes cryptographically secure random
    b := make([]byte, 32)
    if _, err = rand.Read(b); err != nil {
        return
    }
    
    rawKey = fmt.Sprintf("osv_%s", base64.URLEncoding.EncodeToString(b)[:40])
    prefix = rawKey[:16]
    
    // Store ONLY the hash — never the raw key
    h := sha256.Sum256([]byte(rawKey))
    hashedKey = base64.StdEncoding.EncodeToString(h[:])
    
    return
}
```

**`POST /api/v1/api-keys` request body:**
```json
{
  "name": "CI/CD Pipeline",
  "scopes": ["scan:write", "finding:read"],
  "expires_at": "2026-12-31T00:00:00Z"
}
```

**`POST /api/v1/api-keys` response (`CreateAPIKeyResponse`):**
```json
{
  "key": {
    "id": "k-001",
    "name": "CI/CD Pipeline",
    "prefix": "osv_prod_xK7m",
    "scopes": ["scan:write", "finding:read"],
    "status": "active",
    "created_at": "2026-06-19T...",
    "created_by": "carol@company.com",
    "expires_at": "2026-12-31T00:00:00Z"
  },
  "raw_key": "osv_Kj8mN2pQr7xL9vWzY4uT6cBhFdG3s0eA"
}
```

> ⚠️ **`raw_key` chỉ trả về DUY NHẤT 1 LẦN.** Backend chỉ lưu `hashed_key` vào DB. Không thể recover sau này.

**Breaking change note:** Response field names thay đổi:
- `api_key` → `key`
- `secret` → `raw_key`

Frontend đã được cập nhật. Backend cần match mới.

---

### 2.2 [HIGH] `APIKey` Schema — Thêm `status` và `created_by`

`GET /api/v1/api-keys` trả về `APIKeysListResponse`:

```json
{
  "keys": [
    {
      "id": "k-001",
      "name": "CI/CD Pipeline",
      "prefix": "osv_prod_xK7m",
      "scopes": ["scan:write", "finding:read"],
      "status": "active",
      "created_at": "2026-06-01T00:00:00Z",
      "created_by": "carol@company.com",
      "last_used_at": "2026-06-19T03:00:00Z",
      "expires_at": "2026-12-31T00:00:00Z"
    }
  ],
  "total": 4
}
```

**DB schema update:**
```sql
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'revoked')),
    ADD COLUMN IF NOT EXISTS created_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS prefix VARCHAR(20);
```

**`DELETE /api/v1/api-keys/{id}`** nên là **soft delete** (set `status = 'revoked'`), không xóa record:
```go
func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    err := h.repo.UpdateAPIKey(r.Context(), id, APIKeyUpdate{Status: "revoked"})
    // ...
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
```

---

### 2.3 [HIGH] `GET/PUT /api/v1/admin/settings` — Typed Schema

Endpoint tồn tại nhưng response/request schema là `object` untyped. Cần upgrade lên `SystemSettings`:

**`GET /api/v1/admin/settings` response:**
```json
{
  "general": {
    "platform_name": "OSV Platform",
    "organization": "ACME Security Corp",
    "support_email": "security@acme.com",
    "timezone": "Asia/Ho_Chi_Minh",
    "logo_url": null
  },
  "smtp": {
    "host": "smtp.gmail.com",
    "port": 587,
    "username": "noreply@acme.com",
    "use_tls": true,
    "from_email": "noreply@acme.com"
  },
  "security": {
    "password_min_length": 12,
    "password_max_age_days": 90,
    "session_timeout_minutes": 60,
    "max_concurrent_sessions": 3,
    "mfa_required": false,
    "allow_oauth": true
  },
  "ai": {
    "active_provider_id": "openai",
    "providers": [
      {
        "id": "openai",
        "name": "OpenAI",
        "model": "gpt-4o",
        "status": "active",
        "latency_ms": 450,
        "requests_per_day": 1247,
        "cost_per_day": 3.42
      },
      {
        "id": "ollama",
        "name": "Ollama (Local)",
        "model": "llama3:8b",
        "status": "inactive",
        "latency_ms": null,
        "requests_per_day": null,
        "cost_per_day": 0
      }
    ]
  }
}
```

**Validation rules khi `PUT`:**
- `general.support_email`: valid email format
- `smtp.port`: 1-65535
- `security.password_min_length`: min 8
- `security.session_timeout_minutes`: 5-480

**Implementation:**
```go
// services/identity-service/internal/handler/settings.go
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
    settings, _ := h.repo.GetSystemSettings(r.Context())
    json.NewEncoder(w).Encode(settings)
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
    var req SystemSettings
    json.NewDecoder(r.Body).Decode(&req)
    
    if err := req.Validate(); err != nil {
        http.Error(w, err.Error(), 400)
        return
    }
    
    updated, _ := h.repo.UpsertSystemSettings(r.Context(), req)
    json.NewEncoder(w).Encode(updated)
}
```

---

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

### API Keys
1. `POST /api/v1/api-keys` với `{ name, scopes }` → server generate key bằng `crypto/rand` — KHÔNG từ frontend.
2. Response có `key` (object) và `raw_key` (string) — **không** phải `api_key` và `secret`.
3. Sau khi tạo, `raw_key` không thể lấy lại qua bất kỳ API nào.
4. `GET /api/v1/api-keys` trả về `{ keys: [...], total: N }` với `status` và `created_by` trong mỗi key.
5. `DELETE /api/v1/api-keys/{id}` set `status = "revoked"` — không xóa record.
6. Key có `status = "revoked"` vẫn hiển thị trong list nhưng không thể dùng để authenticate.

### System Settings
7. `GET /api/v1/admin/settings` trả về `SystemSettings` object đầy đủ 4 sections — HTTP 200.
8. `PUT /api/v1/admin/settings` với body đúng cập nhật settings — trả `SystemSettings` đã update.
9. `PUT /api/v1/admin/settings` với email sai format → HTTP 400.
10. `PUT /api/v1/admin/settings` với `password_min_length < 8` → HTTP 400.
11. Settings được lưu persistent (survive service restart).
