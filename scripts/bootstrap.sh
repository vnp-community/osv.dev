#!/usr/bin/env bash
# =============================================================================
# OSV Platform — Master Bootstrap Script v1.0
# =============================================================================
# Kiến trúc: specs/01-architecture.md
# Thiết kế:  specs/02-technical-design.md
#
# Thứ tự khởi tạo (theo dependency graph):
#   0. Security validation (JWT_SECRET, admin password)
#   1. Infrastructure check (PostgreSQL, Redis, MongoDB, NATS)
#   2. identity-service  — schema auth, RSA keys, admin account
#   3. data-service      — schema vuln, pgvector, migrations
#   4. search-service    — OpenSearch index, Redis check
#   5. ranking-service   — MongoDB connectivity
#   6. notification-svc  — schema notif, migrations
#   7. ai-service        — LLM backend validation
#   8. gateway-service   — JWT_SECRET, upstream check
#   9. apps/osv          — JWKS verify, upstream check
#
# Usage:
#   ./scripts/bootstrap.sh              # Init tất cả
#   ./scripts/bootstrap.sh identity     # Chỉ init identity-service
#   FORCE_INSECURE=true ./scripts/bootstrap.sh
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# ── Terminal colors ────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'

info()   { echo -e "${BLUE}ℹ${NC}  $*"; }
ok()     { echo -e "${GREEN}✓${NC}  $*"; }
warn()   { echo -e "${YELLOW}⚠${NC}  $*"; }
fail()   { echo -e "${RED}✗${NC}  $*"; }
header() { echo -e "\n${BOLD}${BLUE}══ $* ══${NC}"; }

ERRORS=0; WARNINGS=0
err()   { fail  "$*"; ERRORS=$((ERRORS + 1)); }
warnc() { warn  "$*"; WARNINGS=$((WARNINGS + 1)); }

# ── Load .env ──────────────────────────────────────────────────────────────────
ENV_FILE="${PROJECT_ROOT}/.env"
if [[ ! -f "${ENV_FILE}" ]]; then
  fail ".env không tìm thấy tại: ${ENV_FILE}"
  echo ""
  echo "  Tạo từ template:"
  echo "    cp .env.bootstrap .env"
  echo "    # Chỉnh sửa JWT_SECRET và INIT_ADMIN_PASSWORD"
  exit 1
fi
set -o allexport; source "${ENV_FILE}"; set +o allexport
ok "Loaded .env từ ${ENV_FILE}"

# ── Parse args ─────────────────────────────────────────────────────────────────
TARGET="${1:-all}"
FORCE_INSECURE="${FORCE_INSECURE:-false}"
SKIP_INFRA="${SKIP_INFRA_CHECK:-false}"

# ── Helper: run a service init script ─────────────────────────────────────────
run_init() {
  local svc="$1" script="$2"
  if [[ "${TARGET}" != "all" ]] && [[ "${TARGET}" != "${svc}" ]]; then return 0; fi
  header "Initializing ${svc}"
  if [[ ! -f "${script}" ]]; then
    warnc "Init script không có: ${script}"
    return 0
  fi
  if bash "${script}"; then
    ok "${svc} initialized"
  else
    err "${svc} init failed"
  fi
}

# ══════════════════════════════════════════════════════════════════════════════
header "OSV Platform Bootstrap"
echo "  Project: ${PROJECT_ROOT}"
echo "  Target:  ${TARGET}"
echo "  Time:    $(date '+%Y-%m-%d %H:%M:%S %Z')"

# ══ Step 0: Security Checks ═══════════════════════════════════════════════════
header "Security Validation"

JWT_SECRET="${JWT_SECRET:-}"
DEFAULT_SECRETS=("CHANGE_ME_use_openssl_rand_hex_32" "production-secret-key-change-me" "")
IS_DEFAULT=false
for ds in "${DEFAULT_SECRETS[@]}"; do
  [[ "${JWT_SECRET}" == "${ds}" ]] && IS_DEFAULT=true && break
done

if [[ "${IS_DEFAULT}" == "true" ]]; then
  if [[ "${FORCE_INSECURE}" == "true" ]]; then
    warnc "JWT_SECRET là default — INSECURE (FORCE_INSECURE=true)"
  else
    fail "JWT_SECRET là default hoặc trống!"
    echo ""
    echo "  Tạo secure secret:"
    echo "    echo JWT_SECRET=\$(openssl rand -hex 32) >> .env"
    echo ""
    echo "  Hoặc bypass (local dev only):"
    echo "    FORCE_INSECURE=true ./scripts/bootstrap.sh"
    exit 1
  fi
else
  ok "JWT_SECRET configured"
fi

ADMIN_PW="${INIT_ADMIN_PASSWORD:-Admin@123!ChangeMe}"
if [[ "${ADMIN_PW}" == "Admin@123!ChangeMe" ]]; then
  warnc "INIT_ADMIN_PASSWORD là default — thay đổi trước production"
else
  ok "Admin password configured"
fi

