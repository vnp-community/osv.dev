# SOL-004 — Fix Schema Mismatch: Auth, KEV, Assets, Products, Admin, Webhooks

| Trường | Giá trị |
|---|---|
| **Bugs** | BUG-BE-005, 006, 007, 008, 009, 012 |
| **Services** | `identity-service`, `data-service`, `finding-service`, `asset-service`, `notification-service` |
| **Priority** | P1 |
| **Estimated effort** | 6–10h |
| **Kiến trúc** | Tuân thủ [02-technical-design.md §2.2](../../../02-technical-design.md) — Clean Architecture, Adapter layer |

---

## Nguyên Tắc Chung

Tất cả các fix này đều ở **Adapter layer** (HTTP handler) — không chạm vào domain/usecase/infra, đúng Clean Architecture theo `technical-design.md §2.2`:

> _Adapter → HTTP handlers, NATS publishers/subscribers_

Chỉ thay đổi cách serialize response JSON. Logic nghiệp vụ giữ nguyên.

---

## FIX 1 — `GET /auth/me` thiếu `{ "user": {...} }` wrapper (BUG-BE-005)

**File**: `services/identity-service/internal/delivery/http/handlers.go`

Từ code thực tế (line 147–152):
```go
// Login handler trả đúng format, nhưng /me handler chưa có
```

Thêm handler `/auth/me` (hiện chưa thấy trong `handlers.go`):

```go
// services/identity-service/internal/delivery/http/handlers.go
// THÊM MỚI vào router (line 73-87 trong authenticated group):
r.Get("/auth/me", h.GetMe)

// THÊM handler:
// GetMe handles GET /api/v1/auth/me
// Returns: { "user": { id, email, name, role, permissions, mfa_enabled } }
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
    // Lấy user ID từ header do gateway inject (theo auth design)
    userID := r.Header.Get("X-User-ID")
    if userID == "" {
        jsonError(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    user, err := h.registerUC.GetByID(r.Context(), userID)
    if err != nil {
        h.handleError(w, err)
        return
    }

    // Wrap trong "user" key — đúng spec
    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "user": map[string]interface{}{
            "id":          user.ID,
            "email":       user.Email,
            "name":        user.Username, // hoặc user.FullName nếu có
            "role":        user.Role,
            "permissions": user.Permissions,
            "mfa_enabled": user.MFAEnabled,
            "created_at":  user.CreatedAt,
        },
    })
}
```

---

## FIX 2 — `POST /auth/logout` trả 200 → 204 (BUG-BE-006)

**File**: `services/identity-service/internal/delivery/http/handlers.go`

Từ code thực tế (line 180–183):
```go
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
    // Session ID and JTI come from auth middleware context
    w.WriteHeader(http.StatusNoContent)  // ← Đã đúng 204!
}
```

**Handler đã trả 204** — nhưng test nhận được 200. Nguyên nhân có thể:
1. Gateway hoặc middleware gọi `w.Write()` sau handler → override thành 200
2. Một Logout handler khác đang được dùng

**Fix**: Kiểm tra gateway không intercept response:
```go
// apps/osv/internal/gateway/router.go - line 68
// Route logout forward thẳng đến identity-service — OK, không cần sửa
mux.Handle("POST /api/v1/auth/logout", protected(proxy.Forward("identity-service:8081")))
```

Kiểm tra thực tế:
```bash
docker logs osv-backend-identity-service-1 --tail 50 | grep logout
# Nếu handler đang bị gọi và trả 200 → có middleware nào đó override
```

Nếu cần force 204:
```go
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
    // Invalidate session/token từ context
    if sessionID := r.Header.Get("X-Session-ID"); sessionID != "" {
        h.logoutUC.Execute(r.Context(), sessionID)
    }
    // Đảm bảo không write body
    w.Header().Set("Content-Length", "0")
    w.WriteHeader(http.StatusNoContent)
    // KHÔNG gọi w.Write() hay json.Encode()
}
```

---

## FIX 3 — KEV Response Schema Sai (BUG-BE-007)

**File**: `services/data-service/internal/delivery/http/kev_handler.go`

Từ code thực tế (line 101–121) — handler dùng `entries`, `total`, `limit`, `has_more`:
```go
type enrichedResponse struct {
    Entries interface{}            `json:"entries"`   // ← spec muốn "data"
    Total   int64                  `json:"total"`
    Page    int                    `json:"page"`
    Limit   int                    `json:"limit"`     // ← spec muốn "page_size"
    HasMore bool                   `json:"has_more"`  // ← không cần trong spec
    Stats   map[string]interface{} `json:"stats"`
}
```

