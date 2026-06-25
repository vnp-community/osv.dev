#!/bin/bash
set -e

echo "=================================================="
echo "    OSV BACKEND DEPLOYMENT (to 172.20.2.48)       "
echo "=================================================="

# --- Configuration ---
SSH_USER="ubuntu"
BACKEND_SERVER_IP="172.20.2.48"
REMOTE_DEPLOY_DIR="/opt/osv-backend"
# ---------------------

# 1. Compile backend locally (Linux/amd64)
echo "[1/4] Compiling backend binary locally (Linux/amd64)..."
cd ../../apps/osv
export PATH=/usr/local/go/bin:$PATH
go mod tidy 2>/dev/null || true
GOOS=linux GOARCH=amd64 go build -o ../../deploy/dev/osv-server ./cmd/osv/
cd ../../deploy/dev
echo "    Binary: $(ls -lh osv-server | awk '{print $5, $9}')"

# 2. Create bin/ dir on server and sync files
echo "[2/4] Syncing binary and docker-compose to ${BACKEND_SERVER_IP}..."
ssh "${SSH_USER}@${BACKEND_SERVER_IP}" "mkdir -p ${REMOTE_DEPLOY_DIR}/secrets"
rsync -avz --progress \
    osv-server \
    docker-compose.server.yml \
    .env \
    "${SSH_USER}@${BACKEND_SERVER_IP}:${REMOTE_DEPLOY_DIR}/"

# 3. Check server memory before deploy
echo "[3/4] Checking server memory..."
ssh "${SSH_USER}@${BACKEND_SERVER_IP}" "free -h && echo '---' && df -h /"

# 4. Restart backend container on remote server
echo "[4/4] Restarting backend container on remote server..."
ssh "${SSH_USER}@${BACKEND_SERVER_IP}" <<EOF
  set -e
  cd ${REMOTE_DEPLOY_DIR}
  chmod +x osv-server
  
  # Stop existing containers gracefully
  docker compose -f docker-compose.server.yml down --remove-orphans 2>/dev/null || true
  
  # Pull only lightweight infra images (skip opensearch pull — large)
  docker compose -f docker-compose.server.yml pull postgres mongodb redis nats 2>/dev/null || true
  
  # Start all services
  docker compose -f docker-compose.server.yml up -d --force-recreate

  # Wait for osv-server to be healthy (max 120s)
  echo "Waiting for osv-server health..."
  for i in \$(seq 1 24); do
    if curl -sf http://localhost:8080/health | grep -q ok; then
      echo "osv-server is healthy!"
      break
    fi
    echo "  [\$i/24] waiting..."
    sleep 5
  done

  # Final status
  docker compose -f docker-compose.server.yml ps
EOF

echo "=================================================="
echo "    Backend Deployment Completed Successfully!    "
echo "=================================================="
echo ""
echo "Post-deploy checks:"
echo "  Health:    curl http://172.20.2.48:8080/health"
echo "  Data:      curl http://172.20.2.48:8082/health"
echo "  Logs:      ssh ${SSH_USER}@${BACKEND_SERVER_IP} 'cd ${REMOTE_DEPLOY_DIR} && docker compose -f docker-compose.server.yml logs -f --tail=50 osv-server'"
