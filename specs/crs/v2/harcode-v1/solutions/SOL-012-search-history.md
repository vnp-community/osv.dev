# SOL-012: Search History Persistence — search-service

**CR:** CR-HC-012 | **Priority:** 🟡 Medium | **Sprint:** 2  
**Service:** `services/search-service`

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-007
**Note:** Search history persist vào bảng search_history (PostgreSQL)
**Build:** ✅ `go build ./...` passes

---

---

> **Lưu ý:** SOL-002 đã bao gồm toàn bộ implementation cho search history. File này là cross-reference.

## Context

Search history persistence là **Part B** của SOL-002 (Remove MockEmbedder).  
Xem chi tiết đầy đủ tại: [SOL-002-search-mock-embedder.md](SOL-002-search-mock-embedder.md)

## Tóm tắt triển khai

### Files cần tạo
- `search-service/migrations/003_search_history.sql`
- `search-service/internal/domain/repository/search_history.go`
- `search-service/internal/infra/postgres/search_history_repo.go`

### Files cần sửa
- `search-service/internal/delivery/http/search_handler.go` — thêm async save + history endpoint

### API endpoints mới
- `GET /api/v1/search/history` — list history cho current user
- `DELETE /api/v1/search/history/{id}` — xóa một entry
- `DELETE /api/v1/search/history` — clear all

## Schema Summary

```sql
CREATE TABLE search_history (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    query        TEXT NOT NULL,
    result_count INT  NOT NULL DEFAULT 0,
    search_type  VARCHAR(20) NOT NULL DEFAULT 'fulltext',
    filters      JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Implementation Detail bổ sung (ngoài SOL-002)

### Redis caching (Optional — cho performance)

Nếu muốn caching 5 recent searches trong Redis (để load nhanh hơn):

```go
// Sau khi save vào Postgres, push vào Redis list
func (r *SearchHistoryRepo) SaveWithCache(ctx context.Context, e *SearchHistoryEntry, redis *redis.Client) error {
    if err := r.Save(ctx, e); err != nil {
        return err
    }
    
    // Cache trong Redis: key = "search_history:{userID}"
    key := fmt.Sprintf("search_history:%s", e.UserID)
    raw, _ := json.Marshal(e)
    redis.LPush(ctx, key, raw)
    redis.LTrim(ctx, key, 0, 9)   // keep last 10
    redis.Expire(ctx, key, 7*24*time.Hour) // 7 days TTL
    return nil
}
```

### Auto-cleanup cron (trong search-service)

```go
// Định kỳ xóa entries > 90 ngày để tiết kiệm DB space
func runHistoryCleanup(ctx context.Context, repo *SearchHistoryRepo) {
    ticker := time.NewTicker(24 * time.Hour)
    for {
        select {
        case <-ticker.C:
            n, err := repo.DeleteOlderThan(ctx, 90*24*time.Hour)
            if err == nil {
                log.Info().Int("deleted", n).Msg("search history cleanup done")
            }
        case <-ctx.Done():
            return
        }
    }
}
```

```go
// Repository method
func (r *SearchHistoryRepo) DeleteOlderThan(ctx context.Context, age time.Duration) (int, error) {
    cutoff := time.Now().Add(-age)
    tag, err := r.pool.Exec(ctx,
        `DELETE FROM search_history WHERE created_at < $1`, cutoff)
    if err != nil {
        return 0, err
    }
    return int(tag.RowsAffected()), nil
}
```

## Verification

```bash
# Test search saves history
curl -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v2/cves/search?q=apache+log4j"

# Get history
curl -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/search/history"
# Expect: {"history":[{"id":"...","query":"apache log4j","search_type":"fulltext",...}],"total":1}

# Verify in DB
psql $DATABASE_URL -c "SELECT query, result_count, created_at FROM search_history ORDER BY created_at DESC LIMIT 5;"
```
