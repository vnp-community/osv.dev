# Data Models — report-service

> **Service**: `services/report-service`  
> **Mô tả**: Generate security reports theo nhiều định dạng (PDF, HTML, CSV, Excel, JSON). Hỗ trợ async generation với job tracking, lọc findings theo severity và CVSS score, và lưu trữ artifacts trên S3/MinIO.  
> **Storage**: PostgreSQL (report runs, jobs), S3/MinIO (generated report files)  
> **Go package**: `services/report-service/internal/domain/entity`

---

## 1. ReportRun

Async report generation job — tracking toàn bộ quá trình tạo report.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `scan_id` | *UUID | Yes | FK → Scan (null nếu report theo product) |
| `product_id` | *UUID | Yes | FK → Product (null nếu report theo scan) |
| `formats` | []OutputFormat | No | Danh sách định dạng cần generate |
| `min_severity` | string | Yes | Lọc: `critical` \| `high` \| `medium` \| `low` \| rỗng = tất cả |
| `min_score` | *float64 | Yes | Lọc theo CVSS score tối thiểu |
| `theme` | Theme | No | `light` \| `dark` (cho HTML report) |
| `status` | ReportRunStatus | No | Trạng thái job |
| `exit_code` | *int | Yes | `0` = không có issues, `1` = có issues |
| `error_msg` | string | Yes | Error message nếu failed |
| `created_by` | UUID | No | FK → User |
| `created_at` | timestamp | No | |
| `completed_at` | *timestamp | Yes | |
| `artifacts` | []*ReportArtifact | Yes | Danh sách files được tạo |

**Enums — OutputFormat**:

| Giá trị | Mô tả |
|---------|-------|
| `pdf` | PDF report |
| `html` | HTML report (có theme) |
| `csv` | CSV spreadsheet |
| `excel` | Excel (.xlsx) |
| `json` | JSON data export |
| `console` | Console text output |

**Enums — Theme**:

| Giá trị | Mô tả |
|---------|-------|
| `light` | Light color scheme |
| `dark` | Dark color scheme |

**Enums — ReportRunStatus**:

| Giá trị | Mô tả |
|---------|-------|
| `pending` | Chờ xử lý |
| `generating` | Đang generate |
| `completed` | Hoàn thành |
| `failed` | Thất bại |

---

## 2. ReportArtifact

Generated report file được lưu trên S3/MinIO.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `format` | OutputFormat | No | Định dạng file |
| `storage_path` | string | No | S3/MinIO object path |
| `size_bytes` | int64 | No | Kích thước file |
| `content_type` | string | No | MIME type, e.g. `application/pdf` |
| `url` | string | Yes | Presigned download URL |
| `url_expires_at` | *timestamp | Yes | Thời điểm URL hết hạn |

---

## 3. ReportInput

Input data cho report generation engine.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `scan_id` | *UUID | Yes | |
| `product_id` | *UUID | Yes | |
| `scan_target` | string | Yes | Target được scan (IP, hostname, package) |
| `generated_at` | timestamp | No | |
| `theme` | Theme | No | |
| `min_severity` | string | Yes | |
| `min_score` | *float64 | Yes | |
| `findings` | []*ReportFinding | No | Danh sách findings được include |
| `stats` | ScanStats | No | Thống kê tổng hợp |
| `products` | []*ProductSection | Yes | Sections chia theo product (cho HTML) |

---

## 4. ReportFinding

Finding data model được report service dùng (simplified copy từ finding-service).

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | string | No | |
| `title` | string | No | |
| `description` | string | Yes | |
| `mitigation` | string | Yes | |
| `severity` | string | No | `Critical` \| `High` \| `Medium` \| `Low` \| `Info` |
| `cve` | string | Yes | |
| `cwe` | int | Yes | |
| `cvss_v3_score` | *float64 | Yes | |
| `epss_score` | *float64 | Yes | |
| `is_exploit` | bool | No | Có public exploit |
| `is_in_cisa_kev` | bool | No | Có trong CISA KEV catalog |
| `status` | string | Yes | `active` \| `mitigated` \| `false_positive` \| etc. |
| `component_name` | string | Yes | |
| `component_version` | string | Yes | |
| `date` | timestamp | No | Ngày phát hiện |
| `sla_expiration_date` | *timestamp | Yes | |
| `days_until_sla` | *int | Yes | Âm = đã breach |
| `data_source` | string | Yes | `nmap` \| `zap` \| `agent` \| `manual` |
| `product_id` | string | Yes | |
| `product_name` | string | Yes | |
| `engagement_id` | string | Yes | |
| `engagement_name` | string | Yes | |
| `tags` | []string | Yes | |

---

## 5. ScanStats

Thống kê tổng hợp finding counts cho report header.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `critical_count` | int | Số Critical findings |
| `high_count` | int | Số High findings |
| `medium_count` | int | Số Medium findings |
| `low_count` | int | Số Low findings |
| `info_count` | int | Số Info findings |
| `total_count` | int | Tổng số findings |

---

## 6. ProductSection

Nhóm findings theo product cho HTML report.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `product_name` | string | Tên product |
| `product_id` | string | |
| `total_findings` | int | Tổng findings trong section này |
| `findings` | []*ReportFinding | Chi tiết findings |

---

## 7. Relationships

```
ReportRun ──────── ReportArtifact (1:N, embedded)
ReportRun ──────── User (N:1, created_by)
ReportInput ─────── ReportFinding (1:N)
ReportInput ─────── ScanStats (1:1)
ReportInput ─────── ProductSection (1:N)
ProductSection ───── ReportFinding (1:N)
```
