# SOL-GROUP-C — Version Strings & AI Service Config

> **Fixes**: BUG-006, BUG-007  
> **Services**: `gateway-service`, `notification-service`, `product-service`, `ai-service`  
> **Priority**: BUG-006 🟢 Low | BUG-007 🟡 Medium

---

## BUG-006 — Hardcoded Service Version `"1.0.0"`

### Root Cause

Version string `"1.0.0"` được hardcode trực tiếp trong `main()` của nhiều services.
Logs, traces, health endpoints luôn báo version `1.0.0` bất kể thực tế đang chạy version nào.

### Files Cần Sửa

- `services/gateway-service/cmd/server/main.go`
- `services/gateway-service/internal/health/info_handler.go`
- `services/notification-service/cmd/server/main.go`
- `services/product-service/internal/delivery/http/handlers.go`

### Solution

**Bước 1**: Khai báo package-level variable trong mỗi `cmd/server/main.go`:

```go
// services/gateway-service/cmd/server/main.go

package main

// Version và BuildTime được inject lúc build qua -ldflags.
// Khi build local không có ldflags, giá trị mặc định là "dev" và "unknown".
var (
    Version   = "dev"     // overridden by: go build -ldflags "-X main.Version=v2.2.1"
    BuildTime = "unknown" // overridden by: go build -ldflags "-X main.BuildTime=20260622T100000Z"
)

func main() {
    ctx := context.Background()

    // [FIX] Dùng Version variable thay vì hardcode "1.0.0"
    // Version ưu tiên theo thứ tự: ldflags > SERVICE_VERSION env > "dev"
    version := resolveVersion(Version)

    log := observability.InitLogger("gateway-service", version)
    shutdown, err := observability.InitTracer(ctx, "gateway-service", version)
    // ...
    healthUseCase := health.NewAggregateUseCase(upstreams, version)
}

// resolveVersion ưu tiên: ldflags build value → env var → "dev"
func resolveVersion(buildVersion string) string {
    if buildVersion != "dev" && buildVersion != "" {
        return buildVersion
    }
    if v := os.Getenv("SERVICE_VERSION"); v != "" {
        return v
    }
    return "dev"
}
```

Áp dụng tương tự cho `notification-service/cmd/server/main.go`:

```go
// services/notification-service/cmd/server/main.go

var (
    Version   = "dev"
    BuildTime = "unknown"
)

func main() {
    version := resolveVersion(Version)
    log := observability.InitLogger("notification-service", version)
    shutdown, err := observability.InitTracer(ctx, "notification-service", version)
    // ...
}
```

**Bước 2**: Sửa `info_handler.go` để dùng injected version:

```go
// services/gateway-service/internal/health/info_handler.go

type InfoHandler struct {
    version   string  // injected — không hardcode
    buildTime string
    startedAt time.Time
}

// NewInfoHandler tạo InfoHandler với version được inject từ ngoài.
func NewInfoHandler(version, buildTime string) *InfoHandler {
    return &InfoHandler{
        version:   version,
        buildTime: buildTime,
        startedAt: time.Now(),
    }
}

func (h *InfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "service":    "gateway-service",
        "version":    h.version,   // [FIX] was: "1.0.0" hardcoded
        "build_time": h.buildTime,
        "started_at": h.startedAt.UTC().Format(time.RFC3339),
        "uptime":     time.Since(h.startedAt).String(),
    })
}
```

**Bước 3**: Sửa `product-service/handlers.go` — version mặc định của engagement:

```go
// services/product-service/internal/delivery/http/handlers.go

// [FIX] Thay vì:
//   if req.Version == "" { req.Version = "1.0.0" }
//
// Engagement version không nên có default cứng — để trống hoặc dùng build ID:
if req.Version == "" {
    // Nếu là CI/CD engagement, version nên từ build context
    if req.BuildID != "" {
        req.Version = req.BuildID  // dùng build ID làm version
    }
    // Nếu interactive engagement, version là optional — để ""
}
```

