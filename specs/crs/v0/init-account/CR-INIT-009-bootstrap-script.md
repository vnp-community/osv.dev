# CR-INIT-009 — Script Bootstrap Tổng hợp

## Mục tiêu

Tạo một script duy nhất `scripts/bootstrap.sh` tại root project để:
1. Đọc toàn bộ config từ `.env`
2. Khởi tạo tất cả services theo đúng thứ tự
3. Verify từng service sau khi init
4. Report tổng hợp kết quả

Sau khi chạy `./scripts/bootstrap.sh`, người dùng chỉ cần start các service binaries và có thể sử dụng ngay.

## Biến môi trường đặc biệt cho bootstrap

| Biến | Mô tả | Default |
|------|-------|---------|
| `SKIP_INFRA_CHECK` | Bỏ qua kiểm tra infrastructure | `false` |
| `FORCE_INSECURE` | Cho phép JWT_SECRET mặc định | `false` |
| `BOOTSTRAP_SERVICES` | Services cần init (comma-separated hoặc `all`) | `all` |
| `BOOTSTRAP_TIMEOUT` | Timeout chờ service (giây) | `30` |

## Các thay đổi cần thực hiện

### [NEW] `scripts/bootstrap.sh`

```bash
#!/usr/bin/env bash
# =============================================================================
# OSV.dev — Master Bootstrap Script
# =============================================================================
# Khởi tạo toàn bộ hệ thống từ .env:
#   - Apply database migrations
#   - Tạo JWT keys
#   - Tạo admin account
#   - Setup OpenSearch index
#   - Setup MongoDB indexes
#   - Validate tất cả cấu hình
#
# Usage:
#   ./scripts/bootstrap.sh              # Init tất cả services
#   ./scripts/bootstrap.sh identity     # Chỉ init identity-service
#   FORCE_INSECURE=true ./scripts/bootstrap.sh  # Bỏ qua security checks
#
# Requirements:
#   - PostgreSQL client (psql)
#   - Redis client (redis-cli) [optional]
#   - OpenSSL
#   - curl, jq [optional but recommended]
#   - MongoDB client (mongosh/mongo) [chỉ cần cho ranking-service]
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# ── Colors ────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info()    { echo -e "${BLUE}ℹ${NC}  $*"; }
success() { echo -e "${GREEN}✓${NC}  $*"; }
warning() { echo -e "${YELLOW}⚠${NC}  $*"; }
error()   { echo -e "${RED}✗${NC}  $*"; }
header()  { echo -e "\n${BLUE}══════════════════════════════════════════${NC}"; 
            echo -e "${BLUE}  $*${NC}";
            echo -e "${BLUE}══════════════════════════════════════════${NC}"; }

# ── Load .env ─────────────────────────────────────────────────────────────
ENV_FILE="${PROJECT_ROOT}/.env"
if [ ! -f "$ENV_FILE" ]; then
  error ".env file not found at: ${ENV_FILE}"
  echo ""
  echo "Create it from template:"
  echo "  cp .env.bootstrap .env"
  echo "  # Edit .env with your values"
  exit 1
fi

set -o allexport
source "$ENV_FILE"
set +o allexport
success "Loaded configuration from .env"

# ── Parse arguments ────────────────────────────────────────────────────────
TARGET_SERVICES="${1:-all}"
FORCE_INSECURE="${FORCE_INSECURE:-false}"
SKIP_INFRA_CHECK="${SKIP_INFRA_CHECK:-false}"

# ── Global state ──────────────────────────────────────────────────────────
ERRORS=0
WARNINGS=0

track_error()   { ERRORS=$((ERRORS + 1)); }
track_warning() { WARNINGS=$((WARNINGS + 1)); }

run_service_init() {
  local name="$1"
  local script="$2"
  
  if [ "$TARGET_SERVICES" != "all" ] && [ "$TARGET_SERVICES" != "$name" ]; then
    return 0
  fi
  
  if [ -f "$script" ]; then
    header "Initializing: ${name}"
    bash "$script" || {
      error "Init failed for ${name}"
      track_error
    }
  else
    warning "Init script not found: ${script} (skip)"
    track_warning
  fi
}

# ═══════════════════════════════════════════════════════════════════════════
header "OSV.dev Bootstrap v1.0"
echo "  Project: ${PROJECT_ROOT}"
echo "  Target:  ${TARGET_SERVICES}"
echo "  Time:    $(date '+%Y-%m-%d %H:%M:%S')"

# ── Step 0: Security check ────────────────────────────────────────────────
header "Security Validation"

JWT_SECRET="${JWT_SECRET:-production-secret-key-change-me}"
if [ "$JWT_SECRET" = "production-secret-key-change-me" ]; then
  if [ "$FORCE_INSECURE" = "true" ]; then
    warning "JWT_SECRET is default — INSECURE (FORCE_INSECURE=true)"
    track_warning
  else
    error "JWT_SECRET is using default value!"
    echo ""
    echo "  Generate a secure secret:"
    echo "    openssl rand -hex 32"
    echo ""
    echo "  Then add to .env:"
    echo "    JWT_SECRET=<generated value>"
    echo ""
    echo "  Or bypass for local dev:"
    echo "    FORCE_INSECURE=true ./scripts/bootstrap.sh"
    exit 1
  fi
else
  success "JWT_SECRET configured"
fi

INIT_ADMIN_PASSWORD="${INIT_ADMIN_PASSWORD:-Admin@123!ChangeMe}"
if [ "$INIT_ADMIN_PASSWORD" = "Admin@123!ChangeMe" ]; then
  warning "INIT_ADMIN_PASSWORD is using default — change before production"
  track_warning
else
  success "Admin password configured"
fi

# ── Step 1: Infrastructure check ──────────────────────────────────────────
if [ "$SKIP_INFRA_CHECK" = "false" ]; then
  header "Infrastructure Check"
  
  # PostgreSQL
  POSTGRES_DSN="${POSTGRES_DSN:-postgres://osv:osv_dev_secret_change_me@localhost:5432/osvdb?sslmode=disable}"
  if psql "${POSTGRES_DSN}" -c "SELECT 1;" &>/dev/null; then
    success "PostgreSQL: connected"
  else
    error "PostgreSQL: not available"
    echo "  DSN: ${POSTGRES_DSN}"
    echo "  Start PostgreSQL or check connection string"
    track_error
  fi
  
  # Redis
  REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"
  REDIS_HOST="${REDIS_ADDR%%:*}"
  REDIS_PORT="${REDIS_ADDR##*:}"
  if redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" ping 2>/dev/null | grep -q "PONG"; then
    success "Redis: connected at ${REDIS_ADDR}"
  else
    warning "Redis: not available at ${REDIS_ADDR} (some features may not work)"
    track_warning
  fi
  
  # MongoDB
  MONGO_URI="${MONGO_URI:-mongodb://localhost:27017}"
  if mongosh --quiet --eval "db.runCommand({ping:1}).ok" "$MONGO_URI" 2>/dev/null | grep -q "1" || \
     mongo --quiet --eval "db.runCommand({ping:1}).ok" "$MONGO_URI" 2>/dev/null | grep -q "1"; then
    success "MongoDB: connected at ${MONGO_URI}"
  else
    warning "MongoDB: not available (ranking-service will fail)"
    track_warning
  fi
  
  # NATS
  NATS_URL="${NATS_URL:-nats://localhost:4222}"
  NATS_HOST="${NATS_URL#nats://}"
  if nc -z "${NATS_HOST%%:*}" "${NATS_HOST##*:}" 2>/dev/null; then
    success "NATS: available at ${NATS_URL}"
  else
    warning "NATS: not available (notification events disabled)"
    track_warning
  fi
fi

# ── Step 2: Run service init scripts ──────────────────────────────────────
run_service_init "identity"     "${PROJECT_ROOT}/services/identity-service/scripts/init.sh"
run_service_init "data"         "${PROJECT_ROOT}/services/data-service/scripts/init.sh"
run_service_init "search"       "${PROJECT_ROOT}/services/search-service/scripts/init.sh"
run_service_init "ranking"      "${PROJECT_ROOT}/services/ranking-service/scripts/init.sh"
run_service_init "notification" "${PROJECT_ROOT}/services/notification-service/scripts/init.sh"
run_service_init "ai"           "${PROJECT_ROOT}/services/ai-service/scripts/init.sh"
run_service_init "gateway"      "${PROJECT_ROOT}/services/gateway-service/scripts/init.sh"
run_service_init "osv"          "${PROJECT_ROOT}/apps/osv/scripts/init.sh"

# ── Summary ────────────────────────────────────────────────────────────────
header "Bootstrap Summary"

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
  success "All services initialized successfully!"
elif [ $ERRORS -eq 0 ]; then
  success "Bootstrap completed with ${WARNINGS} warning(s)"
  warning "Review warnings above before production deployment"
else
  error "Bootstrap completed with ${ERRORS} error(s) and ${WARNINGS} warning(s)"
  echo ""
  echo "Fix the errors above and re-run: ./scripts/bootstrap.sh"
fi

echo ""
echo "┌─────────────────────────────────────────────────────────┐"
echo "│  Service Endpoints after starting                       │"
echo "├─────────────────────────────────────────────────────────┤"
printf "│  %-30s %-25s │\n" "identity-service (HTTP)" "http://localhost:${IDENTITY_HTTP_PORT:-9101}"
printf "│  %-30s %-25s │\n" "identity-service (gRPC)" "localhost:${IDENTITY_GRPC_PORT:-9001}"
printf "│  %-30s %-25s │\n" "data-service (HTTP)"     "http://localhost:${DATA_HTTP_PORT:-8080}"
printf "│  %-30s %-25s │\n" "search-service (HTTP)"   "http://localhost:${SEARCH_HTTP_PORT:-8082}"
printf "│  %-30s %-25s │\n" "ranking-service (HTTP)"  "http://localhost:${RANKING_PORT:-8088}"
printf "│  %-30s %-25s │\n" "notification-service"    "http://localhost:${NOTIFICATION_HTTP_PORT:-8086}"
printf "│  %-30s %-25s │\n" "gateway-service"         "http://localhost:${GATEWAY_HTTP_PORT:-8080}"
echo "├─────────────────────────────────────────────────────────┤"
printf "│  %-30s %-25s │\n" "Admin login endpoint"    "http://localhost:${IDENTITY_HTTP_PORT:-9101}/auth/login"
printf "│  %-30s %-25s │\n" "Admin email"             "${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}"
echo "└─────────────────────────────────────────────────────────┘"
echo ""
echo "Quick test:"
echo "  curl -X POST http://localhost:${IDENTITY_HTTP_PORT:-9101}/auth/login \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"email\":\"${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}\",\"password\":\"...\"}'"
echo ""

[ $ERRORS -eq 0 ]  # Exit with error code if there were errors
```

