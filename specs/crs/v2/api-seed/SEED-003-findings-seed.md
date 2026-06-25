# Change Request SEED-003: Seed Findings từ Client qua Gateway

**Cập nhật:** 2026-06-18  
**Status:** Proposed  
**Domain:** finding-service  
**Priority:** 🔴 CRITICAL — Đây là domain data chính của hệ thống  
**Depends on:** SEED-001 (users), SEED-002 (products, engagements, tests)

---

## 1. Bối cảnh

Finding là entity trung tâm của hệ thống. Client cần có khả năng:
1. **Import hàng loạt** findings từ external security tools (không qua scan-service) — ví dụ: kết quả từ công cụ nội bộ, spreadsheet, JIRA import.
2. **Tạo manual findings** từ analyst tay (không qua automated scan).
3. **Bulk update** trạng thái và metadata của nhiều findings cùng lúc.

Phân tích hiện trạng:

| Use-case | Endpoint hiện tại | Trạng thái |
|---------|------------------|-----------|
| Tạo 1 finding | `POST /api/v2/findings` | ✅ Có |
| Bulk create findings | **THIẾU** | ❌ Không hỗ trợ |
| Import findings từ JSON file | `POST /api/v1/scans/import` | ⚠️ Chỉ cho scan result files (nmap, ZAP formats), không cho raw JSON |
| Import findings từ CSV | **THIẾU** | ❌ Không hỗ trợ |
| Bulk update severity/status | `POST /api/v2/findings/bulk` | ⚠️ Có nhưng không rõ schema |
| Bulk set tags trên findings | **THIẾU** | ❌ Không hỗ trợ |
| Import kèm CVE enrichment auto | **THIẾU** | ❌ Phải enrich thủ công sau khi tạo |
| Tạo finding với SLA auto-compute | ⚠️ Chưa rõ | ⚠️ SLA expiration date có được tính ngay khi tạo? |
| Validate CVE ID khi tạo finding | **THIẾU** | ❌ Không rõ backend validate CVE-xxxx-xxxx format |

---

## 2. Thay đổi Đề Xuất

### 2.1 [CRITICAL] `POST /api/v2/findings/bulk-create` — Bulk create findings

Cho phép seed nhiều findings trong một request. Khác với `POST /api/v2/findings/bulk` (là bulk update).

**Gateway**:
```
POST /api/v2/findings/bulk-create  →  finding-service:8085  (authenticated, Writer+)
```

**Request body**:
```json
{
  "test_id": "test-uuid",
  "findings": [
    {
      "title": "SQL Injection in login form",
      "description": "The login endpoint is vulnerable to SQL injection via the 'username' parameter.",
      "mitigation": "Use parameterized queries or prepared statements.",
      "severity": "Critical",
      "cve": "CVE-2021-44228",
      "cwe": 89,
      "cvss_v3_score": 9.8,
      "component_name": "auth-service",
      "component_version": "1.2.3",
      "date": "2026-06-01T00:00:00Z",
      "tags": ["webapp", "injection"]
    },
    {
      "title": "Cross-Site Scripting (XSS)",
      "severity": "High",
      "cwe": 79,
      "component_name": "frontend",
      "date": "2026-06-02T00:00:00Z"
    }
  ],
  "auto_close_duplicates": true,
  "auto_enrich_cve": true
}
```

**Request options**:

| Option | Type | Mô tả |
|--------|------|-------|
| `test_id` | UUID | Test mà findings thuộc về |
| `auto_close_duplicates` | bool | Tự động đóng findings trùng hash_code |
| `auto_enrich_cve` | bool | Kích hoạt AI enrichment cho CVE IDs (async) |
| `compute_sla` | bool | Tính SLA expiration date ngay khi tạo (default: true) |

**Response** `207 Multi-Status`:
```json
{
  "created_count": 2,
  "duplicate_count": 0,
  "failed_count": 0,
  "results": [
    { "index": 0, "status": "created", "id": "finding-uuid-1", "hash_code": "abc123" },
    { "index": 1, "status": "created", "id": "finding-uuid-2", "hash_code": "def456" }
  ],
  "errors": []
}
```

---

### 2.2 [CRITICAL] `POST /api/v2/findings/import` — Import từ JSON/CSV file

Cho phép upload file để import hàng loạt findings. Hỗ trợ định dạng JSON array và CSV.

**Gateway**:
```
POST /api/v2/findings/import  →  finding-service:8085  (authenticated, API Importer+)
Content-Type: multipart/form-data
```

**Form fields**:
| Field | Type | Bắt buộc | Mô tả |
|-------|------|----------|-------|
| `file` | File | ✅ | JSON array hoặc CSV file |
| `test_id` | string | ✅ | UUID của Test |
| `format` | string | ✅ | `json` \| `csv` |
| `auto_close_duplicates` | string | No | `true`/`false` |
| `minimum_severity` | string | No | Lọc, chỉ import từ severity này trở lên |

**JSON file format**:
```json
[
  {
    "title": "SQL Injection",
    "severity": "Critical",
    "cve": "CVE-2021-44228",
    "cwe": 89,
    "description": "...",
    "mitigation": "...",
    "component_name": "auth-service",
    "component_version": "1.2.3"
  }
]
```

