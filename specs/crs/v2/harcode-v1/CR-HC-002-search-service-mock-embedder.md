# CR-HC-002: search-service — Xóa MockEmbedder; Implement Search History Persistence

## Trạng thái: 🔴 Critical

## Vấn đề 1: MockEmbedder trong production code
File: `services/search-service/internal/infra/pgvector/semantic_search.go`

```go
// MockEmbedder returns zero vectors (development/testing only).
type MockEmbedder struct{}

func (m *MockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
    return make([]float32, 768), nil  // ← trả zero vector — search luôn fail
}
```

`MockEmbedder` đang được sử dụng trong production khi AI gRPC không kết nối được.
Trả zero vector → semantic search không có ý nghĩa, tất cả kết quả sẽ sai.

### Giải pháp
- **Xóa** `MockEmbedder` khỏi production code
- Nếu AI service không available → trả `503 Service Unavailable` với message rõ ràng
- Chỉ dùng `NullEmbedder` trong `*_test.go`

```go
// THAY THẾ: Graceful error khi embedder không available
func (r *SemanticSearchRepo) Search(ctx context.Context, query string, topK int) ([]*CVEResult, error) {
    if r.embedder == nil {
        return nil, ErrEmbedderNotConfigured
    }
    // ... real implementation
}
```

---

## Vấn đề 2: Search History không lưu DB
File: `services/search-service/internal/delivery/http/search_handler.go:427`

```go
// TODO: integrate with a user-specific search history store (Redis or Postgres)
```

Lịch sử tìm kiếm không được lưu lại → không thể:
- Hiển thị "recent searches" trên UI
- Analytics / auditing
- Personalization

### Giải pháp: Persist search history trong PostgreSQL + Redis

#### 1. Domain model
```go
type SearchHistory struct {
    ID        uuid.UUID `db:"id"`
    UserID    uuid.UUID `db:"user_id"`
    Query     string    `db:"query"`
    ResultCount int     `db:"result_count"`
    SearchType string   `db:"search_type"` // "fulltext" | "semantic"
    CreatedAt time.Time `db:"created_at"`
}
```

#### 2. Repository interface
```go
type SearchHistoryRepository interface {
    Save(ctx context.Context, h *SearchHistory) error
    ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*SearchHistory, error)
    DeleteOlderThan(ctx context.Context, userID uuid.UUID, days int) error
}
```

#### 3. Migration SQL
```sql
CREATE TABLE IF NOT EXISTS search_history (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    query        TEXT NOT NULL,
    result_count INT NOT NULL DEFAULT 0,
    search_type  VARCHAR(20) NOT NULL DEFAULT 'fulltext',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_search_history_user ON search_history(user_id, created_at DESC);
```

#### 4. Wire trong embedded.go
```go
historyRepo := postgres.NewSearchHistoryRepo(pool)
handler := deliveryhttp.NewCVEHandler(cveRepo, semanticUC, historyRepo, cacheRepo, log)
```

## Files cần thay đổi
- `services/search-service/internal/infra/pgvector/semantic_search.go` — xóa MockEmbedder
- `services/search-service/internal/domain/repository/search_history.go` [NEW]
- `services/search-service/internal/infra/postgres/search_history_repo.go` [NEW]
- `services/search-service/internal/delivery/http/search_handler.go` — wire history save
- `services/search-service/embedded.go` — wire SearchHistoryRepo
- `services/search-service/migrations/002_search_history.sql` [NEW]

## Acceptance Criteria
- [ ] `MockEmbedder` không tồn tại trong production package
- [ ] Khi AI không available → trả 503 với `{"error":"embedding service unavailable"}`
- [ ] Sau khi search → `search_history` table có record
- [ ] `GET /api/v1/search/history` trả danh sách recent searches của user
