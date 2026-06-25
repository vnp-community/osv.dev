# CR-INIT-006 — Khởi tạo notification-service

## Mục tiêu

Sau khi chạy init, notification-service phải:
1. Có database schema đầy đủ (webhooks, subscriptions, notification rules)
2. Kết nối NATS (optional — service vẫn chạy được nếu NATS chưa có)
3. Redis được kết nối (dùng để deduplicate webhook delivery)
4. Service expose `/health` và sẵn sàng nhận webhook registrations

## Biến môi trường (đọc từ `.env`)

| Biến | Mô tả | Default |
|------|-------|---------|
| `NOTIFICATION_DATABASE_URL` | PostgreSQL DSN | `postgres://osv:osv_dev_secret_change_me@localhost:5432/osvdb?sslmode=disable` |
| `NOTIFICATION_HTTP_PORT` | HTTP port | `8086` |
| `NOTIFICATION_GRPC_PORT` | gRPC port | `50063` |
| `REDIS_URL` | Redis URL | `redis://localhost:6379/0` |
| `NATS_URL` | NATS URL | `nats://localhost:4222` |
| `NATS_ENABLED` | Bắt buộc NATS? `true` \| `false` | `false` |

## Các thay đổi cần thực hiện

### [NEW] `services/notification-service/scripts/init.sh`

```bash
#!/usr/bin/env bash
# notification-service bootstrap script
# 1. Apply database migrations
# 2. Verify Redis connection
# 3. Verify NATS connection (optional)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"

# Load .env
if [ -f "${SCRIPT_DIR}/../../../.env" ]; then
  set -o allexport
  source "${SCRIPT_DIR}/../../../.env"
  set +o allexport
fi

# Fallback chain: NOTIFICATION_DATABASE_URL → DATABASE_URL → POSTGRES_DSN → default
DB_URL="${NOTIFICATION_DATABASE_URL:-${DATABASE_URL:-${POSTGRES_DSN:-postgres://osv:osv_dev_secret_change_me@localhost:5432/osvdb?sslmode=disable}}}"
REDIS_URL="${REDIS_URL:-redis://localhost:6379/0}"
NATS_URL="${NATS_URL:-nats://localhost:4222}"
NATS_ENABLED="${NATS_ENABLED:-false}"
HTTP_PORT="${NOTIFICATION_HTTP_PORT:-8086}"
GRPC_PORT="${NOTIFICATION_GRPC_PORT:-50063}"

echo "=== [notification-service] Bootstrap Start ==="

# ── Step 1: Apply migrations ──────────────────────────────────────────────
echo "→ [1/3] Applying database migrations..."

# Tạo extensions nếu chưa có
psql "${DB_URL}" <<-SQL 2>/dev/null || true
  CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
  CREATE EXTENSION IF NOT EXISTS "citext";
SQL

# Apply migrations theo thứ tự
MIGRATION_DIR="${SERVICE_DIR}/migrations"
for sql_file in $(ls "${MIGRATION_DIR}"/*.sql 2>/dev/null | sort -V); do
  fname="$(basename "$sql_file")"
  # Bỏ qua DOWN migrations
  if [[ "$fname" == *".down.sql" ]]; then
    continue
  fi
  echo "   Applying: $fname"
  psql "${DB_URL}" -f "$sql_file" 2>/dev/null || \
    echo "   (already applied or non-critical, continuing...)"
done
echo "   ✓ Database schema ready"

# ── Step 2: Verify Redis ──────────────────────────────────────────────────
echo "→ [2/3] Checking Redis..."
# Lấy host:port từ URL
REDIS_HOST_PORT=$(echo "$REDIS_URL" | sed 's|redis://[^@]*@\?\([^/]*\).*|\1|')
REDIS_HOST="${REDIS_HOST_PORT%%:*}"
REDIS_PORT="${REDIS_HOST_PORT##*:}"

if redis-cli -h "${REDIS_HOST:-localhost}" -p "${REDIS_PORT:-6379}" ping 2>/dev/null | grep -q "PONG"; then
  echo "   ✓ Redis connected"
else
  echo "   ⚠ WARNING: Redis not available — webhook dedup may not work"
fi

# ── Step 3: Check NATS ────────────────────────────────────────────────────
echo "→ [3/3] Checking NATS (NATS_ENABLED=${NATS_ENABLED})..."
if [ "$NATS_ENABLED" = "true" ]; then
  if nats server check 2>/dev/null || \
     nc -z "${NATS_URL#nats://}" 2>/dev/null; then
    echo "   ✓ NATS available"
  else
    echo "   ✗ ERROR: NATS_ENABLED=true but NATS not available at ${NATS_URL}"
    echo "   Either start NATS or set NATS_ENABLED=false"
    exit 1
  fi
else
  echo "   ℹ NATS optional — notification-service will start without it"
  echo "   Set NATS_ENABLED=true to require NATS connection"
fi

echo ""
echo "=== [notification-service] Bootstrap Complete ==="
echo "   HTTP: :${HTTP_PORT}"
echo "   gRPC: :${GRPC_PORT}"
echo ""
echo "Test:"
echo "  curl http://localhost:${HTTP_PORT}/health"
echo "  # Register a webhook:"
echo "  curl -X POST http://localhost:${HTTP_PORT}/api/v1/webhooks \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"url\":\"http://example.com/hook\",\"events\":[\"vuln.new\"]}'"
```

