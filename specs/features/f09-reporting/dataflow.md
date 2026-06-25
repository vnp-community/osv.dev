# F09 — Reporting: Data Flow

---

## 1. Luồng Generate Report

```
Client → POST /api/v2/reports/generate
    {scope: "product", scope_id: "prod-123", format: "pdf", filters: {...}}
    │
    ▼
finding-service:
    1. Validate: user có quyền trong product/engagement
    2. INSERT report_jobs {status='queued', scope, format}
    3. Return 202 {report_id: "rpt-456"}
    │
    ▼ (async worker)
    4. Query findings theo scope + filters
    5. Fetch metadata: product info, engagement, grade
    6. Generate content theo format:
       PDF → render HTML template → convert to PDF → bytes
       CSV → build rows → CSV bytes
       JSON → serialize → JSON bytes
    7. Upload to MinIO: reports/prod-123/rpt-456/report.pdf
    8. UPDATE report_jobs SET status='completed', file_url, generated_at
    9. Publish NATS: report.generated {report_id, product_id, format}
    │
    ▼
notification-service ← report.generated → (optional) email "report ready" to requester
```

---

## 2. Luồng Download Report

```
Client → GET /api/v2/reports/{id}
    ← {status: "completed", download_url: "https://minio.../...?token=xxx"}

Client → GET {download_url}  (presigned, trực tiếp từ MinIO, bypass gateway)
    ← file bytes (PDF/CSV/JSON/...)
```

---

## 3. Luồng Report List

```
Client → GET /api/v2/reports?product_id=prod-123
    │
    ▼
finding-service:
    SELECT * FROM report_jobs
    WHERE product_id=$1 AND user has access
    ORDER BY created_at DESC
    │
    ▼
Client ← [{report_id, format, status, generated_at, file_size, ...}]
```

---

## 4. CI/CD Integration Flow

```
CI/CD Pipeline:
    1. POST /api/v2/reports/generate {format: "json", scope: "test", scope_id: test_id}
    2. Poll GET /api/v2/reports/{id} until status='completed'
    3. GET download_url → parse JSON
    4. Read exit_code:
        0 → pipeline success
        1 → pipeline warning (annotate PR)
        2 → pipeline fail (block merge)
```

---

## 5. NATS Events

| Event | Publisher | Trigger | Subscribers |
|-------|-----------|---------|------------|
| `report.generated` | finding-service | Report generation completed | notification-service (optional email), audit-service |
| `report.failed` | finding-service | Generation error | audit-service |
