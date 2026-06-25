# AI Task 004: Assets Subresources & BFF Gateway (SOL-004)

**Status**: ✅ COMPLETED — 2026-06-18

## Checklist Công Việc

### 1. Asset Service: Lấy danh sách Tags
- [x] Mở file `services/asset-service/internal/delivery/http/handlers.go`
- [x] Viết handler `GetTags(w, r)` — lấy unique tags từ tất cả assets qua `listUC.List()`
- [x] Endpoint `GET /assets/tags` registered trong `Router()` — literal route TRƯỚC wildcards

### 2. Cập nhật Gateway Router (Asset Service) — CR-004
- [x] Thêm route `GET /api/v1/assets/tags` → `asset-service:8091` (literal trước wildcard)
- [x] Thêm đầy đủ asset CRUD routes:
  - `GET /api/v1/assets` → asset-service
  - `POST /api/v1/assets` → asset-service
  - `GET /api/v1/assets/{id}` → asset-service
  - `PUT /api/v1/assets/{id}` → asset-service
  - `DELETE /api/v1/assets/{id}` → asset-service
  - `PUT /api/v1/assets/{id}/tags` → asset-service
  - `GET /api/v1/assets/{id}/risk` → asset-service
  - `GET /api/v1/assets/{id}/history` → asset-service

### 3. Finding Service: Bộ Lọc (Filter)
- [x] `GET /api/v2/findings` đã hỗ trợ `asset_id` query param (đã có sẵn trong FindingFilter)
- [x] BFF handler sẽ inject đúng `?asset_id=` khi proxy qua gateway

### 4. API Gateway: BFF Pattern
- [x] Implement `assetFindingsBFF()` inline trong `apps/osv/internal/gateway/router.go`
  - Đọc `{id}` từ URL path (`r.PathValue("id")` với fallback parse)
  - Rewrite URL path: `/api/v1/assets/{id}/findings` → `/api/v2/findings?asset_id={id}`
  - Forward về `finding-service:8085`
- [x] Route `GET /api/v1/assets/{id}/findings` → `assetFindingsBFF(proxy)` registered

## Build Status
- `apps/osv/internal/gateway/...` → ✅ Build OK
- `services/asset-service/internal/...` → N/A (no go.mod, belongs to monorepo)

## Ghi Chú
- `GetTags` hiện dùng `listUC.List(limit=1000)` để aggregate tags. Có thể optimize sau bằng cách thêm `GetUniqueTags()` SQL native query vào repo.
- Port asset-service mặc định là `8091` theo convention của project.
