# Data Models — product-service

> **Service**: `services/product-service`  
> **Mô tả**: Quản lý ProductTypes, Products, Engagements và Tests — hierarchy cấu trúc tổ chức các lần kiểm tra bảo mật. Là upstream dependency của finding-service.  
> **Storage**: PostgreSQL  
> **Go package**: `services/product-service/internal/domain/entity`

---

## 1. ProductType

Phân loại sản phẩm cấp cao.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `name` | string | No | Ví dụ: "Web Application", "Mobile App", "Infrastructure", "API" |
| `description` | string | Yes | |
| `critical_product` | bool | No | Product type có mức ưu tiên cao |
| `key_product` | bool | No | Quan trọng với business |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

---

## 2. Product

Đơn vị phần mềm/hệ thống được kiểm tra.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `product_type_id` | UUID | No | FK → ProductType |
| `name` | string | No | Tên sản phẩm |
| `description` | string | Yes | |
| `prod_numeric_grade` | int | No | Điểm tổng hợp 1–100 |
| `business_criticality` | BusinessCriticality | No | Mức độ quan trọng business |
| `platform` | Platform | No | Nền tảng triển khai |
| `lifecycle` | Lifecycle | No | Giai đoạn vòng đời |
| `origin` | string | Yes | `internal` \| `external` \| `partner` |
| `external_audience` | bool | No | Phục vụ người dùng bên ngoài |
| `internet_accessible` | bool | No | Có thể truy cập từ internet |
| `enable_full_risk_acceptance` | bool | No | |
| `enable_simple_risk_acceptance` | bool | No | |
| `tags` | []string | No | |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums — BusinessCriticality**: `very high`, `high`, `medium`, `low`, `very low`  
**Enums — Platform**: `web`, `api`, `mobile`, `desktop`  
**Enums — Lifecycle**: `construction`, `production`, `retirement`

---

## 3. Engagement

Sự kiện kiểm tra bảo mật trong một Product.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `product_id` | UUID | No | FK → Product |
| `name` | string | No | |
| `description` | string | Yes | |
| `lead_id` | UUID | Yes | FK → User |
| `engagement_type` | EngagementType | No | Loại engagement |
| `status` | EngagementStatus | No | Trạng thái |
| `start_date` | timestamp | No | |
| `end_date` | timestamp | Yes | |
| `version` | string | Yes | Software version |
| `build_id` | string | Yes | CI/CD build ID |
| `commit_hash` | string | Yes | Git commit |
| `branch_tag` | string | Yes | Git branch/tag |
| `source_code_management_uri` | string | Yes | SCM URL |
| `deduplication_on_engagement` | bool | No | |
| `tags` | []string | No | |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums — EngagementType**: `Interactive`, `CI/CD`

**Enums — EngagementStatus**:

| Giá trị | Mô tả |
|---------|-------|
| `Not Started` | Chưa bắt đầu |
| `In Progress` | Đang thực hiện |
| `On Hold` | Tạm dừng |
| `Completed` | Hoàn thành |
| `Cancelled` | Hủy |

---

## 4. Test

Một lần scan cụ thể trong một Engagement.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `engagement_id` | UUID | No | FK → Engagement |
| `title` | string | No | |
| `description` | string | Yes | |
| `test_type` | string | No | `nmap` \| `zap` \| `agent` \| `manual` \| `dast` \| `sast` |
| `target_start` | timestamp | Yes | |
| `target_end` | timestamp | Yes | |
| `scan_id` | UUID | Yes | FK → scan-service Scan |
| `finding_count` | int | No | Số findings (denormalized) |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

---

## 5. Hierarchy

```
ProductType
  └─→ Product (N)
        └─→ Engagement (N)
              └─→ Test (N)
                    └─→ Finding (N) [managed by finding-service]
```

---

## 6. Relationships

```
ProductType ──────────────── Product (1:N)
Product ──────────────────── Engagement (1:N)
Engagement ───────────────── Test (1:N)
Test ──────────────────────── scan-service.Scan (optional FK)
```

> **Note**: Product-service và finding-service chia sẻ cùng domain hierarchy (Product → Engagement → Test → Finding). product-service quản lý cấu trúc; finding-service quản lý findings.
