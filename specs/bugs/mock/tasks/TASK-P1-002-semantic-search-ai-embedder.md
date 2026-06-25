# TASK-P1-002 — Thay MockEmbedder bằng AI Service gRPC Client

**Bug:** MOCK-008  
**Priority:** 🔴 P1 — Data Correctness  
**Effort:** ~1.5 giờ  
**Service:** `search-service`  
**Loại thay đổi:** New file (AI gRPC adapter) + Sửa embedded.go + Nil-check handler

---

## Mục tiêu

`embedded.go` đang wire `MockEmbedder{}` cho semantic search — embedder này trả all-zero vector, khiến mọi CVE đều có cosine similarity như nhau (kết quả vô nghĩa). Cần wire `AIServiceEmbedder` gọi `ai-service` (port 9103) qua gRPC.

---

## Preconditions

- [ ] Đọc `services/search-service/embedded.go` — xem cách `semanticUC` đang được khởi tạo
- [ ] Đọc `services/search-service/internal/infra/pgvector/` — xem `Embedder` interface
- [ ] Kiểm tra `services/search-service/internal/delivery/http/` — xem `SemanticSearch` handler
- [ ] Kiểm tra xem ai-service có gRPC proto đã generate chưa:
  ```bash
  find . -name "*.pb.go" | grep -i ai | head -10
  find . -path "*/proto/gen/*" | head -10
  ```
- [ ] Xác định module name: `grep "^module" services/search-service/go.mod`

---

## Steps

### Step 1 — Xác định Embedder interface

```bash
grep -rn "type Embedder interface\|Embedder interface" services/search-service/
grep -rn "MockEmbedder" services/search-service/
```

Ghi lại:
- Package path của `Embedder` interface
- Method signature (thường là `Embed(ctx, text) ([]float32, error)`)

### Step 2 — Kiểm tra ai-service gRPC proto

```bash
# Tìm proto files cho ai-service
find . -name "*.proto" | xargs grep -l "GenerateEmbedding\|Embedding" 2>/dev/null

# Tìm generated Go files
find . -name "*.pb.go" | xargs grep -l "GenerateEmbedding\|Embedding" 2>/dev/null | head -5
```

Nếu proto đã generate → dùng client đó.  
Nếu chưa → tạo HTTP client gọi ai-service REST thay vì gRPC (xem Step 3B).

### Step 3A — Tạo AIServiceEmbedder (gRPC — nếu proto đã có)

**File mới**: `services/search-service/internal/infra/grpc/ai_embedder.go`

```go
package aigrpc

import (
    "context"
    "fmt"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    // import đúng package proto đã generate
    aiv1 "github.com/your-module/shared/proto/gen/go/ai/v1"
)

// AIServiceEmbedder implements the pgvector.Embedder interface via ai-service gRPC.
type AIServiceEmbedder struct {
    client aiv1.AIServiceClient
    conn   *grpc.ClientConn
}

// NewAIServiceEmbedder creates a new embedder connected to ai-service at target.
// target format: "host:port" e.g. "localhost:9103"
func NewAIServiceEmbedder(target string) (*AIServiceEmbedder, error) {
    conn, err := grpc.NewClient(target,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        return nil, fmt.Errorf("ai-service gRPC dial %q: %w", target, err)
    }
    return &AIServiceEmbedder{
        client: aiv1.NewAIServiceClient(conn),
        conn:   conn,
    }, nil
}

func (e *AIServiceEmbedder) Close() error {
    return e.conn.Close()
}

// Embed generates a 1536-dim vector embedding for the given text.
func (e *AIServiceEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    resp, err := e.client.GenerateEmbedding(ctx, &aiv1.GenerateEmbeddingRequest{
        Text: text,
    })
    if err != nil {
        return nil, fmt.Errorf("ai-service GenerateEmbedding: %w", err)
    }
    return resp.GetEmbedding(), nil
}
```

### Step 3B — Tạo AIServiceEmbedder (HTTP REST — nếu không có gRPC proto)

**File mới**: `services/search-service/internal/infra/aihttp/ai_embedder.go`

