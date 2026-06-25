# SOL-008: Asset-Service & Product-Service — PATCH Method

> **Bugs giải quyết**: BUG-009 (PATCH Assets, PATCH Products)  
> **Service**: `services/asset-service` (port 8091), `services/product-service` (port 8089, via finding-service:8085)  
> **Architecture ref**: §3.13 Asset-Service, §3.12 Product-Service  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**Đã hoàn thành trong asset-service:**

| Fix | File | Trạng thái |
|---|---|---|
| `PatchAsset` handler → partial update | `internal/delivery/http/handlers.go` | ✅ Thêm mới (TASK-008) |
| `Patch()` method trong `UpdateAssetUseCase` | `internal/usecase/asset/update_asset.go` | ✅ Thêm mới (TASK-008) |
| `PATCH /assets/{id}` route registered | `internal/delivery/http/router.go` | ✅ Thêm mới (TASK-008) |
| `UpdateAssetUseCase` wired trong embedded | `embedded.go` | ✅ Fixed (TASK-008) |
| Gateway: `PATCH /api/v1/assets/{id}` | `apps/osv/internal/gateway/router.go` | ✅ Fixed (TASK-008) |

**Build verify**: `go build ./...` ✅ asset-service, apps/osv


---

## Phân Tích

Architecture §3.13 (Asset-Service) liệt kê routes:
```
PUT /api/v1/assets/{id}       → Update asset (FULL update)
PUT /api/v1/assets/{id}/tags  → Update tags
```

Routes trong architecture dùng **PUT** (full update), nhưng UI gửi **PATCH** (partial update). Đây là **method mismatch** giữa frontend convention và backend implementation.

**Hai cách giải quyết**:
1. **Option A** (Preferred): Thêm PATCH handler vào service — xử lý partial update (chỉ update fields có trong body).
2. **Option B**: Đồng bộ UI dùng PUT + gửi full object.

→ **Chọn Option A**: PATCH là chuẩn REST cho partial update, phù hợp hơn cho trường hợp chỉ cập nhật một vài field.

---

## Asset-Service: PATCH /api/v1/assets/{id}

```go
// services/asset-service/internal/delivery/http/asset_handler.go

// PATCH /api/v1/assets/{id} — partial update
func (h *AssetHandler) PartialUpdate(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    assetID, err := uuid.Parse(id)
    if err != nil {
        respondError(w, http.StatusBadRequest, "invalid asset ID")
        return
    }
    
    // Parse partial update — only update fields present in body
    var req AssetPatchRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, err)
        return
    }
    
    // Get existing asset
    asset, err := h.assetUC.GetByID(r.Context(), assetID)
    if err != nil {
        respondError(w, http.StatusNotFound, err)
        return
    }
    
    // Apply only non-nil fields from patch request
    if req.Name != nil        { asset.Name = *req.Name }
    if req.Tags != nil        { asset.Tags = req.Tags }
    if req.Criticality != nil { asset.Criticality = *req.Criticality }
    if req.Owner != nil       { asset.Owner = *req.Owner }
    if req.OS != nil          { asset.OS = *req.OS }
    if req.OSVersion != nil   { asset.OSVersion = *req.OSVersion }
    if req.Description != nil { asset.Description = *req.Description }
    
    if err := h.assetUC.Update(r.Context(), asset); err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, asset)
}

// Request với con trỏ (pointer) để phân biệt "field absent" vs "field = zero value"
type AssetPatchRequest struct {
    Name        *string   `json:"name,omitempty"`
    Tags        []string  `json:"tags,omitempty"`
    Criticality *string   `json:"criticality,omitempty"` // "critical", "high", "medium", "low"
    Owner       *string   `json:"owner,omitempty"`
    OS          *string   `json:"os,omitempty"`
    OSVersion   *string   `json:"os_version,omitempty"`
    Description *string   `json:"description,omitempty"`
}
```

```go
// services/asset-service/internal/delivery/http/router.go

// Thêm PATCH handler bên cạnh PUT đã có
r.GET("/api/v1/assets/tags",         authMiddleware(h.GetTags))     // TRƯỚC
r.GET("/api/v1/assets",              authMiddleware(h.List))
r.POST("/api/v1/assets",             authMiddleware(h.Create))
r.GET("/api/v1/assets/{id}",         authMiddleware(h.GetByID))
r.PUT("/api/v1/assets/{id}",         authMiddleware(h.FullUpdate))   // Giữ nguyên
r.PATCH("/api/v1/assets/{id}",       authMiddleware(h.PartialUpdate)) // THÊM MỚI
r.PUT("/api/v1/assets/{id}/tags",    authMiddleware(h.UpdateTags))
r.GET("/api/v1/assets/{id}/findings", authMiddleware(h.GetFindings))
r.GET("/api/v1/assets/{id}/risk",    authMiddleware(h.GetRisk))
r.GET("/api/v1/assets/{id}/history", authMiddleware(h.GetHistory))
```

