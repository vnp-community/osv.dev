# CR-INIT-008 — Khởi tạo gateway-service & apps/osv

## Mục tiêu

Sau khi chạy init:
1. **gateway-service** (services/gateway-service): API gateway có đúng upstream addresses
2. **apps/osv**: OSV orchestrator app có đúng config để kết nối tất cả services
3. Cả hai expose `/health` endpoint hoạt động đầy đủ

## A. gateway-service

### Biến môi trường (đọc từ `.env`)

| Biến | Mô tả | Default |
|------|-------|---------|
| `GATEWAY_HTTP_PORT` | HTTP port | `8080` |
| `GATEWAY_GRPC_PORT` | gRPC proxy port | `9090` |
| `DATA_SERVICE_HTTP` | data-service REST URL | `http://localhost:8080` |
| `SEARCH_SERVICE_HTTP` | search-service REST URL | `http://localhost:8082` |
| `NOTIFICATION_SERVICE_HTTP` | notification-service URL | `http://localhost:8086` |
| `ALLOWED_ORIGINS` | CORS origins | `http://localhost:3001,http://localhost:5173` |

### [NEW] `services/gateway-service/scripts/init.sh`

```bash
#!/usr/bin/env bash
# gateway-service bootstrap script
# Validate upstream service addresses từ .env

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load .env
if [ -f "${SCRIPT_DIR}/../../../.env" ]; then
  set -o allexport
  source "${SCRIPT_DIR}/../../../.env"
  set +o allexport
fi

HTTP_PORT="${GATEWAY_HTTP_PORT:-8080}"
DATA_HTTP="${DATA_SERVICE_HTTP:-http://localhost:8080}"
SEARCH_HTTP="${SEARCH_SERVICE_HTTP:-http://localhost:8082}"
NOTIF_HTTP="${NOTIFICATION_SERVICE_HTTP:-http://localhost:8086}"

echo "=== [gateway-service] Bootstrap Start ==="

check_upstream() {
  local name="$1"
  local url="$2"
  local health_url="${url}/health"
  
  if curl -s --max-time 3 "$health_url" 2>/dev/null | grep -q '"status"'; then
    echo "   ✓ ${name}: ${url} → healthy"
  else
    echo "   ⚠ ${name}: ${url} → not ready (will retry at runtime)"
  fi
}

echo "→ Checking upstream services..."
check_upstream "data-service"         "$DATA_HTTP"
check_upstream "search-service"       "$SEARCH_HTTP"
check_upstream "notification-service" "$NOTIF_HTTP"

echo ""
echo "=== [gateway-service] Bootstrap Complete ==="
echo "   HTTP: :${HTTP_PORT}"
echo ""
echo "Test:"
echo "  curl http://localhost:${HTTP_PORT}/health"
echo "  curl http://localhost:${HTTP_PORT}/ready"
echo "  curl http://localhost:${HTTP_PORT}/info"
```

### [MODIFY] `services/gateway-service/cmd/server/main.go`

Cập nhật để đọc `GATEWAY_HTTP_PORT`:

```go
// Thay:
httpPort := envOrDefault("HTTP_PORT", "8080")

// Thành:
httpPort := envOrDefault("GATEWAY_HTTP_PORT", envOrDefault("HTTP_PORT", "8080"))

// Thêm CORS từ env:
r.Use(cors.Handler(cors.Options{
    AllowedOrigins: strings.Split(
        envOrDefault("ALLOWED_ORIGINS", "http://localhost:3001,http://localhost:5173"), ","),
    AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
}))
```

---

## B. apps/osv (OSV Gateway Orchestrator)

### Biến môi trường (đọc từ `.env`)

