# Data Models — finding-service

> **Service**: `services/finding-service`  
> **Mô tả**: Quản lý toàn bộ vòng đời lỗ hổng bảo mật (vulnerability lifecycle): từ khi phát hiện đến khi khắc phục. Bao gồm Products, Engagements, Tests, Findings, Risk Acceptances, SLA và Notes.  
> **Storage**: PostgreSQL  
> **Go packages**: `domain/finding`, `domain/engagement`, `domain/note`, `domain/group`, `domain/riskacceptance`, `domain/sla`

---

## 1. ProductType

Nhóm phân loại sản phẩm cấp cao (ví dụ: "Web Application", "Mobile App").

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `name` | string | No | Tên loại sản phẩm, duy nhất |
| `description` | string | Yes | |
| `critical_product` | bool | No | Đánh dấu là sản phẩm ưu tiên cao |
| `key_product` | bool | No | Sản phẩm quan trọng với business |
| `enable_full_risk_acceptance` | bool | No | |
| `enable_simple_risk_acceptance` | bool | No | |
| `tags` | []string | No | |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

---

## 2. Product

Đơn vị phần mềm được kiểm tra bảo mật.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `product_type_id` | UUID | No | FK → ProductType |
| `name` | string | No | Tên sản phẩm, duy nhất |
| `description` | string | Yes | |
| `prod_numeric_grade` | int | No | Điểm tổng hợp 1–100 |
| `business_criticality` | BusinessCriticality | No | Mức độ quan trọng với business |
| `platform` | Platform | No | Nền tảng triển khai |
| `lifecycle` | Lifecycle | No | Giai đoạn vòng đời |
| `origin` | Origin | No | Nguồn gốc sản phẩm |
| `external_audience` | bool | No | Có phục vụ người dùng bên ngoài |
| `internet_accessible` | bool | No | Có thể truy cập từ internet |
| `enable_full_risk_acceptance` | bool | No | |
| `enable_simple_risk_acceptance` | bool | No | |
| `enable_product_tag_inheritance` | bool | No | Tag kế thừa từ product xuống finding |
| `sla_configuration_id` | UUID | Yes | FK → SLAConfiguration |
| `tags` | []string | No | |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums**:

| Enum | Giá trị |
|------|---------|
| BusinessCriticality | `very high`, `high`, `medium`, `low`, `very low`, `none` |
| Platform | `web`, `mobile`, `desktop`, `api`, `iot` |
| Lifecycle | `construction`, `production`, `retirement` |
| Origin | `internal`, `contractor`, `outsourced`, `open source`, `purchased` |

---

## 3. Engagement

Sự kiện kiểm tra bảo mật trong một Product. Có thể là kiểm tra thủ công hoặc CI/CD tự động.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `product_id` | UUID | No | FK → Product |
| `name` | string | No | Tên engagement |
| `description` | string | Yes | |
| `lead_id` | UUID | Yes | FK → User (người phụ trách) |
| `engagement_type` | EngagementType | No | `Interactive` \| `CI/CD` |
| `status` | Status | No | Trạng thái vòng đời |
| `start_date` | timestamp | No | |
| `end_date` | timestamp | Yes | |
| `version` | string | Yes | Phiên bản phần mềm |
| `build_id` | string | Yes | Build ID (CI/CD) |
| `commit_hash` | string | Yes | Git commit hash |
| `branch_tag` | string | Yes | Branch/tag |
| `source_code_management_uri` | string | Yes | URL SCM repository |
| `deduplication_on_engagement` | bool | No | Dedup findings trong scope engagement này |
| `build_server_id` | UUID | Yes | FK → ToolConfiguration (build server) |
| `orchestration_engine_id` | UUID | Yes | FK → ToolConfiguration (CI/CD engine) |
| `tags` | []string | No | |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums — EngagementType**: `Interactive`, `CI/CD`  
**Enums — Status**: `Not Started`, `In Progress`, `On Hold`, `Completed`, `Cancelled`

---

## 4. Test

