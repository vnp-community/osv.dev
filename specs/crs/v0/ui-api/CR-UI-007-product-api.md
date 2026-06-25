# CR-UI-007 — Product Security API

**Series:** UI-API v2  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🟢 Mock Layer Complete / Backend Schema Pending  
**Ưu tiên:** P0 — Critical  
**Nguồn yêu cầu:** `ui/specs/TDD.md` §8, `docs/SRS.md` §3.4 FR-04-05, FR-04-09  
**Services ảnh hưởng:** `gateway (:8080)`, `finding-service (:8085)`

---

## 1. Bối cảnh

Module Product Security (`/products/*`) quản lý Product/Engagement/Test hierarchy:
- **Product Security** (`/products`): List products với grade, finding summary, scorecard
- **Product Detail** (`/products/:id`): Engagements list, SLA config, finding breakdown
- **Scorecards**: Product grade A-F với trend

Hierarchy: `ProductType → Product → Engagement → Test → Finding`

Tất cả entities đã có trong `finding-service`. CR này xác định chính xác API schema cho UI.

---

## 2. Endpoints yêu cầu

### 2.1 GET /api/v1/products

**Mô tả:** List products với grade và finding summary.

**Auth:** Required (`finding:read`)

**Query Params:** `page=1`, `page_size=20`, `q=banking`, `product_type=web_app`, `criticality=critical`, `lifecycle=production`

**Response 200:**
```json
{
  "products": [
    {
      "id": "prod_1",
      "name": "Banking Portal",
      "description": "Customer-facing online banking application",
      "type": "web_app",
      "criticality": "critical",
      "lifecycle": "production",
      "grade": "D",
      "score": 42,
      "finding_summary": {
        "critical": 2,
        "high": 8,
        "medium": 15,
        "low": 20,
        "total_active": 45
      },
      "sla_config": {
        "product_id": "prod_1",
        "critical_days": 3,
        "high_days": 14,
        "medium_days": 60,
        "low_days": 120
      },
      "tags": ["banking", "pci-dss", "production"],
      "created_at": "2026-01-15T08:00:00Z"
    },
    {
      "id": "prod_2",
      "name": "Mobile App",
      "description": "iOS and Android mobile banking app",
      "type": "mobile",
      "criticality": "high",
      "lifecycle": "production",
      "grade": "B",
      "score": 76,
      "finding_summary": {
        "critical": 0,
        "high": 4,
        "medium": 12,
        "low": 18,
        "total_active": 34
      },
      "sla_config": null,
      "tags": ["mobile", "ios", "android"],
      "created_at": "2026-02-10T08:00:00Z"
    }
  ],
  "total": 8,
  "page": 1,
  "page_size": 20
}
```

---

### 2.2 POST /api/v1/products

**Mô tả:** Tạo product mới.

**Auth:** Required (`finding:write`)

**Request Body:**
```json
{
  "name": "API Gateway",
  "description": "Internal API gateway for microservices",
  "type": "api",
  "criticality": "critical",
  "lifecycle": "production",
  "tags": ["api", "internal", "critical"]
}
```

**Response 201:** Product object

---

### 2.3 GET /api/v1/products/{id}

**Mô tả:** Chi tiết product với engagements.

**Auth:** Required (`finding:read`)

**Response 200:**
```json
{
  "id": "prod_1",
  "name": "Banking Portal",
  "description": "...",
  "type": "web_app",
  "criticality": "critical",
  "lifecycle": "production",
  "grade": "D",
  "score": 42,
  "finding_summary": {
    "critical": 2,
    "high": 8,
    "medium": 15,
    "low": 20,
    "total_active": 45
  },
  "sla_config": {
    "product_id": "prod_1",
    "critical_days": 3,
    "high_days": 14,
    "medium_days": 60,
    "low_days": 120
  },
  "tags": ["banking", "pci-dss"],
  "engagements": [
    {
      "id": "eng_001",
      "product_id": "prod_1",
      "name": "Q2 2026 Security Assessment",
      "type": "interactive",
      "start_date": "2026-04-01",
      "end_date": "2026-04-30",
      "status": "completed",
      "lead_id": "usr_bob123",
      "cicd_url": null
    },
    {
      "id": "eng_002",
      "product_id": "prod_1",
      "name": "CI/CD Pipeline Integration",
      "type": "cicd",
      "start_date": "2026-01-15",
      "end_date": null,
      "status": "in_progress",
      "lead_id": null,
      "cicd_url": "https://github.com/company/banking-portal/actions"
    }
  ],
  "created_at": "2026-01-15T08:00:00Z"
}
```

---

