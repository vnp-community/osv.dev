> **✅ COMPLETED** — 13 migration files merged.

# T03 — Database Migrations Merge

## Thông tin
| | |
|---|---|
| **Phase** | 1 — Khung sườn |
| **Ước tính** | 2–3 giờ |
| **Depends on** | T01 |
| **Blocks** | T04, T05, T08, T10 |

## Mục tiêu
Thu thập và merge SQL migration files từ tất cả services thành một chuỗi migrations thống nhất, đảm bảo không có conflict về tên bảng, column, và foreign key constraints.

---

## Các bước thực hiện

### 3.1 Thu thập migrations từ tất cả services

```bash
# Liệt kê migration files hiện có
find osv.dev/services -name "*.sql" -path "*/migrations/*" | sort

# Directories cần kiểm tra:
# services/auth-service/migrations/
# services/scan-service/migrations/
# services/finding-service/migrations/
# services/product-service/migrations/
# services/vulnerability-service/migrations/
# services/report-service/migrations/
# services/notification-service/migrations/
# services/ingestion-service/migrations/
# services/query-service/migrations/
```

### 3.2 Phân tích từng migration file

Với mỗi service, đọc nội dung và ghi ra:
- Danh sách bảng được tạo
- Foreign key references
- Có extension nào cần không (uuid-ossp, pg_trgm, etc.)

### 3.3 Đánh số lại migrations theo thứ tự hợp lệ

Thứ tự đề xuất (phụ thuộc foreign key):

```
001_extensions.sql          # CREATE EXTENSION IF NOT EXISTS "uuid-ossp"
002_users.sql               # auth-service: users, api_keys
003_products.sql            # product-service: products, product_types
004_engagements.sql         # product-service: engagements, tests
005_scans.sql               # scan-service: scans, scheduled_scans
006_scan_findings.sql       # scan-service: findings, web_alerts, discovery_hosts
007_assets.sql              # scan-service: assets (upsert_asset)
008_agent_reports.sql       # scan-service: agent_reports, packages
009_findings.sql            # finding-service: findings (defectdojo model)
010_findings_sla.sql        # finding-service: sla_configs
011_findings_audit.sql      # finding-service: audit_logs
012_cves.sql                # vulnerability-service: cves, kev_entries
013_notifications.sql       # notification-service: notification_rules, delivery_records
014_report_configs.sql      # report-service: report_configs (nếu có)
015_siem_config.sql         # App-level: siem_config table (mới, không có service)
```

### 3.4 Tạo migration file `015_siem_config.sql` (mới)

```sql
-- 015_siem_config.sql
-- SIEM configuration table (không có trong services gốc)
CREATE TABLE IF NOT EXISTS siem_configs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enabled     BOOLEAN NOT NULL DEFAULT FALSE,
    host        VARCHAR(255) NOT NULL DEFAULT '',
    port        INTEGER NOT NULL DEFAULT 514,
    protocol    VARCHAR(10) NOT NULL DEFAULT 'udp' CHECK (protocol IN ('udp', 'tcp')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default config
INSERT INTO siem_configs (enabled, host, port, protocol)
VALUES (false, '', 514, 'udp')
ON CONFLICT DO NOTHING;
```

### 3.5 Tạo migration runner

Chọn một trong hai approach:

**Option A: golang-migrate (recommended)**

```bash
# Cài golang-migrate
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Chạy migrations
migrate -path ./migrations -database "postgres://openvulnscan:secret@localhost:5432/openvulnscan?sslmode=disable" up
```

**Option B: Tích hợp vào main.go**

```go
// cmd/server/main.go hoặc cmd/migrate/main.go
import "github.com/golang-migrate/migrate/v4"
import _ "github.com/golang-migrate/migrate/v4/database/postgres"
import _ "github.com/golang-migrate/migrate/v4/source/file"

m, _ := migrate.New("file://migrations", cfg.Database.URL)
m.Up()
```