**Bước 4**: Cập nhật `Makefile`:

```makefile
# Makefile tại root project

VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME  := $(shell date -u +%Y%m%dT%H%M%SZ)
LDFLAGS     := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: build-gateway build-notification build-ai build-scan build-finding build-data build-product

build-gateway:
	go build $(LDFLAGS) -o bin/gateway-service ./services/gateway-service/cmd/server/

build-notification:
	go build $(LDFLAGS) -o bin/notification-service ./services/notification-service/cmd/server/

build-ai:
	go build $(LDFLAGS) -o bin/ai-service ./services/ai-service/cmd/server/

build-scan:
	go build $(LDFLAGS) -o bin/scan-service ./services/scan-service/cmd/server/

build-finding:
	go build $(LDFLAGS) -o bin/finding-service ./services/finding-service/cmd/server/

build-data:
	go build $(LDFLAGS) -o bin/data-service ./services/data-service/cmd/server/

build-product:
	go build $(LDFLAGS) -o bin/product-service ./services/product-service/cmd/server/

build-all: build-gateway build-notification build-ai build-scan build-finding build-data build-product
```

**Bước 5**: Dockerfile — inject version lúc build:

```dockerfile
# services/gateway-service/Dockerfile

ARG VERSION=dev
ARG BUILD_TIME=unknown

FROM golang:1.22-alpine AS builder
ARG VERSION
ARG BUILD_TIME
WORKDIR /app
COPY . .
RUN go build \
    -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /bin/gateway-service \
    ./services/gateway-service/cmd/server/

FROM alpine:3.19
COPY --from=builder /bin/gateway-service /bin/gateway-service
ENTRYPOINT ["/bin/gateway-service"]
```

```bash
# Build với version:
docker build \
    --build-arg VERSION=$(git describe --tags --always) \
    --build-arg BUILD_TIME=$(date -u +%Y%m%dT%H%M%SZ) \
    -t gateway-service:$(git describe --tags --always) \
    -f services/gateway-service/Dockerfile .
```

---

## BUG-007 — AI Service: Hardcoded Model Names & Ollama URL Inconsistency

### Root Cause

1. `ollama_adapter.go` dùng `const defaultOllamaURL = "http://localhost:11434"` (localhost)
2. `embed.go` dùng `"http://ollama:11434"` (container hostname) — **inconsistency**
3. Model names (`qwen2.5:1.5b`, `gpt-4o-mini`) hardcode — phải sửa code khi đổi model

### Files Cần Sửa

- `services/ai-service/internal/infra/ai/ollama/ollama_adapter.go`
- `services/ai-service/internal/infra/ai/openai/openai_adapter.go`
- `services/ai-service/cmd/server/embed.go`

### Solution

**Bước 1**: Xóa hardcoded default trong `ollama_adapter.go` — yêu cầu caller cung cấp URL:

```go
// services/ai-service/internal/infra/ai/ollama/ollama_adapter.go

// [REMOVE] Xóa constant này — không nên có default trong adapter layer
// const defaultOllamaURL = "http://localhost:11434"

// OllamaAdapter kết nối đến Ollama LLM server.
type OllamaAdapter struct {
    baseURL        string
    chatModel      string
    embeddingModel string
    httpClient     *http.Client
}

// NewOllamaAdapter tạo OllamaAdapter. baseURL là bắt buộc — fail fast nếu rỗng.
// [FIX] Xóa logic fallback localhost khỏi adapter — config phải đến từ caller.
func NewOllamaAdapter(baseURL, chatModel, embeddingModel string, timeout time.Duration) (*OllamaAdapter, error) {
    if baseURL == "" {
        return nil, fmt.Errorf("ollama: baseURL is required — set OLLAMA_BASE_URL env var")
    }
    if chatModel == "" {
        return nil, fmt.Errorf("ollama: chatModel is required — set AI_MODEL env var")
    }
    if embeddingModel == "" {
        return nil, fmt.Errorf("ollama: embeddingModel is required — set AI_EMBEDDING_MODEL env var")
    }
    if timeout <= 0 {
        timeout = 30 * time.Second
    }
    return &OllamaAdapter{
        baseURL:        baseURL,
        chatModel:      chatModel,
        embeddingModel: embeddingModel,
        httpClient:     &http.Client{Timeout: timeout},
    }, nil
}
```