| Biến | Mô tả | Default |
|------|-------|---------|
| `OSV_MODE` | `standalone` hoặc `microservices` | `microservices` |
| `HTTP_PORT` | HTTP port | `8080` |
| `POSTGRES_DSN` | PostgreSQL DSN | `postgres://osv:...@localhost:5432/osvdb?sslmode=disable` |
| `REDIS_ADDR` | Redis address | `redis://localhost:6379` |
| `JWT_SECRET` | JWT signing secret (embedded mode) | **MUST change** |
| `DATA_SERVICE_ADDR` | data-service gRPC | `localhost:50053` |
| `AI_SERVICE_ADDR` | ai-service gRPC | `localhost:50052` |
| `FINDING_SERVICE_ADDR` | finding-service gRPC | `localhost:50060` |
| `IDENTITY_SERVICE_ADDR` | identity-service gRPC | `localhost:9001` |
| `NOTIFICATION_HTTP` | notification-service HTTP | `http://localhost:8086` |
| `DATA_SERVICE_HTTP` | data-service HTTP | `http://localhost:8080` |
| `SEARCH_SERVICE_HTTP` | search-service HTTP | `http://localhost:8082` |
| `RANKING_SERVICE_HTTP` | ranking-service HTTP | `http://localhost:8088` |
| `IDENTITY_SERVICE_HTTP` | identity-service HTTP | `http://localhost:9101` |
| `JWKS_URL` | JWKS endpoint | `http://localhost:9101/.well-known/jwks.json` |

### [NEW] `apps/osv/scripts/init.sh`

```bash
#!/usr/bin/env bash
# apps/osv bootstrap script
# Validate config và kiểm tra kết nối tất cả services

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(dirname "$SCRIPT_DIR")"

# Load .env từ root project
ENV_FILE="${APP_DIR}/../../.env"
if [ -f "$ENV_FILE" ]; then
  set -o allexport
  source "$ENV_FILE"
  set +o allexport
elif [ -f "${APP_DIR}/.env" ]; then
  set -o allexport
  source "${APP_DIR}/.env"
  set +o allexport
fi

OSV_MODE="${OSV_MODE:-microservices}"
HTTP_PORT="${HTTP_PORT:-8080}"
JWT_SECRET="${JWT_SECRET:-}"

echo "=== [apps/osv] Bootstrap Start ==="
echo "   Mode: ${OSV_MODE}"

# ── Validate JWT_SECRET ───────────────────────────────────────────────────
echo "→ [1/3] Validating configuration..."
if [ "$JWT_SECRET" = "production-secret-key-change-me" ] || [ -z "$JWT_SECRET" ]; then
  echo ""
  echo "   ⚠ WARNING: JWT_SECRET is using default value!"
  echo "   Generate a secure secret:"
  echo "     openssl rand -hex 32"
  echo ""
  if [ "${FORCE_INSECURE:-false}" != "true" ]; then
    echo "   Set JWT_SECRET in .env or export FORCE_INSECURE=true to skip this check"
    exit 1
  fi
fi
echo "   ✓ JWT_SECRET configured"

# ── Check upstream services ───────────────────────────────────────────────
echo "→ [2/3] Checking upstream services (mode: ${OSV_MODE})..."

check_http() {
  local name="$1"
  local url="$2"
  if curl -s --max-time 3 "${url}/health" 2>/dev/null | grep -q '"status"'; then
    echo "   ✓ ${name}: healthy"
  else
    echo "   ⚠ ${name}: not ready"
  fi
}

check_grpc() {
  local name="$1"
  local addr="$2"
  if command -v grpc_health_probe &>/dev/null; then
    if grpc_health_probe -addr="$addr" -connect-timeout=3s 2>/dev/null; then
      echo "   ✓ ${name} (gRPC): healthy"
    else
      echo "   ⚠ ${name} (gRPC): not ready"
    fi
  else
    echo "   ℹ ${name} (gRPC): grpc_health_probe not installed, skipping"
  fi
}

if [ "$OSV_MODE" = "microservices" ]; then
  check_http  "identity-service"     "${IDENTITY_SERVICE_HTTP:-http://localhost:9101}"
  check_http  "data-service"         "${DATA_SERVICE_HTTP:-http://localhost:8080}"
  check_http  "search-service"       "${SEARCH_SERVICE_HTTP:-http://localhost:8082}"
  check_http  "notification-service" "${NOTIFICATION_HTTP:-http://localhost:8086}"
  check_http  "ranking-service"      "${RANKING_SERVICE_HTTP:-http://localhost:8088}"
  check_grpc  "identity-service"     "${IDENTITY_SERVICE_ADDR:-localhost:9001}"
  check_grpc  "data-service"         "${DATA_SERVICE_ADDR:-localhost:50053}"
  check_grpc  "ai-service"           "${AI_SERVICE_ADDR:-localhost:50052}"
fi

# ── Validate JWKS ─────────────────────────────────────────────────────────
echo "→ [3/3] Validating JWKS endpoint..."
JWKS_URL="${JWKS_URL:-http://localhost:9101/.well-known/jwks.json}"
if curl -s --max-time 3 "$JWKS_URL" 2>/dev/null | grep -q '"keys"'; then
  echo "   ✓ JWKS available at ${JWKS_URL}"
else
  echo "   ⚠ JWKS not available — JWT validation will fail until identity-service is running"
fi

echo ""
echo "=== [apps/osv] Bootstrap Complete ==="
echo "   HTTP: :${HTTP_PORT}"
echo "   Mode: ${OSV_MODE}"
echo ""
echo "Start the OSV gateway:"
echo "  cd apps/osv && ./server"
echo ""
echo "Test:"
echo "  curl http://localhost:${HTTP_PORT}/health"
```

