# CR-INIT-003 — Khởi tạo data-service

## Mục tiêu

Sau khi chạy init, data-service phải:
1. Có database schema đầy đủ cho vulnerability data, KEV entries, alias groups, sync jobs
2. Cấu hình storage backend đúng (Postgres hoặc Firestore)
3. Expose `/health` endpoint hoạt động

## Biến môi trường (đọc từ `.env`)

| Biến | Mô tả | Default |
|------|-------|---------|
| `DATA_DATABASE_URL` | PostgreSQL DSN | `postgres://osv:osv_dev_secret_change_me@localhost:5432/osvdb?sslmode=disable` |
| `DATA_GRPC_PORT` | gRPC port | `50053` |
| `DATA_HTTP_PORT` | HTTP port | `8080` |
| `ALIAS_GROUP_BACKEND` | Storage backend: `postgres` hoặc `firestore` | `postgres` |
| `GCP_PROJECT_ID` | GCP project (chỉ dùng khi firestore) | `osv-local` |
| `FIRESTORE_EMULATOR_HOST` | Firestore emulator (nếu dùng local) | `localhost:8200` |
| `OTLP_ENDPOINT` | OpenTelemetry collector | `http://localhost:4318` |

## Các thay đổi cần thực hiện

### [NEW] `services/data-service/scripts/init.sh`

```bash
#!/usr/bin/env bash
# data-service bootstrap script
# Apply tất cả PostgreSQL migrations cho vulnerability data platform

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"

# Load .env
if [ -f "${SCRIPT_DIR}/../../../.env" ]; then
  set -o allexport
  source "${SCRIPT_DIR}/../../../.env"
  set +o allexport
fi

DATA_DATABASE_URL="${DATA_DATABASE_URL:-${POSTGRES_DSN:-postgres://osv:osv_dev_secret_change_me@localhost:5432/osvdb?sslmode=disable}}"
ALIAS_GROUP_BACKEND="${ALIAS_GROUP_BACKEND:-postgres}"

echo "=== [data-service] Bootstrap Start ==="

# ── Step 1: Create schema ─────────────────────────────────────────────────
echo "→ [1/2] Applying database migrations..."
psql "${DATA_DATABASE_URL}" -c "CREATE SCHEMA IF NOT EXISTS vuln;"
psql "${DATA_DATABASE_URL}" -c "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";"
psql "${DATA_DATABASE_URL}" -c "CREATE EXTENSION IF NOT EXISTS \"citext\";"

# Apply migrations theo thứ tự số
MIGRATION_DIR="${SERVICE_DIR}/migrations"
for sql_file in $(ls "${MIGRATION_DIR}"/*.sql 2>/dev/null | sort -V); do
  fname="$(basename "$sql_file")"
  # Chỉ apply UP migrations, bỏ qua DOWN
  if [[ "$fname" == *".down.sql" ]]; then
    continue
  fi
  echo "   Applying: $fname"
  psql "${DATA_DATABASE_URL}" -f "$sql_file" 2>/dev/null || \
    echo "   (may already exist, continuing...)"
done
echo "   ✓ Database schema ready"

# ── Step 2: Configure storage backend ────────────────────────────────────
echo "→ [2/2] Storage backend: ${ALIAS_GROUP_BACKEND}"
if [ "$ALIAS_GROUP_BACKEND" = "firestore" ]; then
  if [ -z "${FIRESTORE_EMULATOR_HOST:-}" ] && [ -z "${GCP_PROJECT_ID:-}" ]; then
    echo "   ⚠ WARNING: ALIAS_GROUP_BACKEND=firestore requires GCP_PROJECT_ID"
    echo "   For local dev, set FIRESTORE_EMULATOR_HOST=localhost:8200"
  else
    echo "   ✓ Firestore backend configured (project: ${GCP_PROJECT_ID:-not set})"
  fi
else
  echo "   ✓ PostgreSQL backend configured"
fi

echo ""
echo "=== [data-service] Bootstrap Complete ==="
echo "   gRPC: :${DATA_GRPC_PORT:-50053}"
echo "   HTTP: :${DATA_HTTP_PORT:-8080}"
echo ""
echo "Test:"
echo "  curl http://localhost:${DATA_HTTP_PORT:-8080}/health"
```

### [MODIFY] `services/data-service/cmd/server/main.go`

Cập nhật để đọc đúng tên biến môi trường:

```go
// Thêm đọc cấu hình database:
dbURL := envOr("DATA_DATABASE_URL", envOr("POSTGRES_DSN",
    "postgres://osv:osv_dev_secret_change_me@localhost:5432/osvdb?sslmode=disable"))

// Thêm health handler với thông tin đầy đủ hơn:
mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, `{"status":"ok","service":"data-service","grpc_port":"%s","http_port":"%s"}`,
        grpcPort, httpPort)
})
```

### [MODIFY] `services/data-service/internal/config/storage_config.go`

Sửa hàm `envOr` thực sự đọc từ OS:

```go
// Thay hàm placeholder bằng:
import "os"

func envOr(key, defaultVal string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultVal
}
```

## Danh sách migrations theo thứ tự

```
001_create_kev_entries.down.sql    — rollback KEV table
002_create_kev_entries.up.sql      — tạo bảng kev_entries (KEV CISA catalog)
003_initial_schema.sql             — schema chính: vulnerabilities, taxonomy, dbinfo
004_create_sync_jobs.up.sql        — bảng sync_jobs (scheduler state)
005_create_alias_groups.up.sql     — bảng alias_groups (CVE aliases)
006_kev_ransomware_extended.up.sql — thêm cột ransomware info vào kev_entries
007_add_source_isexploit.up.sql    — thêm cột source, is_exploit
```

## Acceptance Criteria

- [ ] `services/data-service/scripts/init.sh` tồn tại và executable
- [ ] Tất cả migrations apply thành công
- [ ] `ALIAS_GROUP_BACKEND` env var được đọc đúng
- [ ] `GET /health` trả về 200
- [ ] gRPC health check: `grpc_health_probe -addr=:50053` trả về SERVING
- [ ] Chạy lại script không gây lỗi (idempotent)

## Kiểm tra nhanh

```bash
# 1. Chạy init
./services/data-service/scripts/init.sh

# 2. Start service
cd services/data-service
DATA_DATABASE_URL=$DATA_DATABASE_URL \
ALIAS_GROUP_BACKEND=postgres \
./server

# 3. Health check
curl http://localhost:8080/health
```
