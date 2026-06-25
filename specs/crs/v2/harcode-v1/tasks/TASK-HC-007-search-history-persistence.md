# TASK-HC-007: Search History Persistence

**Status:** ✅ DONE  
**Sprint:** 2 | **Ước lượng:** 3 giờ  
**Solution:** [SOL-002 Part B](../solutions/SOL-002-search-mock-embedder.md) + [SOL-012](../solutions/SOL-012-search-history.md)  
**Service:** `services/search-service`

---

## Mô tả

Search handler có `// TODO: integrate with a user-specific search history store`. Cần tạo migration, repository, và wire vào handler để search history được persist.

---

## Acceptance Criteria

- [x] Table `search_history` tồn tại trong DB
- [x] Mỗi search request (khi user authenticated) được lưu vào DB async
- [x] `GET /api/v1/search/history` trả list search history của user
- [x] History được lưu với đúng `user_id`, `query`, `search_type`, `result_count`
- [x] `go build ./...` pass trong `services/search-service`

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| NEW | `services/search-service/migrations/003_search_history.sql` | Schema |
| NEW | `services/search-service/internal/domain/repository/search_history.go` | Interface |
| NEW | `services/search-service/internal/infra/postgres/search_history_repo.go` | PostgreSQL impl |
| MODIFY | `services/search-service/internal/delivery/http/search_handler.go` | Wire async save + GET history |
| MODIFY | `services/search-service/embedded.go` | Wire historyRepo |

---

## Bước thực thi

### 1. Tạo migration

**File:** `services/search-service/migrations/003_search_history.sql`

```sql
CREATE TABLE IF NOT EXISTS search_history (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    query        TEXT NOT NULL,
    result_count INT  NOT NULL DEFAULT 0,
    search_type  VARCHAR(20) NOT NULL DEFAULT 'fulltext'
                 CHECK (search_type IN ('fulltext', 'semantic', 'cve_id', 'browse')),
    filters      JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_search_history_user ON search_history(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_search_history_created ON search_history(created_at);
```

Chạy migration:
```bash
psql $DATABASE_URL -f services/search-service/migrations/003_search_history.sql
```

Verify:
```bash
psql $DATABASE_URL -c "\d search_history"
```

### 2. Tạo domain interface

**File:** `services/search-service/internal/domain/repository/search_history.go`

```go
package repository

import (
    "context"
    "time"
    "github.com/google/uuid"
)

type SearchHistoryEntry struct {
    ID          uuid.UUID `json:"id"           db:"id"`
    UserID      uuid.UUID `json:"user_id"      db:"user_id"`
    Query       string    `json:"query"        db:"query"`
    ResultCount int       `json:"result_count" db:"result_count"`
    SearchType  string    `json:"search_type"  db:"search_type"`
    CreatedAt   time.Time `json:"created_at"   db:"created_at"`
}

type SearchHistoryRepository interface {
    Save(ctx context.Context, e *SearchHistoryEntry) error
    ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*SearchHistoryEntry, error)
    DeleteByID(ctx context.Context, id, userID uuid.UUID) error
    ClearByUser(ctx context.Context, userID uuid.UUID) error
}
```

### 3. Tạo PostgreSQL implementation

**File:** `services/search-service/internal/infra/postgres/search_history_repo.go`

```go
package postgres

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/osv/search-service/internal/domain/repository"
)

type SearchHistoryRepo struct {
    pool *pgxpool.Pool
}

func NewSearchHistoryRepo(pool *pgxpool.Pool) *SearchHistoryRepo {
    return &SearchHistoryRepo{pool: pool}
}

func (r *SearchHistoryRepo) Save(ctx context.Context, e *repository.SearchHistoryEntry) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO search_history (user_id, query, result_count, search_type)
        VALUES ($1, $2, $3, $4)
    `, e.UserID, e.Query, e.ResultCount, e.SearchType)
    if err != nil {
        return fmt.Errorf("search_history.Save: %w", err)
    }
    return nil
}