Thêm vào `Makefile`:
```makefile
migrate-up:
	migrate -path ./migrations -database "${DATABASE_URL}" up

migrate-down:
	migrate -path ./migrations -database "${DATABASE_URL}" down 1

migrate-status:
	migrate -path ./migrations -database "${DATABASE_URL}" version
```

### 3.6 Xử lý conflicts

Nếu phát hiện conflict (cùng tên bảng, tên khác nhau nhưng concept giống nhau):

| Conflict | Xử lý |
|----------|-------|
| `findings` table tồn tại ở cả scan-service và finding-service | Chọn schema của finding-service (phức tạp hơn), rename scan-service's findings thành `scan_findings` |
| `products` vs `assets` | product-service gọi là `products`, scan-service gọi là `assets` — giữ cả hai, map trong application layer |
| UUID extension | Đảm bảo chỉ tạo 1 lần trong `001_extensions.sql` |

### 3.7 Tạo migration cho admin user

```sql
-- 016_seed_admin.sql
-- Seed default admin user (password: admin123, bcrypt hashed)
-- Hash được tạo bằng: bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
INSERT INTO users (id, email, hashed_password, is_admin, role, created_at)
VALUES (
    gen_random_uuid(),
    'admin@openvulnscan.local',
    '$2a$10$...', -- bcrypt hash của "admin123" — generate trước và hardcode
    true,
    'admin',
    NOW()
)
ON CONFLICT (email) DO NOTHING;
```

---

## Output

- [x] Thư mục `migrations/` với 13 file SQL (đánh số 001–013)
- [x] Không có conflict về tên bảng (dùng schema: auth, scan, agent, asset, cve, report)
- [x] Số thứ tự: extensions → auth → product → scan → agent → asset → finding → sla → cve → notification → report → audit → siem

## Trạng thái: ✅ HOÀN THÀNH
> Thực thi: 2026-06-09
> 13 migrations file tạo (được merge từ tất cả services)
> Giải pháp conflict: scan-service findings → `scan_findings`, giữ cả `findings` (finding-service)
> Tất cả dùng `CREATE TABLE IF NOT EXISTS` — safe to re-run

| Migration | Nội dung | Source |
|-----------|---------|--------|
| 001_extensions.sql | uuid-ossp, citext, vector, schemas | Mới |
| 002_auth.sql | users, sessions, oauth_accounts, api_keys, audit_log | auth-service |
| 003_product.sql | product_types, products, engagements, tests, scan_imports | product-service |
| 004_scan.sql | scans, scan_findings, web_alerts, discovery_hosts | scan-service |
| 005_agent.sql | agents, agent_reports, packages, package_cves | scan-service |
| 006_asset.sql | assets, tags, asset_tags, vulnerabilities | scan-service |
| 007_finding.sql | findings (DefectDojo model) | finding-service |
| 008_finding_sla.sql | sla_configurations | finding-service |
| 009_cve.sql | cves, cve_references, cve_affected_packages, sync_jobs | vulnerability-service |
| 010_notification.sql | notification_rules, alerts, delivery_records | notification-service |
| 011_report.sql | reports | report-service |
| 012_audit_events.sql | audit_events (partitioned) | finding-service |
| 013_siem_config.sql | siem_configs | Mới (app-level) |

## Acceptance Criteria

```bash
# Start PostgreSQL
docker-compose up -d postgres

# Chạy all migrations
make migrate-up DATABASE_URL="postgres://openvulnscan:secret@localhost:5432/openvulnscan?sslmode=disable"

# Kiểm tra bảng tồn tại
psql -h localhost -U openvulnscan -d openvulnscan -c "\dt"
# Phải thấy: users, scans, findings, products, cves, notifications, siem_configs, ...
```

## Lưu ý

- Đọc kỹ từng migration file gốc trước khi merge — một số có thể dùng `CREATE TABLE` thay vì `CREATE TABLE IF NOT EXISTS`
- Nếu services dùng `golang-migrate`, file migration có thể có cả `.up.sql` và `.down.sql`
- Kiểm tra có cần tạo PostgreSQL extensions: `uuid-ossp`, `pg_trgm`, `btree_gin`
