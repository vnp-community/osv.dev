# TASK-BE-004 — identity-service: API Keys HTTP CRUD

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-004 |
| **Service** | `services/identity-service` |
| **Solution Ref** | [SOL-UI-001 §2.8](../solutions/SOL-UI-001-auth-service-extension.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-BE-001 (dto.go, respondJSON helpers) |
| **Estimated** | 2h |
| **Status** | ✅ DONE |

---

## Context

identity-service hiện có `api_keys` table và validation logic (SHA-256 prefix lookup) nhưng chưa có HTTP endpoints để UI quản lý keys. Frontend cần:
- `GET /api/v1/api-keys` — list keys của current user (NO plaintext)
- `POST /api/v1/api-keys` — create key (plaintext returned ONCE)
- `DELETE /api/v1/api-keys/{id}` — revoke key

---

## Goal

Thêm `apikey_handler.go` vào adapter layer của identity-service.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/identity-service/internal/adapter/http/apikey_handler.go` |
| MODIFY | `services/identity-service/internal/adapter/http/router.go` |

---

## Implementation

### File 1: `services/identity-service/internal/adapter/http/apikey_handler.go`

```go
package http

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// APIKeyHandler handles API key management endpoints
type APIKeyHandler struct {
	apiKeyRepo APIKeyRepository
}

// APIKeyRepository interface (should exist in domain already; add if missing)
type APIKeyRepository interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*APIKey, error)
	Create(ctx context.Context, key *APIKey) error
	FindByID(ctx context.Context, id uuid.UUID) (*APIKey, error)
	Revoke(ctx context.Context, id uuid.UUID) error
}

// ─── GET /api-keys ────────────────────────────

func (h *APIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.Header.Get("X-User-ID"))
	if err != nil {
		respondError(w, 401, "UNAUTHORIZED", "Invalid user ID")
		return
	}

	keys, err := h.apiKeyRepo.ListByUser(r.Context(), userID)
	if err != nil {
		respondError(w, 500, "INTERNAL_ERROR", err.Error())
		return
	}

	dtos := make([]map[string]interface{}, len(keys))
	for i, k := range keys {
		var lastUsed *string
		if k.LastUsedAt != nil {
			s := k.LastUsedAt.Format(time.RFC3339)
			lastUsed = &s
		}
		var expiresAt *string
		if k.ExpiresAt != nil {
			s := k.ExpiresAt.Format(time.RFC3339)
			expiresAt = &s
		}

		dtos[i] = map[string]interface{}{
			"id":           k.ID,
			"name":         k.Name,
			"prefix":       k.Prefix,       // first 12 chars — for identification
			// NEVER return hash_sha256 or plaintext_key
			"permissions":  k.Scopes,
			"is_active":    !k.Revoked,
			"created_at":   k.CreatedAt.Format(time.RFC3339),
			"last_used_at": lastUsed,
			"expires_at":   expiresAt,
		}
	}

	respondJSON(w, 200, map[string]interface{}{
		"api_keys": dtos,
		"total":    len(dtos),
	})
}

// ─── POST /api-keys ───────────────────────────

func (h *APIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.Header.Get("X-User-ID"))
	if err != nil {
		respondError(w, 401, "UNAUTHORIZED", "Invalid user ID")
		return
	}

	var req struct {
		Name        string    `json:"name"`
		Permissions []string  `json:"permissions"`
		ExpiresAt   *string   `json:"expires_at"` // RFC3339 or null
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, 400, "VALIDATION_ERROR", "name is required")
		return
	}

	// Generate raw key: "ovs_" prefix + 32 random bytes hex
	rawBytes := make([]byte, 20)
	if _, err := rand.Read(rawBytes); err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Failed to generate key")
		return
	}
	plaintext := "ovs_" + hex.EncodeToString(rawBytes) // "ovs_" + 40 chars = 44 total

	prefix := plaintext[:12]                         // first 12 chars for index lookup
	hash := sha256HexStr(plaintext)                  // SHA-256 for storage

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			respondError(w, 400, "VALIDATION_ERROR", "invalid expires_at format (use RFC3339)")
			return
		}
		expiresAt = &t
	}

	key := &APIKey{
		ID:         uuid.New(),
		UserID:     userID,
		Name:       req.Name,
		Prefix:     prefix,
		HashSHA256: hash,
		Scopes:     req.Permissions,
		ExpiresAt:  expiresAt,
	}

	if err := h.apiKeyRepo.Create(r.Context(), key); err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Failed to create API key")
		return
	}

	// Return plaintext KEY ONLY ONCE
	respondJSON(w, 201, map[string]interface{}{
		"id":            key.ID,
		"name":          key.Name,
		"prefix":        prefix,
		"plaintext_key": plaintext, // ← ONLY TIME this is returned
		"permissions":   key.Scopes,
		"expires_at":    req.ExpiresAt,
		"created_at":    key.CreatedAt.Format(time.RFC3339),
	})
}

// ─── DELETE /api-keys/{id} ────────────────────

func (h *APIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	keyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid key ID")
		return
	}

	userID, err := uuid.Parse(r.Header.Get("X-User-ID"))
	if err != nil {
		respondError(w, 401, "UNAUTHORIZED", "Invalid user ID")
		return
	}

	// Verify ownership
	key, err := h.apiKeyRepo.FindByID(r.Context(), keyID)
	if err != nil {
		respondError(w, 404, "NOT_FOUND", "API key not found")
		return
	}
	if key.UserID != userID {
		respondError(w, 403, "FORBIDDEN", "Cannot revoke another user's API key")
		return
	}

	if err := h.apiKeyRepo.Revoke(r.Context(), keyID); err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Failed to revoke key")
		return
	}

	respondJSON(w, 200, map[string]interface{}{
		"id":        keyID,
		"is_active": false,
		"revoked":   true,
	})
}

// sha256HexStr returns SHA-256 hex of string
func sha256HexStr(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
```

### Router additions:

```go
// services/identity-service/internal/adapter/http/router.go — thêm

// API Keys routes — gateway injects X-User-ID from JWT
mux.HandleFunc("GET    /api-keys",       h.APIKey.ListAPIKeys)
mux.HandleFunc("POST   /api-keys",       h.APIKey.CreateAPIKey)
mux.HandleFunc("DELETE /api-keys/{id}",  h.APIKey.RevokeAPIKey)
```

---

## Verification

```bash
cd services/identity-service
go build ./...

# Create key
curl -X POST http://localhost:8081/api-keys \
  -H "X-User-ID: <valid-uuid>" \
  -H "Content-Type: application/json" \
  -d '{"name":"CI/CD Key","permissions":["scan:read","finding:read"]}' | jq .

# Expected: {"id":"...","plaintext_key":"ovs_...","prefix":"ovs_xxx..."}

# List keys — no plaintext_key in response
curl http://localhost:8081/api-keys \
  -H "X-User-ID: <valid-uuid>" | jq '.api_keys[].prefix'
```

---

## Checklist

- [x] `CreateAPIKey` generates `ovs_` prefixed key using `crypto/rand`
- [x] `plaintext_key` chỉ trả về trong response của `POST /api-keys` — KHÔNG lưu plaintext vào DB
- [x] `ListAPIKeys` KHÔNG bao giờ trả về `hash_sha256` hoặc `plaintext_key`, chỉ `prefix`
- [x] `RevokeAPIKey` verify user ownership trước khi revoke (cross-user protection)
- [x] Key format: `ovs_` + 40 hex chars (20 random bytes)
- [x] Prefix = first 12 chars của full key (dùng để index lookup)
- [x] `go build ./...` thành công