func (r *SearchHistoryRepo) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*repository.SearchHistoryEntry, error) {
    if limit <= 0 || limit > 50 { limit = 20 }
    rows, err := r.pool.Query(ctx, `
        SELECT id, user_id, query, result_count, search_type, created_at
        FROM search_history
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2
    `, userID, limit)
    if err != nil {
        return nil, fmt.Errorf("search_history.ListByUser: %w", err)
    }
    defer rows.Close()

    var entries []*repository.SearchHistoryEntry
    for rows.Next() {
        e := &repository.SearchHistoryEntry{}
        if err := rows.Scan(&e.ID, &e.UserID, &e.Query, &e.ResultCount, &e.SearchType, &e.CreatedAt); err != nil {
            return nil, fmt.Errorf("search_history.ListByUser scan: %w", err)
        }
        entries = append(entries, e)
    }
    return entries, rows.Err()
}

func (r *SearchHistoryRepo) DeleteByID(ctx context.Context, id, userID uuid.UUID) error {
    _, err := r.pool.Exec(ctx,
        `DELETE FROM search_history WHERE id=$1 AND user_id=$2`, id, userID)
    return err
}

func (r *SearchHistoryRepo) ClearByUser(ctx context.Context, userID uuid.UUID) error {
    _, err := r.pool.Exec(ctx, `DELETE FROM search_history WHERE user_id=$1`, userID)
    return err
}
```

### 4. Wire trong embedded.go
```bash
grep -n "pool\|pgxpool\|postgres" services/search-service/embedded.go | head -10
```

```go
// Thêm vào WireEmbedded:
historyRepo := pgpostgres.NewSearchHistoryRepo(pool)
searchHandler.SetHistoryRepo(historyRepo)
```

### 5. Sửa search_handler.go — thêm async save

Tìm dòng TODO:
```bash
grep -n "TODO.*search history\|TODO.*history" services/search-service/internal/delivery/http/search_handler.go
```

Thêm sau khi có kết quả search:
```go
// [FIX CR-HC-012] Save search history async
if h.historyRepo != nil {
    go func(q string, count int) {
        userIDStr := r.Header.Get("X-User-ID")
        if userIDStr == "" { return }
        userID, err := uuid.Parse(userIDStr)
        if err != nil { return }
        _ = h.historyRepo.Save(context.Background(), &repository.SearchHistoryEntry{
            UserID:      userID,
            Query:       q,
            ResultCount: count,
            SearchType:  "fulltext",
        })
    }(query, len(results))
}
```

### 6. Thêm GET /api/v1/search/history handler

```go
func (h *SearchHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
    if h.historyRepo == nil {
        writeJSON(w, http.StatusOK, map[string]interface{}{"history": []interface{}{}, "total": 0})
        return
    }
    userID, err := uuid.Parse(r.Header.Get("X-User-ID"))
    if err != nil {
        writeError(w, http.StatusUnauthorized, "invalid user context")
        return
    }
    entries, err := h.historyRepo.ListByUser(r.Context(), userID, 20)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to get history")
        return
    }
    if entries == nil { entries = []*repository.SearchHistoryEntry{} }
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "history": entries,
        "total":   len(entries),
    })
}
```

### 7. Register route (nếu chưa có)
```bash
grep -n "search/history\|SearchHistory\|GetHistory" services/search-service/internal/delivery/http/*.go
```

Nếu chưa có → thêm vào router.

### 8. Build check
```bash
cd services/search-service && go build ./...
```

---

## Verification

```bash
# Trigger search để lưu history
curl -s -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v2/cves/search?q=apache+log4j"

# Get history
curl -s -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v1/search/history" | jq '.total, .history[0].query'
# PASS nếu total > 0 và query = "apache log4j"

# Verify in DB
psql $DATABASE_URL -c "SELECT user_id, query, result_count FROM search_history ORDER BY created_at DESC LIMIT 3;"
```
