# Change Request SEED-002: Seed Products, ProductTypes & Engagements qua Gateway

**Cập nhật:** 2026-06-18  
**Status:** Proposed  
**Domain:** product-service / finding-service  
**Priority:** 🔴 CRITICAL — Tất cả findings, tests, risk acceptances đều phụ thuộc vào Product hierarchy  
**Depends on:** SEED-001 (cần có users trước)

---

## 1. Bối cảnh

Product hierarchy (`ProductType → Product → Engagement → Test`) là nền tảng của toàn bộ hệ thống. Mọi finding, scan kết quả đều phải gắn vào ít nhất một Product.

Phân tích hiện trạng — những gì đang có và đang thiếu:

| Use-case | Endpoint hiện tại | Trạng thái |
|---------|------------------|-----------|
| Tạo ProductType | `POST /api/v2/product-types` (finding-service) | ✅ Có, nhưng **chưa được route qua gateway** |
| Bulk create ProductTypes | **THIẾU** | ❌ Không hỗ trợ |
| Tạo Product | `POST /api/v1/products` hoặc `POST /api/v2/products` | ✅ Có |
| Bulk create Products | **THIẾU** | ❌ Không hỗ trợ |
| Tạo Engagement | `POST /api/v1/engagements` hoặc `POST /api/v2/engagements` | ✅ Có |
| Tạo Test | `POST /api/v2/tests` | ✅ Có |
| Gán SLA cho Product | `POST /api/v2/sla-configurations/{id}/assign/{product_id}` | ✅ Có |
| Gán member cho Product | `POST /api/v2/products/{id}/members` | ✅ Có qua gateway |
| Seed toàn bộ hierarchy trong 1 call | **THIẾU** | ❌ Phải gọi nhiều requests tuần tự |
| Xóa Product và toàn bộ dữ liệu con | `DELETE /api/v2/products/{id}` | ✅ Có, nhưng không rõ cascade behavior |
| Import Products từ file JSON/CSV | **THIẾU** | ❌ Không hỗ trợ |

**Vấn đề chính với seed scenario**:
- Cần tạo `ProductType` trước → lấy ID → tạo `Product` → lấy ID → tạo `Engagement` → lấy ID → tạo `Test`. Toàn bộ là 4 sequential round-trips.
- Không có bulk endpoint → seed script bị chậm khi tạo hàng chục products.
- `product-service` (`POST /products`) và `finding-service` (`POST /api/v2/products`) là 2 service riêng biệt — cần làm rõ service nào là canonical.

---

## 2. Thay đổi Đề Xuất

### 2.1 [CRITICAL] `POST /api/v2/product-types` — Đảm bảo được route qua Gateway

ProductType hiện tồn tại trong finding-service nhưng **chưa chắc được expose qua gateway** (specs hiện tại có ghi nhưng cần xác nhận).

**Gateway** — Xác nhận/thêm route trong `apps/osv/internal/gateway/router.go`:
```
POST   /api/v2/product-types      →  finding-service:8085  (authenticated, Maintainer+)
GET    /api/v2/product-types      →  finding-service:8085  (authenticated)
GET    /api/v2/product-types/{id} →  finding-service:8085  (authenticated)
PUT    /api/v2/product-types/{id} →  finding-service:8085  (authenticated, Owner)
DELETE /api/v2/product-types/{id} →  finding-service:8085  (authenticated, Admin)
```

**Request body** (`POST /api/v2/product-types`):
```json
{
  "name": "Web Application",
  "description": "Web-based applications exposed to internet",
  "critical_product": false,
  "key_product": true
}
```

**Response** `201 Created`:
```json
{
  "id": "uuid",
  "name": "Web Application",
  "description": "...",
  "critical_product": false,
  "key_product": true,
  "created_at": "2026-06-18T00:00:00Z",
  "updated_at": "2026-06-18T00:00:00Z"
}
```

---

### 2.2 [CRITICAL] `POST /api/v2/products/bulk` — Bulk create Products

Cho phép seed script tạo nhiều products trong 1 request.

**Gateway**:
```
POST /api/v2/products/bulk  →  finding-service:8085  (authenticated, Maintainer+)
```

**Request body**:
```json
{
  "products": [
    {
      "name": "Customer Portal",
      "product_type_id": "type-uuid-1",
      "description": "B2C customer-facing portal",
      "business_criticality": "high",
      "platform": "web",
      "lifecycle": "production",
      "origin": "internal",
      "external_audience": true,
      "internet_accessible": true,
      "tags": ["b2c", "production"]
    },
    {
      "name": "Internal HR System",
      "product_type_id": "type-uuid-1",
      "description": "HR management system",
      "business_criticality": "medium",
      "platform": "web",
      "lifecycle": "production"
    }
  ]
}
```

