# Data Models — scan-service

> **Service**: `services/scan-service`  
> **Mô tả**: Quản lý vulnerability scanning: điều phối scan jobs (nmap, OWASP ZAP), quản lý agents, lưu kết quả assets/findings và hỗ trợ scheduled scans.  
> **Storage**: PostgreSQL

---

## 1. Scan

Job scan lỗ hổng bảo mật.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `user_id` | UUID | No | User khởi tạo scan |
| `targets` | []string | No | IPs, CIDRs, hostnames, URLs |
| `scan_type` | ScanType | No | Loại scan |
| `status` | ScanStatus | No | Trạng thái hiện tại |
| `priority` | int | No | Ưu tiên 1–10 (cao hơn = khẩn hơn) |
| `options` | ScanOptions | No | Cấu hình scan |
| `scheduled_for` | timestamp | Yes | Thời điểm scan được lên lịch |
| `started_at` | timestamp | Yes | |
| `completed_at` | timestamp | Yes | |
| `failed_at` | timestamp | Yes | |
| `error_msg` | string | Yes | Thông báo lỗi nếu thất bại |
| `progress` | int | No | Tiến độ 0–100% |
| `finding_count` | int | No | Số findings phát hiện được |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums — ScanType**:

| Giá trị | Mô tả |
|---------|-------|
| `full` | nmap -sV -O --script=vulners |
| `discovery` | nmap -sn (host discovery only) |
| `web` | OWASP ZAP active scan |
| `agent` | Triggered bởi agent report |

**Enums — ScanStatus** (State Machine):

```
pending → queued → running → completed
                          ↘ failed
         ← cancelled (from queued or running)
```

| Giá trị | Mô tả |
|---------|-------|
| `pending` | Chờ được queue |
| `queued` | Đã vào hàng chờ |
| `running` | Đang chạy |
| `completed` | Hoàn thành |
| `failed` | Thất bại |
| `cancelled` | Bị hủy |

---

## 2. ScanOptions

Tham số cấu hình cho scan job.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `ports` | string | Yes | Ví dụ `1-1024,8080,8443` |
| `timeout` | int | Yes | Timeout tính bằng giây |
| `intensity` | int | Yes | nmap -T1..-T5 (1=sneaky, 5=insane) |
| `max_depth` | int | Yes | Chiều sâu crawl web |
| `zap_config.spider_timeout` | int | Yes | ZAP spider timeout |
| `zap_config.active_scan_timeout` | int | Yes | ZAP active scan timeout |

---

## 3. Finding (Scan Finding)

Host hoặc endpoint được phát hiện trong scan, kèm thông tin vulnerability.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `scan_id` | UUID | No | FK → Scan |
| `ip_address` | string | No | IP của host |
| `hostname` | string | Yes | |
| `os` | string | Yes | Hệ điều hành phát hiện |
| `open_ports` | []Port | Yes | Danh sách cổng mở |
| `services` | []Service | Yes | Dịch vụ phát hiện trên các cổng |
| `web_tech` | []WebTechnology | Yes | Web technologies phát hiện |
| `cve_ids` | []string | Yes | CVE IDs liên quan |
| `severity` | Severity | No | Mức nghiêm trọng tổng hợp |
| `raw_data` | JSON | Yes | Raw output từ nmap/zap |
| `created_at` | timestamp | No | |

---

## 4. Port

Cổng mạng và trạng thái.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `port` | int | Số cổng |
| `protocol` | string | `tcp` \| `udp` |
| `state` | string | `open` \| `closed` \| `filtered` |

---

## 5. Service (Network Service)

Dịch vụ phát hiện trên một cổng.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `port` | int | |
| `name` | string | Tên dịch vụ, ví dụ `http`, `ssh` |
| `product` | string | Tên product, ví dụ `Apache httpd` |
| `version` | string | Phiên bản |

---

## 6. WebTechnology

Web framework/library phát hiện.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `name` | string | Tên technology |
| `version` | string | Phiên bản (optional) |
| `categories` | []string | Danh mục: `CMS`, `Framework`, `Database`, v.v. |

