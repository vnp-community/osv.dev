# CR-INIT-007 — Khởi tạo ai-service

## Mục tiêu

Sau khi chạy init, ai-service phải:
1. Cấu hình LLM backend phù hợp (Ollama local, Vertex AI, hoặc OpenAI)
2. Validate config trước khi start (tránh crash sau vài giây)
3. gRPC server hoạt động ổn định

## Biến môi trường (đọc từ `.env`)

| Biến | Mô tả | Default | Bắt buộc khi |
|------|-------|---------|-------------|
| `AI_BACKEND` | LLM backend: `ollama` \| `vertex` \| `openai` | `ollama` | Luôn |
| `AI_MODEL` | Model name | `llama3` | Luôn |
| `AI_BASE_URL` | Ollama base URL | `http://localhost:11434` | `AI_BACKEND=ollama` |
| `AI_GRPC_PORT` | gRPC port | `50052` | Luôn |
| `VERTEX_PROJECT_ID` | GCP Project ID | — | `AI_BACKEND=vertex` |
| `VERTEX_LOCATION` | GCP Region | `us-central1` | `AI_BACKEND=vertex` |
| `OPENAI_API_KEY` | OpenAI API key | — | `AI_BACKEND=openai` |

## Các thay đổi cần thực hiện

### [NEW] `services/ai-service/scripts/init.sh`

```bash
#!/usr/bin/env bash
# ai-service bootstrap script
# 1. Validate AI backend configuration
# 2. Verify backend reachability
# 3. Pull model nếu dùng Ollama và model chưa có

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load .env
if [ -f "${SCRIPT_DIR}/../../../.env" ]; then
  set -o allexport
  source "${SCRIPT_DIR}/../../../.env"
  set +o allexport
fi

AI_BACKEND="${AI_BACKEND:-ollama}"
AI_MODEL="${AI_MODEL:-llama3}"
AI_BASE_URL="${AI_BASE_URL:-http://localhost:11434}"
AI_GRPC_PORT="${AI_GRPC_PORT:-50052}"

echo "=== [ai-service] Bootstrap Start ==="
echo "   Backend: ${AI_BACKEND}"
echo "   Model:   ${AI_MODEL}"

case "$AI_BACKEND" in
  ollama)
    echo "→ [1/2] Checking Ollama availability..."
    if curl -s "${AI_BASE_URL}/api/tags" 2>/dev/null | grep -q "models"; then
      echo "   ✓ Ollama running at ${AI_BASE_URL}"
      
      # Check if model exists
      echo "→ [2/2] Checking model: ${AI_MODEL}..."
      if curl -s "${AI_BASE_URL}/api/tags" | grep -q "\"${AI_MODEL}\""; then
        echo "   ✓ Model '${AI_MODEL}' available"
      else
        echo "   ℹ Model '${AI_MODEL}' not found locally"
        echo "   Pulling model (this may take a while)..."
        if curl -s -X POST "${AI_BASE_URL}/api/pull" \
           -d "{\"name\":\"${AI_MODEL}\"}" | tail -1 | grep -q '"done":true'; then
          echo "   ✓ Model pulled successfully"
        else
          echo "   ⚠ WARNING: Could not pull model '${AI_MODEL}'"
          echo "   Run manually: ollama pull ${AI_MODEL}"
        fi
      fi
    else
      echo "   ⚠ WARNING: Ollama not available at ${AI_BASE_URL}"
      echo "   ai-service will start but enrichment will fail"
      echo "   Install Ollama: https://ollama.ai"
    fi
    ;;

  vertex)
    echo "→ [1/2] Validating Vertex AI config..."
    VERTEX_PROJECT_ID="${VERTEX_PROJECT_ID:-}"
    if [ -z "$VERTEX_PROJECT_ID" ]; then
      echo "   ✗ ERROR: VERTEX_PROJECT_ID is required for vertex backend"
      exit 1
    fi
    echo "   ✓ Project: ${VERTEX_PROJECT_ID}"
    echo "   ✓ Region:  ${VERTEX_LOCATION:-us-central1}"
    
    echo "→ [2/2] Checking GCP credentials..."
    if gcloud auth application-default print-access-token &>/dev/null; then
      echo "   ✓ GCP credentials configured"
    else
      echo "   ⚠ WARNING: GCP Application Default Credentials not set"
      echo "   Run: gcloud auth application-default login"
    fi
    ;;

  openai)
    echo "→ [1/2] Validating OpenAI config..."
    OPENAI_API_KEY="${OPENAI_API_KEY:-}"
    if [ -z "$OPENAI_API_KEY" ]; then
      echo "   ✗ ERROR: OPENAI_API_KEY is required for openai backend"
      exit 1
    fi
    
    echo "→ [2/2] Verifying OpenAI API key..."
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
      -H "Authorization: Bearer ${OPENAI_API_KEY}" \
      "https://api.openai.com/v1/models")
    if [ "$HTTP_CODE" = "200" ]; then
      echo "   ✓ OpenAI API key valid"
    else
      echo "   ⚠ WARNING: OpenAI API returned HTTP ${HTTP_CODE}"
      echo "   Check your OPENAI_API_KEY"
    fi
    ;;

  *)
    echo "   ✗ ERROR: Unknown AI_BACKEND: '${AI_BACKEND}'"
    echo "   Valid values: ollama | vertex | openai"
    exit 1
    ;;
esac

echo ""
echo "=== [ai-service] Bootstrap Complete ==="
echo "   gRPC: :${AI_GRPC_PORT}"
echo "   Backend: ${AI_BACKEND} / ${AI_MODEL}"
```

