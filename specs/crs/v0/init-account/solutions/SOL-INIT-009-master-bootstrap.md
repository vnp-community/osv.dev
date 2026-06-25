# SOL-INIT-009 — Giải Pháp: Master Bootstrap Script

> **CR tham chiếu**: [CR-INIT-009](../CR-INIT-009-bootstrap-script.md)  
> **Kiến trúc cơ sở**: Tổng hợp toàn bộ `specs/01-architecture.md` và `specs/02-technical-design.md`

## Phân Tích Yêu Cầu

**Mục tiêu**: Sau `cp .env.bootstrap .env && ./scripts/bootstrap.sh`, người dùng chỉ cần build binaries và start services → có thể đăng nhập ngay.

**Phụ thuộc khởi động từ architecture**:
```
PostgreSQL → identity-service (schema auth) → Admin account
          → data-service     (schema vuln)
          → notification-svc (schema notif)
Redis     → identity-service (token cache)
          → search-service   (CPE browse cache)
          → apps/osv         (rate limiting)
MongoDB   → ranking-service  (cpe_rankings)
(OpenSearch → search-service — optional, fallback PG)
(NATS      → notification-service — optional, NATS_ENABLED=false)
```

## Files cần tạo

### [NEW] `scripts/bootstrap.sh`

```bash
#!/usr/bin/env bash
# =============================================================================
# OSV Platform — Master Bootstrap Script v1.0
# =============================================================================
# Kiến trúc: specs/01-architecture.md
# Thiết kế:  specs/02-technical-design.md
#
# Thứ tự khởi tạo:
#   0. Security validation (JWT_SECRET, admin password)
#   1. Infrastructure check (PostgreSQL, Redis, MongoDB, NATS)
#   2. identity-service  — schema auth, RSA keys, admin account
#   3. data-service      — schema vuln, pgvector, migrations
#   4. search-service    — OpenSearch index, Redis check
#   5. ranking-service   — MongoDB indexes
#   6. notification-svc  — schema notif, migrations
#   7. ai-service        — LLM backend validation
#   8. gateway-service   — upstream check
#   9. apps/osv          — JWKS verify, JWT_SECRET guard
#
# Usage:
#   ./scripts/bootstrap.sh              # Init tất cả
#   ./scripts/bootstrap.sh identity     # Chỉ init identity-service
#   FORCE_INSECURE=true ./scripts/bootstrap.sh
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# ── Terminal colors ────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'

info()    { echo -e "${BLUE}ℹ${NC}  $*"; }
ok()      { echo -e "${GREEN}✓${NC}  $*"; }
warn()    { echo -e "${YELLOW}⚠${NC}  $*"; }
fail()    { echo -e "${RED}✗${NC}  $*"; }
header()  { echo -e "\n${BOLD}${BLUE}══ $* ══${NC}"; }

# ── Global error/warning tracking ─────────────────────────────────────────
ERRORS=0; WARNINGS=0
err()  { fail  "$*"; ERRORS=$((ERRORS + 1)); }
warnc(){ warn  "$*"; WARNINGS=$((WARNINGS + 1)); }

# ── Load .env ─────────────────────────────────────────────────────────────
ENV_FILE="${PROJECT_ROOT}/.env"
if [[ ! -f "${ENV_FILE}" ]]; then
  fail ".env không tìm thấy tại: ${ENV_FILE}"
  echo ""
  echo "  Tạo từ template:"
  echo "    cp .env.bootstrap .env"
  echo "    # Chỉnh sửa .env với giá trị của bạn"
  exit 1
fi
set -o allexport; source "${ENV_FILE}"; set +o allexport
ok "Loaded .env từ ${ENV_FILE}"

# ── Parse args ────────────────────────────────────────────────────────────
TARGET="${1:-all}"
FORCE_INSECURE="${FORCE_INSECURE:-false}"
SKIP_INFRA="${SKIP_INFRA_CHECK:-false}"

# ── Helper: run a service init script ─────────────────────────────────────
run_init() {
  local svc="$1"
  local script="$2"
  
  # Filter: chạy tất cả hoặc chỉ service được chỉ định
  if [[ "${TARGET}" != "all" ]] && [[ "${TARGET}" != "${svc}" ]]; then
    return 0
  fi
  
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

# ═══════════════════════════════════════════════════════════════════════════
header "OSV Platform Bootstrap"
echo "  Project: ${PROJECT_ROOT}"
echo "  Target:  ${TARGET}"
echo "  Time:    $(date '+%Y-%m-%d %H:%M:%S %Z')"

# ═══ Step 0: Security Checks ═══════════════════════════════════════════════
header "Security Validation"

# JWT_SECRET check
# Spec §7.1: JWT phải được cấu hình đúng
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
    echo "  Hoặc bypass:"
    echo "    FORCE_INSECURE=true ./scripts/bootstrap.sh"
    exit 1
  fi
else
  ok "JWT_SECRET configured"
fi

# Admin password check
ADMIN_PW="${INIT_ADMIN_PASSWORD:-Admin@123!ChangeMe}"
if [[ "${ADMIN_PW}" == "Admin@123!ChangeMe" ]]; then
  warnc "INIT_ADMIN_PASSWORD là default — thay đổi trước production"
else
  ok "Admin password configured"
fi

# ═══ Step 1: Infrastructure Check ══════════════════════════════════════════
if [[ "${SKIP_INFRA}" == "false" ]]; then
  header "Infrastructure Check"
  
  # PostgreSQL — spec §4.1
  PG_DSN="${POSTGRES_DSN:-postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable}"
  if psql "${PG_DSN}" -c "SELECT 1;" &>/dev/null; then
    ok "PostgreSQL: connected"
  else
    err "PostgreSQL: NOT available — ${PG_DSN}"
    echo "  Khởi động PostgreSQL hoặc kiểm tra POSTGRES_DSN"
  fi
  
  # Redis — spec §4.3
  REDIS_HOST="${REDIS_HOST:-localhost}"
  REDIS_PORT="${REDIS_PORT:-6379}"
  REDIS_PASSWORD="${REDIS_PASSWORD:-}"
  _redis_opts="-h ${REDIS_HOST} -p ${REDIS_PORT} --no-auth-warning"
  [[ -n "${REDIS_PASSWORD}" ]] && _redis_opts="${_redis_opts} -a ${REDIS_PASSWORD}"
  if redis-cli ${_redis_opts} ping 2>/dev/null | grep -q "PONG"; then
    ok "Redis: connected (${REDIS_HOST}:${REDIS_PORT})"
  else
    warnc "Redis: not available — rate limiting và cache sẽ không hoạt động"
  fi
  
  # MongoDB — spec §3.11 ranking-service
  MONGO_URI="${MONGO_URI:-mongodb://localhost:27017}"
  MONGO_CMD=""
  command -v mongosh &>/dev/null && MONGO_CMD="mongosh"
  command -v mongo   &>/dev/null && [[ -z "${MONGO_CMD}" ]] && MONGO_CMD="mongo"
  if [[ -n "${MONGO_CMD}" ]] && \
     ${MONGO_CMD} --quiet --eval "db.runCommand({ping:1}).ok" "${MONGO_URI}" 2>/dev/null | grep -q "1"; then
    ok "MongoDB: connected (${MONGO_URI})"
  else
    warnc "MongoDB: not available — ranking-service sẽ fail"
  fi
  
  # NATS — spec §4.4 (optional — NATS_ENABLED controls requirement)
  NATS_URL="${NATS_URL:-nats://localhost:4222}"
  NATS_HOST="${NATS_URL#nats://}"
  NATS_HOST="${NATS_HOST%%/*}"
  if nc -z "${NATS_HOST%%:*}" "${NATS_HOST##*:}" 2>/dev/null; then
    ok "NATS: available (${NATS_URL})"
  else
    warnc "NATS: not available — event-driven features sẽ bị ảnh hưởng"
  fi
fi

# ═══ Steps 2-9: Service Init ════════════════════════════════════════════════
run_init "identity"     "${PROJECT_ROOT}/services/identity-service/scripts/init.sh"
run_init "data"         "${PROJECT_ROOT}/services/data-service/scripts/init.sh"
run_init "search"       "${PROJECT_ROOT}/services/search-service/scripts/init.sh"
run_init "ranking"      "${PROJECT_ROOT}/services/ranking-service/scripts/init.sh"
run_init "notification" "${PROJECT_ROOT}/services/notification-service/scripts/init.sh"
run_init "ai"           "${PROJECT_ROOT}/services/ai-service/scripts/init.sh"
run_init "gateway"      "${PROJECT_ROOT}/services/gateway-service/scripts/init.sh"
run_init "osv"          "${PROJECT_ROOT}/apps/osv/scripts/init.sh"

# ═══ Summary ════════════════════════════════════════════════════════════════
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

# ── Service endpoints table ────────────────────────────────────────────────
echo ""
printf '%s\n' "┌─────────────────────────────────────────────────────────────┐"
printf "│  %-55s │\n" "Service Endpoints (sau khi start binaries)"
printf '%s\n' "├─────────────────────────────────────────────────────────────┤"
printf "│  %-30s %-25s │\n" "identity-service (HTTP)"  "http://localhost:${IDENTITY_HTTP_PORT:-9101}"
printf "│  %-30s %-25s │\n" "identity-service (gRPC)"  "localhost:${IDENTITY_GRPC_PORT:-9001}"
printf "│  %-30s %-25s │\n" "JWKS endpoint"            "http://localhost:${IDENTITY_HTTP_PORT:-9101}/.well-known/jwks.json"
printf "│  %-30s %-25s │\n" "data-service (HTTP)"      "http://localhost:${DATA_HTTP_PORT:-8082}"
printf "│  %-30s %-25s │\n" "search-service (HTTP)"    "http://localhost:${SEARCH_HTTP_PORT:-8083}"
printf "│  %-30s %-25s │\n" "ranking-service"          "http://localhost:${RANKING_PORT:-8088}"
printf "│  %-30s %-25s │\n" "notification-service"     "http://localhost:${NOTIFICATION_HTTP_PORT:-8086}"
printf "│  %-30s %-25s │\n" "ai-service (gRPC)"        "localhost:${AI_GRPC_PORT:-50052}"
printf "│  %-30s %-25s │\n" "apps/osv (gateway)"       "http://localhost:${HTTP_PORT:-8080}"
printf '%s\n' "├─────────────────────────────────────────────────────────────┤"
printf "│  %-30s %-25s │\n" "Admin login"              "${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}"
printf '%s\n' "└─────────────────────────────────────────────────────────────┘"
echo ""
echo "Quick test sau khi start services:"
echo "  ./scripts/health-check.sh"
echo ""
echo "Login:"
echo "  curl -X POST http://localhost:${IDENTITY_HTTP_PORT:-9101}/auth/login \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"email\":\"${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}\",\"password\":\"...\"}'"

# Exit với error nếu có lỗi
[[ ${ERRORS} -eq 0 ]]
```