**Response** `207 Multi-Status`:
```json
{
  "results": [
    { "name": "Customer Portal", "status": "created", "id": "uuid-1" },
    { "name": "Internal HR System", "status": "created", "id": "uuid-2" }
  ],
  "created_count": 2,
  "failed_count": 0
}
```

---

### 2.3 [HIGH] `POST /api/v2/products/{id}/seed` — Tạo Engagement + Test trong 1 call

Endpoint composite để giảm round-trips khi seeding. Cho phép tạo Engagement và optional Test đồng thời.

**Gateway**:
```
POST /api/v2/products/{id}/seed  →  finding-service:8085  (authenticated, Writer+)
```

**Request body**:
```json
{
  "engagement": {
    "name": "Q2 2026 Security Assessment",
    "engagement_type": "Interactive",
    "start_date": "2026-06-01T00:00:00Z",
    "version": "2.1.0",
    "tags": ["quarterly", "manual"]
  },
  "test": {
    "title": "Manual Pentest",
    "test_type": "manual",
    "target_start": "2026-06-10T00:00:00Z",
    "target_end": "2026-06-20T00:00:00Z"
  }
}
```

**Response** `201 Created`:
```json
{
  "product_id": "product-uuid",
  "engagement": {
    "id": "engagement-uuid",
    "name": "Q2 2026 Security Assessment",
    "status": "Not Started"
  },
  "test": {
    "id": "test-uuid",
    "title": "Manual Pentest",
    "test_type": "manual"
  }
}
```

---

### 2.4 [HIGH] `POST /api/v2/products/import` — Import từ JSON file

Cho phép upload file JSON chứa danh sách products để import hàng loạt.

**Gateway**:
```
POST /api/v2/products/import  →  finding-service:8085  (adminOnly)
Content-Type: multipart/form-data
```

**Request**: multipart với field `file` chứa JSON array của products.

**File format**:
```json
[
  {
    "name": "Product A",
    "product_type_name": "Web Application",
    "business_criticality": "high",
    "platform": "web",
    "lifecycle": "production",
    "tags": ["critical"]
  }
]
```

> **Note**: `product_type_name` được resolve thành ID tự động; nếu không tồn tại thì tạo mới.

**Response** `200 OK`:
```json
{
  "imported_count": 15,
  "created_types": 2,
  "errors": []
}
```

---

### 2.5 [MEDIUM] `POST /api/v2/product-types/bulk` — Bulk create ProductTypes

**Gateway**:
```
POST /api/v2/product-types/bulk  →  finding-service:8085  (adminOnly)
```

**Request body**:
```json
{
  "product_types": [
    { "name": "Web Application", "critical_product": false, "key_product": true },
    { "name": "Mobile App",      "critical_product": false, "key_product": false },
    { "name": "API Service",     "critical_product": true,  "key_product": true }
  ]
}
```

**Response** `207 Multi-Status` (tương tự SEED-002.2).

---

### 2.6 [MEDIUM] Thêm `sla_configuration_id` vào `POST /api/v2/products`

Khi tạo product, client có thể chỉ định SLA configuration ngay lập tức thay vì phải gọi thêm assign endpoint.

**Thêm field vào request body `POST /api/v2/products`**:
```json
{
  "name": "Customer Portal",
  "product_type_id": "type-uuid",
  "sla_configuration_id": "sla-config-uuid"
}
```

Nếu `sla_configuration_id` được cung cấp, finding-service sẽ gọi sla-service để tạo `SLAProductAssignment` atomically.

---

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `POST /api/v2/product-types` với valid body → `201` với ProductType object.
2. `POST /api/v2/products/bulk` với 5 products → `207` với `created_count: 5`.
3. `POST /api/v2/products/bulk` với 1 product thiếu `product_type_id` → `207` với entry đó `status: "error"`.
4. `POST /api/v2/products/{id}/seed` với engagement + test → `201` với cả 2 IDs.
5. `POST /api/v2/products/{id}/seed` chỉ với engagement (không có test) → `201` với `test: null`.
6. `POST /api/v2/products/import` với file 10 products → `200` với `imported_count: 10`.
7. `POST /api/v2/products` với `sla_configuration_id` → product có SLA ngay, không cần gọi assign riêng.
