#!/usr/bin/env bash
# =============================================================================
# ranking-service — Bootstrap Script
# Spec: 01-architecture.md §3.11 — CPE Popularity Ranking via MongoDB
# Note: MongoDB indexes are created by ensureIndexes() in main.go on startup.
#       This script validates connectivity and config only.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

MONGO_URI="${MONGO_URI:-mongodb://localhost:27017}"
MONGO_DB="${MONGO_DB:-cvedb}"
PORT="${RANKING_PORT:-${PORT:-8088}}"

echo "══════════════════════════════════════════════════════════"
echo "  ranking-service Bootstrap"
echo "══════════════════════════════════════════════════════════"

# ── MongoDB connectivity check ─────────────────────────────────────────────────
echo "→ [1/1] Checking MongoDB connectivity..."
MONGO_CMD=""
command -v mongosh &>/dev/null && MONGO_CMD="mongosh"
command -v mongo   &>/dev/null && [[ -z "${MONGO_CMD}" ]] && MONGO_CMD="mongo"

if [[ -n "${MONGO_CMD}" ]]; then
  if ${MONGO_CMD} --quiet --eval "db.runCommand({ping:1}).ok" "${MONGO_URI}" 2>/dev/null | grep -q "1"; then
    echo "   ✓ MongoDB: connected (${MONGO_URI})"
    echo "   ✓ Database: ${MONGO_DB}"
    echo "   ℹ Indexes will be created by ensureIndexes() on service startup"
    echo "     (ranking_cpe_unique, ranking_group — see cmd/server/main.go)"
  else
    echo "   ✗ MongoDB: connection failed at ${MONGO_URI}"
    echo "   ranking-service requires MongoDB — please start MongoDB first"
    exit 1
  fi
else
  echo "   ⚠ mongosh/mongo CLI not found — skipping connectivity check"
  echo "   Service will attempt connection on startup"
fi

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  ranking-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  HTTP: :${PORT}  (REST + /health)"
echo "  DB:   ${MONGO_DB} @ ${MONGO_URI}"
echo "  Test: curl http://localhost:${PORT}/health"