### [NEW] `scripts/start-all.sh`

Script tiện ích để start tất cả services sau khi bootstrap:

```bash
#!/usr/bin/env bash
# Start tất cả services (mỗi service trong tmux pane hoặc background)
# Usage: ./scripts/start-all.sh [--tmux | --bg]

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd .. && pwd)"
MODE="${1:---bg}"

source "${PROJECT_ROOT}/.env" 2>/dev/null || true

start_service() {
  local name="$1"
  local binary="$2"
  local dir="$3"
  
  if [ ! -f "$binary" ]; then
    echo "⚠ Binary not found: $binary (skipping $name)"
    return 0
  fi
  
  if [ "$MODE" = "--tmux" ] && command -v tmux &>/dev/null; then
    tmux new-window -t osv -n "$name" "cd $dir && $binary; read"
  else
    echo "Starting $name..."
    (cd "$dir" && "$binary" > "/tmp/osv-${name}.log" 2>&1 &)
    echo "  PID: $! — Log: /tmp/osv-${name}.log"
  fi
}

echo "Starting OSV.dev services..."

start_service "identity"     "${PROJECT_ROOT}/services/identity-service/server"     "${PROJECT_ROOT}/services/identity-service"
start_service "data"         "${PROJECT_ROOT}/services/data-service/server"         "${PROJECT_ROOT}/services/data-service"
start_service "search"       "${PROJECT_ROOT}/services/search-service/server"       "${PROJECT_ROOT}/services/search-service"
start_service "ranking"      "${PROJECT_ROOT}/services/ranking-service/server"      "${PROJECT_ROOT}/services/ranking-service"
start_service "notification" "${PROJECT_ROOT}/services/notification-service/server" "${PROJECT_ROOT}/services/notification-service"
start_service "ai"           "${PROJECT_ROOT}/services/ai-service/server"           "${PROJECT_ROOT}/services/ai-service"
start_service "gateway"      "${PROJECT_ROOT}/services/gateway-service/server"      "${PROJECT_ROOT}/services/gateway-service"
start_service "osv-app"      "${PROJECT_ROOT}/apps/osv/server"                      "${PROJECT_ROOT}/apps/osv"

echo ""
echo "All services starting. Check logs in /tmp/osv-*.log"
sleep 3

echo ""
echo "Health check:"
for svc in "identity:${IDENTITY_HTTP_PORT:-9101}" \
           "data:${DATA_HTTP_PORT:-8080}" \
           "search:${SEARCH_HTTP_PORT:-8082}" \
           "ranking:${RANKING_PORT:-8088}" \
           "notification:${NOTIFICATION_HTTP_PORT:-8086}"; do
  name="${svc%%:*}"
  port="${svc##*:}"
  status=$(curl -s --max-time 2 "http://localhost:${port}/health" 2>/dev/null | grep -c '"status"' || echo "0")
  if [ "$status" = "1" ]; then
    echo "  ✓ ${name} (port ${port}): healthy"
  else
    echo "  ⚠ ${name} (port ${port}): not responding"
  fi
done
```

