# SOL-008: Wire CWERepo — data-service

**CR:** CR-HC-008 | **Priority:** 🟡 Medium | **Sprint:** 1  
**Service:** `services/data-service` | **Độ phức tạp:** Low

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-002
**Note:** CWERepo wire vào data-service, không còn nil
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**File:** `data-service/embed/server.go:210`
```go
var cweH *httpdelivery.CWEHandler // TODO: wire postgres.CWERepo when available
```

**Infrastructure đã có:**
- `data-service/internal/delivery/http/cwe_handler.go` — `CWEHandler` ✅ implemented
- `data-service/internal/delivery/http/cwe_handler.go` — `CWERepository` interface ✅ defined
- `public.cwe_weaknesses` table tồn tại trong DB

**Khoảng trống duy nhất:** Chưa có `postgres.CWERepo` implementation.

---

## Solution

### Bước 1: Kiểm tra schema cwe_weaknesses

```bash
psql $DATABASE_URL -c "\d cwe_weaknesses"
```

Dự kiến columns: `cwe_id`, `name`, `description`, có thể có `abstraction`, `structure`, `status`.

### Bước 2: PostgreSQL CWE Repository implementation

**File mới:** `data-service/internal/infra/persistence/postgres/cwe_repo.go`

```go
package postgres

import (
    "context"
    "fmt"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"

    httpdelivery "github.com/osv/data-service/internal/delivery/http"
)

// CWERepo implements httpdelivery.CWERepository via PostgreSQL.
type CWERepo struct {
    pool *pgxpool.Pool
}

// NewCWERepo creates a CWERepo backed by a pgxpool.
func NewCWERepo(pool *pgxpool.Pool) *CWERepo {
    return &CWERepo{pool: pool}
}

// List returns CWE entries with optional full-text search and pagination.
// Implements CWERepository interface from cwe_handler.go
func (r *CWERepo) List(ctx context.Context, query string, page, pageSize int) ([]httpdelivery.CWEItem, int64, error) {
    if page < 1 {
        page = 1
    }
    if pageSize < 1 || pageSize > 100 {
        pageSize = 20
    }
    offset := (page - 1) * pageSize

    // Build WHERE clause
    var where string
    var args []interface{}
    if query != "" {
        where = "WHERE (cwe_id ILIKE $1 OR name ILIKE $1 OR description ILIKE $1)"
        args = append(args, "%"+query+"%")
    }

    // Count query
    countSQL := "SELECT COUNT(*) FROM cwe_weaknesses " + where
    var total int64
    if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
        return nil, 0, fmt.Errorf("cwe_repo.List count: %w", err)
    }

    // Data query — join with cves to get related_cve_count
    dataArgs := append(args, pageSize, offset)
    limitPlaceholder := len(dataArgs) - 1
    offsetPlaceholder := len(dataArgs)

    dataSQL := fmt.Sprintf(`
        SELECT 
            cw.cwe_id,
            cw.name,
            COALESCE(cw.description, '') AS description,
            COUNT(DISTINCT c.cve_id)     AS related_cve_count
        FROM cwe_weaknesses cw
        LEFT JOIN cves c ON c.cve_id = ANY(
            SELECT unnest(regexp_matches(c2.description, 'CWE-\d+', 'g'))
            FROM cves c2 WHERE c2.cve_id = c.cve_id LIMIT 1
        )
        %s
        GROUP BY cw.cwe_id, cw.name, cw.description
        ORDER BY cw.cwe_id
        LIMIT $%d OFFSET $%d
    `, where, limitPlaceholder, offsetPlaceholder)

    rows, err := r.pool.Query(ctx, dataSQL, dataArgs...)
    if err != nil {
        return nil, 0, fmt.Errorf("cwe_repo.List query: %w", err)
    }
    defer rows.Close()

    var items []httpdelivery.CWEItem
    for rows.Next() {
        var item httpdelivery.CWEItem
        if err := rows.Scan(&item.CWEID, &item.Name, &item.Description, &item.RelatedCVECount); err != nil {
            return nil, 0, fmt.Errorf("cwe_repo.List scan: %w", err)
        }
        items = append(items, item)
    }
    return items, total, rows.Err()
}

// GetByID returns a single CWE entry by its ID (e.g., "CWE-79").
// Implements CWERepository.GetByID
func (r *CWERepo) GetByID(ctx context.Context, cweID string) (*httpdelivery.CWEItem, error) {
    // Normalize: accept "79", "CWE-79", "cwe-79"
    normalized := cweID
    if !strings.HasPrefix(strings.ToUpper(normalized), "CWE-") {
        normalized = "CWE-" + normalized
    }
    normalized = strings.ToUpper(normalized)

    var item httpdelivery.CWEItem
    err := r.pool.QueryRow(ctx, `
        SELECT cwe_id, name, COALESCE(description, '')
        FROM cwe_weaknesses
        WHERE UPPER(cwe_id) = $1
    `, normalized).Scan(&item.CWEID, &item.Name, &item.Description)
    if err != nil {
        return nil, fmt.Errorf("cwe_repo.GetByID %s: %w", cweID, err)
    }
    return &item, nil
}
```

### Bước 3: Wire CWEHandler dalam embed/server.go

**File sửa:** `data-service/embed/server.go`

```go
import (
    pgpersist "github.com/osv/data-service/internal/infra/persistence/postgres"
)

// [FIX CR-HC-008] Wire CWERepo — CWEHandler no longer nil
cweRepo := pgpersist.NewCWERepo(pgPool)
cweH = httpdelivery.NewCWEHandler(cweRepo)
log.Info().Msg("data-service: CWEHandler wired (PostgreSQL)")

// Xóa dòng: var cweH *httpdelivery.CWEHandler // TODO: wire postgres.CWERepo
```

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| NEW | `data-service/internal/infra/persistence/postgres/cwe_repo.go` |
| MODIFY | `data-service/embed/server.go` — wire cweH |

---

## Verification

```bash
# Build
cd services/data-service && go build ./...

# Test CWE list
curl -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v2/cwe?q=injection&page=1"
# Expect: list of CWE entries (không 404 hoặc empty)

# Test CWE by ID
curl -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v2/cwe/CWE-79"
# Expect: {"cwe_id":"CWE-79","name":"Improper Neutralization of Input..."}
```
