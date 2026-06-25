# Change Request 003: Tính tương thích v1 cho Products, Engagements & Tests

**Cập nhật:** 2026-06-18  
**Status:** Partial — gateway đã route một số v1 paths, nhưng còn thiếu một số endpoints quan trọng.

## 1. Bối cảnh

So sánh frontend (openapi.yaml) với backend hiện tại:

| Endpoint frontend | Backend Gateway | Trạng thái |
|---|---|---|
| `GET /api/v1/products` | ✅ `GET /api/v1/products` → finding-service | ✅ |
| `POST /api/v1/products` | ✅ `POST /api/v1/products` → finding-service | ✅ |
| `GET /api/v1/products/grades` | ✅ `GET /api/v1/products/grades` | ✅ |
| `GET /api/v1/products/types` | ✅ `GET /api/v1/products/types` | ✅ |
| `GET /api/v1/products/{id}` | ✅ `GET /api/v1/products/{id}` | ✅ |
| `PATCH /api/v1/products/{id}` | ❌ **THIẾU** — gateway chỉ có `PUT /api/v1/products/{id}` | ❌ **Path method mismatch** |
| `GET /api/v1/products/{id}/engagements` | ✅ `GET /api/v1/products/{id}/engagements` | ✅ |
| `POST /api/v1/products/{id}/engagements` | ❌ **THIẾU** | ❌ |
| `GET /api/v1/engagements` | ✅ `GET /api/v1/engagements` | ✅ |
| `POST /api/v1/engagements` | ✅ `POST /api/v1/engagements` | ✅ |
| `GET /api/v1/engagements/{id}` | ✅ `GET /api/v1/engagements/{id}` | ✅ |
| `GET /api/v1/engagements/{engId}/tests` | ✅ `GET /api/v1/engagements/{id}/tests` | ⚠️ param name: `engId` vs `id` (không ảnh hưởng runtime) |

**Source gateway** (`apps/osv/internal/gateway/router.go` lines 133-147):
```go
mux.Handle("GET /api/v1/products", ...)
mux.Handle("POST /api/v1/products", ...)
mux.Handle("GET /api/v1/products/{id}", ...)
mux.Handle("PUT /api/v1/products/{id}", ...)      // ← frontend cần PATCH
mux.Handle("DELETE /api/v1/products/{id}", ...)
mux.Handle("GET /api/v1/products/{id}/engagements", ...)   // ← thiếu POST
```

## 2. Thay đổi Đề Xuất

### 2.1 [HIGH] Thêm `PATCH /api/v1/products/{id}`

Frontend dùng `PATCH` (partial update), backend gateway chỉ có `PUT` (full replace).

**Thêm vào `apps/osv/internal/gateway/router.go`**:
```go
mux.Handle("PATCH /api/v1/products/{id}", protected(proxy.Forward("finding-service:8085")))
```

**finding-service** cần xử lý `PATCH /api/v1/products/{id}` với body:
```json
{
  "name": "string",
  "description": "string",
  "product_type": "string"
}
```
Tất cả fields đều optional (partial update).

### 2.2 [HIGH] Thêm `POST /api/v1/products/{id}/engagements`

Frontend cần tạo engagement trực tiếp dưới một product.

**Gateway**:
```go
mux.Handle("POST /api/v1/products/{id}/engagements",
    protected(proxy.Forward("finding-service:8085")))
```

**finding-service handler** — tạo engagement với `product_id` từ path:
```json
// Request body
{
  "name": "string",
  "status": "string",
  "start_date": "2026-06-18",
  "end_date": "2026-12-31"  // optional
}
// Response 201 Created — Engagement schema
{
  "id": "...",
  "name": "...",
  "status": "...",
  "start_date": "...",
  "product_id": "{id}"
}
```

### 2.3 [LOW] Đảm bảo finding-service xử lý đúng v1 endpoints

Finding-service đã có handler cho các v1 paths. Cần verify:
- `POST /api/v1/products` tạo product, response theo schema `Product` (có `grade`, `score`)
- `GET /api/v1/products` trả về `{ products: [...], total: N }`
- `GET /api/v1/products/types` trả về `{ types: ["web_app", "api", ...] }`

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `PATCH /api/v1/products/{id}` với body `{name: "New Name"}` cập nhật thành công, trả về Product đã cập nhật.
2. `POST /api/v1/products/{id}/engagements` tạo engagement liên kết với `product_id`, trả về `201 Created`.
3. `GET /api/v1/products` trả về `{ products: [...], total: N }` — không trả về 404.
4. `GET /api/v1/engagements/{engId}/tests` trả về `{ tests: [...] }` với tests liên quan đến engagement.
5. Tất cả v1 products/engagements endpoints không trả về `404 Not Found`.
