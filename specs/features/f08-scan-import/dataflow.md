# F08 — Scan Import: Data Flow

---

## 1. Luồng Import Scan

```
Client → POST /api/v2/import-scan
    Content-Type: multipart/form-data
    Fields: file, test_id, tool (optional), min_severity (optional)
    │
    ▼
gateway: auth + route → scan-service
    │
    ▼
scan-service:
    1. Validate file (size, format)
    2. INSERT import_jobs {status='processing', test_id, tool}
    3. Chạy 12-step pipeline (bất đồng bộ nếu file lớn):

    [Step 1-6: Parse + Normalize + Filter]
        raw_findings = parser.parse(file)
        normalized   = normalize_all(raw_findings)
        filtered     = apply_filters(normalized, config)
    
    [Step 7: Enrich từ data-service]
        for finding in filtered:
            if finding.cve_id:
                cve = GET data-service /api/v2/cve/{cve_id}
                finding.enrich(cve)
    
    [Step 8-9: Hash + Dedup]
        for finding in filtered:
            finding.hash_code = SHA256(...)
            check existing hashes in product
            → set is_duplicate, state
    
    [Step 10-11: SLA + Create]
        for finding in filtered:
            sla = sla-service.GetSLADays(product_id, severity)
            finding-service.CreateFinding(finding)
    
    [Step 12: Summary]
        UPDATE import_jobs SET status='completed', created=N, duplicates=M

    │
    ▼
Client ← 202 {import_job_id}  (hoặc 200 nếu sync nhỏ)

---

Client → GET /api/v2/import-scans/{id}
    ← 200 {status, created, duplicates, errors, findings[]}
```

---

## 2. Luồng Tạo Finding Từ Import

```
scan-service → finding-service.CreateFinding(finding, test_id)
    │
    ▼
finding-service:
    1. Validate test_id, product_id, user access
    2. INSERT findings với hash_code, is_duplicate, sla_expiration_date
    3. if NOT is_duplicate:
        Publish NATS: finding.state.changed {to: Active}
        sla-service nhận → track SLA
        audit-service nhận → log
    4. if is_duplicate:
        Publish NATS: finding.duplicate.detected
        audit-service nhận → log
```

---

## 3. CVE Enrichment Flow

```
scan-service (step 7)
    │
    ▼
For each finding with cve_id:
    GET data-service: /api/v2/cve/{cve_id}
    │
    ├── [Found] → finding.cvss = cve.cvss3, finding.epss = cve.epss,
    │             finding.is_kev = cve.is_kev
    └── [Not Found / Timeout] → skip enrichment, continue
        (log warning nhưng không fail import)
```

---

## 4. Import Job Status

```
import_jobs states:
    processing → completed
               → failed (nếu lỗi nghiêm trọng, ví dụ unsupported format)
               → partial (parse thành công nhưng có errors)

GET /api/v2/import-scans/{id}
→ {
    status: "completed",
    tool: "trivy",
    file_name: "trivy-report.json",
    created_at: "...",
    summary: {
        total_parsed: 150,
        created: 45,
        duplicates: 98,
        excluded: 5,
        errors: 2
    },
    errors: [{row: 23, message: "..."}]
  }
```

---

## 5. NATS Events

| Event | Publisher | Trigger |
|-------|-----------|---------|
| `finding.state.changed` | finding-service | Mỗi Active finding được tạo |
| `finding.duplicate.detected` | finding-service | Duplicate finding detected |
| `audit.import.completed` | scan-service | Import job done |
