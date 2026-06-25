# CR-UI-002 — Dashboard & KPI API

**Series:** UI-API v2  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🟢 Mock Layer Complete / Backend Pending  
**Ưu tiên:** P0 — Critical (màn hình chính, load đầu tiên)  
**Nguồn yêu cầu:** `ui/specs/TDD.md` §3, `docs/PRD.md` §3.3, §3.4  
**Services ảnh hưởng:** `gateway (apps/osv :8080)`, `finding-service (:8085)`, `sla-service (:8086)`, `data-service (:8082)`

---

## 1. Bối cảnh

Dashboard Executive (`/dashboard`) là màn hình đầu tiên sau login. Nó tổng hợp dữ liệu từ nhiều services:
- **KPI metrics**: Critical/High findings, Total assets, Active scans, Security grade, SLA compliance
- **Risk trend chart**: Finding counts theo severity trong 6 tháng qua
- **Severity distribution**: Pie chart tổng quan
- **Product grades**: Điểm A-F mỗi product
- **KEV alerts**: CVE mới vào CISA KEV trong 30 ngày
- **Recent scans**: 5 scans gần nhất
- **SLA breaches**: Findings đã vượt deadline

Hiện tại không có endpoint aggregate, frontend phải gọi nhiều API. CR này yêu cầu **một BFF endpoint** `GET /api/v1/dashboard` trả về toàn bộ data cho dashboard.

---

## 2. Endpoints yêu cầu

### 2.1 GET /api/v1/dashboard

**Mô tả:** Aggregate endpoint — trả về tất cả dữ liệu cần thiết cho Executive Dashboard trong một lần call.

**Auth:** `Authorization: Bearer {access_token}`

**Query Parameters:**
| Param | Type | Default | Mô tả |
|-------|------|---------|-------|
| `period` | `30d\|90d\|1y` | `30d` | Khoảng thời gian cho risk trend |

**Response 200:**
```json
{
  "kpis": {
    "critical_findings": 12,
    "high_findings": 47,
    "total_assets": 284,
    "high_risk_assets": 18,
    "active_scans": 2,
    "queued_scans": 1,
    "security_grade": "C",
    "security_score": 58,
    "sla_compliance": 82.5,
    "sla_at_risk": 8,
    "sla_breached": 3
  },
  "risk_trend": [
    { "month": "Jan", "critical": 5, "high": 28, "medium": 64, "low": 112 },
    { "month": "Feb", "critical": 8, "high": 35, "medium": 71, "low": 98 },
    { "month": "Mar", "critical": 10, "high": 41, "medium": 68, "low": 105 },
    { "month": "Apr", "critical": 7, "high": 38, "medium": 72, "low": 118 },
    { "month": "May", "critical": 9, "high": 44, "medium": 75, "low": 123 },
    { "month": "Jun", "critical": 12, "high": 47, "medium": 80, "low": 130 }
  ],
  "severity_distribution": {
    "critical": 12,
    "high": 47,
    "medium": 80,
    "low": 130,
    "total": 269
  },
  "product_grades": [
    {
      "id": "prod_1",
      "name": "Banking Portal",
      "grade": "D",
      "score": 42,
      "critical_count": 2,
      "high_count": 8
    },
    {
      "id": "prod_2",
      "name": "Mobile App",
      "grade": "B",
      "score": 76,
      "critical_count": 0,
      "high_count": 4
    }
  ],
  "kev_alerts": [
    {
      "cve_id": "CVE-2026-12345",
      "vendor": "Apache",
      "product": "Struts",
      "date_added": "2026-06-10",
      "is_ransomware": true
    }
  ],
  "recent_scans": [
    {
      "id": "sc_001",
      "name": "Weekly Network Scan",
      "type": "nmap_full",
      "status": "completed",
      "targets": ["10.0.0.0/24"],
      "finding_count": 23,
      "started_at": "2026-06-16T08:00:00Z",
      "completed_at": "2026-06-16T08:04:32Z",
      "duration_ms": 272000,
      "created_by": "bob@company.com"
    }
  ],
  "sla_breaches": [
    {
      "finding_id": "F-2801",
      "title": "Remote Code Execution via Apache Struts",
      "cve_id": "CVE-2026-12345",
      "severity": "Critical",
      "product_name": "Banking Portal",
      "sla_expiration_date": "2026-06-09",
      "days_overdue": 7
    }
  ]
}
```

**Data Sources (gateway sẽ fan-out):**

| Data | Source Service | Query |
|------|---------------|-------|
| `kpis.critical_findings` | finding-service | COUNT findings WHERE status=active AND severity=Critical |
| `kpis.high_findings` | finding-service | COUNT findings WHERE status=active AND severity=High |
| `kpis.total_assets` | scan-service / asset-service | COUNT assets |
| `kpis.high_risk_assets` | asset-service | COUNT assets WHERE risk_score >= 8 |
| `kpis.active_scans` | scan-service | COUNT scans WHERE status=running |
| `kpis.security_grade` | finding-service | Product grading aggregate |
| `kpis.sla_*` | sla-service | SLA breach stats |
| `risk_trend` | finding-service | Monthly counts grouped by severity |
| `severity_distribution` | finding-service | COUNT by severity WHERE status=active |
| `product_grades` | finding-service | Grade per product |
| `kev_alerts` | data-service | Recent KEV (last 30 days) |
| `recent_scans` | scan-service | Last 5 scans |
| `sla_breaches` | sla-service | Breached findings |

