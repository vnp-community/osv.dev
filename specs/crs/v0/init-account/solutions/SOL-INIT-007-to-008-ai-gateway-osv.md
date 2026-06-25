# SOL-INIT-007 — Giải Pháp: Khởi Tạo ai-service

> **CR tham chiếu**: [CR-INIT-007](../CR-INIT-007-ai-service.md)  
> **Kiến trúc cơ sở**: `specs/01-architecture.md §14.5`, `specs/02-technical-design.md §14.5`

## Phân Tích Code Hiện Tại

```
services/ai-service/
├── internal/infra/ai/factory.go    ← FromEnv() đã có ✓, thiếu Validate() ✗
├── internal/infra/ai/ollama.go     ← Ollama provider ✓
├── internal/infra/ai/vertex.go     ← Vertex AI provider ✓
└── cmd/server/main.go              ← thiếu gRPC health check registration ✗
```

**Tech từ spec §14.5**:
- Provider Chain: Ollama → OpenAI → Azure (failover)
- Embedding Cache: Redis, 7-day TTL, key `osv:embed:{cve_id}`
- Parallel EnrichCVE: 4 goroutines (embedding, severity, exploit, MITRE tags)
- gRPC: port 50052

## Files cần tạo/sửa

### [NEW] `services/ai-service/scripts/init.sh`

```bash
#!/usr/bin/env bash
# =============================================================================
# ai-service — Bootstrap Script
# Spec: 01-architecture.md §14.5 (LLM Provider Chain: Ollama → OpenAI → Azure)
# Tech: 02-technical-design.md §14.5 (ProviderChain, EmbeddingService, Redis cache)
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
AI_GRPC_PORT="${AI_GRPC_PORT:-50052}"

echo "══════════════════════════════════════════════════════════"
echo "  ai-service Bootstrap"
echo "══════════════════════════════════════════════════════════"
echo "  Backend: ${AI_BACKEND}"
echo "  Model:   ${AI_MODEL}"

# ── Validate + test backend ───────────────────────────────────────────────
case "${AI_BACKEND}" in

  ollama)
    # Spec §14.5: ProviderChain.providers[0] = Ollama
    echo "→ [1/2] Kiểm tra Ollama server..."
    if curl -s --max-time 5 "${AI_BASE_URL}/api/tags" 2>/dev/null | grep -q '"models"'; then
      echo "   ✓ Ollama running at ${AI_BASE_URL}"
      
      echo "→ [2/2] Kiểm tra model '${AI_MODEL}'..."
      if curl -s "${AI_BASE_URL}/api/tags" 2>/dev/null | grep -q "\"name\":\"${AI_MODEL}\""; then
        echo "   ✓ Model '${AI_MODEL}' available"
      else
        echo "   ℹ Model '${AI_MODEL}' chưa có, pulling..."
        if curl -s -X POST "${AI_BASE_URL}/api/pull" \
             -H 'Content-Type: application/json' \
             -d "{\"name\":\"${AI_MODEL}\"}" 2>/dev/null | tail -1 | grep -q '"done":true'; then
          echo "   ✓ Model '${AI_MODEL}' pulled"
        else
          echo "   ⚠ Không pull được model. Chạy thủ công:"
          echo "     ollama pull ${AI_MODEL}"
          echo "   ai-service sẽ start nhưng enrichment sẽ fail"
        fi
      fi
    else
      echo "   ⚠ Ollama không có tại ${AI_BASE_URL}"
      echo "   Install: curl -fsSL https://ollama.ai/install.sh | sh"
      echo "   ai-service sẽ start nhưng enrichment requests sẽ fail"
    fi
    ;;

  vertex)
    # Spec §14.5: ProviderChain.providers[2] = Azure (or Vertex)
    echo "→ [1/2] Validate Vertex AI config..."
    if [[ -z "${VERTEX_PROJECT_ID:-}" ]]; then
      echo "   ✗ VERTEX_PROJECT_ID bắt buộc khi AI_BACKEND=vertex"
      exit 1
    fi
    echo "   ✓ Project: ${VERTEX_PROJECT_ID}"
    echo "   ✓ Location: ${VERTEX_LOCATION:-us-central1}"
    
    echo "→ [2/2] Kiểm tra GCP credentials..."
    if gcloud auth application-default print-access-token &>/dev/null; then
      echo "   ✓ Application Default Credentials OK"
    else
      echo "   ⚠ GCP ADC chưa cấu hình"
      echo "   Chạy: gcloud auth application-default login"
    fi
    ;;

  openai)
    echo "→ [1/2] Validate OpenAI config..."
    if [[ -z "${OPENAI_API_KEY:-}" ]]; then
      echo "   ✗ OPENAI_API_KEY bắt buộc khi AI_BACKEND=openai"
      exit 1
    fi
    echo "   ✓ API key: ${OPENAI_API_KEY:0:8}..."
    
    echo "→ [2/2] Test API key..."
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 \
      -H "Authorization: Bearer ${OPENAI_API_KEY}" \
      "https://api.openai.com/v1/models")
    if [[ "${HTTP_CODE}" == "200" ]]; then
      echo "   ✓ OpenAI API key valid"
    else
      echo "   ⚠ OpenAI API key validation failed (HTTP ${HTTP_CODE})"
      echo "   Kiểm tra lại OPENAI_API_KEY"
    fi
    ;;

  *)
    echo "   ✗ Unknown AI_BACKEND: '${AI_BACKEND}'"
    echo "   Valid: ollama | vertex | openai"
    exit 1
    ;;
esac

# ── Redis cache check ──────────────────────────────────────────────────────
echo "→ Kiểm tra Redis (dùng cho embedding cache)..."
# Spec §14.5: EmbeddingService — Redis cache 7-day TTL, key: osv:embed:{cve_id}
REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"

_redis_opts="-h ${REDIS_HOST} -p ${REDIS_PORT}"
[[ -n "${REDIS_PASSWORD}" ]] && _redis_opts="${_redis_opts} -a ${REDIS_PASSWORD}"

if redis-cli ${_redis_opts} ping 2>/dev/null | grep -q "PONG"; then
  echo "   ✓ Redis connected (embedding cache ready)"
else
  echo "   ⚠ Redis unavailable — embeddings sẽ không được cache"
fi

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  ai-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  gRPC: :${AI_GRPC_PORT}"
echo "  Backend: ${AI_BACKEND} / ${AI_MODEL}"
echo ""
echo "Test (gRPC health):"
echo "  grpc_health_probe -addr=:${AI_GRPC_PORT}"
```

