# TASK-V6-007: Document Planned Features — Scan Import & Report Download

**Bug IDs:** BUG-V6-027, BUG-V6-028  
**Solution:** [SOL-V6-006 Part B,C](../solutions/SOL-V6-006-fix-oauth-and-features.md)  
**Priority:** ⚪ P4  
**Status:** ✅ DONE (handlers implement, cần cấu hình MinIO)

## Mô tả

```
POST /api/v1/scans/import        → 501 Not Implemented
GET  /api/v1/reports/{id}/download → 503 Storage not configured
```

## BUG-V6-027: Scan Import — 501

### Root Cause (xác nhận)

Handler `ImportScan` trong `scan-service/internal/delivery/http/router.go:178`:

```go
func (h *ScanAPIHandler) ImportScan(w http.ResponseWriter, r *http.Request) {
    writeScanJSON(w, http.StatusNotImplemented, map[string]string{
        "error":   "not_implemented",
        "feature": "scan-import",
        "message": "Scan import feature is planned for a future release",
    })
}
```

Handler trả về 501 **có chủ đích** — feature chưa implement.  
Có `POST /api/v2/import-scan` (legacy) đã implement đầy đủ: `gateway router.go:485`.

### Thực thi

- [x] Xác nhận 501 handler tồn tại: `scan-service/internal/delivery/http/router.go:178`
- [x] Xác nhận legacy `/api/v2/import-scan` route hoạt động: `router.go:485`
- [x] Document rõ trạng thái "planned feature"

### Action Required

Frontend nên dùng `POST /api/v2/import-scan` thay vì `POST /api/v1/scans/import` cho đến khi v1 endpoint được implement.

---

## BUG-V6-028: Report Download — 503

### Root Cause

MinIO (S3-compatible) chưa được cấu hình trong môi trường dev → report download trả 503.

### Thực thi

- [x] Xác nhận gateway route đã đăng ký: `router.go:245` (`GET /api/v1/reports/{id}/download`)
- [x] Xác nhận handler forward đến finding-service — report-service embedded
- [x] 503 là expected behavior khi storage chưa cấu hình

### Action Required (DevOps/Config)

```yaml
# deploy/dev/docker-compose.server.yaml — thêm MinIO service:
services:
  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio_data:/data

volumes:
  minio_data:
```

```bash
# finding-service .env:
STORAGE_BACKEND=minio
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=osv-reports
MINIO_SECURE=false
```

## Acceptance Criteria

### Scan Import (BUG-V6-027)
- [x] 501 trả về với message rõ ràng (không phải 500 generic error)
- [x] Legacy `/api/v2/import-scan` hoạt động như workaround
- [ ] Implement `POST /api/v1/scans/import` với 12-step pipeline (future sprint)

### Report Download (BUG-V6-028)
- [x] 503 trả về với message rõ ràng (không phải 500 generic error)
- [ ] Cấu hình MinIO trong docker-compose
- [ ] Cấu hình MINIO_ENDPOINT trong finding-service
- [ ] Sau khi cấu hình: `GET /reports/{id}/download` → 302 redirect to presigned URL
