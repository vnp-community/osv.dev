# F09 — Reporting: Business Logic

> Mô tả bằng ngôn ngữ tự nhiên + pseudo-code.

---

## 1. Report Generation Logic

### 1.1 Report Scope

Report có thể được tạo cho các scope khác nhau:

```
scope = {
    "product":    all findings in product,
    "engagement": all findings in engagement,
    "test":       findings from specific test/scan
}

Khi tạo report:
    validate_scope(report_request.scope, report_request.scope_id)
    findings = query_findings(scope, scope_id, filters)
```

### 1.2 Filters khi generate report

```
Report có thể lọc theo:
    - severity: chỉ lấy findings từ severity chỉ định
    - state: mặc định lấy cả Active + Mitigated
    - include_false_positives: boolean (default false)
    - include_risk_accepted: boolean (default true)
    - date_range: findings trong khoảng thời gian
```

### 1.3 Report Content Structure

Nội dung report bao gồm:

```
Section 1: Executive Summary
    - Product grade, tổng số findings, % closed
    - Risk score overall
    - Top 3 critical findings (nếu có)

Section 2: Statistics
    - Findings by severity (bar chart)
    - Findings by state pie chart
    - Trend over time (line chart)
    - SLA compliance rate

Section 3: Detailed Findings (sorted by severity DESC)
    For each finding:
        - Title, severity, state, CVE ID (nếu có)
        - Description, component, version
        - CVSS score, EPSS score, KEV status
        - Remediation guidance
        - SLA status (on-track / breached)

Section 4: Appendix
    - Methodology
    - Tool versions
    - Test scope
```

---

## 2. Format-specific Logic

### 2.1 PDF

```
generate_pdf(findings, metadata):
    1. Render HTML template với findings data
    2. PDF renderer (headless Chrome hoặc WeasyPrint): HTML → PDF bytes
    3. Embed charts (PNG từ charting library)
    4. Add watermark nếu configured (CONFIDENTIAL)
    5. Upload to MinIO
```

### 2.2 CSV/Excel

```
generate_csv(findings):
    header = [cve_id, title, severity, state, component, version,
              cvss, epss, is_kev, sla_deadline, sla_status, url, description]
    rows = [to_row(f) for f in findings]
    return CSV bytes

generate_excel(findings):
    Sheet 1: "Findings" → findings table (styled, conditional formatting by severity)
    Sheet 2: "Summary" → counts by severity/state, grade
    Sheet 3: "KEV & High EPSS" → critical intelligence
```

### 2.3 JSON (CI/CD)

```
generate_json(findings):
    return {
        generated_at: timestamp,
        scope: {type, id, name},
        grade: "B",
        summary: {critical: 0, high: 3, ...},
        exit_code: calculateExitCode(findings),
        findings: [{...}, ...]
    }

calculateExitCode(findings):
    if any f.severity == 'Critical' AND f.state == 'Active':
        return 2  // CI fail
    if any f.severity == 'High' AND f.state == 'Active':
        return 1  // CI warning
    return 0     // CI pass
```

---

## 3. Async Generation

Vì report generation có thể mất nhiều thời gian (PDF render, large datasets):

```
POST /api/v2/reports/generate
    → Tạo report_jobs record với status='queued'
    → Trả về 202 {report_id}
    → Worker (goroutine) pick up job:
        generate_report(report_id)
        upload to MinIO
        UPDATE report_jobs SET status='completed', url=download_url
        Publish NATS: report.generated

Khi client poll GET /api/v2/reports/{id}:
    status = 'queued' | 'processing' | 'completed' | 'failed'
    url = presigned MinIO URL (valid 24h) nếu completed
```

---

## 4. Storage (MinIO/S3)

```
Path pattern: reports/{product_id}/{report_id}/{format}.{ext}
Ví dụ: reports/prod-123/rpt-456/report.pdf

Retention: 30 ngày (configurable)
Access: Presigned URL với TTL 24h
Permissions: Chỉ product members có quyền download
```

---

## 5. Business Rules

| Rule | Chi tiết |
|------|---------|
| Report chứa snapshot | Findings tại thời điểm generate, không real-time |
| PDF có watermark | Configurable: CONFIDENTIAL, DRAFT, ... |
| JSON exit code | 0=pass, 1=warning(high), 2=fail(critical) — cho CI/CD |
| Retention 30 ngày | File tự động xóa sau 30 ngày (MinIO lifecycle rule) |
| FalsePositive excluded | Mặc định không include FalsePositive trong report |
| Presigned URL | Download URL có hạn 24h, không cần auth thêm |