### [NEW] `scripts/health-check.sh`

Script kiểm tra status sau khi start:

```bash
#!/usr/bin/env bash
# Kiểm tra tất cả services có hoạt động không

source "$(dirname "$0")/../.env" 2>/dev/null || true

check() {
  local name="$1"
  local url="$2"
  local response
  response=$(curl -s --max-time 3 "$url" 2>/dev/null || echo '{}')
  if echo "$response" | grep -q '"status"'; then
    echo "✓ ${name}: $(echo "$response" | grep -o '"status":"[^"]*"' | head -1)"
  else
    echo "✗ ${name}: not responding (${url})"
  fi
}

echo "=== OSV.dev Health Check ==="
check "identity-service"     "http://localhost:${IDENTITY_HTTP_PORT:-9101}/health"
check "data-service"         "http://localhost:${DATA_HTTP_PORT:-8080}/health"
check "search-service"       "http://localhost:${SEARCH_HTTP_PORT:-8082}/health"
check "ranking-service"      "http://localhost:${RANKING_PORT:-8088}/health"
check "notification-service" "http://localhost:${NOTIFICATION_HTTP_PORT:-8086}/health"
check "gateway-service"      "http://localhost:${GATEWAY_HTTP_PORT:-8080}/health"

echo ""
echo "Admin login test:"
curl -s -X POST "http://localhost:${IDENTITY_HTTP_PORT:-9101}/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}\",\"password\":\"${INIT_ADMIN_PASSWORD:-Admin@123!ChangeMe}\"}" \
  | python3 -m json.tool 2>/dev/null || echo "(login endpoint not ready)"
```

