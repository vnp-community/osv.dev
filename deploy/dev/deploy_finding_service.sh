#!/usr/bin/env bash
# =============================================================
# OSV Platform — Deploy finding-service binary to 172.20.2.48
# Usage:
#   ./deploy_finding_service.sh              # build + sync + run migration + restart
#   ./deploy_finding_service.sh --build-only # chỉ compile
#   ./deploy_finding_service.sh --sync-only  # chỉ rsync + restart (dùng binary cũ đã có)
#   ./deploy_finding_service.sh --migrate-only # chỉ chạy migration
# =============================================================

set -euo pipefail

# ---- Config ----
DEPLOY_SERVER="172.20.2.48"
DEPLOY_USER="${DEPLOY_USER:-ubuntu}"
DEPLOY_DIR="/opt/osv-backend"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_ed25519}"
SSH_OPTS="-i ${SSH_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=10"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BINARY_SRC="${REPO_ROOT}/deploy/dev/finding-server"

# ---- Colors ----
CYAN='\033[36m'; GREEN='\033[32m'; YELLOW='\033[33m'; RED='\033[31m'; RESET='\033[0m'
log()  { echo -e "${CYAN}[finding-deploy]${RESET} $*"; }
ok()   { echo -e "${GREEN}[ok]${RESET} $*"; }
warn() { echo -e "${YELLOW}[warn]${RESET} $*"; }
die()  { echo -e "${RED}[error]${RESET} $*" >&2; exit 1; }

# ---- Parse args ----
BUILD_ONLY=false; SYNC_ONLY=false; MIGRATE_ONLY=false
for arg in "$@"; do
  case "$arg" in
    --build-only)   BUILD_ONLY=true   ;;
    --sync-only)    SYNC_ONLY=true    ;;
    --migrate-only) MIGRATE_ONLY=true ;;
    *) die "Unknown argument: $arg" ;;
  esac
done

# ============================================================
# Step 1: Compile finding-service binary for Linux/amd64
# ============================================================
build() {
  log "Compiling finding-service for linux/amd64..."
  (
    cd "${REPO_ROOT}/services/finding-service"
    export PATH=/usr/local/go/bin:$PATH
    go mod tidy 2>/dev/null || true
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
      go build \
        -trimpath \
        -ldflags="-s -w" \
        -o "${BINARY_SRC}" \
        ./cmd/server/
  )
  ok "Binary built: ${BINARY_SRC} ($(du -sh "${BINARY_SRC}" | cut -f1))"
}

# ============================================================
# Step 2: Rsync binary + compose file + migration to server
# ============================================================
sync_to_server() {
  log "Syncing finding-server binary + compose file to ${DEPLOY_USER}@${DEPLOY_SERVER}:${DEPLOY_DIR}..."

  # Sync binary
  rsync -avz --checksum \
    -e "ssh ${SSH_OPTS}" \
    "${BINARY_SRC}" \
    "${DEPLOY_USER}@${DEPLOY_SERVER}:${DEPLOY_DIR}/finding-server"

  # Sync updated docker-compose.server.yml
  rsync -avz --checksum \
    -e "ssh ${SSH_OPTS}" \
    "${REPO_ROOT}/deploy/dev/docker-compose.server.yml" \
    "${DEPLOY_USER}@${DEPLOY_SERVER}:${DEPLOY_DIR}/docker-compose.server.yml"

  # Sync migration file
  rsync -avz --checksum \
    -e "ssh ${SSH_OPTS}" \
    "${REPO_ROOT}/services/finding-service/migrations/015_fix_created_by_assigned_to.sql" \
    "${DEPLOY_USER}@${DEPLOY_SERVER}:${DEPLOY_DIR}/015_fix_created_by_assigned_to.sql"

  ok "Files synced."
}

# ============================================================
# Step 3: Run migration 015 on server PostgreSQL
# ============================================================
run_migration() {
  log "Running migration 015 on ${DEPLOY_SERVER}..."
  ssh ${SSH_OPTS} "${DEPLOY_USER}@${DEPLOY_SERVER}" bash <<'REMOTE'
set -euo pipefail
cd /opt/osv-backend

# Load DB password from .env
source .env 2>/dev/null || true
DB="${POSTGRES_DB:-osvdb}"
PW="${POSTGRES_PASSWORD}"

echo "[migration] Running 015_fix_created_by_assigned_to.sql..."
docker compose -f docker-compose.server.yml exec -T postgres \
  psql -U osv -d "$DB" -f - < 015_fix_created_by_assigned_to.sql

echo "[migration] Done."
REMOTE
  ok "Migration 015 applied."
}

# ============================================================
# Step 4: Restart finding-service container on server
# ============================================================
restart_service() {
  log "Restarting finding-service on ${DEPLOY_SERVER}..."
  ssh ${SSH_OPTS} "${DEPLOY_USER}@${DEPLOY_SERVER}" bash <<'REMOTE'
set -euo pipefail
cd /opt/osv-backend

# chmod binary
chmod +x finding-server

# Start/restart finding-service only (infra stays running)
docker compose -f docker-compose.server.yml up -d --force-recreate finding-service

# Wait for health (max 60s)
echo "Waiting for finding-service on port 8085..."
for i in $(seq 1 12); do
  if curl -sf http://172.20.2.48:8085/health | grep -q '"status":"ok"'; then
    echo "  [ok] finding-service is up!"
    break
  fi
  echo "  [$i/12] waiting..."
  sleep 5
done

docker compose -f docker-compose.server.yml ps finding-service
REMOTE
  ok "finding-service restarted."
}

# ============================================================
# Main
# ============================================================
main() {
  echo "=================================================="
  echo "  OSV: Deploy finding-service → ${DEPLOY_SERVER}"
  echo "=================================================="

  if $MIGRATE_ONLY; then
    run_migration
    exit 0
  fi

  if ! $SYNC_ONLY; then
    build
  fi

  if ! $BUILD_ONLY; then
    sync_to_server
    run_migration
    restart_service
  fi

  echo "=================================================="
  ok "finding-service deployed!"
  echo ""
  echo "Post-deploy checks:"
  echo "  Health:  curl http://${DEPLOY_SERVER}:8085/health"
  echo "  Logs:    ssh ${DEPLOY_USER}@${DEPLOY_SERVER} 'cd ${DEPLOY_DIR} && docker compose -f docker-compose.server.yml logs -f finding-service --tail=50'"
  echo "  Verify:  curl -s https://c12.openledger.vn/api/v1/findings?status\\[\\]=active"
}

main "$@"