---

## 7. WebAlert

Cảnh báo bảo mật từ OWASP ZAP scanner.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `scan_id` | UUID | No | FK → Scan |
| `target_url` | string | No | URL bị ảnh hưởng |
| `alert_name` | string | No | Tên cảnh báo, ví dụ `SQL Injection` |
| `risk` | string | No | `High` \| `Medium` \| `Low` \| `Informational` |
| `confidence` | string | No | `High` \| `Medium` \| `Low` \| `False Positive` |
| `description` | string | Yes | |
| `solution` | string | Yes | |
| `reference` | string | Yes | |
| `evidence` | string | Yes | |
| `created_at` | timestamp | No | |

---

## 8. DiscoveryHost

Host phát hiện trong discovery scan.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `scan_id` | UUID | No | FK → Scan |
| `ip_address` | string | No | |
| `hostname` | string | Yes | |
| `status` | string | No | `up` \| `down` |
| `created_at` | timestamp | No | |

---

## 9. Asset

Network host/device đã được phát hiện và lưu trữ cùng services và metadata.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `ip_address` | string | No | |
| `hostname` | string | Yes | |
| `os` | string | Yes | |
| `mac_address` | string | Yes | |
| `services` | []Service | Yes | Dịch vụ đang chạy |
| `web_tech` | []WebTechnology | Yes | Web technologies |
| `labels` | map[string]string | Yes | Custom labels |
| `tags` | []Tag | Yes | Tags đính kèm |
| `last_scanned_at` | timestamp | Yes | Lần scan gần nhất |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |
| `vuln_summary` | VulnSummary | Yes | Tổng hợp số lỗ hổng (computed) |

---

## 10. Tag

Nhãn đính vào asset.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `id` | UUID | |
| `name` | string | Tên nhãn |
| `color` | string | Mã màu hex, ví dụ `#FF0000` |
| `created_at` | timestamp | |

---

## 11. Vulnerability (Asset Vulnerability)

CVE được phát hiện và liên kết với một asset.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `asset_id` | UUID | No | FK → Asset |
| `cve_id` | string | No | CVE ID |
| `summary` | string | Yes | |
| `severity` | Severity | No | |
| `cvss` | float64 | No | |
| `scan_id` | UUID | No | FK → Scan |
| `detected_at` | timestamp | No | |
| `remediated_at` | timestamp | Yes | |

---

## 12. VulnSummary

Tổng hợp số lỗ hổng theo severity cho asset.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `critical` | int | |
| `high` | int | |
| `medium` | int | |
| `low` | int | |

---

## 13. Agent

Agent scanning deployed tại host.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `name` | string | No | Tên agent |
| `hostname` | string | No | |
| `ip_address` | string | No | |
| `os` | string | No | |
| `agent_version` | string | No | |
| `api_key_id` | UUID | No | API key xác thực |
| `status` | AgentStatus | No | Trạng thái liveness |
| `last_seen_at` | timestamp | Yes | |
| `tags` | []string | No | |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums — AgentStatus**: `active`, `inactive`, `unknown`  
> Agent được coi là active nếu `last_seen_at` trong vòng 24 giờ.

---

## 14. AgentReport

Snapshot dữ liệu gửi từ agent.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `agent_id` | UUID | No | FK → Agent |
| `hostname` | string | No | |
| `ip_address` | string | No | |
| `os_info` | string | No | |
| `kernel_version` | string | Yes | |
| `packages` | []Package | Yes | Danh sách packages cài đặt |
| `package_count` | int | No | |
| `cve_count` | int | No | Số CVE phát hiện |
| `reported_at` | timestamp | No | |
| `processed_at` | timestamp | Yes | |
| `created_at` | timestamp | No | |

---

## 15. Package

Software package được cài đặt trên host agent.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `report_id` | UUID | No | FK → AgentReport |
| `name` | string | No | |
| `version` | string | No | |
| `ecosystem` | PackageEcosystem | No | Hệ sinh thái package |
| `architecture` | string | Yes | `amd64`, `arm64`, v.v. |
| `cves` | []PackageCVE | Yes | CVEs liên quan (enriched) |

