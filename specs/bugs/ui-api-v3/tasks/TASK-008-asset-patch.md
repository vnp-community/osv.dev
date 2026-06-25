# TASK-008: Asset-Service — PATCH /api/v1/assets/{id}

> **Bug**: BUG-009  
> **Solution**: SOL-008  
> **Service**: `services/asset-service`  
> **File chính**: Asset service router & handler  
> **Priority**: 🟡 MEDIUM  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**Đã hoàn thành:**
- ✅ Tạo `UpdateAssetUseCase` với `Execute()` (PUT) và `Patch()` (PATCH) tại `internal/usecase/asset/update_asset.go`
- ✅ Thêm `UpdateAsset` (PUT) và `PatchAsset` (PATCH) handlers vào `handlers.go`
- ✅ Register `PUT /assets/{id}` và `PATCH /assets/{id}` trong `Router()`
- ✅ Wire `UpdateAssetUseCase` trong `embedded.go` qua `WithUpdateUC()`
- ✅ Thêm `PATCH /api/v1/assets/{id}` vào gateway router
- ✅ Build `go build ./...` thành công


## Phân Tích Thực Tế

**Gateway đã có** (router.go):
```go
mux.Handle("PUT /api/v1/assets/{id}",   protected(proxy.Forward("asset-service:8091")))
mux.Handle("DELETE /api/v1/assets/{id}", protected(proxy.Forward("asset-service:8091")))
// Nhưng KHÔNG có:
// PATCH /api/v1/assets/{id}  ← missing in gateway AND in asset-service
```

**Kiểm tra thực tế**:
```bash
find services/asset-service -name "*.go" | xargs grep -n "PATCH\|Patch\|patch\|PartialUpdate" 2>/dev/null
```

## Việc Cần Làm

### Bước 1: Khám phá cấu trúc asset-service

```bash
find services/asset-service -name "*.go" | head -20
find services/asset-service -name "*.go" | xargs grep -n "func.*Handler\|func.*Update\|func.*router\|Handle\|Route" 2>/dev/null | grep -v "_test" | head -30
```

### Bước 2: Kiểm tra asset handler hiện tại

```bash
find services/asset-service -name "*handler*" | xargs cat 2>/dev/null | head -100
```

### Bước 3: Thêm PATCH handler

File: `services/asset-service/internal/delivery/http/asset_handler.go` (hoặc tương đương)

```go
// AssetPatchRequest — pointer fields để phân biệt "không gửi" vs "gửi giá trị rỗng"
type AssetPatchRequest struct {
    Name        *string  `json:"name,omitempty"`
    Tags        []string `json:"tags,omitempty"`       // Ghi đè toàn bộ tags list
    Criticality *string  `json:"criticality,omitempty"` // "critical", "high", "medium", "low"
    Owner       *string  `json:"owner,omitempty"`
    OS          *string  `json:"os,omitempty"`
    OSVersion   *string  `json:"os_version,omitempty"`
    Description *string  `json:"description,omitempty"`
}

// PatchAsset handles PATCH /api/v1/assets/{id}
func (h *AssetHandler) PatchAsset(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    assetID, err := uuid.Parse(id)
    if err != nil {
        respondError(w, http.StatusBadRequest, "invalid asset ID")
        return
    }

    var req AssetPatchRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    // Fetch existing asset
    asset, err := h.assetUC.GetByID(r.Context(), assetID)
    if err != nil {
        if isNotFound(err) {
            respondError(w, http.StatusNotFound, "asset not found")
            return
        }
        respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    // Apply only non-nil fields
    if req.Name != nil        { asset.Name = *req.Name }
    if req.Criticality != nil { asset.Criticality = *req.Criticality }
    if req.Owner != nil       { asset.Owner = *req.Owner }
    if req.OS != nil          { asset.OS = *req.OS }
    if req.OSVersion != nil   { asset.OSVersion = *req.OSVersion }
    if req.Description != nil { asset.Description = *req.Description }
    if req.Tags != nil        { asset.Tags = req.Tags } // full replace

    updated, err := h.assetUC.Update(r.Context(), asset)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    respondJSON(w, http.StatusOK, updated)
}
```

### Bước 4: Register PATCH route

**Trong asset-service router** — thêm PATCH:

```go
// TRƯỚC /{id} wildcard patterns, hoặc cùng group:
r.Get("/api/v1/assets/tags", h.GetTags)   // literal trước
r.Get("/api/v1/assets",      h.List)
r.Post("/api/v1/assets",     h.Create)
r.Get("/api/v1/assets/{id}", h.GetByID)
r.Put("/api/v1/assets/{id}", h.FullUpdate)     // Giữ nguyên
r.Patch("/api/v1/assets/{id}", h.PatchAsset)   // THÊM MỚI
r.Delete("/api/v1/assets/{id}", h.Delete)
```

### Bước 5: Thêm route vào Gateway

File: `apps/osv/internal/gateway/router.go`

```go
// THÊM sau dòng:
// mux.Handle("PUT /api/v1/assets/{id}",    protected(proxy.Forward("asset-service:8091")))
mux.Handle("PATCH /api/v1/assets/{id}", protected(proxy.Forward("asset-service:8091")))
```

### Bước 6: Build & Test

```bash
# asset-service
cd services/asset-service && go build ./...

# gateway
cd apps/osv && go build ./...
```

**Test**:
```bash
TOKEN="your_jwt_token"
BASE="https://c12.openledger.vn"
ASSET_ID="some-valid-asset-uuid"

# Test PATCH (partial update — chỉ update name)
curl -s -X PATCH "$BASE/api/v1/assets/$ASSET_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Updated Asset Name", "criticality": "high"}' | jq .
# Expected: 200 OK với asset object

# Test PUT vẫn hoạt động (không regression)
curl -s -X PUT "$BASE/api/v1/assets/$ASSET_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Full Update", "criticality": "high", "os": "Linux"}' | jq .
# Expected: 200 OK
```

## Acceptance Criteria

- [x] `PATCH /api/v1/assets/{id}` → `200 OK` với updated asset
- [x] Partial update: chỉ fields được gửi mới thay đổi
- [x] `PUT /api/v1/assets/{id}` → không bị regression
- [x] `go build ./...` cho cả `asset-service` và `apps/osv`
