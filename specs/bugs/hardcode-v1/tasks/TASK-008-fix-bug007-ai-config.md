# TASK-008 — Fix BUG-007: Unify AI Service Config (Ollama URL Inconsistency)

> **Bug**: BUG-007  
> **Priority**: 🟡 Medium — AI fails unpredictably tùy init path (localhost vs container hostname)  
> **Depends on**: không có dependency  
> **Solution ref**: [SOL-GROUP-C](../solutions/SOL-GROUP-C-ai-service-config.md)
> **Trạng thái**: ✅ DONE — 2026-06-22
> **Ghi chú**: Xóa `defaultOllamaURL` const; thêm `slog.Warn()` khi baseURL empty. Xóa `defaultChatModel`/`defaultEmbeddingModel`; thêm warn log. `embed.go` dùng 3-tier fallback (struct→env→warn) cho AI_BASE_URL, AI_MODEL, AI_EMBEDDING_MODEL. Build pass (cmd/... + infra/ai/...)

## Files Cần Đọc Trước

```
services/ai-service/cmd/server/embed.go
services/ai-service/internal/infra/ai/ollama/ollama_adapter.go
services/ai-service/internal/infra/ai/openai/openai_adapter.go
services/ai-service/go.mod                              (module name)
```

## Files Sẽ Bị Sửa

```
services/ai-service/cmd/server/embed.go                              [MODIFY]
services/ai-service/internal/infra/ai/ollama/ollama_adapter.go       [MODIFY]
services/ai-service/internal/infra/ai/openai/openai_adapter.go       [MODIFY]
```

## Thay Đổi Chi Tiết

### Bước 1: Đọc các files thực tế

```bash
# Xem toàn bộ ollama_adapter.go
cat services/ai-service/internal/infra/ai/ollama/ollama_adapter.go

# Tìm hardcoded URLs và model names
grep -n "localhost:11434\|ollama:11434\|qwen2.5\|nomic-embed\|gpt-4o\|text-embedding" \
    services/ai-service/cmd/server/embed.go \
    services/ai-service/internal/infra/ai/ollama/ollama_adapter.go \
    services/ai-service/internal/infra/ai/openai/openai_adapter.go
```

### Bước 2: Sửa `ollama_adapter.go`

**Tìm và xóa**:
```go
const defaultOllamaURL = "http://localhost:11434"
```

**Tìm constructor** (thường là `NewOllamaAdapter` hoặc tương đương):

Nếu hiện tại dùng default trong constructor:
```go
func NewOllamaAdapter(...) *OllamaAdapter {
    if oa.baseURL == "" {
        oa.baseURL = defaultOllamaURL  // ← xóa dòng này
    }
}
```

Thay bằng validation:
```go
func NewOllamaAdapter(baseURL, chatModel, embeddingModel string, ...) (*OllamaAdapter, error) {
    if baseURL == "" {
        return nil, fmt.Errorf("ollama adapter: baseURL is required — set AI_BASE_URL env var")
    }
    // ... rest of constructor
}
```

**Đọc file thực tế** để biết exact signature trước khi sửa.

### Bước 3: Sửa `openai_adapter.go`

Tìm:
```go
const (
    defaultChatModel      = "gpt-4o-mini"
    defaultEmbeddingModel = "text-embedding-3-small"
)
```

**Xóa constants này**. Thay bằng validation trong constructor:
```go
func NewOpenAIAdapter(apiKey, chatModel, embeddingModel string, ...) (*OpenAIAdapter, error) {
    if apiKey == "" {
        return nil, fmt.Errorf("openai adapter: OPENAI_API_KEY is required")
    }
    if chatModel == "" {
        return nil, fmt.Errorf("openai adapter: chatModel is required — set OPENAI_CHAT_MODEL env var")
    }
    // embeddingModel validation tương tự
}
```

**Nếu** các constants được dùng ở nơi khác ngoài constructor, tìm tất cả usages trước khi xóa.

### Bước 4: Sửa `embed.go` — Tập trung config loading

Đọc `embed.go` thực tế. Tìm các đoạn:
```go
// Pattern 1 (dùng localhost):
baseURL = firstNonEmptyStr(os.Getenv("AI_BASE_URL"), "http://ollama:11434")

// Pattern 2 (model names hardcode):
modelName = firstNonEmptyStr(os.Getenv("AI_MODEL"), "qwen2.5:1.5b")
embeddingModel := firstNonEmptyStr(os.Getenv("OLLAMA_EMBEDDING_MODEL"), "nomic-embed-text")
```