**Bước 2**: Tương tự với `openai_adapter.go`:

```go
// services/ai-service/internal/infra/ai/openai/openai_adapter.go

// [REMOVE] Xóa hardcoded model constants
// const (
//     defaultChatModel      = "gpt-4o-mini"
//     defaultEmbeddingModel = "text-embedding-3-small"
// )

type OpenAIAdapter struct {
    apiKey         string
    chatModel      string
    embeddingModel string
    baseURL        string
    httpClient     *http.Client
}

// NewOpenAIAdapter tạo OpenAIAdapter với config được inject đầy đủ.
func NewOpenAIAdapter(apiKey, chatModel, embeddingModel, baseURL string, timeout time.Duration) (*OpenAIAdapter, error) {
    if apiKey == "" {
        return nil, fmt.Errorf("openai: OPENAI_API_KEY is required")
    }
    if chatModel == "" {
        return nil, fmt.Errorf("openai: chatModel is required — set OPENAI_CHAT_MODEL env var")
    }
    if embeddingModel == "" {
        return nil, fmt.Errorf("openai: embeddingModel is required — set OPENAI_EMBEDDING_MODEL env var")
    }
    if baseURL == "" {
        baseURL = "https://api.openai.com/v1"
    }
    if timeout <= 0 {
        timeout = 60 * time.Second
    }
    return &OpenAIAdapter{
        apiKey:         apiKey,
        chatModel:      chatModel,
        embeddingModel: embeddingModel,
        baseURL:        baseURL,
        httpClient:     &http.Client{Timeout: timeout},
    }, nil
}
```

**Bước 3**: Refactor `embed.go` — tập trung tất cả config loading tại 1 nơi:

```go
// services/ai-service/cmd/server/embed.go

import "github.com/osv/shared/pkg/config"

// AIConfig chứa toàn bộ AI provider configuration.
type AIConfig struct {
    Provider       string        // "ollama" | "openai"
    BaseURL        string        // URL của LLM server (bắt buộc)
    ChatModel      string        // model cho chat/triage (bắt buộc)
    EmbeddingModel string        // model cho embedding (bắt buộc)
    APIKey         string        // chỉ dùng cho OpenAI
    Timeout        time.Duration
}

// LoadAIConfig load AI config từ env vars.
// [FIX] Thống nhất 1 nguồn config thay vì rải rác ở ollama_adapter.go và embed.go.
func LoadAIConfig() (AIConfig, error) {
    provider := config.Str("AI_PROVIDER", "ollama")
    // [FIX] Log WARN nếu dùng default provider

    cfg := AIConfig{
        Provider:       provider,
        BaseURL:        os.Getenv("AI_BASE_URL"),         // bắt buộc — không có default
        ChatModel:      os.Getenv("AI_MODEL"),             // bắt buộc — không có default
        EmbeddingModel: os.Getenv("AI_EMBEDDING_MODEL"),   // bắt buộc — không có default
        APIKey:         os.Getenv("OPENAI_API_KEY"),       // chỉ cần nếu provider=openai
        Timeout:        config.Duration("AI_TIMEOUT", 60*time.Second),
    }

    // Validate bắt buộc
    if cfg.BaseURL == "" {
        return AIConfig{}, fmt.Errorf("AI_BASE_URL env var is required")
    }
    if cfg.ChatModel == "" {
        return AIConfig{}, fmt.Errorf("AI_MODEL env var is required")
    }
    if cfg.EmbeddingModel == "" {
        return AIConfig{}, fmt.Errorf("AI_EMBEDDING_MODEL env var is required")
    }
    if provider == "openai" && cfg.APIKey == "" {
        return AIConfig{}, fmt.Errorf("OPENAI_API_KEY is required when AI_PROVIDER=openai")
    }

    return cfg, nil
}

// WireEmbedded dùng LoadAIConfig để khởi tạo provider chain.
func WireEmbedded(ctx context.Context) error {
    aiCfg, err := LoadAIConfig()
    if err != nil {
        return fmt.Errorf("ai config: %w", err)
    }

    var provider domain.LLMProvider

    switch aiCfg.Provider {
    case "openai":
        provider, err = openai.NewOpenAIAdapter(
            aiCfg.APIKey, aiCfg.ChatModel, aiCfg.EmbeddingModel,
            aiCfg.BaseURL, aiCfg.Timeout,
        )
    case "ollama":
        fallthrough
    default:
        provider, err = ollama.NewOllamaAdapter(
            aiCfg.BaseURL, aiCfg.ChatModel, aiCfg.EmbeddingModel,
            aiCfg.Timeout,
        )
    }

    if err != nil {
        return fmt.Errorf("wire AI provider: %w", err)
    }

    // ... wire provider vào use cases ...
    return nil
}
```