# ══ Step 1: Infrastructure Check ══════════════════════════════════════════════
if [[ "${SKIP_INFRA}" == "false" ]]; then
  header "Infrastructure Check"

  PG_DSN="${POSTGRES_DSN:-postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable}"
  if psql "${PG_DSN}" -c "SELECT 1;" &>/dev/null; then
    ok "PostgreSQL: connected"
  else
    err "PostgreSQL: NOT available — ${PG_DSN}"
    echo "  Khởi động PostgreSQL hoặc kiểm tra POSTGRES_DSN"
  fi

  REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"
  REDIS_HOST="${REDIS_ADDR%%:*}"
  REDIS_PORT_NUM="${REDIS_ADDR##*:}"
  _redis_opts="-h ${REDIS_HOST} -p ${REDIS_PORT_NUM} --no-auth-warning"
  [[ -n "${REDIS_PASSWORD:-}" ]] && _redis_opts="${_redis_opts} -a ${REDIS_PASSWORD}"
  if redis-cli ${_redis_opts} ping 2>/dev/null | grep -q "PONG"; then
    ok "Redis: connected (${REDIS_ADDR})"
  else
    warnc "Redis: not available — cache và rate-limiting sẽ không hoạt động"
  fi

  MONGO_URI="${MONGO_URI:-mongodb://localhost:27017}"
  MONGO_CMD=""; command -v mongosh &>/dev/null && MONGO_CMD="mongosh"
  command -v mongo &>/dev/null && [[ -z "${MONGO_CMD}" ]] && MONGO_CMD="mongo"
  if [[ -n "${MONGO_CMD}" ]] && \
     ${MONGO_CMD} --quiet --eval "db.runCommand({ping:1}).ok" "${MONGO_URI}" 2>/dev/null | grep -q "1"; then
    ok "MongoDB: connected (${MONGO_URI})"
  else
    warnc "MongoDB: not available — ranking-service sẽ fail"
  fi

  NATS_URL="${NATS_URL:-nats://localhost:4222}"
  NATS_HOST="${NATS_URL#nats://}"; NATS_HOST="${NATS_HOST%%/*}"
  if nc -z "${NATS_HOST%%:*}" "${NATS_HOST##*:}" 2>/dev/null; then
    ok "NATS: available (${NATS_URL})"
  else
    warnc "NATS: not available — NATS_ENABLED=false sẽ dùng graceful fallback"
  fi
fi

# ══ Steps 2-9: Service Init ════════════════════════════════════════════════════
run_init "identity"     "${PROJECT_ROOT}/services/identity-service/scripts/init.sh"
run_init "data"         "${PROJECT_ROOT}/services/data-service/scripts/init.sh"
run_init "search"       "${PROJECT_ROOT}/services/search-service/scripts/init.sh"
run_init "ranking"      "${PROJECT_ROOT}/services/ranking-service/scripts/init.sh"
run_init "notification" "${PROJECT_ROOT}/services/notification-service/scripts/init.sh"
run_init "ai"           "${PROJECT_ROOT}/services/ai-service/scripts/init.sh"
run_init "gateway"      "${PROJECT_ROOT}/services/gateway-service/scripts/init.sh"
run_init "osv"          "${PROJECT_ROOT}/apps/osv/scripts/init.sh"

# ══ Summary ════════════════════════════════════════════════════════════════════
header "Bootstrap Summary"

if [[ ${ERRORS} -eq 0 ]] && [[ ${WARNINGS} -eq 0 ]]; then
  ok "All services initialized successfully!"
elif [[ ${ERRORS} -eq 0 ]]; then
  ok "Bootstrap complete with ${WARNINGS} warning(s)"
  warn "Review warnings above trước production"
else
  fail "Bootstrap completed with ${ERRORS} error(s) and ${WARNINGS} warning(s)"
  echo ""
  echo "  Fix errors và chạy lại: ./scripts/bootstrap.sh"
fi

echo ""
printf '%s\n' "┌─────────────────────────────────────────────────────────────┐"
printf "│  %-55s │\n" "Service Endpoints"
printf '%s\n' "├─────────────────────────────────────────────────────────────┤"
printf "│  %-30s %-24s │\n" "identity-service HTTP"  ":${IDENTITY_HTTP_PORT:-9101}"
printf "│  %-30s %-24s │\n" "identity-service gRPC"  ":${IDENTITY_GRPC_PORT:-9001}"
printf "│  %-30s %-24s │\n" "JWKS endpoint"          ":${IDENTITY_HTTP_PORT:-9101}/.well-known/jwks.json"
printf "│  %-30s %-24s │\n" "data-service HTTP"      ":${DATA_HTTP_PORT:-8082}"
printf "│  %-30s %-24s │\n" "search-service HTTP"    ":${SEARCH_HTTP_PORT:-8083}"
printf "│  %-30s %-24s │\n" "ranking-service"        ":${RANKING_PORT:-8088}"
printf "│  %-30s %-24s │\n" "notification-service"   ":${NOTIFICATION_HTTP_PORT:-8086}"
printf "│  %-30s %-24s │\n" "ai-service gRPC"        ":${AI_GRPC_PORT:-50052}"
printf "│  %-30s %-24s │\n" "apps/osv gateway"       ":${HTTP_PORT:-8080}"
printf '%s\n' "├─────────────────────────────────────────────────────────────┤"
printf "│  %-30s %-24s │\n" "Admin login"            "${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}"
printf '%s\n' "└─────────────────────────────────────────────────────────────┘"
echo ""
echo "Tiếp theo: build binaries và start services"
echo "  make build-all   (hoặc: cd services/identity-service && go build -o server ./cmd/server)"
echo "  ./scripts/start-all.sh"
echo ""
echo "Test login:"
echo "  curl -X POST http://localhost:${IDENTITY_HTTP_PORT:-9101}/api/v1/auth/login \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"email\":\"${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}\",\"password\":\"...\"}'"

[[ ${ERRORS} -eq 0 ]]
