#!/bin/bash
set -e

echo "=================================================="
echo "    OSV FRONTEND DEPLOYMENT (to 103.67.184.32)    "
echo "=================================================="

# --- Configuration ---
# Update these default values if needed
SSH_USER="ubuntu"
BACKEND_SERVER_IP="172.20.2.48"
REMOTE_DEPLOY_DIR="/opt/osv-backend"
UI_SOURCE_DIR="../../ui"
UI_BUILD_DIR="${UI_SOURCE_DIR}/dist" # Note: NextJS uses out/ or .next/, React uses build/ or dist/
# ---------------------

# 1. Compile locally
echo "[1/3] Compiling frontend locally..."
cd "${UI_SOURCE_DIR}"
pnpm install --no-frozen-lockfile --ignore-scripts
pnpm run build
cd - > /dev/null

if [ ! -d "${UI_BUILD_DIR}" ]; then
  echo "Error: Build directory ${UI_BUILD_DIR} does not exist!"
  echo "Please check if your UI framework outputs to 'dist', 'build', or 'out'."
  exit 1
fi

# 2. Sync to backend server
echo "[2/3] Syncing to backend server ${BACKEND_SERVER_IP}..."
ssh "${SSH_USER}@${BACKEND_SERVER_IP}" "mkdir -p ${REMOTE_DEPLOY_DIR}/ui-dist"

# Sync files
rsync -avz --delete "${UI_BUILD_DIR}/" "${SSH_USER}@${BACKEND_SERVER_IP}:${REMOTE_DEPLOY_DIR}/ui-dist/"
rsync -avz --progress ui-nginx.conf docker-compose.server.yml "${SSH_USER}@${BACKEND_SERVER_IP}:${REMOTE_DEPLOY_DIR}/"

# 3. Restart frontend container
echo "[3/3] Restarting frontend container on server..."
ssh "${SSH_USER}@${BACKEND_SERVER_IP}" << EOF
  cd ${REMOTE_DEPLOY_DIR}
  docker compose -f docker-compose.server.yml up -d --force-recreate osv-frontend
EOF

echo "=================================================="
echo "    Frontend Deployment Completed Successfully!   "
echo "=================================================="