### Env Vars Cần Set

```yaml
# docker-compose.yml — ai-service

ai-service:
  environment:
    # Provider selection
    AI_PROVIDER: ollama                  # hoặc "openai"

    # Ollama config
    AI_BASE_URL:        http://ollama:11434   # [FIX] thống nhất — không dùng localhost
    AI_MODEL:           qwen2.5:7b           # [FIX] configurable — không hardcode
    AI_EMBEDDING_MODEL: nomic-embed-text     # [FIX] configurable

    # OpenAI config (chỉ khi AI_PROVIDER=openai)
    # OPENAI_API_KEY:        ${SECRET_OPENAI_KEY}
    # OPENAI_CHAT_MODEL:     gpt-4o-mini
    # OPENAI_EMBEDDING_MODEL: text-embedding-3-small

    AI_TIMEOUT: 60s
```

---

## Tóm Tắt Thay Đổi

| Bug | File | Thay Đổi Chính |
|-----|------|----------------|
| BUG-006 | `gateway-service/main.go` | Thêm `var Version = "dev"`; dùng `resolveVersion()` |
| BUG-006 | `notification-service/main.go` | Tương tự |
| BUG-006 | `info_handler.go` | Inject version qua constructor |
| BUG-006 | `product-service/handlers.go` | Bỏ hardcode `"1.0.0"` default |
| BUG-006 | `Makefile` | Thêm LDFLAGS với git version |
| BUG-007 | `ollama_adapter.go` | Xóa `defaultOllamaURL`; require caller cung cấp URL |
| BUG-007 | `openai_adapter.go` | Xóa hardcoded model constants |
| BUG-007 | `embed.go` | Tập trung config vào `LoadAIConfig()` với validation |

## Test Verification

```bash
# BUG-006: Verify version được inject đúng
git tag v2.2.1
go build -ldflags "-X main.Version=v2.2.1" ./services/gateway-service/cmd/server/
./gateway-service &
curl http://localhost:8080/api/v1/admin/health | jq .version
# → "v2.2.1"

# BUG-007: Verify fail fast khi thiếu AI_BASE_URL
unset AI_BASE_URL AI_MODEL AI_EMBEDDING_MODEL
go run ./services/ai-service/cmd/server/
# → error: "AI_BASE_URL env var is required"

# BUG-007: Verify Ollama dùng container hostname nhất quán
AI_BASE_URL=http://ollama:11434 AI_MODEL=qwen2.5:1.5b AI_EMBEDDING_MODEL=nomic-embed-text \
go run ./services/ai-service/cmd/server/
# → no localhost in logs
```
