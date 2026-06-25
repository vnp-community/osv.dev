# Change Request 004: Assets Service — Expose Sub-resources & Gateway Routing

**Cập nhật:** 2026-06-18  
**Status:** Critical — Gateway chưa route bất kỳ `/api/v1/assets/*` nào. Asset-service có handlers nhưng không được expose qua gateway.

## 1. Bối cảnh

So sánh frontend (openapi.yaml) với backend hiện tại:

| Endpoint frontend | Backend Gateway | Asset-service | Trạng thái |
|---|---|---|---|
| `GET /api/v1/assets` | ❌ **THIẾU trong gateway** | ✅ `GET /assets` | ❌ Gateway chưa route |
| `GET /api/v1/assets/tags` | ❌ **THIẾU** | ❌ **THIẾU** | ❌ Cần tạo mới |
| `GET /api/v1/assets/{id}` | ❌ **THIẾU trong gateway** | ✅ `GET /assets/{id}` | ❌ Gateway chưa route |
| `PATCH /api/v1/assets/{id}` | ❌ **THIẾU** | ✅ `PUT /assets/{id}/tags` | ❌ Method mismatch, path khác |
| `GET /api/v1/assets/{id}/findings` | ❌ **THIẾU trong gateway** | ✅ `GET /assets/{id}/findings` | ❌ Gateway chưa route |

**Nguồn asset-service** (`services/asset-service/internal/delivery/http/handlers.go`):
```go
r.Get("/assets", h.ListAssets)
r.Get("/assets/{id}", h.GetAsset)
r.Put("/assets/{id}/tags", h.UpdateTags)      // ← frontend cần PATCH /assets/{id}
r.Get("/assets/{id}/risk", h.GetRiskScore)
r.Get("/assets/{id}/history", h.GetHistory)
r.Get("/assets/{id}/findings", h.GetFindings)
```

**Vấn đề chính**: Gateway (`apps/osv/internal/gateway/router.go`) **không có bất kỳ route nào** cho `/api/v1/assets/*`.

## 2. Thay đổi Đề Xuất

### 2.1 [CRITICAL] Thêm toàn bộ asset routes vào Gateway

Thêm vào `apps/osv/internal/gateway/router.go`:

```go
// ═══════════════════════════════════════════════════════
// ASSETS (asset-service:<port>)
// ═══════════════════════════════════════════════════════
mux.Handle("GET /api/v1/assets",              protected(proxy.Forward("asset-service:8091")))
mux.Handle("GET /api/v1/assets/tags",         protected(proxy.Forward("asset-service:8091")))
mux.Handle("GET /api/v1/assets/{id}",         protected(proxy.Forward("asset-service:8091")))
mux.Handle("PATCH /api/v1/assets/{id}",       protected(proxy.Forward("asset-service:8091")))
mux.Handle("GET /api/v1/assets/{id}/findings",protected(proxy.Forward("asset-service:8091")))
mux.Handle("GET /api/v1/assets/{id}/risk",    protected(proxy.Forward("asset-service:8091")))
mux.Handle("GET /api/v1/assets/{id}/history", protected(proxy.Forward("asset-service:8091")))
```

> **Lưu ý**: Cần xác nhận port thực tế của asset-service trong `deploy/dev/docker-compose.server.yaml`.

### 2.2 [CRITICAL] Thêm `GET /api/v1/assets/tags` vào asset-service

Frontend gọi endpoint này để lấy unique tags cho filter autocomplete.

```go
// services/asset-service/internal/delivery/http/handlers.go
r.Get("/assets/tags", h.GetTags)   // PHẢI đứng TRƯỚC /assets/{id}

func (h *Handler) GetTags(w http.ResponseWriter, r *http.Request) {
    tags, err := h.repo.GetUniqueTags(r.Context())
    // ...
    json.NewEncoder(w).Encode(map[string]interface{}{"tags": tags})
}
```

```go
// services/asset-service/internal/infra/postgres/asset_repo.go
func (r *AssetRepository) GetUniqueTags(ctx context.Context) ([]string, error) {
    query := `SELECT DISTINCT unnest(tags) AS tag FROM assets ORDER BY tag ASC`
    // execute & return []string
}
```

**Response**:
```json
{ "tags": ["production", "critical", "dmz", "internal"] }
```

### 2.3 [HIGH] Thêm `PATCH /api/v1/assets/{id}` vào asset-service

Frontend gửi `PATCH /assets/{id}` với body `{tags, hostname}` để partial update. Backend hiện chỉ có `PUT /assets/{id}/tags`.

```go
// services/asset-service/internal/delivery/http/handlers.go
r.Patch("/assets/{id}", h.UpdateAsset)   // thêm mới

func (h *Handler) UpdateAsset(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    var req struct {
        Tags     []string `json:"tags"`
        Hostname string   `json:"hostname"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    // update & return Asset
}
```

**Request body**:
```json
{
  "tags": ["production", "critical"],
  "hostname": "web-server-01.internal"
}
```

**Response**: `Asset` object đầy đủ.

### 2.4 [HIGH] Đảm bảo `GET /api/v1/assets/{id}/findings` hoạt động đúng

Asset-service hiện có handler `GetFindings` tại `GET /assets/{id}/findings`. Cần đảm bảo:
- Trả về `FindingsListResponse` schema (consistent với finding-service)
- Hỗ trợ query params: `status`, `severity`, `page`, `pageSize`

**Option A (khuyến nghị)**: Asset-service gọi nội bộ finding-service để lấy findings theo `asset_ip` hoặc `asset_hostname`.

**Option B**: Gateway rewrite `/api/v1/assets/{id}/findings` → finding-service với param lọc theo asset.

### 2.5 [MEDIUM] Asset list response schema alignment

Frontend yêu cầu `GET /api/v1/assets` trả về:
```json
{
  "assets": [ Asset ],
  "total": 42
}
```

Asset-service cần đảm bảo response wrapper đúng format này (không trả về bare array).

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `GET /api/v1/assets` không trả về `404` — trả về `{ assets: [...], total: N }`.
2. `GET /api/v1/assets/tags` trả về `{ tags: ["string"] }` với unique, sorted tags.
3. `GET /api/v1/assets/{id}` trả về Asset object đầy đủ bao gồm `services`, `tags`, `risk_score`.
4. `PATCH /api/v1/assets/{id}` với body `{ tags: [...] }` cập nhật thành công, trả về Asset đã cập nhật.
5. `GET /api/v1/assets/{id}/findings` trả về `FindingsListResponse` hỗ trợ filter `status`, `severity`.
6. Route ordering đúng: `/assets/tags` được register TRƯỚC `/assets/{id}` trong chi router để tránh shadowing.