**CSV file format** (header row required):
```
title,severity,cve,cwe,description,mitigation,component_name,component_version
SQL Injection,Critical,CVE-2021-44228,89,"Vulnerable to SQLi","Use prepared statements",auth-service,1.2.3
```

**Response** `200 OK`:
```json
{
  "total_rows": 25,
  "imported_count": 23,
  "duplicate_count": 1,
  "failed_count": 1,
  "errors": [
    { "row": 15, "message": "Invalid severity value: 'Crit'" }
  ]
}
```

**Giới hạn**: Tối đa 1000 findings/file, file size tối đa 10MB.

---

### 2.3 [HIGH] `POST /api/v2/findings/bulk` — Làm rõ và mở rộng schema

Endpoint `POST /api/v2/findings/bulk` hiện tồn tại nhưng schema chưa được document rõ. Cần chuẩn hóa.

**Gateway**:
```
POST /api/v2/findings/bulk  →  finding-service:8085  (authenticated, Writer+)
```

**Request body** (chuẩn hóa):
```json
{
  "action": "update_status",
  "finding_ids": ["uuid-1", "uuid-2", "uuid-3"],
  "payload": {
    "active": false,
    "is_mitigated": true,
    "mitigated_at": "2026-06-18T00:00:00Z",
    "notes": "Patched in version 2.1.1"
  }
}
```

**Supported actions**:

| Action | Payload fields | Mô tả |
|--------|---------------|-------|
| `update_status` | `active`, `is_mitigated` | Thay đổi trạng thái |
| `set_severity` | `severity` | Điều chỉnh severity hàng loạt |
| `set_tags` | `tags`, `mode` (add/remove/set) | Quản lý tags |
| `set_assignee` | `assigned_to` | Gán người phụ trách |
| `mark_false_positive` | `false_positive: true` | Đánh dấu false positive |
| `close` | `notes` | Đóng findings |
| `reopen` | — | Mở lại findings |
| `delete` | — | Xóa findings (requires Maintainer+) |

---

### 2.4 [HIGH] Đảm bảo `POST /api/v2/findings` trả về đầy đủ — SLA tự động

Khi tạo finding, `sla_expiration_date` phải được tính tự động từ SLA configuration của product.

**Yêu cầu với finding-service**:
- Khi `POST /api/v2/findings`, service phải:
  1. Load `SLAConfiguration` của product (hoặc global default)
  2. Tính `sla_expiration_date = date + sla_config.days_for(severity)`
  3. Lưu `sla_expiration_date` và trả về trong response

**Response bổ sung**:
```json
{
  "id": "uuid",
  "title": "SQL Injection",
  "severity": "Critical",
  "date": "2026-06-01T00:00:00Z",
  "sla_expiration_date": "2026-06-08T00:00:00Z",
  "days_until_sla": 7,
  ...
}
```

---

### 2.5 [MEDIUM] `POST /api/v2/findings/{id}/notes` — Bulk add notes

Khi seeding, cần thêm nhiều notes vào một finding (lịch sử, context).

**Thêm `notes` array vào `POST /api/v2/findings/bulk-create`**:
```json
{
  "findings": [
    {
      "title": "SQL Injection",
      "severity": "Critical",
      "notes": [
        { "content": "Discovered during manual pentest Q1 2026", "is_private": false },
        { "content": "Assigned to security team for remediation", "is_private": false }
      ]
    }
  ]
}
```

---

### 2.6 [MEDIUM] `POST /api/v2/finding-groups/bulk` — Bulk create finding groups

**Gateway**:
```
POST /api/v2/finding-groups/bulk  →  finding-service:8085  (authenticated, Writer+)
```

**Request body**:
```json
{
  "groups": [
    {
      "name": "Log4Shell variants",
      "product_id": "product-uuid",
      "finding_ids": ["finding-uuid-1", "finding-uuid-2"]
    }
  ]
}
```

---

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `POST /api/v2/findings/bulk-create` với 10 findings → `207` với `created_count: 10`, mỗi finding có `sla_expiration_date` tự động tính.
2. `POST /api/v2/findings/bulk-create` với 2 findings trùng hash_code và `auto_close_duplicates: true` → `207` với `duplicate_count: 1`, finding thứ 2 không tạo mới.
3. `POST /api/v2/findings/import` với JSON file 25 findings → `200` với `imported_count >= 24`.
4. `POST /api/v2/findings/import` với CSV file → `200` với results tương tự JSON.
5. `POST /api/v2/findings/import` với `minimum_severity: "High"` → chỉ import Critical + High findings.
6. `POST /api/v2/findings/bulk` action `set_tags` với `mode: "add"` → tags được thêm vào, không thay thế.
7. `POST /api/v2/findings` mà không có `sla_expiration_date` → backend tự tính và trả về field này.
8. `POST /api/v2/findings/import` với file > 10MB → `413 Payload Too Large`.
9. Caller với role `API Importer` có thể dùng `/import` và `/bulk-create`; role `Reader` → `403 Forbidden`.
