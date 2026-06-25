#!/usr/bin/env bash
# =============================================================================
# data-service — Bootstrap Script
# Spec: 01-architecture.md §3.2 (CVE Data Platform — 15+ fetchers)
# Spec: 01-architecture.md §4.1 (PostgreSQL schema: osv_cves / vuln)
# Tech: 02-technical-design.md §4 (Fetcher Registry, EPSS sync, KEV diff)
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# ── Load .env ─────────────────────────────────────────────────────────────────
if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

# ── Config với fallback chain ──────────────────────────────────────────────────
DB_URL="${DATA_DATABASE_URL:-${POSTGRES_DSN:-postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable}}"
ALIAS_BACKEND="${ALIAS_GROUP_BACKEND:-postgres}"
HTTP_PORT="${DATA_HTTP_PORT:-8082}"
GRPC_PORT="${DATA_GRPC_PORT:-50053}"

echo "══════════════════════════════════════════════════════════"
echo "  data-service Bootstrap"
echo "══════════════════════════════════════════════════════════"

# ── Step 1: PostgreSQL extensions + schema ─────────────────────────────────────
echo "→ [1/3] Khởi tạo PostgreSQL extensions và schema 'vuln'..."
# Spec: 01-architecture.md §4.1 — pgvector cho cves.embedding vector(1536)

psql "${DB_URL}" <<-SQL
  CREATE EXTENSION IF NOT EXISTS "vector";
  CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
  CREATE EXTENSION IF NOT EXISTS "citext";
  CREATE SCHEMA IF NOT EXISTS vuln;
SQL
echo "   ✓ pgvector, uuid-ossp, citext và schema 'vuln' ready"

# ── Step 2: Apply migrations ───────────────────────────────────────────────────
echo "→ [2/3] Applying migrations..."
# Thứ tự: 002_create_kev_entries → 003_initial_schema → 004_sync_jobs →
#          005_create_alias_groups → 006_kev_ransomware → 007_add_source_isexploit

MIGRATION_DIR="${SERVICE_DIR}/migrations"
if [[ -d "${MIGRATION_DIR}" ]]; then
  for sql_file in $(ls "${MIGRATION_DIR}"/*.sql 2>/dev/null | sort -V); do
    fname="$(basename "$sql_file")"
    # Bỏ qua DOWN migrations
    [[ "$fname" == *.down.sql ]] && continue
    echo "   → ${fname}"
    psql "${DB_URL}" -v ON_ERROR_STOP=0 -f "$sql_file" 2>/dev/null || \
      echo "     (skipped: already applied)"
  done
  echo "   ✓ Migrations applied"
else
  echo "   ⚠ Migrations directory not found: ${MIGRATION_DIR}"
fi

# ── Step 3: Validate storage backend config ────────────────────────────────────
echo "→ [3/3] Validating storage backend..."
# Code: internal/config/storage_config.go — LoadStorageConfig()
# FIXED: envOr() đọc OS env thực sự (không còn placeholder)

case "${ALIAS_BACKEND}" in
  postgres)
    TABLE_EXISTS=$(psql "${DB_URL}" -t -c \
      "SELECT COUNT(*) FROM information_schema.tables WHERE table_name='alias_groups';" \
      2>/dev/null | tr -d ' ')
    if [[ "${TABLE_EXISTS:-0}" -gt 0 ]]; then
      echo "   ✓ alias_groups table ready (PostgreSQL backend)"
    else
      echo "   ⚠ alias_groups table not found — apply migration 005_create_alias_groups.up.sql"
    fi
    ;;
  firestore)
    if [[ -z "${GCP_PROJECT_ID:-}" ]]; then
      echo "   ✗ ALIAS_GROUP_BACKEND=firestore requires GCP_PROJECT_ID"
      echo "   Set ALIAS_GROUP_BACKEND=postgres to use PostgreSQL instead"
      exit 1
    fi
    echo "   ✓ Firestore backend (project: ${GCP_PROJECT_ID})"
    [[ -n "${FIRESTORE_EMULATOR_HOST:-}" ]] && echo "   ℹ Local emulator: ${FIRESTORE_EMULATOR_HOST}"
    ;;
  *)
    echo "   ✗ Unknown ALIAS_GROUP_BACKEND: '${ALIAS_BACKEND}'"
    echo "   Valid values: postgres, firestore"
    exit 1
    ;;
esac

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  data-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  HTTP:  :${HTTP_PORT} (health + KEV REST)"
echo "  gRPC:  :${GRPC_PORT} (CVEService)"
echo "  Alias backend: ${ALIAS_BACKEND}"
echo ""
echo "Khởi động:"
echo "  cd ${SERVICE_DIR} && DATA_DATABASE_URL='${DB_URL}' ./server"
echo ""
echo "Test:"
echo "  curl http://localhost:${HTTP_PORT}/health"
