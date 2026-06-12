> **✅ COMPLETED** — Bridge Pattern, go build && go vet passed.

# T12 — Report Service Wiring (PDF Generation)

## Thông tin
| | |
|---|---|
| **Phase** | 4 — Reports |
| **Ước tính** | 2–3 giờ |
| **Depends on** | T08 (findings), T10 (product/engagement) |
| **Blocks** | — |

---

## Packages cần import

| Import path | Thành phần |
|-------------|------------|
| `report-service/internal/usecase/generatereport/` | PDF generator |
| `report-service/internal/usecase/generateavailablefix/` | Available fixes |
| `report-service/internal/formatters/` | HTML/PDF formatter |
| `report-service/internal/infrastructure/` | MinIO/S3 storage |
| `report-service/adapter/` | Data fetcher |

---

## Các bước thực hiện

### 12.1 Đọc report-service API

```bash
cat osv.dev/services/report-service/internal/usecase/generatereport/*.go
cat osv.dev/services/report-service/internal/usecase/generateavailablefix/*.go
cat osv.dev/services/report-service/internal/infrastructure/*.go
cat osv.dev/services/report-service/adapter/*.go
```

### 12.2 Khởi tạo storage (MinIO)

```go
import (
    reportinfra "github.com/osv/report-service/internal/infrastructure"
)

storage := reportinfra.NewMinIOStorage(
    cfg.Storage.Endpoint,
    cfg.Storage.AccessKey,
    cfg.Storage.SecretKey,
    cfg.Storage.Bucket,
    cfg.Storage.UseSSL,
)
```

### 12.3 Khởi tạo report usecase

```go
import (
    reportuc    "github.com/osv/report-service/internal/usecase/generatereport"
    fixuc       "github.com/osv/report-service/internal/usecase/generateavailablefix"
    reportadapt "github.com/osv/report-service/adapter"
)

// Data fetcher (cần inject scan + finding repos)
dataFetcher := reportadapt.New(a.ScanRepo, a.FindingRepo, cveRepo)

reportUC := reportuc.New(dataFetcher, storage, a.log)
fixUC    := fixuc.New(dataFetcher, a.log)
```

### 12.4 Mount report routes

```go
// Trong router protected group:
r.Get("/api/v1/scans/{id}/pdf", func(w http.ResponseWriter, r *http.Request) {
    id, _ := uuid.Parse(chi.URLParam(r, "id"))

    pdfPath, err := a.ReportUC.Execute(r.Context(), reportuc.Input{
        ScanID: id,
        Format: "pdf",
    })
    if err != nil {
        writeJSON(w, 500, map[string]string{"error": err.Error()})
        return
    }

    // Stream file từ MinIO hoặc disk
    pdfData, _ := storage.Get(r.Context(), pdfPath)
    defer pdfData.Close()

    w.Header().Set("Content-Type", "application/pdf")
    w.Header().Set("Content-Disposition", fmt.Sprintf(
        `attachment; filename="scan-%s-report.pdf"`, id,
    ))
    io.Copy(w, pdfData)
})

r.Get("/api/v1/scans/{id}/fixes", func(w http.ResponseWriter, r *http.Request) {
    id, _ := uuid.Parse(chi.URLParam(r, "id"))
    fixes, _ := a.FixUC.Execute(r.Context(), fixuc.Input{ScanID: id})
    writeJSON(w, 200, fixes)
})
```

### 12.5 Cập nhật App struct

```go
type App struct {
    // ... existing fields
    ReportUC   *reportuc.UseCase
    FixUC      *fixuc.UseCase
    Storage    reportinfra.Storage
}
```

---

## Output

- [x] MinIO storage khởi tạo ✓ (StorageType: local/minio/s3, local fallback at /tmp/openvulnscan-reports)
- [x] GenerateReport usecase khởi tạo ✓ (reportBridge.generateReport: JSON/CSV/HTML/PDF)
- [x] PDF endpoint `GET /api/v1/scans/{id}/pdf` ✓ (report_runner.go: format=pdf)
- [x] Fixes endpoint `GET /api/v1/scans/{id}/fixes` ✓ (getScanFixes handler)

## Acceptance Criteria

```bash
TOKEN=<token>
SCAN_ID=<completed_scan_id>

# Download PDF report
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/scans/$SCAN_ID/pdf" \
  --output report.pdf
# → report.pdf được download, có thể mở bằng PDF viewer

# Get available fixes
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/scans/$SCAN_ID/fixes"
# → {"fixes":[{"package":"openssl","current":"1.1.1","fixed":"3.0.0"},...]}
```

## Lưu ý

- MinIO phải đang chạy (`docker-compose up -d minio`)
- Bucket phải được tạo tự động khi report-service init
- Nếu report-service dùng WeasyPrint/Puppeteer cho PDF, cần cài thêm dependencies