## Acceptance Criteria

- [ ] `scripts/bootstrap.sh` tồn tại, executable
- [ ] `cp .env.bootstrap .env && ./scripts/bootstrap.sh` chạy end-to-end không lỗi
- [ ] Script fail nếu `JWT_SECRET` là default (trừ khi `FORCE_INSECURE=true`)
- [ ] Script report rõ ràng mỗi service init thành công hay lỗi
- [ ] `scripts/health-check.sh` confirm tất cả services healthy sau khi start
- [ ] Admin có thể đăng nhập ngay: `POST /auth/login` trả về JWT tokens
- [ ] Summary table hiển thị tất cả endpoint addresses

## End-to-end workflow cho người dùng mới

```bash
# 1. Clone và setup
git clone <repo>
cd osv.dev

# 2. Tạo .env
cp .env.bootstrap .env
# Chỉnh sửa .env: đặt passwords, JWT_SECRET, etc.

# 3. Bootstrap (một lần duy nhất)
./scripts/bootstrap.sh

# 4. Build binaries (nếu chưa có)
make build-all  # hoặc go build cho từng service

# 5. Start services
./scripts/start-all.sh

# 6. Verify
./scripts/health-check.sh

# 7. Đăng nhập
curl -X POST http://localhost:9101/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@openvulnscan.io","password":"<your-password>"}'
```
