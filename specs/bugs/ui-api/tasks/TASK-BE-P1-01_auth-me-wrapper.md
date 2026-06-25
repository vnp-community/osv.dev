# TASK-BE-P1-01 — Fix identity-service: auth/me wrapper + logout 204

**Phase:** Sprint 2 — P1 Schema Fixes  
**Nguồn giải pháp:** [`solutions/SOL-004_fix-schema-mismatches.md — FIX 1, 2`](../solutions/SOL-004_fix-schema-mismatches.md)  
**Ưu tiên:** 🟠 P1 — Header user menu không hiển thị  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19

---

## Mục tiêu

1. `GET /api/v1/auth/me` thiếu wrapper `{ "user": {...} }` → Frontend đọc `response.user` nhận `undefined`
2. `POST /api/v1/auth/logout` có thể trả 200 thay vì 204

---

## Files cần sửa

### [MODIFY] `services/identity-service/internal/delivery/http/handlers.go`

**File hiện tại**: [`services/identity-service/internal/delivery/http/handlers.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service/internal/delivery/http/handlers.go)

#### Fix 1 — Thêm `GetMe` handler (line 73-87 trong Router)

Tìm `/auth/me` handler:
```bash
grep -n "GetMe\|/auth/me\|auth/me" \
  services/identity-service/ -r --include="*.go"
```

Nếu chưa có handler, thêm vào `Router()` function (line 57-90):
```go
// THÊM vào authenticated group:
r.Group(func(r chi.Router) {
    r.Use(h.AuthMiddleware)
    // ... existing routes ...
    r.Get("/auth/me", h.GetMe)    // THÊM MỚI
})
```

Thêm handler function sau `Register`:
```go
// GetMe handles GET /api/v1/auth/me
// Returns: { "user": { id, email, name, role, permissions, mfa_enabled } }
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
    // Gateway inject X-User-ID sau khi xác thực JWT
    userID := r.Header.Get("X-User-ID")
    if userID == "" {
        jsonError(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    // Lookup user từ DB
    user, err := h.registerUC.GetByID(r.Context(), userID)
    if err != nil {
        jsonError(w, "user not found", http.StatusNotFound)
        return
    }

    // PHẢI wrap trong "user" key theo spec
    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "user": map[string]interface{}{
            "id":          user.ID,
            "email":       user.Email,
            "name":        user.Username,
            "role":        user.Role,
            "permissions": user.Permissions, // []string
            "mfa_enabled": user.TOTPEnabled,
            "created_at":  user.CreatedAt,
        },
    })
}
```

**Lưu ý**: Cần thêm `GetByID` vào `register.UseCase` nếu chưa có:
```bash
grep -r "GetByID\|FindByID" services/identity-service/ --include="*.go"
```

Nếu chưa có, thêm method:
```go
// services/identity-service/internal/usecase/register/usecase.go
func (uc *UseCase) GetByID(ctx context.Context, id string) (*domain.User, error) {
    uid, err := uuid.Parse(id)
    if err != nil {
        return nil, ErrInvalidID
    }
    return uc.userRepo.FindByID(ctx, uid)
}
```

#### Fix 2 — Verify Logout trả 204

Từ code hiện tại (line 180-183) — handler ĐÃ có `w.WriteHeader(http.StatusNoContent)`:
```go
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNoContent)
}
```

Nếu test vẫn nhận 200, nguyên nhân là middleware gọi `w.Write()` hoặc `json.Encode()` sau handler. Kiểm tra:

```bash
# Test trực tiếp identity-service (bypass gateway)
docker exec osv-backend-gateway-1 \
  curl -v -X POST http://identity-service:8081/auth/logout \
  -H "Authorization: Bearer <token>"
# Nếu 204 → gateway middleware override
# Nếu 200 → handler thực sự trả 200
```

Nếu `Logout` handler có code ghi body, sửa lại:
```go
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
    // Invalidate session (nếu có)
    if sessionID := r.Header.Get("X-Session-ID"); sessionID != "" {
        _ = h.logoutUC.Execute(r.Context(), sessionID)
    }
    // Đảm bảo không ghi body
    w.Header().Set("Content-Length", "0")
    w.WriteHeader(http.StatusNoContent)
    // KHÔNG gọi json.Encode() hoặc w.Write()
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/auth/me` trả HTTP 200 với `{ "user": { "id": "...", "email": "...", "name": "...", "role": "...", "mfa_enabled": false } }`
- [ ] Frontend `response.user` không còn là `undefined`
- [ ] `POST /api/v1/auth/logout` trả HTTP 204 No Content (không có body)

## Verification

```bash
TOKEN=$(curl -s -X POST https://c12.openledger.vn/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@openvulnscan.io","password":"<pass>"}' \
  | jq -r '.access_token')

# Test /auth/me — must have "user" wrapper
curl -H "Authorization: Bearer $TOKEN" \
  https://c12.openledger.vn/api/v1/auth/me
# Expected: { "user": { "id": "...", "email": "...", "name": "...", ... } }
# NOT:       { "id": "...", "email": "...", ... }

# Test logout — must be 204
curl -v -X POST -H "Authorization: Bearer $TOKEN" \
  https://c12.openledger.vn/api/v1/auth/logout
# Expected: HTTP/1.1 204 No Content
# NOT: HTTP/1.1 200 OK
```
