# TASK-GCV-012 — API Key CRUD Endpoints (gateway-service)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-012 |
| **Service** | `gateway-service` |
| **CR** | CR-GCV-008 |
| **Phase** | 1 — Core Pipeline |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-GCV-009 |

## Context

Thêm 3 HTTP endpoints vào `gateway-service` cho phép người dùng tự quản lý API keys của họ. Plain key chỉ được trả về duy nhất một lần khi tạo — sau đó chỉ có hash được lưu. Endpoints yêu cầu authentication (JWT hoặc API key với scope phù hợp).

## Reference

- Solution: [SOL-GCV-008](../solutions/SOL-GCV-008-api-gateway-enhancement.md) §2.6
- CR: [CR-GCV-008](../CR-GCV-008-api-gateway-enhancement.md) §4.3

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/delivery/http/apikey_handler.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/delivery/http/router.go
        (thêm routes /api/v2/api-keys)
```

**Đọc trước**: `gateway-service/internal/delivery/http/` để xác định cấu trúc router và helper functions hiện có.

## Implementation Spec

### apikey_handler.go

```go
// Package http — API Key CRUD handler for gateway-service.
package http

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/osv/gateway-service/internal/auth"
    "github.com/osv/gateway-service/internal/domain/entity"
    "github.com/osv/gateway-service/internal/domain/repository"
)

// APIKeyHandler handles API key management endpoints.
type APIKeyHandler struct {
    repo repository.APIKeyRepository
}

func NewAPIKeyHandler(repo repository.APIKeyRepository) *APIKeyHandler {
    return &APIKeyHandler{repo: repo}
}

// CreateAPIKeyRequest is the request body for POST /api/v2/api-keys.
type CreateAPIKeyRequest struct {
    Description string   `json:"description"` // required
    Scopes      []string `json:"scopes"`       // e.g. ["cve:read", "webhook:write"]
    ExpiresIn   *int     `json:"expires_in"`  // optional: days until expiry
}

// POST /api/v2/api-keys
// Returns plain key ONCE — not stored in DB.
func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
    claims := auth.ClaimsFromContext(r.Context())
    if claims == nil {
        respondError(w, http.StatusUnauthorized, "authentication required")
        return
    }

    var req CreateAPIKeyRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if req.Description == "" {
        respondError(w, http.StatusBadRequest, "description is required")
        return
    }
    if len(req.Scopes) == 0 {
        req.Scopes = []string{entity.ScopeCVERead} // default scope
    }

    // Validate scopes
    for _, s := range req.Scopes {
        if !isValidScope(s) {
            respondError(w, http.StatusBadRequest, "invalid scope: "+s)
            return
        }
    }

    // Generate cryptographically secure key
    rawBytes := make([]byte, 32)
    if _, err := rand.Read(rawBytes); err != nil {
        respondError(w, http.StatusInternalServerError, "key generation failed")
        return
    }
    plainKey := "gcve_" + base64.URLEncoding.EncodeToString(rawBytes)

    keyHash := sha256Hex(plainKey)

    apiKey := &entity.APIKey{
        ID:          uuid.New().String(),
        KeyHash:     keyHash,
        OwnerID:     claims.UserID,
        Description: req.Description,
        Scopes:      req.Scopes,
        IsActive:    true,
        CreatedAt:   time.Now().UTC(),
    }

    if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
        exp := time.Now().UTC().AddDate(0, 0, *req.ExpiresIn)
        apiKey.ExpiresAt = &exp
    }

    if err := h.repo.Save(r.Context(), apiKey); err != nil {
        respondError(w, http.StatusInternalServerError, "failed to create api key")
        return
    }

    // Return plain key ONCE — this is the only time it's shown
    resp := map[string]interface{}{
        "id":          apiKey.ID,
        "key":         plainKey, // ONLY ONCE!
        "description": apiKey.Description,
        "scopes":      apiKey.Scopes,
        "created_at":  apiKey.CreatedAt.Format(time.RFC3339),
        "expires_at":  apiKey.ExpiresAt,
        "warning":     "Store this key securely — it will not be shown again.",
    }
    respondJSON(w, http.StatusCreated, resp)
}

// GET /api/v2/api-keys
// Lists owner's API keys — no plain keys returned.
func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
    claims := auth.ClaimsFromContext(r.Context())
    if claims == nil {
        respondError(w, http.StatusUnauthorized, "authentication required")
        return
    }

    keys, err := h.repo.ListByOwner(r.Context(), claims.UserID)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list api keys")
        return
    }

    type keyResponse struct {
        ID          string     `json:"id"`
        Description string     `json:"description"`
        Scopes      []string   `json:"scopes"`
        LastUsedAt  *time.Time `json:"last_used_at"`
        ExpiresAt   *time.Time `json:"expires_at"`
        CreatedAt   time.Time  `json:"created_at"`
    }

    result := make([]keyResponse, len(keys))
    for i, k := range keys {
        result[i] = keyResponse{
            ID:          k.ID,
            Description: k.Description,
            Scopes:      k.Scopes,
            LastUsedAt:  k.LastUsedAt,
            ExpiresAt:   k.ExpiresAt,
            CreatedAt:   k.CreatedAt,
        }
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{"api_keys": result})
}

// DELETE /api/v2/api-keys/{id}
// Revokes (soft-deletes) an API key.
func (h *APIKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
    claims := auth.ClaimsFromContext(r.Context())
    if claims == nil {
        respondError(w, http.StatusUnauthorized, "authentication required")
        return
    }

    keyID := chi.URLParam(r, "id")
    if err := h.repo.Revoke(r.Context(), keyID, claims.UserID); err != nil {
        if errors.Is(err, repository.ErrAPIKeyNotFound) {
            respondError(w, http.StatusNotFound, "api key not found")
            return
        }
        respondError(w, http.StatusInternalServerError, "failed to revoke api key")
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

var validScopes = map[string]bool{
    entity.ScopeCVERead:   true,
    entity.ScopeKEVRead:   true,
    entity.ScopeWebhook:   true,
    entity.ScopeSyncAdmin: true,
    entity.ScopeReadAll:   true,
}

func isValidScope(s string) bool { return validScopes[s] }
```

### router.go — ADD routes

Thêm vào router setup:

```go
// API Key management (gateway-service handles these locally, NOT proxied)
apiKeyHandler := http.NewAPIKeyHandler(apiKeyRepo)
r.Route("/api/v2/api-keys", func(r chi.Router) {
    r.Use(authMiddleware.Required) // Must be authenticated
    r.Get("/", apiKeyHandler.List)
    r.Post("/", apiKeyHandler.Create)
    r.Delete("/{id}", apiKeyHandler.Revoke)
})
```

## Acceptance Criteria

- [x] `POST /api/v2/api-keys` (authenticated) → 201 với `id`, `key` (plain), `scopes`, `warning` field
- [x] `POST /api/v2/api-keys` (unauthenticated) → 401
- [x] `POST /api/v2/api-keys` với empty `description` → 400
- [x] `POST /api/v2/api-keys` với scope `invalid:scope` → 400
- [x] Plain key format: `gcve_` prefix + 44 chars base64
- [x] `GET /api/v2/api-keys` → list chỉ owner's keys, không có `key` field (chỉ metadata)
- [x] `DELETE /api/v2/api-keys/{id}` của key thuộc owner → 204
- [x] `DELETE /api/v2/api-keys/{id}` của key thuộc owner khác → 404
- [x] Sau khi revoke, key không còn auth được nữa (Redis cache invalidated)
- [x] `expires_in: 30` → key expires sau 30 ngày
- [x] `go build ./...` pass không lỗi
