# F16 — Executive Dashboard & KPIs

**Status:** 🔶 Planned v3.1 (CR-UI-002)  
**CR References:** CR-UI-002  
**Services:** Gateway BFF (fan-out), `finding-service`, `sla-service`, `data-service`  
**UI Routes:** `/dashboard`, `/dashboard/sla`  
**UI Components:** `Dashboard`, `SLADashboard`

---

## 1. Mô tả

Executive Dashboard cung cấp cái nhìn tổng quan toàn diện về tình trạng bảo mật của tổ chức trong một màn hình: KPI metrics, SLA compliance, KEV coverage, và real-time notifications. Dashboard BFF (Backend-for-Frontend) aggregate data từ nhiều services với < 500ms.

---

## 2. Dashboard BFF Architecture

```
GET /api/v1/dashboard
    │
    Gateway BFF (parallel fan-out)
    ├── finding-service → Finding counts, severity distribution
    ├── sla-service     → SLA compliance rate
    ├── data-service    → KEV stats, new CVEs
    └── identity-service → User context
    │
    Redis Cache (TTL 60 giây)
    │
    Aggregated response < 500ms
```

**Cache:** Redis key `dashboard:{user_id}`, TTL 60 giây

---

## 3. Dashboard KPIs

### 3.1 Main API Response
```json
{
  "timestamp": "2026-06-18T10:00:00Z",
  "user": {
    "name": "Carol Johnson",
    "role": "admin"
  },
  "findings": {
    "total_active": 47,
    "critical": 3,
    "high": 12,
    "medium": 28,
    "low": 4,
    "trend": {
      "vs_last_week": -5,
      "direction": "improving"
    }
  },
  "security_posture": {
    "overall_grade": "C",
    "score": 68,
    "products_at_risk": 2
  },
  "sla": {
    "compliance_rate": 0.87,
    "breached": 4,
    "at_risk": 8,
    "trend": "stable"
  },
  "kev": {
    "total": 1100,
    "unmitigated_in_platform": 3,
    "added_this_week": 12
  },
  "recent_activity": [
    {
      "type": "finding.sla.breached",
      "message": "SLA breached for CVE-2021-44228 in Payment API",
      "timestamp": "2026-06-18T09:00:00Z"
    }
  ],
  "top_products": [
    {"name": "Payment API", "grade": "D", "critical": 2},
    {"name": "Auth Service", "grade": "B", "critical": 0}
  ]
}
```

---

## 4. Dashboard UI Sections

### 4.1 KPI Cards Row
- **Critical Findings:** Count với trend arrow (↑↓)
- **Security Grade:** A–F badge với score
- **SLA Compliance:** Percentage với color coding
- **KEV Unmitigated:** Count của KEV CVEs chưa fix

### 4.2 Findings Trend Chart
- Area chart: Active findings theo thời gian (30/90 ngày)
- Breakdown by severity (stacked)
- Drill-down: Click → Findings list filtered

### 4.3 Product Security Overview
- Table: Tất cả products với grade + finding counts
- Sort by grade (worst first)
- Quick links đến Product Security page

### 4.4 Recent Activity Feed
- Timeline: Events trong 24 giờ gần nhất
- Types: SLA breach, KEV mới, scan completed, risk acceptance expired
- Click → Navigate đến entity

### 4.5 SLA Compliance Chart
- Stacked bar: Compliant / At-Risk / Breached per product
- Overall compliance rate gauge

### 4.6 KEV Intelligence Panel
- Total KEV count
- Unmitigated in platform (linked to KEV Catalog)
- Last updated timestamp
- Link: → KEV Catalog

---

## 5. SLA Dashboard

**Route:** `/dashboard/sla`  
**Component:** `SLADashboard`

### 5.1 API
```
GET /api/v1/dashboard/sla
```

### 5.2 View Features
- Compliance rate per product (table + chart)
- Findings at risk (< 3 days remaining)
- Breached findings (action required)
- SLA trend (weekly compliance rate)
- Filter by: product, severity, time range

---

## 6. Real-Time Notifications (SSE)

**Endpoint:** `GET /api/v1/notifications/stream`  
**Protocol:** Server-Sent Events (SSE)

**Events pushed in real-time:**
- `sla.breached` — SLA vừa bị breach
- `kev.new` — CVE mới vào CISA KEV
- `scan.completed` — Scan hoàn thành
- `finding.critical.new` — Critical finding mới
- `risk_acceptance.expired` — Risk acceptance hết hạn

**SSE Message Format:**
```
event: sla.breached
data: {"finding_id":"f-001","product":"Payment API","severity":"CRITICAL","message":"SLA breached for CVE-2021-44228"}

event: kev.new
data: {"cve_ids":["CVE-2024-1234"],"count":3,"message":"3 new CVEs added to CISA KEV"}
```

**Latency:** < 2 giây từ NATS event đến browser

---

## 7. APIs

```
GET /api/v1/dashboard                    → Main dashboard BFF (< 500ms)
GET /api/v1/dashboard/sla                → SLA compliance detail
GET /api/v1/notifications/stream         → SSE real-time stream
```

---

## 8. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| Dashboard BFF response | < 500ms (P95) |
| Redis cache TTL | 60 giây |
| SSE latency | < 2 giây from NATS to browser |
| Dashboard refresh | Auto-refresh mỗi 5 phút |
| Concurrent SSE connections | Không giới hạn (NATS fan-out) |
| Auth | JWT required, BFF fan-out với user context |
