# Change Request 010: Reports — Templates Endpoint & Schema Updates

**Tạo:** 2026-06-19  
**Status:** New — 1 endpoint thiếu + 2 schema updates cho Reports module.  
**Nguồn:** openapi.yaml schemas `ReportTemplate`, `ReportTemplatesResponse`, `ReportsListResponse`, `CreateReportRequest`  
**Target directory:** `specs/crs/v2/api-ui-v1/`

---

## 1. Bối cảnh

`ReportCenter.tsx` sau khi fix hardcode cần:
1. `GET /api/v1/reports/templates` — load danh sách templates (thay thế hardcode array trong component)
2. `GET /api/v1/reports` response cần thêm `last_generated_at` field (dùng cho subtitle)
3. `Report` schema cần thêm `format`, `finding_count`, `file_size_bytes`, `artifact_url` fields

| Thay đổi | Endpoint | Service | Trạng thái |
|---|---|---|---|
| Endpoint mới | `GET /api/v1/reports/templates` | finding-service | ❌ THIẾU |
| Schema update | `GET /api/v1/reports` response | finding-service | ⚠️ Thiếu fields |
| Schema update | `Report` object | finding-service | ⚠️ Thiếu fields |
| Schema update | `POST /api/v1/reports` request body | finding-service | ⚠️ Cần update |

---

## 2. Chi tiết Thay đổi

### 2.1 [HIGH] `GET /api/v1/reports/templates` — Report Templates

**Gateway routing** — thêm vào `apps/osv/internal/gateway/router.go`:

```go
// Report Templates (phải đặt TRƯỚC /api/v1/reports/{id} để tránh path conflict)
mux.Handle("GET /api/v1/reports/templates",
    protected(proxy.Forward("finding-service:8085")))
```

> ⚠️ **Path ordering**: `/reports/templates` phải được đăng ký trước `/reports/{id}` trong router để tránh `"templates"` bị parse thành `{id}`.

**Response `ReportTemplatesResponse`:**
```json
{
  "templates": [
    {
      "id": "exec",
      "name": "Executive Summary",
      "description": "High-level overview for C-level presentations. Bao gồm risk posture, top vulnerabilities, và SLA compliance.",
      "type": "Executive"
    },
    {
      "id": "tech",
      "name": "Technical Report",
      "description": "Chi tiết findings với CVE details, CVSS scores, và hướng dẫn remediation.",
      "type": "Technical"
    },
    {
      "id": "comp",
      "name": "Compliance Report",
      "description": "Mapped tới PCI DSS, ISO 27001, SOC2, NIST frameworks.",
      "type": "Compliance"
    }
  ]
}
```

**Implementation trong finding-service:**

```go
// services/finding-service/internal/handler/reports.go
func (h *Handler) GetReportTemplates(w http.ResponseWriter, r *http.Request) {
    // Templates là static config — không cần DB query
    // Hoặc load từ config file / DB table nếu cần customize
    templates := h.config.ReportTemplates
    json.NewEncoder(w).Encode(ReportTemplatesResponse{Templates: templates})
}
```

Templates có thể là **static config** trong `finding-service` (không cần DB) hoặc load từ config file YAML/JSON.

---

### 2.2 [HIGH] `Report` Schema — Bổ sung Fields

Finding-service cần trả về các fields bổ sung trong `Report` object:

| Field | Type | Mô tả |
|---|---|---|
| `format` | string enum | `"pdf" \| "html" \| "csv" \| "excel" \| "json"` |
| `finding_count` | integer, nullable | Số findings trong báo cáo |
| `file_size_bytes` | integer, nullable | Kích thước file sau khi generate |
| `generated_at` | datetime, nullable | Thời điểm hoàn thành generate |
| `artifact_url` | string, nullable | S3/MinIO presigned download URL |
| `expires_at` | datetime, nullable | URL expiry time |

**Report object đầy đủ:**
```json
{
  "id": "R-047",
  "name": "Q2 2026 Executive Summary",
  "type": "Executive",
  "format": "pdf",
  "status": "completed",
  "finding_count": 127,
  "file_size_bytes": 2516582,
  "generated_at": "2026-06-14T09:00:00Z",
  "artifact_url": "https://minio.internal/reports/R-047.pdf?token=...",
  "expires_at": "2026-06-21T09:00:00Z",
  "created_at": "2026-06-14T08:55:00Z",
  "completed_at": "2026-06-14T09:00:00Z",
  "created_by": "carol@company.com"
}
```

**DB schema update** (nếu report table chưa có các columns này):
```sql
ALTER TABLE reports 
    ADD COLUMN IF NOT EXISTS format VARCHAR(10) NOT NULL DEFAULT 'pdf',
    ADD COLUMN IF NOT EXISTS finding_count INTEGER,
    ADD COLUMN IF NOT EXISTS file_size_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS generated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS artifact_url TEXT,
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
```

---

### 2.3 [HIGH] `ReportsListResponse` — Thêm `last_generated_at`

`GET /api/v1/reports` response cần thêm `last_generated_at` field:

```json
{
  "reports": [...],
  "total": 47,
  "last_generated_at": "2026-06-14T09:00:00Z"
}
```

**Query:**
```sql
SELECT MAX(generated_at) AS last_generated_at 
FROM reports 
WHERE status = 'completed';
```

Frontend dùng field này để render subtitle: _"Last report 6h ago"_.

---

### 2.4 [MEDIUM] `POST /api/v1/reports` — Request Body Schema Update

Request body cần support các fields mới từ `CreateReportRequest`:

```json
{
  "type": "Executive",
  "format": "pdf",
  "product_id": "p-1",
  "engagement_id": null,
  "min_severity": "High",
  "min_score": null,
  "date_from": "2026-01-01",
  "date_to": "2026-06-30"
}
```

**Backward compatible** — chỉ `type` và `format` là required. Các fields khác optional.

---

### 2.5 Report generation — Artifact Upload

Sau khi generate, finding-service cần:
1. Upload file lên **MinIO/S3** bucket `reports/`
2. Generate **presigned URL** với TTL 7 ngày
3. Cập nhật `artifact_url`, `expires_at`, `file_size_bytes`, `generated_at` trong DB
4. Cập nhật `status = "completed"`

```go
// services/finding-service/internal/service/report_generator.go
func (s *Service) finalizeReport(ctx context.Context, reportID string, filePath string) error {
    info, _ := os.Stat(filePath)
    url, expiry, _ := s.storage.UploadAndSign(ctx, filePath, 7*24*time.Hour)
    
    return s.repo.UpdateReport(ctx, reportID, ReportUpdate{
        Status:        "completed",
        FileSizeBytes: info.Size(),
        ArtifactURL:   url,
        ExpiresAt:     expiry,
        GeneratedAt:   time.Now(),
    })
}
```

---

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `GET /api/v1/reports/templates` trả về `{ templates: [3 items] }` — HTTP 200.
2. Template objects có đầy đủ `id`, `name`, `description`, `type`.
3. `GET /api/v1/reports` response có `last_generated_at` field (ISO datetime hoặc `null` nếu chưa có report nào).
4. Mỗi `Report` object trong list có `format` field — không null.
5. Report có status `completed` phải có `artifact_url` (presigned URL) — không null.
6. `artifact_url` phải còn hiệu lực ít nhất 7 ngày từ lúc generate.
7. `POST /api/v1/reports` với `format: "pdf"` tạo report — trả `202 Accepted`.
8. `GET /api/v1/reports/templates` không conflict với `GET /api/v1/reports/{id}` (router ordering).
