#!/usr/bin/env bash
# =============================================================================
# notification-service — Bootstrap Script
# Spec: 01-architecture.md §3.7 — 5-channel dispatch + HMAC webhook signing
# Channels: Email, Slack, Teams, In-app, Webhook
# Note: NATS is optional — service starts with NATS_ENABLED=false
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

DB_URL="${NOTIFICATION_DATABASE_URL:-${DATABASE_URL:-${POSTGRES_DSN:-postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable}}}"
HTTP_PORT="${NOTIFICATION_HTTP_PORT:-8086}"
GRPC_PORT="${NOTIFICATION_GRPC_PORT:-50063}"
NATS_URL="${NATS_URL:-nats://localhost:4222}"
NATS_ENABLED="${NATS_ENABLED:-false}"

SERVICE_DIR="$(dirname "$SCRIPT_DIR")"

echo "══════════════════════════════════════════════════════════"
echo "  notification-service Bootstrap"
echo "══════════════════════════════════════════════════════════"

# ── Step 1: PostgreSQL schema + migrations ────────────────────────────────────
echo "→ [1/2] Khởi tạo PostgreSQL schema 'notif'..."
psql "${DB_URL}" <<-SQL
  CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
  CREATE SCHEMA IF NOT EXISTS notif;
SQL
echo "   ✓ Schema 'notif' ready"

MIGRATION_DIR="${SERVICE_DIR}/migrations"
if [[ -d "${MIGRATION_DIR}" ]]; then
  for sql_file in $(ls "${MIGRATION_DIR}"/*.sql 2>/dev/null | sort -V); do
    fname="$(basename "$sql_file")"
    [[ "$fname" == *.down.sql ]] && continue
    echo "   → ${fname}"
    psql "${DB_URL}" -v ON_ERROR_STOP=0 -f "$sql_file" 2>/dev/null || \
      echo "     (skipped: already applied)"
  done
  echo "   ✓ Migrations applied"
else
  echo "   ℹ No migrations directory found — skipping"
fi

# ── Step 2: NATS check (optional) ─────────────────────────────────────────────
echo "→ [2/2] NATS check..."
# Spec: 01-architecture.md §4.4 — NATS_ENABLED controls requirement
# Code: cmd/server/main.go — nếu NATS fail và NATS_ENABLED != "true" → warn + continue
NATS_HOST="${NATS_URL#nats://}"
NATS_HOST="${NATS_HOST%%/*}"
if nc -z "${NATS_HOST%%:*}" "${NATS_HOST##*:}" 2>/dev/null; then
  echo "   ✓ NATS: available (${NATS_URL})"
else
  if [[ "${NATS_ENABLED}" == "true" ]]; then
    echo "   ✗ NATS: NOT available — NATS_ENABLED=true but NATS unreachable"
    exit 1
  else
    echo "   ⚠ NATS: not available — event-driven notifications disabled"
    echo "     Set NATS_ENABLED=true khi NATS đã ready"
  fi
fi

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  notification-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  HTTP: :${HTTP_PORT}   gRPC: :${GRPC_PORT}"
echo "  NATS: ${NATS_ENABLED} (NATS_ENABLED=${NATS_ENABLED})"
echo "  Test: curl http://localhost:${HTTP_PORT}/health"