```go
package aihttp

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type AIServiceEmbedder struct {
    baseURL    string
    httpClient *http.Client
}

func NewAIServiceEmbedder(baseURL string) *AIServiceEmbedder {
    return &AIServiceEmbedder{
        baseURL: baseURL,
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }
}

func (e *AIServiceEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    body, _ := json.Marshal(map[string]string{"text": text})
    req, err := http.NewRequestWithContext(ctx, "POST",
        e.baseURL+"/api/v1/ai/embed", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := e.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("ai-service embed request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("ai-service embed: status %d", resp.StatusCode)
    }

    var result struct {
        Embedding []float32 `json:"embedding"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("ai-service embed: decode response: %w", err)
    }
    return result.Embedding, nil
}
```

### Step 4 — Sửa embedded.go để wire AI embedder

Mở `services/search-service/embedded.go`.

Tìm dòng `MockEmbedder`:
```bash
grep -n "MockEmbedder\|semanticUC" services/search-service/embedded.go
```

Thay phần khởi tạo `semanticUC`:

```go
// FIX MOCK-008: Wire real AI embedder thay vì MockEmbedder
var semanticUC *pgvector.UseCase  // nil = disabled

aiAddr := os.Getenv("AI_SERVICE_GRPC")
if aiAddr == "" {
    aiAddr = "localhost:9103"  // default
}

if postgresDSN != "" {  // chỉ init pgvector khi có postgres
    sqlxDB, err := sqlx.ConnectContext(ctx, "postgres", postgresDSN)
    if err == nil {
        searcher := pgvector.New(sqlxDB)

        // Thử kết nối AI service
        aiEmbedder, embErr := aigrpc.NewAIServiceEmbedder(aiAddr)
        // hoặc: aihttp.NewAIServiceEmbedder("http://"+aiAddr)
        if embErr != nil {
            log.Warn().Err(embErr).Str("ai_addr", aiAddr).
                Msg("AI embedder unavailable — semantic search disabled")
            // semanticUC = nil → route sẽ trả 503
        } else {
            semanticUC = pgvector.NewUseCase(searcher, aiEmbedder)
            log.Info().Str("ai_addr", aiAddr).
                Msg("Semantic search enabled via ai-service")
        }
    }
}
```

### Step 5 — Thêm nil-check trong SemanticSearch handler

Mở `services/search-service/internal/delivery/http/` — tìm handler SemanticSearch:
```bash
grep -rn "SemanticSearch\|semantic" services/search-service/internal/delivery/http/
```

Thêm nil-check đầu hàm:

```go
func (h *Handler) SemanticSearch(w http.ResponseWriter, r *http.Request) {
    // FIX MOCK-008: trả 503 khi AI embedder chưa được wire
    if h.semanticUC == nil {
        respondError(w, http.StatusServiceUnavailable,
            "semantic search not available: AI embedding service not configured")
        return
    }
    // ... phần còn lại giữ nguyên
}
```

### Step 6 — Cấu hình môi trường

Kiểm tra `deploy/dev/docker-compose.yml` hoặc `deploy/dev/docker-compose.server.yaml`:

```bash
grep -n "AI_SERVICE_GRPC\|ai.service\|ai-service" deploy/dev/docker-compose*.yml
```

Thêm env var nếu chưa có:
```yaml
services:
  osv-monolith:  # hoặc search-service
    environment:
      - AI_SERVICE_GRPC=ai-service:9103
```

---

## Acceptance Criteria

- [ ] `MockEmbedder` không còn được dùng trong production path (embedded.go)
- [ ] Khi `AI_SERVICE_GRPC` không set hoặc ai-service unavailable → `semanticUC = nil`, route trả 503
- [ ] Khi ai-service available → `POST /api/v2/cves/search/semantic` trả kết quả thực dựa trên embedding
- [ ] `go build ./services/search-service/...` — thành công

---

## Test Commands

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/search-service/...
go vet ./services/search-service/...

# Verify MockEmbedder không còn trong embedded.go
grep -n "MockEmbedder" services/search-service/embedded.go
# Expected: không có output (hoặc chỉ còn trong test files)

# Test semantic search khi AI not configured
curl -X POST http://localhost:8083/api/v2/cves/search/semantic \
  -H "Content-Type: application/json" \
  -d '{"query":"buffer overflow"}'
# Expect: 503 {"error":"semantic search not available..."}
```