### [NEW] `scripts/health-check.sh`

```bash
#!/usr/bin/env bash
# =============================================================================
# OSV Platform — Health Check Script
# Kiểm tra tất cả services sau khi start
# =============================================================================
source "$(dirname "$0")/../.env" 2>/dev/null || true

echo "════════════════════════════════════════"
echo "  OSV Platform Health Check"
echo "  $(date '+%Y-%m-%d %H:%M:%S')"
echo "════════════════════════════════════════"

check_http() {
  local name="$1" url="$2"
  resp=$(curl -s --max-time 3 "${url}" 2>/dev/null || echo '{}')
  if echo "${resp}" | grep -q '"status"'; then
    status=$(echo "${resp}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status','?'))" 2>/dev/null || echo "?")
    echo "  ✓ ${name}: ${status}"
  else
    echo "  ✗ ${name}: not responding (${url})"
  fi
}

check_http "identity-service"     "http://localhost:${IDENTITY_HTTP_PORT:-9101}/health"
check_http "data-service"         "http://localhost:${DATA_HTTP_PORT:-8082}/health"
check_http "search-service"       "http://localhost:${SEARCH_HTTP_PORT:-8083}/health"
check_http "ranking-service"      "http://localhost:${RANKING_PORT:-8088}/health"
check_http "notification-service" "http://localhost:${NOTIFICATION_HTTP_PORT:-8086}/health"
check_http "apps/osv gateway"     "http://localhost:${HTTP_PORT:-8080}/health"

echo ""
echo "JWKS check:"
if curl -s --max-time 3 "http://localhost:${IDENTITY_HTTP_PORT:-9101}/.well-known/jwks.json" \
   2>/dev/null | grep -q '"keys"'; then
  echo "  ✓ JWKS: available"
else
  echo "  ✗ JWKS: not available"
fi

echo ""
echo "Admin login test:"
curl -s -X POST "http://localhost:${IDENTITY_HTTP_PORT:-9101}/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}\",\"password\":\"${INIT_ADMIN_PASSWORD:-Admin@123!ChangeMe}\"}" \
  | python3 -m json.tool 2>/dev/null || echo "  (login endpoint not ready)"
```

