# Data Models — asset-service

> **Service**: `services/asset-service`  
> **Mô tả**: Quản lý asset (network hosts/devices) với tagging, risk scoring, network service discovery và tích hợp với scan-service findings.  
> **Storage**: PostgreSQL  
> **Go package**: `services/asset-service/internal/domain/entity`

---

## 1. Asset

Network asset (host, server, device) được phát hiện qua scan.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `ip_address` | string | No | IPv4/IPv6 address |
| `hostname` | string | Yes | DNS hostname |
| `os` | string | Yes | Hệ điều hành |
| `mac_address` | string | Yes | MAC address |
| `services` | []ServicePort | Yes | Danh sách network services được phát hiện |
| `tags` | []string | No | Tags để phân nhóm |
| `labels` | map[string]string | Yes | Key-value labels tùy chỉnh |
| `risk_score` | float64 | No | Risk score 0.0–10.0 (computed) |
| `finding_count` | int | No | Số active findings (denormalized) |
| `status` | AssetStatus | No | Trạng thái vòng đời |
| `last_seen_at` | timestamp | Yes | Lần cuối phát hiện |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums — AssetStatus**:

| Giá trị | Mô tả |
|---------|-------|
| `active` | Asset đang hoạt động |
| `inactive` | Asset không còn hoạt động |
| `decommissioned` | Asset đã ngừng vận hành |

---

## 2. ServicePort

Network service được phát hiện trên một asset.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `port` | int | No | Port number |
| `protocol` | string | No | `tcp` \| `udp` |
| `service` | string | Yes | Tên service (e.g. `ssh`, `http`, `mysql`) |
| `version` | string | Yes | Phiên bản service |

---

## 3. Vulnerability

CVE vulnerability được liên kết với một asset (từ scan results).

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | Yes | |
| `asset_id` | UUID | No | FK → Asset |
| `cve_id` | string | No | CVE ID (e.g. `CVE-2021-44228`) |
| `severity` | string | No | `critical` \| `high` \| `medium` \| `low` \| `none` |
| `cvss` | float64 | Yes | CVSS score |
| `detected_at` | timestamp | No | Thời điểm phát hiện |

---

## 4. AssetFilter

Bộ lọc cho danh sách assets.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `status` | AssetStatus | Yes | Lọc theo trạng thái |
| `tags` | []string | Yes | Lọc theo danh sách tags (containment) |
| `tag` | string | Yes | Lọc theo một tag |
| `os` | string | Yes | Lọc theo OS |
| `query` | string | Yes | Full-text search trên IP/hostname |
| `has_port` | *int | Yes | Lọc assets có port mở cụ thể |
| `ip_address` | string | Yes | Exact hoặc CIDR match |
| `hostname` | string | Yes | Partial match |
| `page` | int | No | Trang (mặc định 1) |
| `limit` | int | No | Số kết quả mỗi trang |

---

## 5. AssetCreateInput

Input cho thao tác tạo asset mới.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `ip_address` | string | No | IPv4/IPv6 bắt buộc |
| `hostname` | string | Yes | |
| `os` | string | Yes | |
| `mac_address` | string | Yes | |
| `services` | []ServicePort | Yes | |
| `tags` | []string | Yes | |
| `labels` | map[string]string | Yes | |

---

## 6. BulkAssetResult

Kết quả per-item của bulk create operation.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `ip_address` | string | No | IP của asset được xử lý |
| `status` | string | No | `created` \| `updated` \| `skipped` \| `error` |
| `id` | *UUID | Yes | UUID asset nếu created/updated |
| `message` | string | Yes | Error message nếu failed |

---

## 7. ScanSchedule

Lịch scan định kỳ được gắn với một asset.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `asset_id` | UUID | No | FK → Asset |
| `scan_type` | string | No | `nmap` \| `zap` \| `agent` \| `manual` |
| `schedule_cron` | string | No | Cron expression (e.g. `0 2 * * *`) |
| `enabled` | bool | No | Có đang kích hoạt không |
| `last_run_at` | timestamp | Yes | Lần chạy cuối |
| `next_run_at` | timestamp | Yes | Lần chạy tiếp theo |
| `created_by` | *UUID | Yes | FK → User |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

---

## 8. Risk Score

Điểm rủi ro tổng hợp của asset dựa trên active findings.

**Công thức**:
```
risk_score = min(
  Critical × 9.5 + High × 7.5 + Medium × 4.5 + Low × 1.5,
  10.0
)
```

| Input | Trọng số |
|-------|---------|
| Critical findings | ×9.5 |
| High findings | ×7.5 |
| Medium findings | ×4.5 |
| Low findings | ×1.5 |
| Score tối đa | 10.0 |

> Risk score được lưu vào `Asset.risk_score` và `Asset.finding_count` sau mỗi lần tính.

---

## 9. HTTP API Endpoints

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/assets` | Danh sách assets có filter và phân trang |
| `POST` | `/assets` | Tạo asset mới |
| `POST` | `/assets/bulk` | Tạo nhiều assets cùng lúc |
| `GET` | `/assets/tags` | Lấy danh sách unique tags |
| `GET` | `/assets/{id}` | Chi tiết một asset |
| `PUT` | `/assets/{id}` | Cập nhật asset |
| `DELETE` | `/assets/{id}` | Xóa asset |
| `PUT` | `/assets/{id}/tags` | Cập nhật tags của asset |
| `GET` | `/assets/{id}/risk` | Tính và trả về risk score |
| `GET` | `/assets/{id}/history` | Lịch sử scan của asset |
| `GET` | `/assets/{id}/findings` | Findings liên quan đến asset |
| `GET` | `/assets/{id}/schedules` | Danh sách lịch scan |
| `POST` | `/assets/{id}/schedules` | Tạo lịch scan |

---

## 10. Relationships

```
Asset ─── ServicePort (1:N, embedded)
Asset ─── Vulnerability (1:N)
Asset ─── ScanSchedule (1:N)
Asset ─── finding-service.Finding (via asset_ip/asset_hostname)
AssetFilter ─── Asset (query params)
BulkAssetResult ─── Asset (1:1, result per item)
```
