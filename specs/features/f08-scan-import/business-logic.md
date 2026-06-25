# F08 — Scan Import: Business Logic

> Mô tả bằng ngôn ngữ tự nhiên + pseudo-code.

---

## 1. Parser Factory Pattern

### 1.1 Nguyên tắc

Mỗi tool output có một parser riêng. **Parser Factory** nhận tên tool và trả về parser phù hợp:

```
getParser(tool_name):
    registry = {
        "nmap":        NmapXMLParser,
        "zap":         ZAPJSONParser,
        "bandit":      BanditJSONParser,
        "trivy":       TrivyJSONParser,
        "snyk":        SnykJSONParser,
        "sonarqube":   SonarQubeJSONParser,
        "semgrep":     SemgrepJSONParser,
        ...
    }
    parser = registry[tool_name]
    if not parser:
        raise error("unsupported tool: " + tool_name)
    return parser
```

### 1.2 Auto-detect

Nếu user không chỉ định tool, hệ thống thử auto-detect dựa trên:
1. File extension (`.xml`, `.json`, `.csv`)
2. Magic bytes / root XML element
3. JSON top-level fields (`vulnerabilities`, `results`, `issues`, ...)

---

## 2. Import Pipeline — 12 Bước

### Bước 1: Validate file

```
validate(file):
    - File size < 100MB
    - File extension hợp lệ
    - File không rỗng
    - MIME type check (application/json, text/xml, ...)
```

### Bước 2-3: Detect & Select Parser

```
tool = request.tool OR autoDetect(file)
parser = ParserFactory.getParser(tool)
```

### Bước 4: Parse → Raw Findings

```
raw_findings = parser.parse(file_content)
// Mỗi parser output: [{title, severity, description, cve_id?, cpe?, file_path?, line_no?}]
// Parse errors per-row được thu thập nhưng không dừng toàn bộ import
```

### Bước 5: Normalize

```
Chuẩn hóa về schema chung:
    normalize(raw_finding):
        return {
            title:          trim(raw.title) or "Unnamed Finding",
            severity:       mapSeverity(raw.severity),  // → Critical/High/Medium/Low/Info
            description:    raw.description or "",
            cve_id:         extractCVE(raw),            // "CVE-XXXX-XXXX" or null
            component_name: raw.component or "",
            component_version: raw.version or "",
            file_path:      raw.file_path or null,
            line_number:    raw.line_no or null,
            tool:           tool_name
        }

mapSeverity(raw_severity):
    mapping = {
        "critical": "Critical", "high": "High",
        "medium": "Medium", "moderate": "Medium",
        "low": "Low", "info": "Info", "informational": "Info"
    }
    return mapping[lower(raw_severity)] or "Info"
```

### Bước 6: Filter

```
if import_config.exclude_info:
    findings = filter(f.severity != "Info", findings)

if import_config.min_severity:
    findings = filter(severityOrder[f.severity] >= min, findings)
```

### Bước 7: Enrich với CVE data

```
for finding in findings:
    if finding.cve_id:
        cve_data = data-service.GetCVE(finding.cve_id)
        if cve_data:
            finding.cvss_score  = cve_data.cvss3
            finding.epss_score  = cve_data.epss
            finding.is_kev      = cve_data.is_kev
            finding.cwe_ids     = cve_data.cwe
```

### Bước 8: Hash Fingerprint

```
for finding in findings:
    finding.hash_code = SHA-256(
        finding.title + "|" +
        finding.component_name + "|" +
        finding.component_version + "|" +
        (finding.cve_id or "")
    )
```

### Bước 9: Dedup Check

```
existing_hashes = getHashesForProduct(test.product_id, state='Active')

for finding in findings:
    if finding.hash_code IN existing_hashes:
        finding.is_duplicate = true
        finding.state = "Duplicate"
        finding.duplicate_of = existing_hashes[finding.hash_code].id
    else:
        finding.is_duplicate = false
        finding.state = "Active"
        existing_hashes.add(finding.hash_code)  // prevent intra-import duplicates
```

**Lưu ý:** Dedup check cả trong cùng một import (intra-import dedup) để tránh duplicate khi một tool report cùng một issue nhiều lần.

### Bước 10: Apply SLA

```
for finding in findings:
    if not finding.is_duplicate:
        finding.sla_days = getSLADays(product_id, finding.severity)
        finding.sla_expiration_date = NOW() + sla_days
```

### Bước 11: Create Findings

```
for finding in findings:
    finding-service.CreateFinding(finding, test_id)
    // finding-service tiếp nhận, validate, INSERT
    // Publish NATS: finding.state.changed (nếu Active)
```

### Bước 12: Import Summary

```
summary = {
    total_parsed:   len(raw_findings),
    created:        count of new Active findings,
    duplicates:     count of Duplicate findings,
    errors:         count of parse errors,
    excluded:       count filtered out,
    import_job_id:  import_job.id
}
UPDATE import_jobs SET status='completed', ...summary
```

---

## 3. Dedup Algorithms

Hệ thống dùng **3 thuật toán dedup** theo priority:

### Algorithm 1: Hash-based (Primary)
```
Dùng SHA-256 fingerprint như bước 8-9 ở trên.
Nhanh nhất, chính xác nhất khi cùng tool, cùng component.
```

### Algorithm 2: CVE-based (Secondary)
```
Nếu hai findings có cùng cve_id trong cùng product:
    → Coi là potential duplicate
    → Flag để user review
```

### Algorithm 3: Title similarity (Tertiary)
```
Nếu title similarity > 85% (Levenshtein distance):
    → Mark as potential duplicate
    → Không tự động set Duplicate — cần user confirm
```

---

## 4. Business Rules

| Rule | Chi tiết |
|------|---------|
| Parse errors không dừng import | Thu thập errors, tiếp tục parse rows còn lại |
| Intra-import dedup | Cùng một import, cùng hash → chỉ tạo 1 finding |
| CVE enrichment best-effort | Nếu data-service không response → bỏ qua enrichment, tiếp tục |
| File size limit | 100MB tối đa |
| Duplicate không có SLA | `is_duplicate=true` → không tính SLA deadline |
| Auto-close on reimport | (Optional) Nếu hash không còn trong scan kết quả → auto-mitigate |