### [MODIFY] `services/ai-service/internal/infra/ai/factory.go`

Thêm `Validate()` và cập nhật `FromEnv()` để bao gồm `GRPCPort`:

```go
// Config holds LLM provider configuration.
// Tech: 02-technical-design.md §14.5 ProviderChain
type Config struct {
    Backend    Backend        // AI_BACKEND: ollama | vertex | openai
    ModelName  string         // AI_MODEL
    ProjectID  string         // VERTEX_PROJECT_ID
    Location   string         // VERTEX_LOCATION
    BaseURL    string         // AI_BASE_URL (Ollama URL)
    APIKey     string         // OPENAI_API_KEY
    GRPCPort   string         // AI_GRPC_PORT — THÊM MỚI
}

// FromEnv loads Config from environment variables.
func FromEnv() Config {
    return Config{
        Backend:   Backend(envOrDefault("AI_BACKEND", "ollama")),
        ModelName: envOrDefault("AI_MODEL", "llama3"),
        ProjectID: os.Getenv("VERTEX_PROJECT_ID"),
        Location:  envOrDefault("VERTEX_LOCATION", "us-central1"),
        BaseURL:   envOrDefault("AI_BASE_URL", "http://localhost:11434"),
        APIKey:    os.Getenv("OPENAI_API_KEY"),
        GRPCPort:  envOrDefault("AI_GRPC_PORT", envOrDefault("GRPC_PORT", "50052")), // THÊM MỚI
    }
}

// Validate checks required fields for each backend.
// THÊM MỚI:
func (c *Config) Validate() error {
    switch c.Backend {
    case BackendVertex:
        if c.ProjectID == "" {
            return fmt.Errorf("VERTEX_PROJECT_ID is required when AI_BACKEND=vertex")
        }
    case BackendOpenAI:
        if c.APIKey == "" {
            return fmt.Errorf("OPENAI_API_KEY is required when AI_BACKEND=openai")
        }
    case BackendOllama:
        if c.BaseURL == "" {
            return fmt.Errorf("AI_BASE_URL is required when AI_BACKEND=ollama")
        }
    default:
        return fmt.Errorf("unknown AI_BACKEND %q: must be ollama|vertex|openai", c.Backend)
    }
    return nil
}
```