Một lần scan đơn lẻ trong một Engagement.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `engagement_id` | UUID | No | FK → Engagement |
| `scan_type` | string | No | Loại scan: `Trivy Scan`, `Bandit Scan`, `SARIF`, v.v. |
| `title` | string | No | |
| `description` | string | Yes | |
| `target_start` | timestamp | No | |
| `target_end` | timestamp | Yes | |
| `lead_id` | UUID | Yes | FK → User |
| `version` | string | Yes | |
| `build_id` | string | Yes | |
| `commit_hash` | string | Yes | |
| `branch_tag` | string | Yes | |
| `percent_complete` | int | No | 0–100 |
| `tags` | []string | No | |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

---

## 5. Finding

Lỗ hổng bảo mật được phát hiện. Entity trung tâm của toàn bộ hệ thống.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `title` | string | No | Tiêu đề lỗ hổng |
| `description` | string | Yes | Mô tả chi tiết |
| `mitigation` | string | Yes | Hướng dẫn khắc phục |
| `impact` | string | Yes | Tác động |
| `references` | string | Yes | Tài liệu tham khảo |
| `severity` | Severity | No | Mức nghiêm trọng |
| `numerical_severity` | int | No | Giá trị số tương ứng (4=Critical, 3=High, ...) |
| `cve` | string | Yes | CVE ID, ví dụ `CVE-2021-44228` |
| `cwe` | int | Yes | CWE number, ví dụ `79` |
| `vuln_id_from_tool` | string | Yes | ID từ tool scan |
| `cvss_v3` | string | Yes | CVSS v3 vector string |
| `cvss_v3_score` | float64 | Yes | |
| `cvss_v4` | string | Yes | CVSS v4 vector string |
| `cvss_v4_score` | float64 | Yes | |
| `epss_score` | float64 | Yes | EPSS probability 0–1 |
| `is_kev` | bool | No | Có trong CISA KEV catalog |
| `active` | bool | No | false = đã đóng/giải quyết |
| `verified` | bool | No | Đã xác nhận thủ công bởi analyst |
| `false_positive` | bool | No | False alarm từ tool |
| `duplicate` | bool | No | Trùng lặp với finding khác |
| `out_of_scope` | bool | No | Ngoài phạm vi engagement |
| `is_mitigated` | bool | No | Đã vá/khắc phục |
| `risk_accepted` | bool | No | Stakeholder đã chấp nhận rủi ro |
| `date` | timestamp | No | Ngày phát hiện |
| `mitigated_at` | timestamp | Yes | |
| `mitigated_by_id` | UUID | Yes | FK → User |
| `last_reviewed` | timestamp | Yes | |
| `last_status_update` | timestamp | Yes | |
| `sla_expiration_date` | timestamp | Yes | Deadline SLA |
| `assigned_to` | string | Yes | Email người được giao |
| `test_id` | UUID | No | FK → Test |
| `engagement_id` | UUID | No | FK → Engagement |
| `product_id` | UUID | No | FK → Product |
| `duplicate_finding_id` | UUID | Yes | FK → Finding gốc nếu là duplicate |
| `hash_code` | string | No | SHA-256(title\|component\|version\|cve) để dedup |
| `component_name` | string | Yes | Tên component: IP, tên package, hoặc filename |
| `component_version` | string | Yes | |
| `service` | string | Yes | |
| `file_path` | string | Yes | |
| `line_number` | int | Yes | |
| `asset_ip` | string | Yes | IP của asset liên quan |
| `asset_hostname` | string | Yes | Hostname của asset liên quan |
| `tags` | []string | No | |
| `inherited_tags` | []string | No | Tags kế thừa từ product/engagement |
| `created_by` | *string | Yes | Email của người tạo finding |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums — Severity**: `Critical`, `High`, `Medium`, `Low`, `Info`

---

## 6. FindingNote

Ghi chú của analyst đính vào finding.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `finding_id` | UUID | No | FK → Finding |
| `author_id` | UUID | No | FK → User |
| `content` | string | No | Nội dung ghi chú |
| `note_type` | string | Yes | Nhãn loại ghi chú |
| `edit_count` | int | No | Số lần chỉnh sửa |
| `is_private` | bool | No | Private = chỉ author và manager xem được |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

