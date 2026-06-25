# TASK-V5-001: Migrate Scan Service Database Schema lên Server

## Mô tả
Chạy migration SQL của scan-service vào PostgreSQL server production để tạo schema `scan` và các bảng liên quan.

## Nguyên nhân
Scan-service dùng `scan.scans`, `scan.findings`, `scan.scheduled_scans` nhưng chưa migrate.

## Các bước thực thi

### 1. Kiểm tra trạng thái hiện tại
```bash
ssh ubuntu@172.20.2.48 'cd /opt/osv-backend && docker compose -f docker-compose.server.yml exec -T postgres psql -U osv -d osv -c "SELECT schemaname FROM information_schema.schemata"'
```

### 2. Chạy migration
Chạy các file SQL theo thứ tự:
1. `services/scan-service/migrations/001_initial_schema.sql` — Schema `scan` + bảng `scans`, `findings`
2. `services/scan-service/migrations/002_agent_001_initial_schema.sql` — Agent tables
3. `services/scan-service/migrations/003_asset_001_initial_schema.sql` — Asset tables
4. `services/scan-service/migrations/004_import_pipeline.sql` — Import pipeline

### 3. Cập nhật deploy_backend.sh
Thêm migration step trước khi restart containers.

## Acceptance Criteria
- [x] `SELECT COUNT(*) FROM scan.scans` trả về 0 (không lỗi)
- [x] `GET /api/v1/scans` trả về 200 với `{ scans: [], total: 0 }`
- [x] `GET /api/v1/scans/stats/weekly` trả về 200 với 7-element array

## Status: DONE ✅