### [MODIFY] `services/ai-service/cmd/server/main.go`

Thêm gRPC health check (cần thiết cho `grpc_health_probe`):

```go
import (
    "google.golang.org/grpc/health"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
    cfg := aifactory.FromEnv()
    
    // Validate config trước khi start
    if err := cfg.Validate(); err != nil {
        log.Fatal().Err(err).Msg("invalid AI backend config")
    }
    
    grpcPort := cfg.GRPCPort  // đọc từ AI_GRPC_PORT env var
    
    lis, err := net.Listen("tcp", ":"+grpcPort)
    if err != nil {
        log.Fatal().Err(err).Str("port", grpcPort).Msg("failed to listen")
    }
    
    s := grpc.NewServer()
    
    // gRPC health check — required for grpc_health_probe and gateway health routing
    healthSvc := health.NewServer()
    healthpb.RegisterHealthServer(s, healthSvc)
    healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
    healthSvc.SetServingStatus("ai.AIService", healthpb.HealthCheckResponse_SERVING)
    
    // TODO: register AIServiceServer after proto generation
    
    log.Info().
        Str("backend", string(cfg.Backend)).
        Str("model", cfg.ModelName).
        Str("port", grpcPort).
        Msg("ai-service starting")
    
    // Graceful shutdown...
}
```

## Acceptance Criteria

- [ ] `scripts/init.sh` chạy được với mọi backend
- [ ] `Validate()` fail fast khi thiếu required config
- [ ] `AI_GRPC_PORT` được đọc từ env
- [ ] gRPC health check respond: `grpc_health_probe -addr=:50052` → `status: SERVING`
- [ ] Với `AI_BACKEND=ollama`: script pull model nếu chưa có

## Files Tóm Tắt

| File | Action |
|------|--------|
| `services/ai-service/scripts/init.sh` | **[NEW]** |
| `services/ai-service/internal/infra/ai/factory.go` | **[MODIFY]** Validate() + GRPCPort |
| `services/ai-service/cmd/server/main.go` | **[MODIFY]** gRPC health check |

---

# SOL-INIT-008 — Giải Pháp: Khởi Tạo gateway-service & apps/osv

> **CR tham chiếu**: [CR-INIT-008](../CR-INIT-008-gateway-osv-app.md)  
> **Kiến trúc cơ sở**: `specs/01-architecture.md §3.1`, `§7.1`, `specs/02-technical-design.md §3`

## Phân Tích Code Hiện Tại

```
apps/osv/
├── internal/config/config.go  ← Mode, ServiceAddrs, EmbeddedInfra struct
├── internal/gateway/
│   ├── auth/middleware.go     ← JWT + API key auth chain ✓
│   └── ratelimit/limiter.go   ← Redis token bucket ✓
└── cmd/server/main.go         ← JWT_SECRET fallback check thiếu ✗
```

## Files cần tạo/sửa

### [NEW] `services/gateway-service/scripts/init.sh`