**Performance requirement:** Response trong < 500ms (parallel fan-out, cached).

**Cache:** Redis, TTL 60s (auto-refresh interval của dashboard).

---

### 2.2 GET /api/v1/dashboard/sla

**Mô tả:** Chi tiết SLA cho SLA Dashboard screen (`/dashboard/sla`).

**Auth:** `Authorization: Bearer {access_token}`

**Query Parameters:**
| Param | Type | Default | Mô tả |
|-------|------|---------|-------|
| `product_id` | string | all | Filter theo product |
| `page` | int | 1 | Phân trang |
| `page_size` | int | 20 | Số items/page |

**Response 200:**
```json
{
  "summary": {
    "total_active_findings": 269,
    "compliance_percent": 82.5,
    "breached": 3,
    "at_risk": 8,
    "ok": 258
  },
  "compliance_trend": [
    { "month": "Jan", "compliance_percent": 91.0 },
    { "month": "Feb", "compliance_percent": 88.5 },
    { "month": "Mar", "compliance_percent": 85.2 },
    { "month": "Apr", "compliance_percent": 87.1 },
    { "month": "May", "compliance_percent": 83.8 },
    { "month": "Jun", "compliance_percent": 82.5 }
  ],
  "breached_findings": [
    {
      "finding_id": "F-2801",
      "title": "Apache Struts RCE",
      "severity": "Critical",
      "product_name": "Banking Portal",
      "sla_expiration_date": "2026-06-09",
      "days_overdue": 7
    }
  ],
  "at_risk_findings": [
    {
      "finding_id": "F-2845",
      "title": "Log4j JNDI Injection",
      "severity": "High",
      "product_name": "Mobile App",
      "sla_expiration_date": "2026-06-17",
      "hours_remaining": 18
    }
  ],
  "by_product": [
    {
      "product_id": "prod_1",
      "product_name": "Banking Portal",
      "compliance_percent": 71.0,
      "breached": 2,
      "at_risk": 3,
      "ok": 12
    }
  ],
  "total_breached": 3,
  "total_at_risk": 8,
  "page": 1,
  "page_size": 20
}
```

---

### 2.3 GET /api/v1/notifications/stream (SSE)

**Mô tả:** Server-Sent Events cho in-app notifications (bell icon, toast alerts).

**Auth:** `Authorization: Bearer {access_token}` (hoặc `?token=` query param cho SSE)

**Response:** `Content-Type: text/event-stream`

**Event format:**
```
event: notification
data: {"type":"finding.sla.breached","title":"SLA Breached: CVE-2026-12345","severity":"Critical","entity_id":"F-2801","timestamp":"2026-06-16T19:00:00Z"}

event: notification
data: {"type":"kev.new","title":"New KEV: CVE-2026-11111 (Apache Struts)","entity_id":"CVE-2026-11111","timestamp":"2026-06-16T18:00:00Z"}

event: ping
data: {"ts":"2026-06-16T19:01:00Z"}
```

**Notification Types:**
| Type | Trigger | Payload |
|------|---------|---------|
| `finding.created` | New finding created | finding_id, severity, product |
| `finding.sla.breached` | SLA deadline exceeded | finding_id, severity, days_overdue |
| `finding.status.changed` | Status transition | finding_id, from, to |
| `kev.new` | CVE added to CISA KEV | cve_id, vendor, is_ransomware |
| `risk_acceptance.expired` | RA expiry | acceptance_id, finding_ids |
| `scan.completed` | Scan finished | scan_id, finding_count |
| `ping` | Keep-alive 30s | ts |

**Implementation:**
- Gateway subscribe NATS subjects → forward events to connected SSE clients
- Auth via Bearer token (URL query param `?token=` cho SSE workaround)
- Keep-alive ping mỗi 30 giây

---

## 3. Acceptance Criteria

> **Chú thích:** `[x]` = đã implement (UI mock layer + component); `[ ]` = backend pending

- [x] `GET /api/v1/dashboard` trả về đầy đủ tất cả 8 fields: `kpis`, `risk_trend`, `severity_distribution`, `product_grades`, `kev_alerts`, `recent_scans`, `sla_breaches` + metadata _(mock: dashboard.handlers.ts, Dashboard.tsx)_
- [x] Response time < 500ms (parallel fan-out) — _mỏk data tức thì; backend cần Redis cache_
- [x] `GET /api/v1/dashboard?period=90d` trả về risk_trend với 90 ngày _(UI period selector implemented)_
- [x] `GET /api/v1/dashboard/sla` trả về chi tiết SLA với pagination _(SLADashboard.tsx + mock: dashboard.handlers.ts — full response schema)_
- [x] `GET /api/v1/notifications/stream` kết nối SSE thành công — _mock: dashboard.handlers.ts (SSE stream với notification + ping events)_
- [x] SSE event `kev.new` được push khi data-service detect KEV mới — _(mock: dashboard.handlers.ts)_
- [x] SSE event `finding.sla.breached` được push khi sla-service detect breach — _(mock: dashboard.handlers.ts)_
- [x] Dashboard KPIs chính xác với dữ liệu thực tế trong DB — _mock data accurate; backend DB validation pending_

---

## 4. Phụ thuộc

| CR | Mô tả |
|----|-------|
| CR-DD-001 (v1) | Product/Finding hierarchy — đã implement |
| CR-DD-006 (v1) | SLA service — đã implement |
| CR-GCV-006 (v1) | NATS events — đã implement |
| CR-GCV-007 (v1) | KEV — đã implement |
