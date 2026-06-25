# SOL-002: Remove MockEmbedder + Implement Search History — search-service

**CR:** CR-HC-002 | **Priority:** 🔴 Critical | **Sprint:** 1  
**Service:** `services/search-service`

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-005
**Note:** MockEmbedder đã bị xóa, search-service trả 503 khi AI không available
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**File:** `search-service/internal/infra/pgvector/semantic_search.go:57-61`
```go
// MockEmbedder returns zero vectors (development/testing only).
type MockEmbedder struct{}
func (m *MockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
    return make([]float32, 768), nil  // zero vector → sai kết quả
}
```

**File:** `search-service/embedded.go:86` — MOCK-008 FIX comment tồn tại nhưng `MockEmbedder` vẫn là fallback khi AI gRPC fail.

**File:** `search-service/internal/delivery/http/search_handler.go:427`
```go
// TODO: integrate with a user-specific search history store (Redis or Postgres)
```

**Infrastructure đã có:**
- `search-service/internal/infra/aigrpc/embedder.go` — real gRPC embedder ✅
- `search-service/internal/infra/postgres/cve_repo.go` — postgres pool available ✅
- Pool passed vào `WireEmbedded(ctx, log, pool, mux)` ✅

---

## Part A: Remove MockEmbedder

### Bước 1: Xóa MockEmbedder khỏi production code

**File sửa:** `search-service/internal/infra/pgvector/semantic_search.go`

```diff
-// MockEmbedder returns zero vectors (development/testing only).
-type MockEmbedder struct{}
-
-func (m *MockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
-    return make([]float32, 768), nil
-}
```

Nếu cần dùng cho test, chuyển vào file `mock_embedder_test.go`.

### Bước 2: Khi AI không available → trả 503

**File sửa:** `search-service/embedded.go`

```go
// [FIX CR-HC-002] Remove MockEmbedder fallback — trả 503 khi AI không available
var semanticUC *pgvector.UseCase
aiAddr := os.Getenv("AI_SERVICE_GRPC")
if aiAddr == "" {
    aiAddr = "ai-service:50053"
}
aiEmbedder, embErr := aigrpc.New(aiAddr)
if embErr != nil {
    log.Warn().Err(embErr).Str("ai_addr", aiAddr).
        Msg("search-service: AI embedder unavailable — semantic search disabled")
    // semanticUC remains nil → handler returns 503
} else {
    searcher := pgvector.New(sqlxDB)
    semanticUC = pgvector.NewUseCase(searcher, aiEmbedder)
    log.Info().Str("ai_addr", aiAddr).Msg("search-service: semantic search wired")
}
```

**File sửa:** `search-service/internal/delivery/http/search_handler.go` — semantic search handler

```go
func (h *CVEHandler) SemanticSearch(w http.ResponseWriter, r *http.Request) {
    // [FIX CR-HC-002] Return 503 when embedder not available (not zero vectors)
    if h.semanticUC == nil {
        writeError(w, http.StatusServiceUnavailable,
            "semantic search unavailable: AI embedding service not configured")
        return
    }
    // ... normal implementation
}
```

---

## Part B: Search History Persistence

### Bước 1: Migration SQL

**File mới:** `search-service/migrations/003_search_history.sql`

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

CREATE INDEX IF NOT EXISTS idx_search_history_user 
    ON search_history(user_id, created_at DESC);

-- Auto-cleanup: keep only last 90 days
CREATE INDEX IF NOT EXISTS idx_search_history_created
    ON search_history(created_at);
```

### Bước 2: Domain interface

**File mới:** `search-service/internal/domain/repository/search_history.go`

```go
package repository

import (
    "context"
    "time"

    "github.com/google/uuid"
)

// SearchHistoryEntry records one search query by a user.
type SearchHistoryEntry struct {
    ID          uuid.UUID              `db:"id"`
    UserID      uuid.UUID              `db:"user_id"`
    Query       string                 `db:"query"`
    ResultCount int                    `db:"result_count"`
    SearchType  string                 `db:"search_type"`
    Filters     map[string]interface{} `db:"filters"`
    CreatedAt   time.Time              `db:"created_at"`
}

// SearchHistoryRepository persists user search history.
type SearchHistoryRepository interface {
    Save(ctx context.Context, e *SearchHistoryEntry) error
    ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*SearchHistoryEntry, error)
    DeleteByID(ctx context.Context, id, userID uuid.UUID) error
    ClearByUser(ctx context.Context, userID uuid.UUID) error
}
```

### Bước 3: Repository implementation

**File mới:** `search-service/internal/infra/postgres/search_history_repo.go`

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
        INSERT INTO search_history (user_id, query, result_count, search_type, created_at)
        VALUES ($1, $2, $3, $4, NOW())
    `, e.UserID, e.Query, e.ResultCount, e.SearchType)
    if err != nil {
        return fmt.Errorf("search_history.Save: %w", err)
    }
    return nil
}

func (r *SearchHistoryRepo) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*repository.SearchHistoryEntry, error) {
    if limit <= 0 {
        limit = 20
    }
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
        `DELETE FROM search_history WHERE id = $1 AND user_id = $2`, id, userID)
    return err
}

func (r *SearchHistoryRepo) ClearByUser(ctx context.Context, userID uuid.UUID) error {
    _, err := r.pool.Exec(ctx, `DELETE FROM search_history WHERE user_id = $1`, userID)
    return err
}
```

### Bước 4: Wire vào search handler

**File sửa:** `search-service/internal/delivery/http/search_handler.go`

```go
// Thay TODO bằng implementation thật:
// [FIX CR-HC-002] Save search history asynchronously
go func() {
    userIDStr := r.Header.Get("X-User-ID")
    if userIDStr == "" || h.historyRepo == nil {
        return
    }
    userID, err := uuid.Parse(userIDStr)
    if err != nil {
        return
    }
    _ = h.historyRepo.Save(context.Background(), &repository.SearchHistoryEntry{
        UserID:      userID,
        Query:       query,
        ResultCount: len(results),
        SearchType:  "fulltext",
    })
}()
```

### Bước 5: Thêm GET /api/v1/search/history endpoint

```go
func (h *CVEHandler) SearchHistory(w http.ResponseWriter, r *http.Request) {
    userID, err := uuid.Parse(r.Header.Get("X-User-ID"))
    if err != nil {
        writeError(w, http.StatusUnauthorized, "invalid user context")
        return
    }
    entries, err := h.historyRepo.ListByUser(r.Context(), userID, 20)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to get search history")
        return
    }
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "history": entries,
        "total":   len(entries),
    })
}
```

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| DELETE lines | `search-service/internal/infra/pgvector/semantic_search.go` — xóa MockEmbedder |
| MODIFY | `search-service/embedded.go` — remove MockEmbedder fallback |
| MODIFY | `search-service/internal/delivery/http/search_handler.go` — 503 + history save |
| NEW | `search-service/migrations/003_search_history.sql` |
| NEW | `search-service/internal/domain/repository/search_history.go` |
| NEW | `search-service/internal/infra/postgres/search_history_repo.go` |

---

## Verification

```bash
# Build
cd services/search-service && go build ./...

# Run migration
psql $DATABASE_URL -f migrations/003_search_history.sql

# Test: semantic search returns 503 when AI down
curl -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v2/cves/search/semantic?q=buffer+overflow"
# → {"error":"semantic search unavailable: AI embedding service not configured"}

# Test: search history saved
curl -H "Authorization: Bearer $TOKEN" "https://c12.openledger.vn/api/v1/search/history"
# → {"history":[...],"total":N}
```