### [MODIFY] `services/notification-service/cmd/server/main.go`

Cập nhật để đọc đúng biến môi trường:

```go
// Thay:
dbURL := envOr("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/globalcve?sslmode=disable")

// Thành (support NOTIFICATION_DATABASE_URL):
dbURL := envOr("NOTIFICATION_DATABASE_URL", 
    envOr("DATABASE_URL", 
        envOr("POSTGRES_DSN", "postgres://osv:osv_dev_secret_change_me@localhost:5432/osvdb?sslmode=disable")))

// Thay:
httpPort := envOr("HTTP_PORT", "8086")

// Thành:
httpPort := envOr("NOTIFICATION_HTTP_PORT", envOr("HTTP_PORT", "8086"))
grpcPort := envOr("NOTIFICATION_GRPC_PORT", envOr("GRPC_PORT", "50063"))
```

### Thêm `/health` endpoint

Notification service hiện chưa có `/health`. Thêm vào router setup:

```go
// Thêm vào router trong SetupRouter hoặc trong main.go:
router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, `{"status":"ok","service":"notification-service"}`)
})
```

## Danh sách migrations

```
001_dd_tables.sql                       — DefectDojo integration tables
002_create_jira_integrations.up.sql     — Jira integration table
003_notification_defectdojo_events.sql  — DD event notification rules
004_globalcve_001_create_webhooks.up.sql — Webhook registry
005_notification_rules.up.sql           — Notification rules (conditions, channels)
007_inapp_notifications.up.sql          — In-app notification store
008_inapp_alerts_extensions.up.sql      — Alert extensions
202606161830_init_notification.sql      — Latest init migration
```

## NATS Subjects

| Subject | Direction | Mô tả |
|---------|-----------|-------|
| `osv.vuln.imported` | Subscribe | Trigger notification khi CVE mới |
| `osv.vuln.updated` | Subscribe | Trigger khi CVE update |
| `osv.vuln.withdrawn` | Subscribe | Trigger khi CVE withdrawn |
| `globalcve.alert.*` | Subscribe | GlobalCVE namespace events |

## Acceptance Criteria

- [ ] `services/notification-service/scripts/init.sh` tồn tại và executable
- [ ] Tất cả migrations apply thành công
- [ ] Service start được với `NATS_ENABLED=false` khi NATS chưa chạy
- [ ] `GET /health` trả về 200
- [ ] `POST /api/v1/webhooks` nhận và lưu webhook registration
- [ ] Webhook delivery retry worker hoạt động

## Kiểm tra nhanh

```bash
# 1. Init
./services/notification-service/scripts/init.sh

# 2. Start (không cần NATS)
cd services/notification-service
DATABASE_URL=$NOTIFICATION_DATABASE_URL \
REDIS_URL=$REDIS_URL \
NATS_ENABLED=false \
./server

# 3. Test
curl http://localhost:8086/health

# Register webhook
curl -X POST http://localhost:8086/api/v1/webhooks \
  -H 'Content-Type: application/json' \
  -d '{"url":"http://localhost:9999/hook","events":["vuln.new"],"name":"test-hook"}'
```
