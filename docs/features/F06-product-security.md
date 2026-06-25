# F06 — Product & Engagement Hierarchy

**Status:** ✅ v2.1 Implemented  
**CR References:** CR-DD-001, CR-DD-009  
**Services:** `finding-service` (8085), `product-service` (8061 — v3.0)  
**UI Routes:** `/products`  
**UI Components:** `ProductSecurity`

---

## 1. Mô tả

OSV Platform tổ chức findings theo hierarchy 4 cấp lấy cảm hứng từ DefectDojo: ProductType → Product → Engagement → Test → Finding. Cho phép quản lý bảo mật theo portfolio sản phẩm, với grading tự động (A–F) và CI/CD integration.

---

## 2. Hierarchy Model

```
ProductType (Category)
└── Product (Application under test)
    ├── Engagement (Testing event)
    │   └── Test (Specific scan/assessment)
    │       └── Finding (Vulnerability)
    └── Members (RBAC per product)
```

### 2.1 ProductType
- **Mô tả:** Phân loại sản phẩm
- **Examples:** Web Application, API, Infrastructure, Mobile App, Container

### 2.2 Product
| Field | Mô tả |
|-------|-------|
| `name` | Tên sản phẩm |
| `description` | Mô tả |
| `business_criticality` | very high / high / medium / low |
| `lifecycle` | production / development / staging |
| `team` | Responsible team |
| `grade` | A–F (calculated) |
| `score` | Risk score (calculated) |

### 2.3 Engagement
| Field | Mô tả |
|-------|-------|
| `name` | Tên engagement |
| `type` | `Interactive` hoặc `CI/CD Pipeline` |
| `target_start` | Start date |
| `target_end` | End date |
| `status` | In Progress / Completed |
| `product_id` | Parent product |

### 2.4 Test
| Field | Mô tả |
|-------|-------|
| `title` | Test name |
| `test_type` | Nmap Scan / ZAP Scan / SAST / DAST / Manual |
| `engagement_id` | Parent engagement |
| `lead` | Responsible analyst |

---

## 3. Product Grading

### 3.1 Grade Calculation

| Grade | Điều kiện |
|-------|-----------|
| **A** | 0 Critical, 0 High findings |
| **B** | 0 Critical, ≤ 5 High findings |
| **C** | 0 Critical, > 5 High findings |
| **D** | 1–2 Critical findings |
| **F** | 3+ Critical OR > 20 total findings |

### 3.2 Grade được tính lại
- Mỗi khi finding status thay đổi
- Khi risk acceptance expire
- Mỗi ngày lúc midnight (background recalculation)

### 3.3 APIs
```
GET /api/v1/products                        → List products với grade + score
GET /api/v1/products/{id}                   → Product detail
GET /api/v1/products/grades                 → All products grades (bulk)
GET /api/v1/products/{id}/finding_summary   → Summary by severity/status
```

---

## 4. Finding Summary per Product

```json
{
  "product_id": "prod-001",
  "product_name": "Payment API",
  "grade": "D",
  "score": 72,
  "findings": {
    "total": 15,
    "active": 8,
    "mitigated": 5,
    "false_positive": 2,
    "by_severity": {
      "critical": 2,
      "high": 4,
      "medium": 8,
      "low": 1
    }
  },
  "sla": {
    "breached": 1,
    "at_risk": 3,
    "compliant": 4
  }
}
```

---

## 5. Product Members & RBAC

Mỗi Product có members với roles riêng:

| Role | Quyền |
|------|-------|
| `owner` | Full CRUD, invite members, delete product |
| `maintainer` | CRUD findings, manage engagements |
| `developer` | View findings, close/reopen findings |
| `viewer` | Read-only access |

**API:**
```
GET /api/v1/products/{id}/members          → List members
POST /api/v1/products/{id}/members         → Add member
DELETE /api/v1/products/{id}/members/{uid} → Remove member
```

---

## 6. CI/CD Integration

### 6.1 Exit Code Pattern
Khi generate report cho engagement:
- **Exit 0:** Grade A hoặc B (build PASS)
- **Exit 1:** Grade C/D/F (build FAIL)

### 6.2 CI/CD Engagement Type
- Engagement type = `CI/CD Pipeline`
- Tự động tạo Test khi pipeline push scan results
- Report generated tự động sau scan

### 6.3 API Usage in CI Pipeline
```bash
# Push scan results
curl -X POST /api/v2/import-scan \
  -H "X-API-Key: $OSV_KEY" \
  -F "scan_type=Bandit" \
  -F "product_id=prod-001" \
  -F "engagement=CI Pipeline" \
  -F "file=@bandit-report.json"

# Get exit code
curl /api/v1/products/prod-001/grade → {"grade": "B", "exit_code": 0}
```

---

## 7. Database Schema (`osv_finding`)

| Table | Mô tả |
|-------|-------|
| `product_types` | ProductType catalog |
| `products` | Products với business context |
| `engagements` | Testing events |
| `tests` | Individual scan/assessment runs |
| `findings` | Findings linked to tests |
| `product_members` | Per-product role assignments |

---

## 8. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| Product grade recalc | < 500ms per product |
| Products list | < 100ms |
| Finding summary | < 200ms |