---

## Product-Service / Finding-Service: PATCH /api/v1/products/{id}

Products được quản lý trong finding-service (port 8085) hoặc product-service (port 8089). Theo architecture §3.5, finding-service đang quản lý Product domain.

```go
// services/finding-service/internal/delivery/http/product_handler.go

// PATCH /api/v1/products/{id}
func (h *ProductHandler) PartialUpdate(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    productID, err := uuid.Parse(id)
    if err != nil {
        respondError(w, http.StatusBadRequest, "invalid product ID")
        return
    }
    
    var req ProductPatchRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, err)
        return
    }
    
    product, err := h.productUC.GetByID(r.Context(), productID)
    if err != nil {
        respondError(w, http.StatusNotFound, err)
        return
    }
    
    // Apply partial update
    if req.Name != nil            { product.Name = *req.Name }
    if req.Description != nil     { product.Description = *req.Description }
    if req.TeamManager != nil     { product.TeamManager = *req.TeamManager }
    if req.BusinessCriticality != nil { product.BusinessCriticality = *req.BusinessCriticality }
    if req.Lifecycle != nil       { product.Lifecycle = *req.Lifecycle }
    if req.Tags != nil            { product.Tags = req.Tags }
    
    if err := h.productUC.Update(r.Context(), product); err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    // Publish audit event
    h.nats.PublishJSON("product.updated", map[string]string{
        "product_id": productID.String(),
        "actor_id":   r.Header.Get("X-User-ID"),
    })
    
    respondJSON(w, http.StatusOK, product)
}

type ProductPatchRequest struct {
    Name                *string  `json:"name,omitempty"`
    Description         *string  `json:"description,omitempty"`
    TeamManager         *string  `json:"team_manager,omitempty"`
    BusinessCriticality *string  `json:"business_criticality,omitempty"`
    Lifecycle           *string  `json:"lifecycle,omitempty"` // "production", "staging", "development"
    Tags                []string `json:"tags,omitempty"`
}
```

```go
// services/finding-service/internal/delivery/http/router.go

// CRITICAL: static routes TRƯỚC dynamic routes
r.GET("/api/v1/products/grades", authMiddleware(h.GetGrades))  // BUG-010 — TRƯỚC
r.GET("/api/v1/products/types",  authMiddleware(h.GetTypes))
r.GET("/api/v1/products",        authMiddleware(h.List))
r.POST("/api/v1/products",       authMiddleware(h.Create))
r.GET("/api/v1/products/{id}",   authMiddleware(h.GetByID))
r.PUT("/api/v1/products/{id}",   authMiddleware(h.FullUpdate))   // Giữ nguyên
r.PATCH("/api/v1/products/{id}", authMiddleware(h.PartialUpdate)) // THÊM MỚI
r.GET("/api/v1/products/{id}/engagements", authMiddleware(h.GetEngagements))
```

---

## Admin User Detail: GET /api/v1/admin/users/{id} (BUG-015)

Cùng identity-service. Route conflict vì `GET /admin/users/invite` và `POST /admin/users/invite` share prefix:

```go
// services/identity-service/internal/delivery/http/admin_handler.go

// PHẢI đăng ký static paths TRƯỚC
r.GET("/api/v1/admin/users",           adminMiddleware(h.ListUsers))    // Đã có — hoạt động
r.POST("/api/v1/admin/users/invite",   adminMiddleware(h.InviteUser))   // Đã có
r.GET("/api/v1/admin/users/{id}",      adminMiddleware(h.GetUser))      // THÊM/FIX

// Check nếu đang conflict với:
// POST /admin/users/{id}/unlock — path pattern overlap với /admin/users/{id}?
// Cần verify router implementation
```

**Nếu server dùng middleware-based router** (gorilla/mux, chi):

```go
// Với chi router:
r.Route("/api/v1/admin/users", func(r chi.Router) {
    r.Get("/",          h.ListUsers)        // GET /admin/users
    r.Post("/invite",   h.InviteUser)       // POST /admin/users/invite
    r.Route("/{id}", func(r chi.Router) {
        r.Get("/",               h.GetUser)          // GET /admin/users/{id}  ← FIX
        r.Post("/unlock",        h.UnlockUser)
        r.Post("/reset-password", h.ResetPassword)
    })
})
```

**Với Go stdlib ServeMux** (Go 1.22+):
```go
// Go 1.22 ưu tiên exact match và longer patterns
mux.Handle("GET /api/v1/admin/users",            adminOnly(h.ListUsers))
mux.Handle("POST /api/v1/admin/users/invite",    adminOnly(h.InviteUser))
mux.Handle("GET /api/v1/admin/users/{id}",       adminOnly(h.GetUser))      // FIX
mux.Handle("POST /api/v1/admin/users/{id}/unlock", adminOnly(h.UnlockUser))
```