```bash
#!/usr/bin/env bash
# =============================================================================
# gateway-service — Bootstrap Script
# Spec: 01-architecture.md §3.1 (Reverse Proxy: auth + rate-limit + routing)
# Tech: 02-technical-design.md §3.1 (ReverseProxy.Forward)
# =============================================================================
set -euo pipefail
PROJECT_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"

if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

HTTP_PORT="${GATEWAY_HTTP_PORT:-8080}"
DATA_HTTP="${DATA_SERVICE_HTTP:-http://localhost:8082}"
SEARCH_HTTP="${SEARCH_SERVICE_HTTP:-http://localhost:8083}"
NOTIF_HTTP="${NOTIFICATION_SERVICE_HTTP:-http://localhost:8086}"

echo "═══════════════════════════════════════════════════════"
echo "  gateway-service Bootstrap"
echo "═══════════════════════════════════════════════════════"

check_upstream() {
  local name="$1" url="$2"
  if curl -s --max-time 3 "${url}/health" 2>/dev/null | grep -q '"status"'; then
    echo "   ✓ ${name}: healthy"
  else
    echo "   ⚠ ${name}: not ready (${url})"
  fi
}

echo "→ Checking upstream services..."
check_upstream "data-service"         "$DATA_HTTP"
check_upstream "search-service"       "$SEARCH_HTTP"
check_upstream "notification-service" "$NOTIF_HTTP"

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  gateway-service Bootstrap Complete"
echo "═══════════════════════════════════════════════════════"
echo "  HTTP: :${HTTP_PORT}"
```

### [NEW] `apps/osv/scripts/init.sh`

```bash
#!/usr/bin/env bash
# =============================================================================
# apps/osv — Bootstrap Script
# Spec: 01-architecture.md §2.1 "UNIFIED GATEWAY apps/osv :8080"
# Spec: 01-architecture.md §7.1 (Auth: JWT + API Key dual auth chain)
# Tech: 02-technical-design.md §3.2 (AuthMiddleware, JWT + API key validation)
# =============================================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(cd "${APP_DIR}/../.." && pwd)"

# Load .env với priority: APP_DIR/.env → PROJECT_ROOT/.env
if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi
if [[ -f "${APP_DIR}/.env" ]]; then
  set -o allexport; source "${APP_DIR}/.env"; set +o allexport
fi

OSV_MODE="${OSV_MODE:-microservices}"
HTTP_PORT="${HTTP_PORT:-8080}"
JWT_SECRET="${JWT_SECRET:-CHANGE_ME_use_openssl_rand_hex_32}"
FORCE_INSECURE="${FORCE_INSECURE:-false}"

echo "═══════════════════════════════════════════════════════"
echo "  apps/osv Gateway Bootstrap"
echo "═══════════════════════════════════════════════════════"
echo "  Mode: ${OSV_MODE}"

# ── Security validation ───────────────────────────────────────────────────
echo "→ [1/3] Security validation..."

# Spec §7.1: JWT must be properly configured
if [[ "${JWT_SECRET}" == "CHANGE_ME_use_openssl_rand_hex_32" ]] || \
   [[ "${JWT_SECRET}" == "production-secret-key-change-me" ]] || \
   [[ -z "${JWT_SECRET}" ]]; then
  if [[ "${FORCE_INSECURE}" == "true" ]]; then
    echo "   ⚠ JWT_SECRET là default — INSECURE (FORCE_INSECURE=true override)"
  else
    echo "   ✗ JWT_SECRET là default hoặc trống!"
    echo ""
    echo "   Tạo secure JWT secret:"
    echo "     openssl rand -hex 32"
    echo ""
    echo "   Thêm vào .env:"
    echo "     JWT_SECRET=<generated>"
    echo ""
    echo "   Hoặc bypass cho local dev:"
    echo "     FORCE_INSECURE=true ./scripts/init.sh"
    exit 1
  fi
else
  echo "   ✓ JWT_SECRET configured"
fi

# ── Check upstream services ───────────────────────────────────────────────
echo "→ [2/3] Upstream health check (mode=${OSV_MODE})..."

check_http() {
  local name="$1" url="$2"
  if curl -s --max-time 3 "${url}/health" 2>/dev/null | grep -q '"status"'; then
    echo "   ✓ ${name}: healthy"
    return 0
  else
    echo "   ⚠ ${name}: not ready (${url})"
    return 1
  fi
}

check_grpc() {
  local name="$1" addr="$2"
  if command -v grpc_health_probe &>/dev/null; then
    if grpc_health_probe -addr="${addr}" -connect-timeout=3s &>/dev/null; then
      echo "   ✓ ${name} gRPC: SERVING"
      return 0
    fi
  fi
  echo "   ⚠ ${name} gRPC: not ready (${addr})"
  return 1
}

if [[ "${OSV_MODE}" == "microservices" ]]; then
  check_http "identity-service"     "${IDENTITY_SERVICE_HTTP:-http://localhost:9101}"
  check_http "data-service"         "${DATA_SERVICE_HTTP:-http://localhost:8082}"
  check_http "search-service"       "${SEARCH_SERVICE_HTTP:-http://localhost:8083}"
  check_http "ranking-service"      "${RANKING_SERVICE_HTTP:-http://localhost:8088}"
  check_http "notification-service" "${NOTIFICATION_HTTP:-http://localhost:8086}"
  check_grpc "identity-service"     "${IDENTITY_SERVICE_ADDR:-localhost:9001}"
  check_grpc "data-service"         "${DATA_SERVICE_ADDR:-localhost:50053}"
fi

# ── JWKS validation ───────────────────────────────────────────────────────
echo "→ [3/3] JWKS endpoint check..."
# Spec §3.1: Gateway validates JWT via /.well-known/jwks.json
# Code: apps/osv/internal/gateway/auth/middleware.go — validateJWT()

JWKS_URL="${JWKS_URL:-http://localhost:9101/.well-known/jwks.json}"
if curl -s --max-time 3 "${JWKS_URL}" 2>/dev/null | grep -q '"keys"'; then
  echo "   ✓ JWKS available at ${JWKS_URL}"
else
  echo "   ⚠ JWKS không available — JWT validation sẽ fail"
  echo "   Khởi động identity-service trước khi start apps/osv"
fi

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  apps/osv Bootstrap Complete"
echo "═══════════════════════════════════════════════════════"
echo "  HTTP: :${HTTP_PORT}"
echo "  Mode: ${OSV_MODE}"
```

