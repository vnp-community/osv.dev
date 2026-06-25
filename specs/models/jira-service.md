# Data Models — jira-service

> **Service**: `services/jira-service`  
> **Mô tả**: Tích hợp với Atlassian JIRA để tự động tạo và đồng bộ JIRA issues từ security findings. Lưu trữ connection credentials được mã hóa AES-256-GCM.  
> **Storage**: PostgreSQL  
> **Go package**: `services/jira-service/internal/domain/jiraconfig`  
> **Cập nhật:** 2026-06-24 — Thêm JiraIssueMapping, JiraSyncLog (TASK-HC-013)

---

## 1. JIRAConfig

Cấu hình kết nối giữa một Product và một JIRA project.  
Credentials được lưu dưới dạng mã hóa AES-256-GCM.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `product_id` | UUID | No | FK → Product |
| `url` | string | No | URL JIRA instance, e.g. `https://myorg.atlassian.net` |
| `username` | string | No | JIRA username/email |
| `password_enc` | string | No | AES-256-GCM encrypted API token |
| `project_key` | string | No | JIRA project key, e.g. `SEC` |
| `issue_type_id` | string | Yes | ID của issue type |
| `issue_type_fields` | map[string]interface{} | Yes | Custom fields cho issue type |
| `default_assignee` | string | Yes | Username người được giao mặc định |
| `find_severity_field` | string | Yes | Custom field name cho severity |
| `find_url_field` | string | Yes | Custom field name cho finding URL |
| `push_notes` | bool | No | Sync finding notes sang JIRA comments |
| `push_all_issues` | bool | No | Sync tất cả issues thay vì chỉ active |
| `enable_deduplication` | bool | No | Kiểm tra JIRA issue đã tồn tại trước khi tạo mới |
| `priority_mapping` | map[string]string | No | DefectDojo Severity → JIRA Priority |
| `webhook_secret` | string | Yes | HMAC secret để verify JIRA webhook signatures |
| `is_active` | bool | No | |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Default priority_mapping**:

| DefectDojo Severity | JIRA Priority |
|--------------------|---------------|
| `Critical` | `Highest` |
| `High` | `High` |
| `Medium` | `Medium` |
| `Low` | `Low` |
| `Info` | `Lowest` |

---

## 2. JiraIssueMapping *(NEW — TASK-HC-013)*

Lưu trữ mối liên kết giữa một security finding và JIRA issue tương ứng. Cho phép bidirectional sync.

> **Table:** `jira_issue_mappings`  
> **Migration:** `migrations/002_jira_issue_mappings.sql`  
> **API:** `GET /api/v2/jira-issues`, `POST /api/v2/jira-issues`, `GET /api/v2/jira-issues/{finding_id}`, `DELETE /api/v2/jira-issues/{id}`

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `finding_id` | UUID | No | FK → Finding (finding-service); UNIQUE — 1 finding → 1 JIRA issue |
| `jira_configuration_id` | UUID | Yes | FK → JIRAConfig (SET NULL on delete) |
| `jira_id` | string | No | Internal JIRA issue ID |
| `jira_key` | string | No | JIRA display key, e.g. `SEC-123` |
| `jira_url` | string | No | Browser URL đến JIRA issue |
| `jira_status` | string | Yes | JIRA workflow status: `To Do` \| `In Progress` \| `Done` |
| `jira_priority` | string | Yes | JIRA priority: `Highest` \| `High` \| `Medium` \| `Low` \| `Lowest` |
| `synced` | bool | No | true = mapping đã được sync thành công |
| `last_sync_at` | *timestamp | Yes | Thời điểm sync gần nhất |
| `sync_error` | string | Yes | Thông báo lỗi của lần sync cuối (nếu có) |
| `created_at` | timestamp | No | |

**Indexes**:
- `idx_jira_mapping_key` trên `jira_key`
- `idx_jira_mapping_finding` trên `finding_id` (UNIQUE)
- `idx_jira_mapping_config` trên `jira_configuration_id` (WHERE NOT NULL)

---

## 3. JiraSyncLog *(NEW — TASK-HC-013)*

Ghi lại lịch sử từng sự kiện đồng bộ dữ liệu giữa hệ thống và JIRA.

> **Table:** `jira_sync_log`  
> **Migration:** `migrations/002_jira_issue_mappings.sql`

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `mapping_id` | UUID | No | FK → JiraIssueMapping |
| `direction` | string | No | `push` (hệ thống → JIRA) \| `pull` (JIRA → hệ thống) |
| `status` | string | No | `success` \| `failed` \| `skipped` |
| `error_message` | string | Yes | Chi tiết lỗi nếu status = `failed` |
| `synced_at` | timestamp | No | |

---

## 4. JIRAPriority Helper

Hàm `JIRAPriority(severity string) string` tra cứu JIRA priority từ DefectDojo severity:
- Nếu có trong `priority_mapping` → trả về mapped value
- Nếu không → fallback `"Medium"`

---

## 5. HTTP API Endpoints *(Updated)*

### Jira Config Endpoints

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/v2/jira-configurations/{product_id}` | Lấy JIRA config của product |
| `POST` | `/api/v2/jira-configurations` | Tạo hoặc cập nhật JIRA config |
| `DELETE` | `/api/v2/jira-configurations/{product_id}` | Xóa JIRA config |
| `POST` | `/jira/push/{finding_id}` | Push finding lên JIRA |
| `POST` | `/jira/webhook` | Nhận webhook từ JIRA (status sync) |

### Jira Issue Mapping Endpoints *(NEW — TASK-HC-013)*

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/v2/jira-issues` | Liệt kê tất cả issue mappings |
| `POST` | `/api/v2/jira-issues` | Tạo mapping finding ↔ JIRA issue |
| `GET` | `/api/v2/jira-issues/{finding_id}` | Lấy JIRA issue của finding |
| `DELETE` | `/api/v2/jira-issues/{id}` | Xóa mapping |

**Response khi issueRepo chưa cấu hình**: `503 Service Unavailable`

---

## 6. Relationships *(Updated)*

```
JIRAConfig ─── Product (finding-service) (N:1, per product)
JIRAConfig ──→ Finding (finding-service) (1:N, khi push)
JIRAConfig ──→ JIRA Issue (external, via API)
JiraIssueMapping ─── Finding (finding-service) (1:1, UNIQUE finding_id)
JiraIssueMapping ─── JIRAConfig (N:1, via jira_configuration_id)
JiraIssueMapping ─── JiraSyncLog (1:N, audit trail)
```
