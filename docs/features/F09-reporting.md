# F09 — Reporting (PDF/HTML/CSV/Excel/JSON)

**Status:** ✅ v2.1 Implemented + 🔵 v3.0 Enhanced  
**CR References:** CR-DD-009, CR-GCV-010  
**Services:** `finding-service` (8085), `report-service` (8065 — v3.0)  
**Storage:** MinIO / AWS S3  
**UI Routes:** `/reports`  
**UI Components:** `ReportCenter`

---

## 1. Mô tả

Module Reporting cho phép tạo và xuất báo cáo bảo mật ở nhiều định dạng khác nhau — từ executive PDF summary đến dataset CSV/JSON cho phân tích. Báo cáo được lưu trên MinIO/S3 và có thể download từ UI hoặc API.

---

## 2. Report Formats

### 2.1 HTML Report
**Library:** Bootstrap 5.3 + Chart.js  
**Features:**
- Light/Dark theme toggle
- Severity distribution pie chart
- Top CVEs table với EPSS scores
- Finding list với filters
- Timeline chart (findings by month)
- Có thể share URL trực tiếp với team

**Use case:** Internal review, team sharing

### 2.2 PDF Report
**Library:** GoKit PDF / Chromium headless  
**Contents:**
- Executive summary (1 page)
- Risk overview + Product Grade
- Severity distribution chart
- Critical/High findings table
- Remediation recommendations

**Use case:** CISO presentations, audit submissions

### 2.3 CSV Export
**Fields:** All finding fields bao gồm:
- `cve_id`, `title`, `severity`, `status`
- `component_name`, `component_version`
- `epss_score`, `is_kev`, `has_exploit`
- `sla_expiration_date`, `sla_days_left`
- `product_name`, `engagement_name`
- `created_at`, `updated_at`

**Use case:** Data analysis, custom reporting, compliance

### 2.4 Excel (XLSX)
**Format:** DefectDojo-compatible import format  
**Sheets:**
- Summary Sheet (stats)
- Findings Sheet (all data)
- By Severity Sheet

**Use case:** Compliance teams, import vào DefectDojo

### 2.5 JSON Export
**Endpoint:** `GET /api/v2/cves/export?format=json`  
**Fields:**
```json
{
  "cve_id": "CVE-2021-44228",
  "data_source": "NVD",
  "source_url": "https://nvd.nist.gov/...",
  "last_modified": "2022-01-01",
  ...
}
```

**Use case:** Bulk CVE dataset download, offline analysis, research

---

## 3. Report Generation Flow

```
POST /api/v1/reports
    │
    ├── Validate parameters (product_id, format, date_range)
    ├── Fetch findings from finding-service
    ├── Enrich with CVE data (EPSS, KEV)
    ├── Calculate product grade
    ├── Generate report artifact
    ├── Upload to MinIO/S3
    └── Return report_id + download URL
```

---

## 4. Report Center UI

**Route:** `/reports`  
**Component:** `ReportCenter`

**Features:**
- List reports history (by product, by date)
- Generate new report (form với options)
- Download button cho mỗi report
- Status: `generating` | `ready` | `failed`
- Preview HTML report inline

---

## 5. APIs

### Report Management
```
POST /api/v1/reports                      → Generate new report
GET /api/v1/reports                       → List reports (paginated)
GET /api/v1/reports/{id}                  → Report metadata
GET /api/v1/reports/{id}/download         → Download file (Content-Disposition)
DELETE /api/v1/reports/{id}               → Delete report
```

### CVE Bulk Export
```
GET /api/v2/cves/export?format=json&since=2026-01-01
GET /api/v2/cves/export?format=csv
```

### Generate Report Request
```json
{
  "product_id": "prod-001",
  "format": "pdf",
  "date_range": {
    "start": "2026-01-01",
    "end": "2026-06-18"
  },
  "severity_filter": ["CRITICAL", "HIGH"],
  "include_mitigated": false,
  "title": "Q2 2026 Security Report"
}
```

---

## 6. Product Grading in Reports

Mỗi report bao gồm Product Grade:

| Grade | Điều kiện | Màu sắc |
|-------|-----------|---------|
| **A** | 0 Critical, 0 High | 🟢 Green |
| **B** | 0 Critical, ≤ 5 High | 🟢 Light Green |
| **C** | 0 Critical, > 5 High | 🟡 Yellow |
| **D** | 1–2 Critical | 🟠 Orange |
| **F** | 3+ Critical OR > 20 findings | 🔴 Red |

---

## 7. CI/CD Exit Code Integration

Report service hỗ trợ exit code cho CI/CD pipelines:

```bash
# Generate report + check grade
curl -X POST /api/v1/reports \
  -d '{"product_id":"prod-001","format":"json","check_grade":true}'

# Response
{
  "grade": "B",
  "exit_code": 0,    # 0 = PASS, 1 = FAIL
  "report_id": "rep-001"
}
```

**Gate conditions:**
- Grade A/B → `exit_code = 0` (CI PASS)
- Grade C/D/F → `exit_code = 1` (CI FAIL)

---

## 8. Artifact Storage

- **Backend:** MinIO (development) / AWS S3 (production)
- **TTL:** Reports expire sau 90 ngày (configurable)
- **Access:** Pre-signed URL cho download (TTL 1 giờ)
- **Size limit:** 500MB per report file

---

## 9. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| HTML/PDF generation (1000 findings) | < 30 giây |
| CSV export (10K findings) | < 60 giây |
| JSON bulk export | Streaming response (không buffer toàn bộ) |
| Download URL | Pre-signed S3 URL TTL 1 giờ |
| Report storage | MinIO/S3, 90 ngày retention |
