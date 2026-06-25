# CR-HC-012: search-service — Search History không Persist

## Trạng thái: 🟡 Medium

(Xem chi tiết tại [CR-HC-002](CR-HC-002-search-service-mock-embedder.md) — Phần 2)

## Tóm tắt

File: `services/search-service/internal/delivery/http/search_handler.go:427`

```go
// TODO: integrate with a user-specific search history store (Redis or Postgres)
```

Search history không được lưu → không có:
- Recent searches UI
- Search analytics
- Personalized recommendations

## Thiết kế nhanh

### Schema
```sql
CREATE TABLE IF NOT EXISTS search_history (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    query        TEXT NOT NULL,
    result_count INT  NOT NULL DEFAULT 0,
    search_type  VARCHAR(20) NOT NULL DEFAULT 'fulltext', -- 'fulltext'|'semantic'|'cve_id'
    filters      JSONB,                                    -- severity, vendor, etc.
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_search_history_user ON search_history(user_id, created_at DESC);
```

### API Endpoints cần thêm
```
GET  /api/v1/search/history?limit=10         → recent queries
DELETE /api/v1/search/history/{id}           → delete one
DELETE /api/v1/search/history                → clear all
```

### Implementation
```go
// In search_handler.go after getting results:
go func() {
    _ = h.historyRepo.Save(context.Background(), &SearchHistory{
        UserID:      extractUserID(r),
        Query:       query,
        ResultCount: len(results),
        SearchType:  "fulltext",
    })
}()  // async — không block response
```

## Acceptance Criteria
- [ ] `GET /api/v1/search/history` trả recent searches từ DB
- [ ] Mỗi search request tự động lưu vào `search_history`
- [ ] Unauthenticated searches không được lưu
