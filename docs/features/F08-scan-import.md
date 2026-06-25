# F08 — Scan Import Pipeline

**Status:** ✅ v2.1 Implemented  
**CR References:** CR-DD-002, CR-DD-003  
**Services:** `scan-service`  
**Database Schema:** `osv_scan`

---

## 1. Mô tả

Scan Import Pipeline cho phép nhập kết quả từ **21+ security tools** vào OSV Platform qua một endpoint thống nhất. Pipeline 12 bước tự động: validate → parse → normalize → deduplicate → tạo findings — tất cả với atomic transaction.

---

## 2. Supported Security Tools (21+ Parsers)

### 2.1 Network & Infrastructure
| Tool | Format | Mô tả |
|------|--------|-------|
| **Nmap** | XML | Network port scan + CVE detection |
| **Nessus** | XML | Comprehensive vulnerability scan |
| **OpenVAS** | XML | Open source vulnerability scanner |
| **Qualys** | CSV/XML | Enterprise vulnerability management |

### 2.2 Web Application
| Tool | Format | Mô tả |
|------|--------|-------|
| **OWASP ZAP** | JSON/XML | DAST web application scanner |
| **Burp Suite** | XML | Web security testing |
| **Nikto** | XML | Web server scanner |
| **Acunetix** | XML | Web vulnerability scanner |

### 2.3 SAST — Static Analysis
| Tool | Format | Mô tả |
|------|--------|-------|
| **Bandit** | JSON | Python security linter |
| **Semgrep** | JSON | Multi-language SAST |
| **Checkmarx** | XML | Enterprise SAST |
| **CodeQL** | SARIF | GitHub code scanning |
| **SonarQube** | JSON | Code quality + security |
| **Brakeman** | JSON | Ruby on Rails SAST |

### 2.4 SCA — Software Composition Analysis
| Tool | Format | Mô tả |
|------|--------|-------|
| **Trivy** | JSON | Container + filesystem scanner |
| **Snyk** | JSON | Dependency vulnerability |
| **OWASP Dependency-Check** | JSON/XML | Java dependency scan |
| **npm audit** | JSON | Node.js dependency scan |
| **Safety** | JSON | Python dependency check |

### 2.5 Container & Cloud
| Tool | Format | Mô tả |
|------|--------|-------|
| **Grype** | JSON | Container image scanner |
| **Checkov** | JSON | IaC security scanner |

---

## 3. Parser Factory Pattern

```go
type ScanParser interface {
    Parse(ctx context.Context, r io.Reader) ([]FindingRaw, error)
    ToolName() string
}

// Usage
parser, err := ParserFactory.GetParser("Trivy")
findings, err := parser.Parse(ctx, uploadedFile)
```

**Extension:** Thêm parser mới bằng cách implement interface và đăng ký với factory — không cần sửa core code.

---

## 4. 12-Step Import Pipeline

```
Step 1:  Validate upload (file size, format)
Step 2:  Detect tool type (auto-detect hoặc explicit)
Step 3:  Parse raw findings từ tool format
Step 4:  Normalize severity (tool-specific → Critical/High/Medium/Low)
Step 5:  Normalize CVE IDs (format standardization)
Step 6:  Enrich từ CVE database (EPSS, KEV, CWE)
Step 7:  Resolve Product/Engagement/Test context
Step 8:  Calculate dedup hash: SHA-256(title+component+version+cve_id)
Step 9:  Dedup check trong product scope
Step 10: Create/update findings (atomic batch insert)
Step 11: Publish NATS event: finding.batch_created
Step 12: Return import summary
```

---

## 5. Import Endpoint

```
POST /api/v2/import-scan
Content-Type: multipart/form-data
```

**Parameters:**
```
scan_type     : Tool name (e.g., "Trivy", "Bandit") — optional (auto-detect)
product_id    : Product để import vào
engagement    : Tên engagement (created if not exists)
test_title    : Tên test run
file          : Scan result file
minimum_severity : Severity threshold (bỏ qua findings dưới mức này)
```

**Response:**
```json
{
  "scan_id": "scan-001",
  "import_summary": {
    "total_parsed": 45,
    "created": 30,
    "duplicates": 10,
    "below_threshold": 5,
    "errors": 0
  },
  "findings_created": ["f-001", "f-002", ...],
  "test_id": "test-001"
}
```

---

## 6. Severity Normalization Matrix

| Tool Severity | OSV Severity |
|---------------|-------------|
| Critical (10.0) | CRITICAL |
| High (7.0–9.9) | HIGH |
| Medium (4.0–6.9) | MEDIUM |
| Low (0.1–3.9) | LOW |
| Info / None | INFO |

**Tool-specific mappings:**
- ZAP: High → HIGH, Medium → MEDIUM, Low → LOW, Informational → INFO
- Bandit: HIGH → HIGH, MEDIUM → MEDIUM, LOW → LOW
- Trivy: CRITICAL → CRITICAL, HIGH → HIGH, etc.

---

## 7. NATS Events

| Event | Payload |
|-------|---------|
| `finding.batch_created` | `{scan_id, finding_ids[], product_id}` |

**Subscribers:** notification-service, sla-service, audit-service

---

## 8. Database Schema (`osv_scan`)

| Table | Mô tả |
|-------|-------|
| `test_imports` | Scan import job records |
| `import_findings` | Raw parsed findings trước khi normalize |

---

## 9. Error Handling

| Lỗi | HTTP | Mô tả |
|-----|------|-------|
| Unsupported tool | 400 | `UNSUPPORTED_SCANNER` |
| Parse error | 422 | `PARSE_ERROR` + details |
| Product not found | 404 | `NOT_FOUND` |
| File too large | 413 | Max 100MB per upload |

---

## 10. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| Import 1000 findings | < 30 giây |
| Dedup check | < 50ms per finding |
| File size limit | 100MB |
| Concurrent imports | Hỗ trợ parallel imports per product |