**Enums — PackageEcosystem**: `debian`, `rpm`, `homebrew`, `pypi`, `npm`, `go`

---

## 16. Schedule

Recurring scan schedule aggregate (domain/schedule package).

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `name` | string | No | Tên schedule |
| `description` | string | Yes | |
| `cron_expr` | string | No | Cron expression, e.g. `0 2 * * *` |
| `type` | ScheduleType | No | Loại scan |
| `target_ids` | []string | No | Product/asset IDs cần scan |
| `status` | ScheduleStatus | No | Trạng thái schedule |
| `last_run_at` | *timestamp | Yes | |
| `next_run_at` | *timestamp | Yes | |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums — ScheduleType**:

| Giá trị | Mô tả |
|---------|-------|
| `full_scan` | Full vulnerability scan |
| `incremental_scan` | Chỉ scan những thay đổi mới |
| `targeted_scan` | Scan theo target cụ thể |

**Enums — ScheduleStatus**:

| Giá trị | Mô tả |
|---------|-------|
| `active` | Đang kích hoạt, sẽ trigger scans |
| `paused` | Tạm dừng |
| `disabled` | Vô hiệu hóa vĩnh viễn |

**State transitions**: `active ↔ paused`, `active/paused → disabled`

**Frequency presets**:

| Tên | Cron Expression |
|-----|----------------|
| `hourly` | `0 * * * *` |
| `daily` | `0 2 * * *` |
| `weekly` | `0 2 * * 0` |

---

## 17. Relationships

```
Scan ──────────────────── Finding (1:N)
Scan ──────────────────── WebAlert (1:N)
Scan ──────────────────── DiscoveryHost (1:N)
Scan ──────────────────── AgentReport (trigger)
Asset ─────────────────── Vulnerability (1:N)
Asset ─────────────────── Tag (N:M)
Agent ─────────────────── AgentReport (1:N)
AgentReport ───────────── Package (1:N)
ScheduledScan ─────────── Scan (triggers, 1:N)
```

---

## 17b. ScheduledScan *(NEW — TASK-HC-011)*

Cấu hình scan được lên lịch tự động theo cron expression. Wired với PostgreSQL `ScheduleRepo` thay nil handler.

> **Table:** `scheduled_scans`  
> **API (scan-service):** `POST /api/v1/scans/scheduled`, `GET /api/v1/scans/scheduled`, `GET /api/v1/scans/scheduled/{id}`, `PUT /api/v1/scans/scheduled/{id}`, `DELETE /api/v1/scans/scheduled/{id}`

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `name` | string | No | Tên schedule |
| `description` | string | Yes | Mô tả mục đích |
| `cron_expr` | string | No | Cron expression, e.g. `0 2 * * *` (daily 2AM) |
| `targets` | []string | No | Danh sách IPs, hostnames, URLs cần scan |
| `scan_type` | string | No | `full` \| `discovery` \| `web` \| `agent` |
| `status` | ScheduleStatus | No | `active` \| `paused` \| `disabled` |
| `last_run_at` | *timestamp | Yes | Thời điểm trigger gần nhất |
| `next_run_at` | *timestamp | Yes | Thời điểm trigger tiếp theo |
| `created_by` | UUID | Yes | FK → User |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Frequency presets** (cron aliases):

| Tên | Cron Expression |
|-----|----------------|
| `hourly` | `0 * * * *` |
| `daily` | `0 2 * * *` |
| `weekly` | `0 2 * * 0` |
| `monthly` | `0 2 1 * *` |

> **Wiring:** `ScheduleHandler` → `ScheduleRepository` (PostgreSQL) — không còn nil trong embedded mode (TASK-HC-011)  
> **Khi scheduleHandler nil:** Route `/api/v1/scans/scheduled` trả `{"scheduled_scans":[],"total":0}` (graceful fallback)  
> **Khi importHandler nil:** `POST /api/v1/scans/import` trả `501 Not Implemented`

