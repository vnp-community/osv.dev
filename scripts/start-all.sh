#!/usr/bin/env bash
# =============================================================================
# OSV Platform — Start All Services
# Start binaries theo đúng thứ tự dependency
# =============================================================================
set -euo pipefail
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "${PROJECT_ROOT}/.env" 2>/dev/null || true

MODE="${1:---bg}"    # --bg (background) | --tmux
LOG_DIR="${PROJECT_ROOT}/logs"
mkdir -p "${LOG_DIR}"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'

start_svc() {
  local name="$1" binary="$2" dir="$3"
  if [[ ! -f "${binary}" ]]; then
    echo -e "  ${YELLOW}⚠${NC} ${name}: binary not found at ${binary}"
    echo "     Build: cd ${dir} && go build -o server ./cmd/server"
    return 0
  fi
  if [[ "${MODE}" == "--tmux" ]] && command -v tmux &>/dev/null; then
    tmux new-window -n "${name}" \
      "cd '${dir}' && '${binary}'; echo 'Press Enter...'; read" 2>/dev/null || true
    echo -e "  ${GREEN}✓${NC} ${name}: started in tmux window"
  else
    (cd "${dir}" && "${binary}" > "${LOG_DIR}/osv-${name}.log" 2>&1 &)
    echo -e "  ${GREEN}✓${NC} ${name}: started (PID: $!) — log: logs/osv-${name}.log"
  fi
}

echo "Starting OSV Platform services..."
echo "  Mode:    ${MODE}"
echo "  Logs:    ${LOG_DIR}/"
echo ""

# Thứ tự: identity phải start trước (các service khác dùng JWT validation)
echo "Starting identity-service (auth foundation)..."
start_svc "identity"     "${PROJECT_ROOT}/services/identity-service/server"     "${PROJECT_ROOT}/services/identity-service"
sleep 2   # Chờ identity-service sẵn sàng (migrations, RSA key load)

echo "Starting backend services..."
start_svc "data"         "${PROJECT_ROOT}/services/data-service/server"         "${PROJECT_ROOT}/services/data-service"
start_svc "search"       "${PROJECT_ROOT}/services/search-service/server"       "${PROJECT_ROOT}/services/search-service"
start_svc "ranking"      "${PROJECT_ROOT}/services/ranking-service/server"      "${PROJECT_ROOT}/services/ranking-service"
start_svc "notification" "${PROJECT_ROOT}/services/notification-service/server" "${PROJECT_ROOT}/services/notification-service"
start_svc "ai"           "${PROJECT_ROOT}/services/ai-service/server"           "${PROJECT_ROOT}/services/ai-service"
start_svc "gateway"      "${PROJECT_ROOT}/services/gateway-service/server"      "${PROJECT_ROOT}/services/gateway-service"

sleep 2   # Chờ backend services ready

echo "Starting apps/osv gateway..."
start_svc "osv-app"      "${PROJECT_ROOT}/apps/osv/server"                      "${PROJECT_ROOT}/apps/osv"

echo ""
echo "Waiting for services to start (5s)..."
sleep 5

echo ""
echo "Running health check..."
"${PROJECT_ROOT}/scripts/health-check.sh" || true
