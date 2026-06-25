# Data Models — sla-service

> **Service**: `services/sla-service`  
> **Mô tả**: Quản lý SLA (Service Level Agreement) configurations định nghĩa deadline khắc phục theo severity. Mỗi product có thể có SLA config riêng; nếu không có thì dùng default.  
> **Storage**: PostgreSQL  
> **Go package**: `services/sla-service/internal/domain/slaconfig` (package: `slaconfigdomain`)

---

## 1. SLAConfiguration

Cấu hình thời hạn khắc phục per severity. Có thể được gán cho một hoặc nhiều products.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `name` | string | No | Tên configuration, e.g. "Standard SLA" |
| `description` | string | Yes | Mô tả |
| `critical_days` | int | No | Số ngày khắc phục Critical (thường: 7) |
| `high_days` | int | No | Số ngày khắc phục High (thường: 30) |
| `medium_days` | int | No | Số ngày khắc phục Medium (thường: 90) |
| `low_days` | int | No | Số ngày khắc phục Low (thường: 365) |
| `is_default` | bool | No | Nếu true, products không có SLA assignment sẽ dùng config này |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Default SLA values** (industry standard):
- Critical: 7 ngày
- High: 30 ngày
- Medium: 90 ngày
- Low: 365 ngày
- Info: 0 (không có SLA)

---

## 2. SLAProductAssignment

Liên kết một product với SLA configuration.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `product_id` | UUID | No | FK → Product |
| `sla_configuration_id` | UUID | No | FK → SLAConfiguration |
| `assigned_at` | timestamp | No | Thời điểm gán |
| `assigned_by` | UUID | No | FK → User (người gán) |

---

## 3. DaysForSeverity

Helper method để tính số ngày deadline theo severity string:

| Input severity | Field được dùng |
|---------------|----------------|
| `Critical` | `critical_days` |
| `High` | `high_days` |
| `Medium` | `medium_days` |
| `Low` | `low_days` |
| `Info` / khác | `0` (không có SLA) |

---

## 4. HTTP API Endpoints

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/sla/configs` | Danh sách SLA configurations |
| `POST` | `/sla/configs` | Tạo SLA configuration mới |
| `GET` | `/sla/configs/{id}` | Chi tiết một configuration |
| `PUT` | `/sla/configs/{id}` | Cập nhật configuration |
| `DELETE` | `/sla/configs/{id}` | Xóa configuration |
| `POST` | `/sla/configs/{id}/assign` | Gán config cho product |
| `GET` | `/sla/products/{product_id}` | Lấy SLA config của product |

---

## 5. Relationships

```
SLAConfiguration ─── SLAProductAssignment (1:N)
SLAProductAssignment ─── Product (finding-service) (N:1)
finding-service.SLAConfiguration ─── sla-service.SLAConfiguration (mirror entity, khác package)
```

> **Lưu ý**: `finding-service` cũng có `SLAConfiguration` entity nhưng dùng field names khác (`Critical`, `High`, `Medium`, `Low` thay vì `CriticalDays`, etc.). Entity trong `sla-service` là canonical configuration; entity trong `finding-service` được dùng để tính SLA expiration date cho individual findings.
