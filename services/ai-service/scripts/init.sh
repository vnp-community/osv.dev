#!/usr/bin/env bash
# =============================================================================
# ai-service — Bootstrap Script
# Spec: 01-architecture.md §14.5 — LLM Provider Chain (Ollama → OpenAI → Azure)
# Code: internal/infra/ai/factory.go — FromEnv(), Validate()
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

AI_BACKEND="${AI_BACKEND:-ollama}"
AI_MODEL="${AI_MODEL:-llama3}"
AI_BASE_URL="${AI_BASE_URL:-http://localhost:11434}"
GRPC_PORT="${AI_GRPC_PORT:-${GRPC_PORT:-50052}}"

echo "══════════════════════════════════════════════════════════"
echo "  ai-service Bootstrap"
echo "══════════════════════════════════════════════════════════"
echo "  Backend: ${AI_BACKEND}"
echo "  Model:   ${AI_MODEL}"
echo "  gRPC:    :${GRPC_PORT}"

# ── Step 1: Validate AI backend config ────────────────────────────────────────
echo "→ [1/2] Validating AI backend config..."
# Code: internal/infra/ai/factory.go — Validate() checks required fields per backend

case "${AI_BACKEND}" in
  ollama)
    echo "   Backend: Ollama (local)"
    if curl -sf "${AI_BASE_URL}/api/tags" >/dev/null 2>&1; then
      echo "   ✓ Ollama: running at ${AI_BASE_URL}"
      # Kiểm tra model có sẵn không
      if curl -sf "${AI_BASE_URL}/api/tags" 2>/dev/null | python3 -c "
import sys, json
data = json.load(sys.stdin)
models = [m['name'].split(':')[0] for m in data.get('models', [])]
print('available:', ','.join(models))
ai_model = '${AI_MODEL}'.split(':')[0]
sys.exit(0 if ai_model in models else 1)
" 2>/dev/null; then
        echo "   ✓ Model '${AI_MODEL}' available"
      else
        echo "   ⚠ Model '${AI_MODEL}' not found locally"
        echo "   Pull it: ollama pull ${AI_MODEL}"
        echo "   ai-service sẽ fail khi cố dùng model này"
      fi
    else
      echo "   ⚠ Ollama: not running at ${AI_BASE_URL}"
      echo "   Start Ollama: ollama serve"
      echo "   ai-service sẽ fail khi nhận request AI"
    fi
    ;;
  openai)
    if [[ -z "${OPENAI_API_KEY:-}" ]]; then
      echo "   ✗ OPENAI_API_KEY is required for openai backend"
      exit 1
    fi
    echo "   ✓ OpenAI API key configured"
    ;;
  vertex)
    if [[ -z "${VERTEX_PROJECT_ID:-}" ]]; then
      echo "   ✗ VERTEX_PROJECT_ID is required for vertex backend"
      exit 1
    fi
    echo "   ✓ Vertex AI configured (project: ${VERTEX_PROJECT_ID})"
    ;;
  *)
    echo "   ✗ Unknown AI_BACKEND: '${AI_BACKEND}'"
    echo "   Valid values: ollama, openai, vertex"
    exit 1
    ;;
esac

# ── Step 2: Redis embedding cache check ───────────────────────────────────────
echo "→ [2/2] Checking Redis (embedding cache)..."
# Spec: 02-technical-design.md — Redis TTL=7 days for embedding cache
REDIS_HOST="${REDIS_ADDR:-localhost:6379}"
REDIS_HOSTNAME="${REDIS_HOST%%:*}"
REDIS_PORT_NUM="${REDIS_HOST##*:}"
_redis_opts="-h ${REDIS_HOSTNAME} -p ${REDIS_PORT_NUM} --no-auth-warning"
[[ -n "${REDIS_PASSWORD:-}" ]] && _redis_opts="${_redis_opts} -a ${REDIS_PASSWORD}"

if redis-cli ${_redis_opts} ping 2>/dev/null | grep -q "PONG"; then
  echo "   ✓ Redis: available — embedding cache enabled"
else
  echo "   ⚠ Redis: not available — embeddings will not be cached"
fi

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  ai-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  gRPC: :${GRPC_PORT}"
echo "  Test: grpc_health_probe -addr=:${GRPC_PORT}"
