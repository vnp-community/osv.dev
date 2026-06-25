#!/usr/bin/env bash
# =============================================================================
# apps/osv — Bootstrap Script
# Spec: 01-architecture.md §2.1 — Unified Gateway Orchestrator
# Validates JWT_SECRET, checks JWKS endpoint, tests upstream connectivity
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
elif [[ -f "${APP_DIR}/.env" ]]; then
  set -o allexport; source "${APP_DIR}/.env"; set +o allexport
fi

JWT_SECRET="${JWT_SECRET:-}"
JWKS_URL="${JWKS_URL:-http://localhost:9101/.well-known/jwks.json}"
IDENTITY_HTTP="${IDENTITY_SERVICE_HTTP:-http://localhost:9101}"
HTTP_PORT="${HTTP_PORT:-8080}"
FORCE_INSECURE="${FORCE_INSECURE:-false}"

echo "══════════════════════════════════════════════════════════"
echo "  apps/osv Bootstrap"
echo "══════════════════════════════════════════════════════════"

# ── Step 1: JWT_SECRET security check ────────────────────────────────────────
echo "→ [1/3] Validating JWT_SECRET..."
# Code: apps/osv uses JWT_SECRET for gateway JWT validation
# Code: services/gateway-service/internal/auth/osv_middleware.go — AuthVerify()
DEFAULT_SECRETS=("CHANGE_ME_use_openssl_rand_hex_32" "production-secret-key-change-me" "")
IS_DEFAULT=false
for ds in "${DEFAULT_SECRETS[@]}"; do
  [[ "${JWT_SECRET}" == "${ds}" ]] && IS_DEFAULT=true && break
done

if [[ "${IS_DEFAULT}" == "true" ]]; then
  if [[ "${FORCE_INSECURE}" == "true" ]]; then
    echo "   ⚠ JWT_SECRET là default — INSECURE (FORCE_INSECURE=true)"
  else
    echo "   ✗ JWT_SECRET chưa được thay đổi!"
    echo ""
    echo "   Tạo secure secret:"
    echo "     echo JWT_SECRET=\$(openssl rand -hex 32) >> ${PROJECT_ROOT}/.env"
    echo ""
    echo "   Bypass (local dev only):"
    echo "     FORCE_INSECURE=true ./scripts/init.sh"
    exit 1
  fi
else
  echo "   ✓ JWT_SECRET configured"
fi

# ── Step 2: Identity service + JWKS check ────────────────────────────────────
echo "→ [2/3] Checking identity-service JWKS..."
# Code: services/gateway-service/internal/auth/osv_middleware.go
#       AuthVerify() validates JWT tokens using identity-service's RS256 public key
if curl -sf --max-time 5 "${JWKS_URL}" 2>/dev/null | python3 -c "
import sys, json
data = json.load(sys.stdin)
keys = data.get('keys', [])
if keys:
    print(f'   ✓ JWKS: {len(keys)} key(s) available at ${JWKS_URL}')
    sys.exit(0)
else:
    print('   ✗ JWKS: empty keys array')
    sys.exit(1)
" 2>/dev/null; then
  :  # success printed in python
else
  echo "   ⚠ JWKS not available at ${JWKS_URL}"
  echo "     identity-service phải start trước apps/osv"
fi

# ── Step 3: Upstream service checks ──────────────────────────────────────────
echo "→ [3/3] Checking upstream services..."
check_svc() {
  local name="$1" url="$2"
  if curl -sf --max-time 3 "${url}" >/dev/null 2>&1; then
    echo "   ✓ ${name}"
  else
    echo "   ⚠ ${name}: not available (start service first)"
  fi
}

check_svc "identity-service  :9101" "${IDENTITY_HTTP}/health"
check_svc "data-service      :8082" "${DATA_SERVICE_HTTP:-http://localhost:8082}/health"
check_svc "search-service    :8083" "${SEARCH_SERVICE_HTTP:-http://localhost:8083}/health"
check_svc "ranking-service   :8088" "${RANKING_SERVICE_HTTP:-http://localhost:8088}/health"
check_svc "notification-svc  :8086" "${NOTIFICATION_HTTP:-http://localhost:8086}/health"

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  apps/osv Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  Gateway: http://localhost:${HTTP_PORT}"
echo "  Login:   POST http://localhost:9101/api/v1/auth/login"