### [NEW] `scripts/start-all.sh`

```bash
#!/usr/bin/env bash
# =============================================================================
# OSV Platform — Start All Services
# =============================================================================
set -euo pipefail
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "${PROJECT_ROOT}/.env" 2>/dev/null || true

MODE="${1:---bg}"
LOG_DIR="${PROJECT_ROOT}/logs"
mkdir -p "${LOG_DIR}"

start_svc() {
  local name="$1" binary="$2" dir="$3"
  
  if [[ ! -f "${binary}" ]]; then
    echo "⚠ Binary not found: ${binary} (build với: make build-all)"
    return 0
  fi
  
  if [[ "${MODE}" == "--tmux" ]] && command -v tmux &>/dev/null; then
    tmux new-window -t osv: -n "${name}" \
      "cd ${dir} && ${binary}; echo 'Press Enter...'; read" 2>/dev/null || true
  else
    (cd "${dir}" && "${binary}" > "${LOG_DIR}/osv-${name}.log" 2>&1 &)
    echo "  Started ${name} (PID: $!) — Log: ${LOG_DIR}/osv-${name}.log"
  fi
}

echo "Starting OSV Platform services..."
echo "  Logs: ${LOG_DIR}/"
echo ""

# Theo đúng thứ tự phụ thuộc (spec: identity → data → search → ...)
start_svc "identity"     "${PROJECT_ROOT}/services/identity-service/server"     "${PROJECT_ROOT}/services/identity-service"
sleep 1
start_svc "data"         "${PROJECT_ROOT}/services/data-service/server"         "${PROJECT_ROOT}/services/data-service"
start_svc "search"       "${PROJECT_ROOT}/services/search-service/server"       "${PROJECT_ROOT}/services/search-service"
start_svc "ranking"      "${PROJECT_ROOT}/services/ranking-service/server"      "${PROJECT_ROOT}/services/ranking-service"
start_svc "notification" "${PROJECT_ROOT}/services/notification-service/server" "${PROJECT_ROOT}/services/notification-service"
start_svc "ai"           "${PROJECT_ROOT}/services/ai-service/server"           "${PROJECT_ROOT}/services/ai-service"
start_svc "gateway"      "${PROJECT_ROOT}/services/gateway-service/server"      "${PROJECT_ROOT}/services/gateway-service"
sleep 2
start_svc "osv-app"      "${PROJECT_ROOT}/apps/osv/server"                      "${PROJECT_ROOT}/apps/osv"

echo ""
echo "Waiting for services to start (5s)..."
sleep 5

echo ""
echo "Health check:"
"${PROJECT_ROOT}/scripts/health-check.sh"
```