### [MODIFY] `apps/osv/internal/config/config.go`

Thêm validation cho `JWT_SECRET`:

```go
// Thêm vào Validate():
func (c *Config) Validate() error {
    if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
        return fmt.Errorf("invalid HTTP_PORT: %d", c.HTTP.Port)
    }
    // Warn nếu dùng default JWT secret
    if c.EmbeddedInfra.JWTSecret == "production-secret-key-change-me" {
        fmt.Fprintf(os.Stderr, "[WARN] JWT_SECRET is using default value — not safe for production!\n")
    }
    return nil
}
```

### [NEW] `apps/osv/.env.example` (cập nhật)

Thêm các biến còn thiếu so với file hiện tại:

```bash
# Thêm vào .env.example hiện tại:

# ── Embedded Infra (dùng khi Mode=microservices) ─────────────────────────
POSTGRES_DSN=postgres://osv:osv_dev_secret_change_me@localhost:5432/osvdb?sslmode=disable
REDIS_ADDR=redis://localhost:6379
JWT_SECRET=<run: openssl rand -hex 32>

# ── JWT Validation (dùng JWKS của identity-service) ─────────────────────
JWKS_URL=http://localhost:9101/.well-known/jwks.json
JWT_AUDIENCE=openvulnscan
JWT_ISSUER=https://auth.openvulnscan.io
```

## Acceptance Criteria

### gateway-service
- [ ] `services/gateway-service/scripts/init.sh` tồn tại
- [ ] `GET /health` trả về danh sách upstream services và status
- [ ] `GET /ready` trả về 200 khi tất cả upstreams healthy
- [ ] `GET /info` trả về service info
- [ ] CORS headers đúng cho `ALLOWED_ORIGINS`

### apps/osv
- [ ] `apps/osv/scripts/init.sh` tồn tại
- [ ] Script fail khi `JWT_SECRET` là default (bảo mật)
- [ ] Script kiểm tra và báo cáo status của tất cả services
- [ ] JWKS endpoint được verify
- [ ] `GET /health` trả về 200

## Kiểm tra nhanh

```bash
# Gateway service
./services/gateway-service/scripts/init.sh
cd services/gateway-service
HTTP_PORT=8080 DATA_SERVICE_HTTP=http://localhost:8080 ./server
curl http://localhost:8080/health

# OSV app
JWT_SECRET=$(openssl rand -hex 32) >> .env
./apps/osv/scripts/init.sh
cd apps/osv
./server
curl http://localhost:8080/health
```
