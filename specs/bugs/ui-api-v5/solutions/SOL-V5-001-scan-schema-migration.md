# SOL-V5-001: Migrate Scan Service Database Schema

## Vấn đề
Scan-service sử dụng các bảng trong schema `scan` (tức `scan.scans`, `scan.findings`, `scan.scheduled_scans`) nhưng chưa bao giờ được migrate vào PostgreSQL server production.

Tất cả các file migration nằm tại:
- `services/scan-service/migrations/001_initial_schema.sql`
- `services/scan-service/migrations/002_agent_001_initial_schema.sql`
- `services/scan-service/migrations/003_asset_001_initial_schema.sql`
- `services/scan-service/migrations/004_import_pipeline.sql`

## Root Cause
- Migration chưa được chạy khi deploy.
- `deploy_backend.sh` không có bước migration.
- Khi `osv-server` khởi động, nếu bảng không tồn tại thì `scanRepo.ListRaw()` → SQL Error → 500.

## Giải pháp
1. Chạy migration SQL trực tiếp vào server DB.
2. Cập nhật `deploy_backend.sh` hoặc docker-compose.server.yml để auto-migrate khi startup.

## Files cần thay đổi
- Chạy trực tiếp: `services/scan-service/migrations/*.sql`
- Tùy chọn: `deploy/dev/deploy_backend.sh` (thêm migration step)

## Cách kiểm tra
- `GET /api/v1/scans` → 200
- `GET /api/v1/scans/stats/weekly` → 200 với 7-element array
