# TASK-CR012-P0-01 — API Key: Backend Generation bằng `crypto/rand`

**Phase:** Phase 1 — CRITICAL Security Fix  
**Nguồn giải pháp:** [`solutions/SOL-012 §2`](../solutions/SOL-012-apikey-security-system-settings.md)  
**Ưu tiên:** 🔴 P0 CRITICAL — Frontend dùng `Math.random()` = lỗ hổng bảo mật nghiêm trọng  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19  

---

## Mục tiêu

Thay thế toàn bộ flow tạo API key từ frontend-generated sang backend-generated bằng `crypto/rand`. Frontend chỉ gửi metadata (name, scopes), backend generate key an toàn.

---

## Điều tra trước khi code

```bash
# 1. Tìm handler hiện tại cho POST /api/v1/api-keys
grep -r "api-keys\|api_keys\|CreateAPIKey\|apikey" \
  services/identity-service/ --include="*.go" -l

# 2. Xem handler file
grep -r "POST.*api-keys\|CreateAPIKey" \
  services/identity-service/ --include="*.go" -n

# 3. Xem DB schema hiện tại
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "\d api_keys"
```

---

## Bước 1: DB Migration

**File mới:** `services/identity-service/migrations/20260619_001_apikeys_security.sql`

```sql
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS status     VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'revoked')),
    ADD COLUMN IF NOT EXISTS created_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS prefix     VARCHAR(20);

CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(prefix)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_api_keys_created_by ON api_keys(created_by);
```

```bash
# Chạy migration
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -f /migrations/20260619_001_apikeys_security.sql

# Verify
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "\d api_keys"
# Expected: có columns status, created_by, prefix
```

---

## Bước 2: Service Function — GenerateAPIKey

**Tìm hoặc tạo file:**
```bash
# Tìm service layer
grep -r "GenerateAPIKey\|CreateKey\|NewAPIKey" \
  services/identity-service/ --include="*.go" -l
```

**File:** `services/identity-service/internal/service/api_key.go`

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

type APIKeyCreateResult struct {
    Key    *APIKeyRecord
    RawKey string // Trả về 1 lần duy nhất — KHÔNG lưu DB
}

type APIKeyRecord struct {
    ID         uuid.UUID  `db:"id"`
    Name       string     `db:"name"`
    Prefix     string     `db:"prefix"`
    HashSHA256 string     `db:"hash_sha256"`
    Scopes     []string   `db:"scopes"`
    Status     string     `db:"status"`
    CreatedBy  string     `db:"created_by"`
    LastUsedAt *time.Time `db:"last_used_at"`
    ExpiresAt  *time.Time `db:"expires_at"`
    CreatedAt  time.Time  `db:"created_at"`
}

// GenerateAPIKey — dùng crypto/rand, KHÔNG Math.random()
func GenerateAPIKey(name string, scopes []string, createdBy string, expiresAt *time.Time) (*APIKeyCreateResult, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return nil, fmt.Errorf("generate random bytes: %w", err)
    }

    rawKey := fmt.Sprintf("osv_%s", base64.URLEncoding.EncodeToString(b)[:40])
    prefix := rawKey[:16]

    h := sha256.Sum256([]byte(rawKey))
    hashedKey := base64.StdEncoding.EncodeToString(h[:])

    key := &APIKeyRecord{
        ID:         uuid.New(),
        Name:       name,
        Prefix:     prefix,
        HashSHA256: hashedKey,
        Scopes:     scopes,
        Status:     "active",
        CreatedBy:  createdBy,
        ExpiresAt:  expiresAt,
        CreatedAt:  time.Now().UTC(),
    }

    return &APIKeyCreateResult{Key: key, RawKey: rawKey}, nil
}
```

---

## Bước 3: Update Handler

**Tìm handler hiện tại:**
```bash
grep -r "POST.*api-keys\|func.*CreateAPIKey\|func.*Create.*Key" \
  services/identity-service/ --include="*.go" -n
