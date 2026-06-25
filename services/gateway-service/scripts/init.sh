#!/usr/bin/env bash
# =============================================================================
# gateway-service — Bootstrap Script
# Spec: 01-architecture.md §3.1 — Reverse Proxy: auth + rate-limit + routing
# Note: gateway-service không có DB riêng — validate upstreams và JWT config
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

IDENTITY_HTTP="${IDENTITY_SERVICE_HTTP:-http://localhost:9101}"
JWT_SECRET="${JWT_SECRET:-}"

echo "══════════════════════════════════════════════════════════"
echo "  gateway-service Bootstrap"
echo "══════════════════════════════════════════════════════════"

# ── JWT_SECRET validation ─────────────────────────────────────────────────────
echo "→ [1/2] Validating JWT_SECRET..."
# Code: internal/auth/osv_middleware.go — AuthVerify(secret, ...)
FORCE_INSECURE="${FORCE_INSECURE:-false}"
DEFAULT_SECRETS=("CHANGE_ME_use_openssl_rand_hex_32" "production-secret-key-change-me" "")
IS_DEFAULT=false
for ds in "${DEFAULT_SECRETS[@]}"; do
  [[ "${JWT_SECRET}" == "${ds}" ]] && IS_DEFAULT=true && break
done

if [[ "${IS_DEFAULT}" == "true" ]]; then
  if [[ "${FORCE_INSECURE}" == "true" ]]; then
    echo "   ⚠ JWT_SECRET là default — INSECURE mode (FORCE_INSECURE=true)"
  else
    echo "   ✗ JWT_SECRET là default hoặc trống!"
    echo "   Tạo secure secret:"
    echo "     echo JWT_SECRET=\$(openssl rand -hex 32) >> .env"
    exit 1
  fi
else
  echo "   ✓ JWT_SECRET configured"
fi

# ── Upstream services check ───────────────────────────────────────────────────
echo "→ [2/2] Checking upstream services..."
check_http() {
  local name="$1" url="$2"
  if curl -sf --max-time 3 "${url}" >/dev/null 2>&1; then
    echo "   ✓ ${name}: ${url}"
  else
    echo "   ⚠ ${name}: not available (${url}) — proxy routes will fail until service starts"
  fi
}

check_http "identity-service" "${IDENTITY_HTTP}/health"
check_http "data-service"     "${DATA_SERVICE_HTTP:-http://localhost:8082}/health"

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  gateway-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  Upstreams configured via EmbeddedConfig (embedded.go)"
echo "  JWT validation: AuthVerify() in internal/auth/osv_middleware.go"
