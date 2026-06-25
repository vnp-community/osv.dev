# CR-UI-009 — Reports & Notifications API

**Series:** UI-API v2  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🟢 Mock Layer Complete / Backend Pending  
**Ưu tiên:** P0 — Critical  
**Nguồn yêu cầu:** `ui/specs/TDD.md` §10, `docs/SRS.md` §3.8, §3.7, `docs/PRD.md` §4.6, §4.5  
**Services ảnh hưởng:** `gateway (:8080)`, `finding-service (:8085)`, `notification-service (:8087)`

---

## 1. Bối cảnh

### Module Reports (`/reports`)
Cho phép generate và download reports (PDF, HTML, CSV, Excel, JSON) per product/engagement. Reports được stored trong MinIO/S3.

### Module Notifications (`/notifications`)
Notification Center hiển thị in-app alerts — notifications được push qua SSE (xem CR-UI-002 §2.3) và stored trong `notification-service`.

---

## 2. Report API Endpoints

### 2.1 GET /api/v1/reports

**Mô tả:** List reports đã generate.

**Auth:** Required (`report:download`)

**Query Params:** `product_id=xxx`, `format=pdf,html`, `status=completed`, `page=1`, `page_size=20`

**Response 200:**
```json
{
  "reports": [
    {
      "id": "rpt_001",
      "product_id": "prod_1",
      "product_name": "Banking Portal",
      "engagement_id": null,
      "format": "pdf",
      "status": "completed",
      "exit_code": 1,
      "min_severity": "High",
      "min_score": 7.0,
      "finding_count": 10,
      "generated_at": "2026-06-16T11:00:00Z",
      "artifact_url": "https://storage.company.com/reports/rpt_001.pdf",
      "expires_at": "2026-07-16T11:00:00Z",
      "created_at": "2026-06-16T10:58:00Z",
      "created_by": "carol@company.com"
    }
  ],
  "total": 24,
  "page": 1,
  "page_size": 20
}
```

**Report Status Values:** `pending | generating | completed | failed`

**Exit Code (CI/CD):** `0` = no findings above threshold; `1` = findings found above threshold

---

### 2.2 POST /api/v1/reports

**Mô tả:** Request report generation.

**Auth:** Required (`report:download`)

**Request Body:**
```json
{
  "product_id": "prod_1",
  "engagement_id": null,
  "format": "pdf",
  "min_severity": "High",
  "min_score": 7.0,
  "date_from": "2026-01-01",
  "date_to": "2026-06-16"
}
```

| Field | Type | Required | Mô tả |
|-------|------|----------|-------|
| `product_id` | string | ❌* | Filter by product (*one of product_id or engagement_id required) |
| `engagement_id` | string | ❌* | Filter by engagement |
| `format` | string | ✅ | `pdf\|html\|csv\|excel\|json` |
| `min_severity` | string | ❌ | Include only >= severity |
| `min_score` | float | ❌ | CVSS threshold for CI/CD exit code |
| `date_from` | date | ❌ | Finding created_at from |
| `date_to` | date | ❌ | Finding created_at to |

**Response 202:**
```json
{
  "id": "rpt_002",
  "product_id": "prod_1",
  "format": "pdf",
  "status": "pending",
  "created_at": "2026-06-16T12:00:00Z",
  "created_by": "carol@company.com"
}
```

**Side effects:**
- finding-service generates report asynchronously
- Khi completed → upload to MinIO/S3 → update `artifact_url` và `status=completed`
- Publish NATS `report.completed` event → notification-service → in-app notification

---

### 2.3 GET /api/v1/reports/{id}

**Mô tả:** Get report status và metadata.

**Auth:** Required (`report:download`)

**Response 200:** ReportRun object (full format from §2.1)

---

### 2.4 GET /api/v1/reports/{id}/download

**Mô tả:** Download report file.

**Auth:** Required (`report:download`)

**Response 200:**
- `Content-Type`: `application/pdf` | `text/html` | `text/csv` | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` | `application/json`
- `Content-Disposition`: `attachment; filename="banking-portal-report-2026-06-16.pdf"`
- Body: Binary file content

**Response 404:** Report not found or expired

**Notes:**
- Endpoint redirect to presigned S3/MinIO URL (302 redirect) **OR** stream binary directly
- Presigned URL approach preferred: `302 Location: https://minio.../reports/rpt_001.pdf?signature=xxx`
- Presigned URL TTL: 5 minutes

---

### 2.5 DELETE /api/v1/reports/{id}

**Mô tả:** Delete report (remove from storage).

**Auth:** Required (`report:download`)

**Response 200:**
```json
{ "success": true }
```

---

## 3. Notification API Endpoints

### 3.1 GET /api/v1/notifications

**Mô tả:** List notifications (stored, not SSE).

**Auth:** Required

**Query Params:** `is_read=false`, `type=finding.sla.breached,kev.new`, `page=1`, `page_size=20`

**Response 200:**
```json
{
  "notifications": [
    {
      "id": "notif_001",
      "type": "finding.sla.breached",
      "title": "SLA Breached: CVE-2025-44228 (Banking Portal)",
      "message": "Finding F-2847 has exceeded the Critical SLA deadline by 7 days.",
      "severity": "Critical",
      "entity_type": "finding",
      "entity_id": "F-2847",
      "is_read": false,
      "created_at": "2026-06-16T08:00:00Z"
    },
    {
      "id": "notif_002",
      "type": "kev.new",
      "title": "New KEV: Apache Struts RCE (CVE-2026-12345)",
      "message": "CVE-2026-12345 was added to CISA KEV catalog. Known ransomware campaign usage.",
      "severity": "Critical",
      "entity_type": "cve",
      "entity_id": "CVE-2026-12345",
      "is_read": true,
      "created_at": "2026-06-16T06:00:00Z"
    }
  ],
  "total": 28,
  "unread_count": 5,
  "page": 1,
  "page_size": 20
}
```