### 2.4 PATCH /api/v1/products/{id}

**Auth:** Required (`finding:write`)

**Request Body:** Partial Product fields

**Response 200:** Updated Product

---

### 2.5 GET /api/v1/products/{id}/engagements

**Mô tả:** List engagements của một product.

**Auth:** Required (`finding:read`)

**Response 200:**
```json
{
  "engagements": [
    {
      "id": "eng_001",
      "product_id": "prod_1",
      "name": "Q2 2026 Security Assessment",
      "type": "interactive",
      "start_date": "2026-04-01",
      "end_date": "2026-04-30",
      "status": "completed",
      "lead_id": "usr_bob123",
      "cicd_url": null,
      "test_count": 3,
      "finding_count": 15
    }
  ],
  "total": 2
}
```

---

### 2.6 POST /api/v1/products/{id}/engagements

**Mô tả:** Tạo engagement mới.

**Auth:** Required (`finding:write`)

**Request Body:**
```json
{
  "name": "Q3 2026 Pentest",
  "type": "interactive",
  "start_date": "2026-07-01",
  "end_date": "2026-07-31",
  "lead_id": "usr_bob123"
}
```

**Response 201:** Engagement object

---

### 2.7 GET /api/v1/engagements/{id}/tests

**Mô tả:** List tests của một engagement.

**Auth:** Required (`finding:read`)

**Response 200:**
```json
{
  "tests": [
    {
      "id": "test_001",
      "engagement_id": "eng_001",
      "title": "Nmap Network Scan - Q2",
      "scan_type": "nmap_full",
      "test_date": "2026-04-15",
      "finding_count": 8
    }
  ],
  "total": 3
}
```

---

### 2.8 GET /api/v1/products/types

**Mô tả:** List product types — for filter dropdown.

**Auth:** Required

**Response 200:**
```json
{
  "types": [
    { "value": "web_app", "label": "Web Application" },
    { "value": "api", "label": "API" },
    { "value": "infrastructure", "label": "Infrastructure" },
    { "value": "mobile", "label": "Mobile" }
  ]
}
```

---

### 2.9 GET /api/v1/products/grades

**Mô tả:** All products grades — cho Scorecards tab.

**Auth:** Required (`finding:read`)

**Response 200:**
```json
{
  "products": [
    {
      "id": "prod_1",
      "name": "Banking Portal",
      "grade": "D",
      "score": 42,
      "critical_count": 2,
      "high_count": 8,
      "trend": "worsening"
    },
    {
      "id": "prod_2",
      "name": "Mobile App",
      "grade": "B",
      "score": 76,
      "critical_count": 0,
      "high_count": 4,
      "trend": "improving"
    }
  ],
  "overall_grade": "C",
  "overall_score": 58
}
```

**Grade Calculation (server-side — must match UI util):**
| Condition | Grade |
|-----------|-------|
| critical=0 AND high=0 | A |
| critical=0 AND high≤5 | B |
| critical=0 AND high>5 | C |
| critical=1 or 2 | D |
| critical≥3 OR total>20 | F |

---

## 3. Data Models

### ProductType Values
```
web_app | api | infrastructure | mobile
```

### BusinessCriticality Values
```
critical | high | medium | low
```

### LifecycleStatus Values
```
production | staging | development | deprecated
```

### EngagementType Values
```
interactive | cicd
```

### EngagementStatus Values
```
not_started | in_progress | completed
```

---

## 4. Acceptance Criteria

> **Chú thích:** `[x]` = đã implement (UI mock layer + component); `[~]` = partial; `[ ]` = backend pending

- [x] `GET /api/v1/products` → list với grade và finding_summary per product _(mock: product.handlers.ts; UI updated)_
- [x] `GET /api/v1/products/{id}` → full detail với engagements list _(ProductSecurity.tsx detail)_
- [x] `POST /api/v1/products` → create thành công, grade = "A" (no findings) _(mock)_
- [x] `GET /api/v1/products/{id}/engagements` → list engagements với finding_count _(mock)_
- [x] `POST /api/v1/products/{id}/engagements` → create engagement _(mock)_
- [x] `GET /api/v1/products/grades` → all products sorted by score asc (worst first) _(mock: product.handlers.ts)_
- [x] Grade calculated server-side matches UI formula (§2.9 table) — _(mock)_
- [x] `trend` field phản ánh so sánh với tháng trước — _(mock)_

---

## 5. Phụ thuộc

| CR | Mô tả |
|----|-------|
| CR-DD-001 (v1) | Product/Engagement/Test hierarchy — đã implement |
| CR-DD-009 (v1) | Product grading — đã implement |