### [MODIFY] `services/ai-service/cmd/server/main.go`

Cập nhật để đọc `AI_GRPC_PORT` và thêm graceful startup validation:

```go
func main() {
    log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

    cfg := aifactory.FromEnv()
    if err := cfg.Validate(); err != nil {
        log.Fatal().Err(err).Msg("invalid AI backend config")
    }

    // Support cả AI_GRPC_PORT và GRPC_PORT
    grpcPort := envOrDefault("AI_GRPC_PORT", envOrDefault("GRPC_PORT", "50052"))
    
    log.Info().
        Str("backend", string(cfg.Backend)).
        Str("model", cfg.ModelName).
        Str("port", grpcPort).
        Msg("ai-service starting")

    // Setup gRPC server với health check
    lis, err := net.Listen("tcp", ":"+grpcPort)
    if err != nil {
        log.Fatal().Err(err).Msg("failed to listen")
    }
    
    s := grpc.NewServer()
    healthSvc := health.NewServer()
    healthpb.RegisterHealthServer(s, healthSvc)
    healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
    // TODO: Register AIService after proto generation

    go func() {
        if err := s.Serve(lis); err != nil {
            log.Fatal().Err(err).Msg("gRPC serve failed")
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
    <-quit
    
    log.Info().Msg("ai-service shutting down")
    s.GracefulStop()
}
```

### [MODIFY] `services/ai-service/internal/infra/ai/factory.go`

Thêm `AI_GRPC_PORT` vào Config (để các component khác tham chiếu):

```go
type Config struct {
    Backend    Backend
    ModelName  string
    ProjectID  string // Vertex AI
    Location   string // Vertex AI
    BaseURL    string // Ollama / OpenAI
    APIKey     string // OpenAI
    GRPCPort   string // exposed port
}

func FromEnv() Config {
    return Config{
        Backend:   Backend(envOrDefault("AI_BACKEND", "ollama")),
        ModelName: envOrDefault("AI_MODEL", "llama3"),
        ProjectID: os.Getenv("VERTEX_PROJECT_ID"),
        Location:  envOrDefault("VERTEX_LOCATION", "us-central1"),
        BaseURL:   envOrDefault("AI_BASE_URL", "http://localhost:11434"),
        APIKey:    os.Getenv("OPENAI_API_KEY"),
        GRPCPort:  envOrDefault("AI_GRPC_PORT", envOrDefault("GRPC_PORT", "50052")),
    }
}
```

## Acceptance Criteria

- [ ] `services/ai-service/scripts/init.sh` tồn tại và executable
- [ ] Với `AI_BACKEND=ollama`: script kiểm tra Ollama và pull model nếu chưa có
- [ ] Với `AI_BACKEND=vertex`: script fail nếu `VERTEX_PROJECT_ID` trống
- [ ] Với `AI_BACKEND=openai`: script fail nếu `OPENAI_API_KEY` trống
- [ ] Service start được sau khi init với Ollama backend (không cần cloud)
- [ ] gRPC health check trả về SERVING
- [ ] `AI_GRPC_PORT` được đọc từ env

## Kiểm tra nhanh (Ollama — recommended cho local dev)

```bash
# 0. Install Ollama (nếu chưa có)
curl -fsSL https://ollama.ai/install.sh | sh
ollama serve &

# 1. Init
AI_BACKEND=ollama AI_MODEL=llama3 ./services/ai-service/scripts/init.sh

# 2. Start
cd services/ai-service
AI_BACKEND=ollama AI_MODEL=llama3 ./server

# 3. Health check via grpc_health_probe
grpc_health_probe -addr=:50052
# Expected: status: SERVING
```