**Fix** — thay đổi JSON field tags:
```go
// services/data-service/internal/delivery/http/kev_handler.go
// Line 101-121 — THAY ĐỔI enrichedResponse struct:

// SAU:
respondJSON(w, http.StatusOK, map[string]interface{}{
    "data":      resp.Entries,      // "entries" → "data"
    "total":     resp.Total,
    "page":      resp.Page,
    "page_size": resp.Limit,        // "limit" → "page_size"
    "stats": map[string]interface{}{
        "total":                   resp.Total,
        "added_last_30_days":      getAddedLast30Days(r.Context(), h.kevRepo),
        "ransomware_related":      getRansomwareCount(r.Context(), h.kevRepo),
        "unmitigated_in_platform": unmitigated,
    },
})
```

Thêm helper functions:
```go
// Đếm KEV entries được thêm trong 30 ngày qua
func getAddedLast30Days(ctx context.Context, repo repository.KEVRepository) int {
    count, err := repo.CountSince(ctx, time.Now().AddDate(0, 0, -30))
    if err != nil { return 0 }
    return int(count)
}

// Đếm KEV entries có ransomware
func getRansomwareCount(ctx context.Context, repo repository.KEVRepository) int {
    count, err := repo.CountRansomware(ctx)
    if err != nil { return 0 }
    return int(count)
}
```

**Fix GetStats handler** — trả đúng `{ stats, by_vendor, recent_additions }`:
```go
// kev_handler.go — GetStats (line 197-204)
func (h *KevHandler) GetStats(w http.ResponseWriter, r *http.Request) {
    stats, err := h.kevRepo.Stats(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, "internal error")
        return
    }
    // Wrap đúng spec: { stats: {...}, by_vendor: [...], recent_additions: [...] }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "stats":             stats,
        "by_vendor":         []interface{}{}, // TODO: query groupby vendor
        "recent_additions":  []interface{}{}, // TODO: query last 10 added
    })
}
```

---

## FIX 4 — Assets Response Schema Sai (BUG-BE-008)

**Service**: `services/asset-service` — cần tìm và sửa handler `GET /api/v1/assets`

```go
// services/asset-service/internal/delivery/http/asset_handler.go
func (h *AssetHandler) ListAssets(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    filter := AssetFilter{
        RiskLevel: q.Get("riskLevel"),
        Tags:      q["tags"],
        Page:      parseInt(q.Get("page"), 1),
        PageSize:  parseInt(q.Get("pageSize"), 20),
    }

    assets, total, err := h.assetRepo.List(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list assets")
        return
    }

    // FIX: Wrap trong { "assets": [...], "total": N }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "assets": assets,    // ← đổi từ flat object
        "total":  total,
    })
}
```

---

## FIX 5 — Products Response Schema Sai (BUG-BE-008)

**Service**: `services/finding-service` — handler `GET /api/v1/products`

```go
// services/finding-service/internal/delivery/http/product_handler.go

// GET /api/v1/products — FIX wrapper
func (h *ProductHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
    products, total, err := h.productRepo.List(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list products")
        return
    }

    // FIX: Wrap đúng spec { "products": [...], "total": N }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "products": products,   // ← không trả array trực tiếp
        "total":    total,
    })
}

// GET /api/v1/products/types — FIX trả string array
func (h *ProductHandler) GetTypes(w http.ResponseWriter, r *http.Request) {
    // Spec: { "types": ["web_application", "api", ...] }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "types": []string{
            "web_application",
            "api",
            "mobile",
            "infrastructure",
            "iot",
            "network",
        },
    })
    // Thay vì trả [{ "id": 1, "name": "web_application" }]
}
```

---

## FIX 6 — Admin Health: services Array → Map (BUG-BE-009)

**File**: `apps/osv/internal/gateway/bff/health.go`

