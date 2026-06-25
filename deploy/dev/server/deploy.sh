#!/usr/bin/env bash
# =============================================================
# OSV Platform — Deploy script
# Local: compile → rsync binary → restart container on server
#
# Usage:
#   ./deploy/server/deploy.sh                   # full deploy
#   ./deploy/server/deploy.sh --build-only      # only compile
#   ./deploy/server/deploy.sh --sync-only       # only rsync + restart
#   ./deploy/server/deploy.sh --nginx-only      # only push nginx config
# =============================================================

set -euo pipefail

# ---- Config ----
DEPLOY_SERVER="172.20.2.48"
DEPLOY_USER="${DEPLOY_USER:-ubuntu}"            # override via env
DEPLOY_DIR="/opt/osv"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_ed25519}"     # override via env
NGINX_SERVER="172.20.2.16"
NGINX_USER="${NGINX_USER:-ubuntu}"
NGINX_CONF_DIR="/etc/nginx/conf.d"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BUILD_DIR="${REPO_ROOT}/dist"
BINARY_NAME="osv-server"

# ---- Colors ----
CYAN='\033[36m'
GREEN='\033[32m'
YELLOW='\033[33m'
RED='\033[31m'
RESET='\033[0m'

log()  { echo -e "${CYAN}[deploy]${RESET} $*"; }
ok()   { echo -e "${GREEN}[ok]${RESET} $*"; }
warn() { echo -e "${YELLOW}[warn]${RESET} $*"; }
die()  { echo -e "${RED}[error]${RESET} $*" >&2; exit 1; }

# ---- Parse args ----
BUILD_ONLY=false
SYNC_ONLY=false
NGINX_ONLY=false

for arg in "$@"; do
  case "$arg" in
    --build-only)  BUILD_ONLY=true  ;;
    --sync-only)   SYNC_ONLY=true   ;;
    --nginx-only)  NGINX_ONLY=true  ;;
    *) die "Unknown argument: $arg" ;;
  esac
done

# ============================================================
# Step 1: Compile binary for Linux/amd64
# ============================================================
build() {
  log "Compiling osv-server for linux/amd64..."
  mkdir -p "${BUILD_DIR}"

  VERSION=$(git -C "${REPO_ROOT}" describe --tags --always --dirty 2>/dev/null || echo "dev")
  COMMIT=$(git  -C "${REPO_ROOT}" rev-parse --short HEAD 2>/dev/null || echo "unknown")
  BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}"

  (
    cd "${REPO_ROOT}/apps/osv"
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
      go build \
        -trimpath \
        -ldflags "${LDFLAGS}" \
        -o "${BUILD_DIR}/${BINARY_NAME}" \
        ./cmd/server/
  )

  ok "Binary built: ${BUILD_DIR}/${BINARY_NAME} ($(du -sh "${BUILD_DIR}/${BINARY_NAME}" | cut -f1))"
}

# ============================================================
# Step 2: Rsync binary + compose files to 172.20.2.48
# ============================================================
sync_to_server() {
  local ssh_opts="-i ${SSH_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=10"

  log "Syncing to ${DEPLOY_USER}@${DEPLOY_SERVER}:${DEPLOY_DIR} ..."

  # Create remote directory structure
  ssh ${ssh_opts} "${DEPLOY_USER}@${DEPLOY_SERVER}" "mkdir -p ${DEPLOY_DIR}/bin"

  # Sync binary
  rsync -avz --checksum \
    -e "ssh ${ssh_opts}" \
    "${BUILD_DIR}/${BINARY_NAME}" \
    "${DEPLOY_USER}@${DEPLOY_SERVER}:${DEPLOY_DIR}/bin/${BINARY_NAME}"

  # Sync docker-compose and env example (do NOT overwrite .env if exists)
  rsync -avz --checksum \
    -e "ssh ${ssh_opts}" \
    "${REPO_ROOT}/deploy/server/docker-compose.yml" \
    "${DEPLOY_USER}@${DEPLOY_SERVER}:${DEPLOY_DIR}/docker-compose.yml"

  rsync -avz --checksum --ignore-existing \
    -e "ssh ${ssh_opts}" \
    "${REPO_ROOT}/deploy/server/.env.example" \
    "${DEPLOY_USER}@${DEPLOY_SERVER}:${DEPLOY_DIR}/.env.example"

  ok "Files synced to server."
}

# ============================================================
# Step 3: Restart container on server
# ============================================================
restart_server() {
  local ssh_opts="-i ${SSH_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=10"

  log "Restarting osv-server on ${DEPLOY_SERVER}..."
  ssh ${ssh_opts} "${DEPLOY_USER}@${DEPLOY_SERVER}" bash <<'REMOTE'
set -euo pipefail
cd /opt/osv

# Ensure .env exists
if [ ! -f .env ]; then
  echo "[warn] .env not found — copying from .env.example. Edit it before proceeding!"
  cp .env.example .env
fi

# Pull infra images (no-op if up to date)
docker compose pull --ignore-buildable 2>/dev/null || true

# Recreate only osv-server (infra stays running)
docker compose up -d --no-build --force-recreate osv-server

echo "[ok] osv-server restarted."
docker compose ps osv-server
REMOTE
  ok "Server restart complete."
}

# ============================================================
# Step 4: Push nginx config to 172.20.2.16 and reload
# ============================================================
push_nginx() {
  local ssh_opts="-i ${SSH_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=10"

  log "Deploying nginx config to ${NGINX_USER}@${NGINX_SERVER}..."

  rsync -avz --checksum \
    -e "ssh ${ssh_opts}" \
    "${REPO_ROOT}/deploy/nginx/c12.openledger.vn.conf" \
    "${NGINX_USER}@${NGINX_SERVER}:~/c12.openledger.vn.conf"

  # Move to nginx conf.d (may need sudo) and reload
  ssh ${ssh_opts} "${NGINX_USER}@${NGINX_SERVER}" bash <<'REMOTE'
set -euo pipefail
sudo cp ~/c12.openledger.vn.conf /etc/nginx/conf.d/c12.openledger.vn.conf
# Test and reload nginx via docker
if docker inspect nginx >/dev/null 2>&1; then
  docker exec nginx nginx -t && docker exec nginx nginx -s reload
  echo "[ok] nginx reloaded."
else
  echo "[warn] nginx container not found — reload manually."
fi
REMOTE
  ok "Nginx config deployed."
}

# ============================================================
# Main
# ============================================================
main() {
  if $NGINX_ONLY; then
    push_nginx
    exit 0
  fi

  if ! $SYNC_ONLY; then
    build
  fi

  if ! $BUILD_ONLY; then
    sync_to_server
    restart_server
  fi

  log "---"
  ok "Deploy complete! https://c12.openledger.vn"
}

main "$@"
