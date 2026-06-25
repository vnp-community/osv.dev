# SOL-010 — API Keys: Kiểm tra Route Rewrite (P2)

**Bug**: [BUG-011](../BUG-011-integrations-api-keys.md)  
**Service**: `identity-service`  
**Endpoint**: `GET /api/v1/api-keys`  
**HTTP Error**: `404 Not Found`

**Status**: `✅ Implemented` — via [TASK-012](../../tasks/TASK-012-*.md)

---

## Root Cause

Gateway đã implement **path rewrite** tại [`router.go:110`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L110):

```go
// GET /api/v1/api-keys → identity-service /api/v1/auth/api-keys
mux.Handle("GET /api/v1/api-keys", protected(proxy.ForwardRewrite(
    "identity-service:8081",
    "/api/v1/api-keys",
    "/api/v1/auth/api-keys",
)))
```

**Vấn đề nghi ngờ**: identity-service chưa implement `GET /api/v1/auth/api-keys`, hoặc `ForwardRewrite` không hoạt động đúng.

---

## Giải pháp

### Bước 1: Test route rewrite

```bash
# Test trực tiếp identity-service (bypass gateway)
curl -H "Authorization: Bearer <token>" \
  "http://identity-service:8081/api/v1/auth/api-keys"
# Nếu 404 → identity-service chưa implement
# Nếu 200 → gateway rewrite bị lỗi
```

### Bước 2: Kiểm tra ForwardRewrite implementation

```go
// apps/osv/internal/gateway/proxy.go
// ForwardRewrite phải rewrite path trước khi forward

func (p *ReverseProxy) ForwardRewrite(target, fromPath, toPath string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        r = r.Clone(r.Context())
        // Rewrite path: replace fromPath prefix với toPath
        newPath := strings.Replace(r.URL.Path, fromPath, toPath, 1)
        r.URL.Path = newPath
        r.RequestURI = r.URL.RequestURI()
        
        upstreamURL := "http://" + target + r.URL.RequestURI()
        // ... forward
    }
}
```

### Bước 3: Implement trong identity-service (nếu chưa có)

```go
// services/identity-service/... /api_key_handler.go

// List — GET /api/v1/auth/api-keys
func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    
    keys, err := h.repo.ListByUser(r.Context(), userID)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list api keys")
        return
    }
    
    if keys == nil {
        keys = make([]*APIKey, 0)
    }
    
    // Redact sensitive info
    responses := make([]APIKeyResponse, 0, len(keys))
    for _, k := range keys {
        responses = append(responses, APIKeyResponse{
            ID:        k.ID.String(),
            Name:      k.Name,
            Prefix:    k.Prefix,          // First 12 chars only
            Scopes:    k.Scopes,
            ExpiresAt: k.ExpiresAt,
            Revoked:   k.Revoked,
            CreatedAt: k.CreatedAt,
            LastUsedAt: k.LastUsedAt,
        })
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":  responses,
        "total": len(responses),
    })
}
```

---

## Response Schema

```json
// GET /api/v1/api-keys (hoặc /api/v1/auth/api-keys)
{
  "data": [
    {
      "id": "uuid",
      "name": "CI/CD Key",
      "prefix": "osv_abc123",
      "scopes": ["cve:read", "finding:write"],
      "expires_at": "2027-01-01T00:00:00Z",
      "revoked": false,
      "created_at": "2026-01-01T00:00:00Z",
      "last_used_at": "2026-06-19T12:00:00Z"
    }
  ],
  "total": 1
}
```

> **Note**: Field `key` (full key value) KHÔNG được trả trong List — chỉ trả khi tạo mới.
