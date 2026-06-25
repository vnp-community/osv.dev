# BUG-007 — AI Service: Hardcoded Model Names và Ollama URL

## Metadata
- **ID**: BUG-007
- **Service**: `ai-service`
- **Files**:
  - [`cmd/server/embed.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/ai-service/cmd/server/embed.go)
  - [`internal/infra/ai/ollama/ollama_adapter.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/ai-service/internal/infra/ai/ollama/ollama_adapter.go)
  - [`internal/infra/ai/openai/openai_adapter.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/ai-service/internal/infra/ai/openai/openai_adapter.go)
- **Severity**: Medium
- **Category**: Hardcode / AI Configuration
- **Status**: Open

## Mô tả

### 1. Ollama default URL

```go
// ollama_adapter.go:15
const defaultOllamaURL = "http://localhost:11434"
```

Và trong embed.go:
```go
// embed.go:70
baseURL = firstNonEmptyStr(os.Getenv("AI_BASE_URL"), "http://ollama:11434")
```

Hai file dùng **hai giá trị default khác nhau** (`localhost:11434` vs `ollama:11434`).
Điều này tạo ra inconsistency: adapter dùng localhost nhưng embed dùng container hostname.

### 2. OpenAI default model names

```go
// openai_adapter.go:20-21
defaultChatModel      = "gpt-4o-mini"
defaultEmbeddingModel = "text-embedding-3-small"
```

### 3. Ollama default model trong embed.go

```go
// embed.go:74
modelName = firstNonEmptyStr(os.Getenv("AI_MODEL"), "qwen2.5:1.5b")

// embed.go:81
embeddingModel := firstNonEmptyStr(os.Getenv("OLLAMA_EMBEDDING_MODEL"), "nomic-embed-text")
```

## Tác động

1. **Inconsistency**: `ollama_adapter.go` default là `localhost:11434` nhưng `embed.go`
   default là `ollama:11434`. Khi adapter được khởi tạo trực tiếp (không qua embed), nó
   sẽ dùng localhost và fail trong container.

2. **Model versioning**: `qwen2.5:1.5b` là một model version cụ thể — khi có model update,
   phải sửa source code thay vì chỉ update config.

3. **OpenAI cost risk**: `gpt-4o-mini` là default — nếu env var `OPENAI_MODEL` không
   được set, production có thể dùng model đắt tiền hơn dự kiến.

## Fix Proposal

### Thống nhất một nguồn config

```go
// Xóa const trong ollama_adapter.go, thay bằng tham số bắt buộc:
func NewOllamaAdapter(baseURL, model, embeddingModel string, timeout time.Duration) *OllamaAdapter {
    if baseURL == "" {
        panic("ollama: baseURL is required") // fail fast
    }
    ...
}
```

### Config từ env vars với warning

```go
// embed.go — thống nhất config loading
type AIConfig struct {
    Provider       string // "ollama" | "openai" | "vertex"
    BaseURL        string
    ChatModel      string
    EmbeddingModel string
    Timeout        time.Duration
}

func AIConfigFromEnv() AIConfig {
    provider := os.Getenv("AI_PROVIDER")
    if provider == "" {
        provider = "ollama"
        log.Warn().Msg("AI_PROVIDER not set, defaulting to ollama")
    }
    return AIConfig{
        Provider:       provider,
        BaseURL:        os.Getenv("AI_BASE_URL"),        // no default — required
        ChatModel:      os.Getenv("AI_MODEL"),           // no default — required
        EmbeddingModel: os.Getenv("AI_EMBEDDING_MODEL"), // no default — required
    }
}
```

## Files Affected

| File | Line | Hardcode |
|------|------|----------|
| [ollama_adapter.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/ai-service/internal/infra/ai/ollama/ollama_adapter.go) | 15 | `"http://localhost:11434"` |
| [openai_adapter.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/ai-service/internal/infra/ai/openai/openai_adapter.go) | 20-21 | `"gpt-4o-mini"`, `"text-embedding-3-small"` |
| [embed.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/ai-service/cmd/server/embed.go) | 70, 74, 81 | `"http://ollama:11434"`, `"qwen2.5:1.5b"`, `"nomic-embed-text"` |