---

## 7. FindingGroup

Nhóm logic tập hợp các findings liên quan (ví dụ: cùng CVE ở nhiều nơi).

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `name` | string | No | |
| `product_id` | UUID | No | FK → Product |
| `jira_issue_key` | string | Yes | Link JIRA issue |
| `finding_count` | int | No | Số findings (denormalized) |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

---

## 8. RiskAcceptance

Chấp nhận rủi ro chính thức cho một tập findings, có thể có ngày hết hạn.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `name` | string | No | Mô tả, ví dụ "Accept Log4Shell risk until patch" |
| `product_id` | UUID | No | FK → Product |
| `accepted_by_id` | UUID | No | FK → User (owner chấp nhận) |
| `expiration_date` | timestamp | Yes | Null = không hết hạn |
| `notes` | string | Yes | |
| `proof_file_key` | string | Yes | MinIO object key cho tài liệu hỗ trợ |
| `reactivate_expired` | bool | No | Tái kích hoạt findings khi RA hết hạn |
| `reactivate_note_text` | string | Yes | Ghi chú thêm vào finding khi tái kích hoạt |
| `restart_sla_on_reactivation` | bool | No | Reset SLA start date khi tái kích hoạt |
| `is_expired` | bool | No | |
| `finding_ids` | []UUID | No | Danh sách Finding IDs được chấp nhận |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

---

## 9. SLAConfiguration

Cấu hình thời hạn khắc phục theo severity cho một product.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | Khóa chính |
| `product_id` | UUID | Yes | Null = global default |
| `critical` | int | No | Số ngày để khắc phục Critical (mặc định: 7) |
| `high` | int | No | Số ngày cho High (mặc định: 30) |
| `medium` | int | No | Số ngày cho Medium (mặc định: 90) |
| `low` | int | No | Số ngày cho Low (mặc định: 180) |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

---

## 10. ProductMember & ProductTypeMember

Quản lý RBAC membership cho Products và ProductTypes.

### ProductMember

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `product_id` | UUID | No | FK → Product |
| `user_id` | UUID | No | FK → User |
| `role` | Role | No | RBAC role trong product |
| `created_at` | timestamp | No | |

### ProductTypeMember

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `product_type_id` | UUID | No | FK → ProductType |
| `user_id` | UUID | No | FK → User |
| `role` | Role | No | |
| `created_at` | timestamp | No | |

**Enums — Role**: `Owner` (5) > `Maintainer` (4) > `Writer` (3) > `API Importer` (2) > `Reader` (1)

---

## 11. ToolConfiguration

Cấu hình credentials cho external tools (build server, SCM, v.v.). Passwords/API keys mã hóa AES-256-GCM.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | UUID | No | |
| `name` | string | No | |
| `description` | string | Yes | |
| `tool_type` | string | No | `GitHub`, `GitLab`, `Jira`, `Slack`, `SonarQube`, `Jenkins`, v.v. |
| `url` | string | No | |
| `auth_type` | AuthType | No | Kiểu xác thực |
| `username` | string | Yes | |
| `password_enc` | string | Yes | AES-256-GCM encrypted |
| `api_key_enc` | string | Yes | AES-256-GCM encrypted |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums — AuthType**: `api_key`, `http_basic`, `ssh`, `bearer`

---

## 12. Relationships

```
ProductType ──────────────── Product (1:N)
Product ──────────────────── Engagement (1:N)
Product ──────────────────── ProductMember (1:N)
Product ──────────────────── RiskAcceptance (1:N)
Product ──────────────────── SLAConfiguration (1:1 or 1:N)
ProductType ──────────────── ProductTypeMember (1:N)
Engagement ───────────────── Test (1:N)
Test ──────────────────────── Finding (1:N)
Finding ───────────────────── FindingNote (1:N)
Finding ───────────────────── FindingGroup (N:M via group_finding)
Finding ───────────────────── RiskAcceptance (N:M via finding_ids)
Engagement → ToolConfiguration (build_server_id, orchestration_engine_id)
```