### [MODIFY] `apps/osv/internal/config/config.go`

Thêm `Validate()` method:

```go
// Validate checks required configuration for secure operation.
// Must be called before starting the server.
func (c *Config) Validate() error {
    if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
        return fmt.Errorf("invalid HTTP.Port: %d", c.HTTP.Port)
    }
    
    // Spec §7.1: JWT must be properly configured
    // Tech: auth/middleware.go — validateJWT() dùng JWT_SECRET
    defaultSecrets := []string{
        "production-secret-key-change-me",
        "CHANGE_ME_use_openssl_rand_hex_32",
        "",
    }
    for _, def := range defaultSecrets {
        if c.EmbeddedInfra.JWTSecret == def {
            // Warning không fail (dev mode)
            fmt.Fprintf(os.Stderr,
                "[WARN] apps/osv: JWT_SECRET is default — insecure for production!\n"+
                "       Generate: openssl rand -hex 32\n")
            break
        }
    }
    return nil
}
```

## Acceptance Criteria

- [ ] `gateway-service/scripts/init.sh` chạy được
- [ ] `apps/osv/scripts/init.sh` fail khi `JWT_SECRET` là default (trừ `FORCE_INSECURE=true`)
- [ ] `Validate()` trong config.go warn về JWT_SECRET mặc định
- [ ] JWKS endpoint được verify trong init script
- [ ] Upstream health check được báo cáo đầy đủ
- [ ] `GET /health` hoạt động sau khi start

## Files Tóm Tắt

| File | Action |
|------|--------|
| `services/gateway-service/scripts/init.sh` | **[NEW]** |
| `apps/osv/scripts/init.sh` | **[NEW]** |
| `apps/osv/internal/config/config.go` | **[MODIFY]** thêm Validate() |