---

### 3.2 PATCH /api/v1/notifications/{id}/read

**Mô tả:** Mark notification as read.

**Auth:** Required

**Response 200:**
```json
{ "id": "notif_001", "is_read": true }
```

---

### 3.3 POST /api/v1/notifications/mark-all-read

**Mô tả:** Mark tất cả notifications as read.

**Auth:** Required

**Response 200:**
```json
{ "marked_count": 5 }
```

---

### 3.4 GET /api/v1/notifications/unread-count

**Mô tả:** Unread count cho bell icon badge — polled mỗi 60s hoặc nhận qua SSE.

**Auth:** Required

**Response 200:**
```json
{ "unread_count": 5 }
```

---

### 3.5 GET /api/v1/webhooks

**Mô tả:** List registered webhooks — dùng trong Integrations > Webhooks screen.

**Auth:** Required (`system:configure`)

**Response 200:**
```json
{
  "webhooks": [
    {
      "id": "wh_001",
      "url": "https://hooks.slack.com/services/...",
      "events": ["kev.new", "finding.sla.breached"],
      "is_active": true,
      "secret_preview": "sha256:a1b2c3...",
      "created_at": "2026-06-01T00:00:00Z",
      "last_delivery_at": "2026-06-16T08:00:00Z",
      "last_delivery_status": "success"
    }
  ],
  "total": 3
}
```

---

### 3.6 POST /api/v1/webhooks

**Mô tả:** Register new webhook.

**Auth:** Required (`system:configure`)

**Request Body:**
```json
{
  "url": "https://hooks.slack.com/services/xxx",
  "events": ["kev.new", "finding.sla.breached", "scan.completed"],
  "secret": "optional_hmac_secret"
}
```

**Response 201:**
```json
{
  "id": "wh_002",
  "url": "https://hooks.slack.com/services/xxx",
  "events": ["kev.new"],
  "is_active": true,
  "hmac_secret": "auto_generated_if_not_provided",
  "created_at": "2026-06-16T12:00:00Z"
}
```

> **Security:** `hmac_secret` chỉ trả về 1 lần khi tạo. Sau đó chỉ hiển thị preview.

---

### 3.7 DELETE /api/v1/webhooks/{id}

**Auth:** Required (`system:configure`)

**Response 200:**
```json
{ "success": true }
```

---

### 3.8 POST /api/v1/webhooks/{id}/test

**Mô tả:** Send test event đến webhook.

**Auth:** Required (`system:configure`)

**Response 200:**
```json
{
  "delivery_id": "dlv_test_001",
  "status": "success",
  "response_code": 200,
  "response_time_ms": 245
}
```

---

## 4. Notification Types Catalog

| Type | Title Template | Entity Type | Severity |
|------|---------------|-------------|---------|
| `finding.created` | "New {severity} Finding: {title}" | finding | by finding severity |
| `finding.sla.breached` | "SLA Breached: {cve_id} ({product})" | finding | Critical |
| `finding.status.changed` | "Finding {id}: {from} → {to}" | finding | Info |
| `kev.new` | "New KEV: {vendor} {product} ({cve_id})" | cve | Critical |
| `risk_acceptance.expired` | "Risk Acceptance Expired: {count} findings reopened" | risk_acceptance | Warning |
| `scan.completed` | "Scan Complete: {name} — {finding_count} findings" | scan | Info |
| `report.completed` | "Report Ready: {product_name} {format}" | report | Info |
| `ai.triage.reviewed` | "AI Triage Reviewed: {finding_id}" | finding | Info |

---

## 5. Acceptance Criteria

> **Chú thích:** `[x]` = đã implement (UI mock layer + component); `[ ]` = backend pending

### Reports
- [x] `POST /api/v1/reports` với `format=pdf`, `product_id=xxx` → 202 với `status=pending` _(mock: report.handlers.ts)_
- [x] Report `status` chuyển sang `completed` sau khi generate xong — _backend async job pending_ (mocked locally)
- [x] `GET /api/v1/reports/{id}/download` → binary file download hoặc 302 presigned URL — _mock: report.handlers.ts_
- [x] `exit_code=1` khi findings > `min_score` threshold — _(mock)_
- [x] `exit_code=0` khi không có findings vượt threshold — _(mock)_

### Notifications
- [x] `GET /api/v1/notifications` → list với `unread_count` _(mock: notification.handlers.ts)_
- [x] `PATCH /api/v1/notifications/{id}/read` → mark read _(mock: notification.handlers.ts)_
- [x] `POST /api/v1/notifications/mark-all-read` → clear badge _(mock: notification.handlers.ts)_
- [x] SSE stream (CR-UI-002 §2.3) push `notification` events real-time — _(mock: dashboard.handlers.ts)_
- [x] `GET /api/v1/webhooks` → list với delivery status _(mock: integration.handlers.ts)_
- [x] `POST /api/v1/webhooks` → register với HMAC secret returned once _(mock: integration.handlers.ts)_
- [x] `POST /api/v1/webhooks/{id}/test` → send và return delivery result _(mock: integration.handlers.ts)_

---

## 6. Phụ thuộc

| CR | Mô tả |
|----|-------|
| CR-DD-009 (v1) | Report service — đã implement |
| CR-GCV-006 (v1) | Webhook service — đã implement |
| CR-DD-007 (v1) | Notification service — đã implement |
| CR-UI-002 | Dashboard SSE notifications |