```go
// apps/osv/internal/gateway/bff/health.go
// HandleAdminHealth phải trả services là MAP không phải ARRAY

func (bff *HealthBFF) HandleAdminHealth(w http.ResponseWriter, r *http.Request) {
    // ... ping các services ...
    
    // FIX: services phải là map { "redis": { status, latency_ms }, ... }
    services := map[string]interface{}{
        "postgres": map[string]interface{}{
            "status":     pingPostgres(bff),
            "latency_ms": measureLatency(pingPostgres),
        },
        "redis": map[string]interface{}{
            "status":     pingRedis(bff),
            "latency_ms": measureLatency(pingRedis),
        },
        "nats": map[string]interface{}{
            "status": pingNATS(bff),
        },
        "elasticsearch": map[string]interface{}{
            "status": pingES(),
        },
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "status":   overallStatus(services),
        "services": services,  // MAP, không phải ARRAY
    })
}
```

---

## FIX 7 — Admin Users: thiếu `name`, `mfa_enabled` (BUG-BE-009)

**Service**: `services/identity-service` — admin users handler

```go
// identity-service — admin users list handler
// THÊM name và mfa_enabled vào response DTO:

type AdminUserDTO struct {
    ID         string    `json:"id"`
    Email      string    `json:"email"`
    Name       string    `json:"name"`        // THÊM — map từ user.Username hoặc user.FullName
    Role       string    `json:"role"`
    IsActive   bool      `json:"is_active"`
    MFAEnabled bool      `json:"mfa_enabled"` // THÊM
    CreatedAt  time.Time `json:"created_at"`
}

func toAdminUserDTO(u *domain.User) AdminUserDTO {
    return AdminUserDTO{
        ID:         u.ID.String(),
        Email:      u.Email,
        Name:       u.Username,    // hoặc u.FullName
        Role:       string(u.Role),
        IsActive:   u.IsActive,
        MFAEnabled: u.TOTPEnabled, // map từ totp_enabled
        CreatedAt:  u.CreatedAt,
    }
}
```

---

## FIX 8 — Admin Roles: field name sai (BUG-BE-009)

**Service**: `services/identity-service` — admin roles handler

```go
// identity-service — admin roles handler
type RoleDTO struct {
    Name        string   `json:"name"`        // thay vì "role_name"
    Permissions []string `json:"permissions"` // thay vì "perms"
}

func (h *AdminHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
    roles := []RoleDTO{
        {Name: "admin",  Permissions: []string{"*"}},
        {Name: "analyst", Permissions: []string{"cve:read", "finding:read"}},
        {Name: "operator", Permissions: []string{"scan:create", "finding:write"}},
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{"roles": roles})
}
```

---

## FIX 9 — Webhooks trả Object thay vì Array (BUG-BE-012)

**Service**: `services/notification-service` — webhooks handler

```go
// notification-service — webhooks list handler
func (h *WebhookHandler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
    webhooks, err := h.webhookRepo.List(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list webhooks")
        return
    }

    // FIX: Trả ARRAY trực tiếp theo spec
    respondJSON(w, http.StatusOK, webhooks)
    // Thay vì: respondJSON(w, http.StatusOK, map[string]interface{}{"webhooks": webhooks})
}
```

---

## Xác Nhận Fix

```bash
# 1. auth/me — phải có wrapper:
curl -H "Authorization: Bearer <token>" https://c12.openledger.vn/api/v1/auth/me
# Expected: { "user": { "id": "...", "email": "...", "name": "...", ... } }

# 2. logout — phải là 204:
curl -v -X POST -H "Authorization: Bearer <token>" https://c12.openledger.vn/api/v1/auth/logout
# Expected: HTTP/1.1 204 No Content

# 3. KEV — phải có "data", "page_size":
curl https://c12.openledger.vn/api/v2/kev
# Expected: { "data": [...], "total": N, "page_size": 20, "stats": {...} }

# 4. assets — phải có wrapper:
curl -H "Authorization: Bearer <token>" https://c12.openledger.vn/api/v1/assets
# Expected: { "assets": [...], "total": N }

# 5. products — phải có wrapper:
curl -H "Authorization: Bearer <token>" https://c12.openledger.vn/api/v1/products
# Expected: { "products": [...], "total": N }

# 6. admin health — services phải là map:
curl -H "Authorization: Bearer <token>" https://c12.openledger.vn/api/v1/admin/health
# Expected: { "status": "healthy", "services": { "redis": {...}, ... } }

# 7. webhooks — phải là array:
curl -H "Authorization: Bearer <token>" https://c12.openledger.vn/api/v1/webhooks
# Expected: [ {...}, {...} ]
```

## Không Thay Đổi

- `apps/osv/internal/gateway/router.go` — routing đúng hết, KHÔNG sửa
- Domain và usecase layers — KHÔNG sửa
- Database schema — KHÔNG sửa
