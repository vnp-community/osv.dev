# TASK-012 — identity-service: Verify & Implement API Keys List

**Bug**: [BUG-011](../BUG-011-integrations-api-keys.md)  
**Solution**: [SOL-010](../solutions/SOL-010-api-keys.md)  
**Priority**: 🟡 P2  
**Effort**: ~20 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/api-keys` trả `404`. Gateway đã implement ForwardRewrite `→ /api/v1/auth/api-keys`. Cần kiểm tra:
1. `ForwardRewrite` có hoạt động đúng không
2. identity-service có handler cho `GET /api/v1/auth/api-keys` không

---

## Bước 1 — Verify ForwardRewrite trong gateway

**Test trực tiếp** identity-service (bypass gateway):

```bash
curl -s -H "Authorization: Bearer <token>" \
  "http://identity-service:8081/api/v1/auth/api-keys"
```

- Nếu `200` → identity-service đã có handler, chỉ cần fix ForwardRewrite
- Nếu `404` → cần implement handler trong identity-service (xem bước 2)

---

## Bước 2 — Kiểm tra proxy.go ForwardRewrite

**File**: `apps/osv/internal/gateway/proxy.go`

```bash
grep -n "ForwardRewrite" /Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/proxy.go
```

**Đảm bảo** implementation thực sự rewrite path:

```go
func (p *ReverseProxy) ForwardRewrite(target, fromPath, toPath string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        r = r.Clone(r.Context())
        // Rewrite: thay fromPath bằng toPath
        if strings.HasPrefix(r.URL.Path, fromPath) {
            r.URL.Path = toPath + r.URL.Path[len(fromPath):]
        }
        r.RequestURI = r.URL.RequestURI()

        upstreamURL := "http://" + target + r.URL.RequestURI()
        req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
        if err != nil {
            http.Error(w, "proxy error", http.StatusBadGateway)
            return
        }
        req.Header = r.Header.Clone()
        // ... forward
    }
}
```

---

## Bước 3 — Implement trong identity-service (nếu chưa có)

**Tìm** API key handler trong identity-service:

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service/internal -name "*.go" \
  | xargs grep -l "api.key\|apikey\|api_key" | head -5
```

**Thêm** handler nếu chưa có:

```go
// GET /api/v1/auth/api-keys
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

    // Redact: chỉ trả prefix, không trả full key
    responses := make([]APIKeyResponse, 0, len(keys))
    for _, k := range keys {
        responses = append(responses, APIKeyResponse{
            ID:         k.ID.String(),
            Name:       k.Name,
            Prefix:     k.Prefix,          // first 12 chars, e.g. "osv_abc123de"
            Scopes:     k.Scopes,
            ExpiresAt:  formatTime(k.ExpiresAt),
            Revoked:    k.Revoked,
            CreatedAt:  k.CreatedAt.Format(time.RFC3339),
            LastUsedAt: formatTime(k.LastUsedAt),
        })
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":  responses,
        "total": len(responses),
    })
}
```

**Register route** trong identity-service router:

```go
r.Get("/api/v1/auth/api-keys", apiKeyHandler.List)
r.Post("/api/v1/auth/api-keys", apiKeyHandler.Create)
r.Delete("/api/v1/auth/api-keys/{id}", apiKeyHandler.Delete)
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/api-keys` (qua gateway) trả `200`
- [ ] Response không chứa full key value (chỉ prefix)
- [ ] `data` là array (kể cả `[]`)
- [ ] `go build ./...` trong identity-service không có lỗi

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service
go build ./...

curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/api-keys" | jq '{data_type: (.data | type), total}'
# Expected: {"data_type": "array", "total": N}
```
