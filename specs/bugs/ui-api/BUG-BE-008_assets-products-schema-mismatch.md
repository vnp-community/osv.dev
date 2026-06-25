# BUG-BE-008 — Assets và Products Response Schema Sai

| Trường | Giá trị |
|---|---|
| **ID** | BUG-BE-008 |
| **Severity** | 🟠 High |
| **Priority** | P1 |
| **Component** | Backend / API Gateway / Asset & Product Handlers |
| **Endpoints** | `GET /api/v1/assets`, `GET /api/v1/products`, `GET /api/v1/products/types` |
| **Phát hiện** | 2026-06-19 |
| **Server** | https://c12.openledger.vn |
| **Status** | Open |

---

## Mô tả

Ba endpoints trả về HTTP 200 nhưng **cấu trúc JSON hoàn toàn khác spec**:
- `/assets` trả object trực tiếp thay vì `{ "assets": [...], "total": N }`
- `/products` trả list trực tiếp thay vì `{ "products": [...], "total": N }`
- `/products/types` trả `[{ "id": ..., "name": ... }]` thay vì `["string", ...]`

## Chi Tiết Sai Lệch

### `GET /api/v1/assets`

**Actual response:**
```json
HTTP/1.1 200 OK
{
  "ip": "...",
  "services": [...],
  ...
}
```
*(Trả về 1 Asset object hoặc cấu trúc lạ — không phải paginated list)*

**Expected (theo spec):**
```json
{
  "assets": [
    {
      "id": "uuid",
      "ip": "192.168.1.1",
      "services": [...],
      "web_technologies": [...],
      "tags": [],
      "risk_score": 0.0,
      "active_finding_count": 0,
      "first_seen_at": "...",
      "last_seen_at": "..."
    }
  ],
  "total": 10
}
```

### `GET /api/v1/products`

**Actual response:**
```json
HTTP/1.1 200 OK
[
  { "id": "...", "name": "...", ... }
]
```
*(Array trực tiếp)*

**Expected (theo spec):**
```json
{
  "products": [...],
  "total": 5
}
```

### `GET /api/v1/products/types`

**Actual response:**
```json
[
  { "id": 1, "name": "web_application" },
  { "id": 2, "name": "api" }
]
```

**Expected (theo spec):**
```json
{
  "types": ["web_application", "api", "mobile", "infrastructure"]
}
```

## Fix

### Assets handler
```go
c.JSON(200, gin.H{
    "assets": assets,
    "total":  total,
})
```

### Products handler
```go
c.JSON(200, gin.H{
    "products": products,
    "total":    total,
})
```

### Products/types handler
```go
// Extract chỉ name strings
typeNames := make([]string, len(types))
for i, t := range types { typeNames[i] = t.Name }

c.JSON(200, gin.H{"types": typeNames})
```

## Ảnh hưởng

- Trang `/assets` render lỗi — frontend expect `response.assets` nhưng nhận undefined
- Trang `/products` render lỗi
- Product type filter dropdown không populate đúng