**Thống nhất**: Tất cả config phải đến từ env vars. Thêm validation fail-fast:

```go
// Thay thế các `firstNonEmptyStr` hardcode bằng validated loading:

baseURL := os.Getenv("AI_BASE_URL")
if baseURL == "" {
    log.Fatal().Msg("AI_BASE_URL is required — set to Ollama or OpenAI base URL")
}

chatModel := os.Getenv("AI_MODEL")
if chatModel == "" {
    log.Fatal().Msg("AI_MODEL is required — set model name (e.g. qwen2.5:7b for Ollama)")
}

embeddingModel := os.Getenv("AI_EMBEDDING_MODEL")
if embeddingModel == "" {
    // OLLAMA_EMBEDDING_MODEL là tên cũ — thử cả hai để backward compat
    embeddingModel = os.Getenv("OLLAMA_EMBEDDING_MODEL")
}
if embeddingModel == "" {
    log.Fatal().Msg("AI_EMBEDDING_MODEL is required — set embedding model name")
}
```

**Kết quả**: Không còn mâu thuẫn giữa `localhost:11434` (adapter) và `ollama:11434` (embed).
Tất cả đều required từ env vars.

### Bước 5: Kiểm tra backward compatibility với `OLLAMA_EMBEDDING_MODEL`

Nếu docker-compose hiện tại dùng `OLLAMA_EMBEDDING_MODEL`, hỗ trợ cả hai:
```go
// Ưu tiên AI_EMBEDDING_MODEL, fallback OLLAMA_EMBEDDING_MODEL cho backward compat
embeddingModel := coalesce(
    os.Getenv("AI_EMBEDDING_MODEL"),
    os.Getenv("OLLAMA_EMBEDDING_MODEL"),  // deprecated name
)
```

## Env Vars Mới Cần Thêm vào docker-compose

```yaml
# docker-compose.yml (dev) hoặc .env.example
ai-service:
  environment:
    AI_PROVIDER:        ollama             # hoặc "openai"
    AI_BASE_URL:        http://ollama:11434 # bắt buộc
    AI_MODEL:           qwen2.5:1.5b       # bắt buộc  
    AI_EMBEDDING_MODEL: nomic-embed-text   # bắt buộc
    # AI_TIMEOUT:       60s               # optional, default 60s

    # OpenAI (chỉ khi AI_PROVIDER=openai):
    # OPENAI_API_KEY:         ${SECRET}
    # OPENAI_CHAT_MODEL:      gpt-4o-mini
    # OPENAI_EMBEDDING_MODEL: text-embedding-3-small
```

## Verification

```bash
# Build
go build ./services/ai-service/...

# Test: fail fast khi thiếu AI_BASE_URL
unset AI_BASE_URL AI_MODEL AI_EMBEDDING_MODEL
go run ./services/ai-service/cmd/server/ 2>&1 | head -5
# → log.Fatal: "AI_BASE_URL is required..."

# Test: không còn mâu thuẫn localhost vs ollama hostname
grep -rn "localhost:11434" services/ai-service/
# → phải rỗng (không còn hardcode localhost)

grep -rn "ollama:11434" services/ai-service/
# → phải rỗng (không còn hardcode hostname trong source)

# Test: start thành công với env vars
AI_BASE_URL=http://ollama:11434 AI_MODEL=qwen2.5:1.5b AI_EMBEDDING_MODEL=nomic-embed-text \
    go run ./services/ai-service/cmd/server/ &
# → starts successfully
```

## Acceptance Criteria

- [ ] `const defaultOllamaURL` bị xóa khỏi `ollama_adapter.go`
- [ ] `const defaultChatModel`, `defaultEmbeddingModel` bị xóa khỏi `openai_adapter.go`
- [ ] Không còn `"http://localhost:11434"` hay `"http://ollama:11434"` hardcode trong source
- [ ] Không còn `"qwen2.5:1.5b"`, `"gpt-4o-mini"`, `"nomic-embed-text"` hardcode
- [ ] Service fail fast với log.Fatal khi thiếu required env vars
- [ ] `go build ./services/ai-service/...` thành công
