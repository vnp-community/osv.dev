# TASK-HC-005: Xóa MockEmbedder + Trả 503 khi AI Down

**Status:** ✅ DONE  
**Sprint:** 1 | **Ước lượng:** 1 giờ  
**Solution:** [SOL-002](../solutions/SOL-002-search-mock-embedder.md) — Part A  
**Service:** `services/search-service`

---

## Mô tả

`search-service/internal/infra/pgvector/semantic_search.go` có `MockEmbedder` trả zero vectors trong production code. Cần xóa và trả 503 khi AI embedder không available.

---

## Acceptance Criteria

- [x] `MockEmbedder` không còn tồn tại trong `internal/infra/pgvector/semantic_search.go`
- [x] `GET /api/v2/cves/search/semantic` trả 503 (không 200 với kết quả sai) khi AI service không reachable
- [x] `MockEmbedder` trong `*_test.go` files vẫn OK (chỉ xóa khỏi production)
- [x] `go build ./...` pass trong `services/search-service`

---

## Files cần sửa

| Action | File | Thay đổi |
|--------|------|---------|
| MODIFY | `services/search-service/internal/infra/pgvector/semantic_search.go` | Xóa struct `MockEmbedder` + method `Embed` |
| MODIFY | `services/search-service/embedded.go` | Remove MockEmbedder fallback logic |
| MODIFY | `services/search-service/internal/delivery/http/search_handler.go` | Trả 503 khi semanticUC == nil |

---

## Bước thực thi

### 1. Xác định vị trí MockEmbedder
```bash
grep -n "MockEmbedder" services/search-service/internal/infra/pgvector/semantic_search.go
grep -n "MockEmbedder" services/search-service/embedded.go
```

### 2. Xóa MockEmbedder khỏi semantic_search.go

Xóa các dòng (ví dụ từ dòng 57-63 dựa trên audit trước):
```go
// XÓA hoàn toàn:
// MockEmbedder returns zero vectors (development/testing only).
type MockEmbedder struct{}

func (m *MockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
    return make([]float32, 768), nil
}
```

### 3. Kiểm tra MockEmbedder có được dùng trong embedded.go không
```bash
grep -n "MockEmbedder\|mock.*[Ee]mbed\|noopCVECache\|newMock" services/search-service/embedded.go
```

Nếu có → thay thế bằng `nil` (semantic UC sẽ là nil, handler trả 503):
```go
// OLD:
semanticUC = newSemanticUC(&MockEmbedder{}, ...)

// NEW: để nil, handler sẽ trả 503
// semanticUC = nil  ← không wire khi AI không available
```

### 4. Thêm nil-guard trong semantic search handler
```bash
grep -n "SemanticSearch\|semantic.*Search\|func.*semantic" \
  services/search-service/internal/delivery/http/search_handler.go | head -10
```

Thêm vào đầu handler:
```go
func (h *SearchHandler) SemanticSearch(w http.ResponseWriter, r *http.Request) {
    // [FIX CR-HC-002] Return 503 when embedder not available
    if h.semanticUC == nil {
        writeJSON(w, http.StatusServiceUnavailable, map[string]string{
            "error":   "semantic search unavailable",
            "reason":  "AI embedding service not configured",
        })
        return
    }
    // ... existing implementation
}
```

### 5. Wire AI embedder thật trong embedded.go
```bash
grep -n "aigrpc\|AIGRPCAddr\|AI_SERVICE_GRPC\|50053" services/search-service/embedded.go | head -10
```

Đảm bảo wire aigrpc.Embedder thật (không MockEmbedder):
```go
aiAddr := os.Getenv("AI_SERVICE_GRPC")
if aiAddr == "" {
    aiAddr = "ai-service:50053"
}
embedder, err := aigrpc.New(aiAddr)
if err != nil {
    log.Warn().Err(err).Msg("search-service: AI embedder unavailable — semantic search disabled")
    // semanticUC không được wire → handler trả 503
} else {
    // Wire semantic usecase với embedder thật
    semanticUC = buildSemanticUC(embedder, pool)
}
```

### 6. Build check
```bash
cd services/search-service && go build ./...
```

### 7. Kiểm tra MockEmbedder không còn trong non-test files
```bash
grep -rn "MockEmbedder" services/search-service/ --include="*.go" | grep -v "_test.go"
# PASS nếu không có kết quả
```

---

## Verification

```bash
# Semantic search khi AI không reachable → 503
curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v2/cves/search/semantic?q=buffer+overflow"
# PASS nếu = 200 (AI đang chạy) hoặc 503 (AI down) — không được 200 với zero-vector results

# Normal fulltext search vẫn hoạt động
curl -s -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v2/cves/search?q=log4j" | jq '.total'
# PASS nếu > 0
```