```

**Sửa/tạo handler** trong file handler hiện tại:

```go
// POST /api/v1/api-keys
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name      string     `json:"name"`
        Scopes    []string   `json:"scopes"`
        ExpiresAt *time.Time `json:"expires_at"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, 400, "invalid request body")
        return
    }
    if req.Name == "" || len(req.Scopes) == 0 {
        respondError(w, 400, "name and scopes are required")
        return
    }

    createdBy := r.Header.Get("X-User-Email")
    if createdBy == "" {
        createdBy = r.Header.Get("X-User-ID")
    }

    result, err := service.GenerateAPIKey(req.Name, req.Scopes, createdBy, req.ExpiresAt)
    if err != nil {
        respondError(w, 500, "failed to generate API key")
        return
    }

    if err := h.apiKeyRepo.Create(r.Context(), result.Key); err != nil {
        respondError(w, 500, "failed to persist API key")
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "key":     toAPIKeyDTO(result.Key),
        "raw_key": result.RawKey, // 1 lần duy nhất
    })
}

// DELETE /api/v1/api-keys/{id} — soft delete
func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id") // Go 1.22+, hoặc chi.URLParam(r, "id")
    if err := h.apiKeyRepo.Revoke(r.Context(), id); err != nil {
        respondError(w, 500, "failed to revoke API key")
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

type APIKeyDTO struct {
    ID         string     `json:"id"`
    Name       string     `json:"name"`
    Prefix     string     `json:"prefix"`
    Scopes     []string   `json:"scopes"`
    Status     string     `json:"status"`
    CreatedBy  string     `json:"created_by"`
    LastUsedAt *time.Time `json:"last_used_at"`
    ExpiresAt  *time.Time `json:"expires_at"`
    CreatedAt  time.Time  `json:"created_at"`
}

func toAPIKeyDTO(k *service.APIKeyRecord) APIKeyDTO {
    return APIKeyDTO{
        ID:        k.ID.String(),
        Name:      k.Name,
        Prefix:    k.Prefix,
        Scopes:    k.Scopes,
        Status:    k.Status,
        CreatedBy: k.CreatedBy,
        ExpiresAt: k.ExpiresAt,
        CreatedAt: k.CreatedAt,
    }
}
```

---

## Bước 4: Update Repository

```bash
# Tìm repo hiện tại
grep -r "APIKey\|api_key" \
  services/identity-service/internal/infra/ --include="*.go" -l
```

Thêm/sửa các method:

```go
// Repo.Create — lưu key (không lưu raw key)
func (r *APIKeyRepo) Create(ctx context.Context, k *service.APIKeyRecord) error {
    _, err := r.db.Exec(ctx, `
        INSERT INTO api_keys (id, name, prefix, hash_sha256, scopes, status, created_by, expires_at, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, k.ID, k.Name, k.Prefix, k.HashSHA256, k.Scopes, k.Status, k.CreatedBy, k.ExpiresAt, k.CreatedAt)
    return err
}

// Repo.Revoke — soft delete
func (r *APIKeyRepo) Revoke(ctx context.Context, id string) error {
    _, err := r.db.Exec(ctx,
        `UPDATE api_keys SET status = 'revoked' WHERE id = $1`, id)
    return err
}
```

---

## Bước 5: GET /api/v1/api-keys

```go
// GET /api/v1/api-keys — list user's keys
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    keys, err := h.apiKeyRepo.ListByUser(r.Context(), userID)
    if err != nil {
        respondError(w, 500, "failed to list API keys")
        return
    }

    dtos := make([]APIKeyDTO, len(keys))
    for i, k := range keys {
        dtos[i] = toAPIKeyDTO(&k)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "keys":  dtos,
        "total": len(dtos),
    })
}
```

---

## Bước 6: Verify Gateway Route

```bash
# Kiểm tra routes đã đăng ký chưa
grep -n "api-keys\|api_keys" apps/osv/internal/gateway/router.go
```

Nếu chưa có, thêm vào router:
```go
// identity-service routes
mux.Handle("GET /api/v1/api-keys",        protected(proxy.Forward("identity-service:8081")))
mux.Handle("POST /api/v1/api-keys",       protected(proxy.Forward("identity-service:8081")))
mux.Handle("DELETE /api/v1/api-keys/{id}", protected(proxy.Forward("identity-service:8081")))
```

---

## Acceptance Criteria

- [ ] `POST /api/v1/api-keys` với `{ name, scopes }` → server generate key bằng `crypto/rand`
- [ ] Response có `key` (object) và `raw_key` (string) — **không** phải `api_key`/`secret`
- [ ] `raw_key` không thể lấy lại sau khi tạo
- [ ] `GET /api/v1/api-keys` → `{ keys: [...], total: N }` với `status` và `created_by`
- [ ] `DELETE /api/v1/api-keys/{id}` → `status = "revoked"` trong DB (record vẫn còn)

## Verification

```bash
TOKEN="<your-jwt-token>"

# Tạo API key — backend generate
curl -s -X POST https://c12.openledger.vn/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Key","scopes":["scan:read"]}' | jq .
# Expected: { "key": {...}, "raw_key": "osv_..." }
# raw_key phải bắt đầu bằng "osv_"

# List keys
curl -s https://c12.openledger.vn/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" | jq '.keys[0] | {status, created_by}'
# Expected: { "status": "active", "created_by": "..." }

# Revoke
curl -s -X DELETE https://c12.openledger.vn/api/v1/api-keys/<id> \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: { "success": true }
```