## End-to-End Workflow

```bash
# ─────────────────────────────────────────────
# 1. Setup
# ─────────────────────────────────────────────
git clone <repo> && cd osv.dev

# Tạo .env (REQUIRED: chỉnh sửa JWT_SECRET, passwords)
cp .env.bootstrap .env
echo "JWT_SECRET=$(openssl rand -hex 32)" >> .env
echo "INIT_ADMIN_PASSWORD=YourSecurePass123!" >> .env

# ─────────────────────────────────────────────
# 2. Bootstrap (một lần duy nhất)
# ─────────────────────────────────────────────
./scripts/bootstrap.sh

# ─────────────────────────────────────────────
# 3. Build binaries
# ─────────────────────────────────────────────
make build-all
# Hoặc per-service: cd services/identity-service && go build -o server ./cmd/server

# ─────────────────────────────────────────────
# 4. Start services
# ─────────────────────────────────────────────
./scripts/start-all.sh
# Hoặc với tmux: ./scripts/start-all.sh --tmux

# ─────────────────────────────────────────────
# 5. Verify
# ─────────────────────────────────────────────
./scripts/health-check.sh

# ─────────────────────────────────────────────
# 6. Login
# ─────────────────────────────────────────────
TOKEN=$(curl -s -X POST http://localhost:9101/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@openvulnscan.io","password":"YourSecurePass123!"}' \
  | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('access_token',''))")

echo "Access token: ${TOKEN:0:50}..."

# ─────────────────────────────────────────────
# 7. Use API
# ─────────────────────────────────────────────
curl -H "Authorization: Bearer ${TOKEN}" \
     http://localhost:8080/api/v1/cves/search?q=log4j
```

## Acceptance Criteria

- [ ] `scripts/bootstrap.sh` tồn tại và executable
- [ ] `cp .env.bootstrap .env && ./scripts/bootstrap.sh` chạy end-to-end không lỗi
- [ ] Script fail nếu `JWT_SECRET` là default (bảo mật)
- [ ] `scripts/health-check.sh` confirm tất cả services healthy
- [ ] `scripts/start-all.sh` start tất cả binaries theo đúng thứ tự
- [ ] Summary table hiển thị tất cả endpoints
- [ ] Admin đăng nhập được ngay sau bootstrap

## Files Tóm Tắt

| File | Action |
|------|--------|
| `scripts/bootstrap.sh` | **[NEW]** |
| `scripts/health-check.sh` | **[NEW]** |
| `scripts/start-all.sh` | **[NEW]** |
