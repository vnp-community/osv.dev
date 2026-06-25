# TASK-HC-002: Wire CWERepo vào data-service embed

**Status:** ✅ DONE  
**Sprint:** 1 | **Ước lượng:** 1 giờ  
**Solution:** [SOL-008](../solutions/SOL-008-data-cwe-repo.md)  
**Service:** `services/data-service`

---

## Mô tả

`data-service/embed/server.go` đang có `var cweH *httpdelivery.CWEHandler // TODO: wire`. Cần tạo `postgres.CWERepo` và wire vào handler.

---

## Acceptance Criteria

- [x] `CWEHandler` không còn là `nil` trong embed
- [x] `GET /api/v2/cwe?q=injection` trả list CWE từ DB (không 404/500)
- [x] `GET /api/v2/cwe/CWE-79` trả chi tiết CWE (không 404)
- [x] `go build ./...` pass trong `services/data-service`

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| NEW | `services/data-service/internal/infra/persistence/postgres/cwe_repo.go` | Implement CWERepository với `List` và `GetByID` |
| MODIFY | `services/data-service/embed/server.go` | Thay `var cweH *httpdelivery.CWEHandler // TODO` bằng wire thật |

---

## Bước thực thi

### 1. Kiểm tra CWERepository interface
```bash
grep -n "CWERepository\|interface" services/data-service/internal/delivery/http/cwe_handler.go
```

### 2. Kiểm tra schema cwe_weaknesses tồn tại
```bash
psql $DATABASE_URL -c "\d cwe_weaknesses" 2>&1 | head -20
```
Nếu table không tồn tại → dừng lại, cần tạo migration trước.

### 3. Tạo CWERepo
**File:** `services/data-service/internal/infra/persistence/postgres/cwe_repo.go`

```go
package postgres

import (
    "context"
    "fmt"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
    httpdelivery "github.com/osv/data-service/internal/delivery/http"
)

type CWERepo struct {
    pool *pgxpool.Pool
}

func NewCWERepo(pool *pgxpool.Pool) *CWERepo {
    return &CWERepo{pool: pool}
}

func (r *CWERepo) List(ctx context.Context, query string, page, pageSize int) ([]httpdelivery.CWEItem, int64, error) {
    if page < 1 { page = 1 }
    if pageSize < 1 || pageSize > 100 { pageSize = 20 }
    offset := (page - 1) * pageSize

    var whereClause string
    var args []interface{}
    if query != "" {
        whereClause = "WHERE (cwe_id ILIKE $1 OR name ILIKE $1 OR description ILIKE $1)"
        args = append(args, "%"+query+"%")
    }

    // Count
    var total int64
    if err := r.pool.QueryRow(ctx,
        "SELECT COUNT(*) FROM cwe_weaknesses "+whereClause, args...,
    ).Scan(&total); err != nil {
        return nil, 0, fmt.Errorf("cwe_repo.List count: %w", err)
    }

    // Data
    limitIdx := len(args) + 1
    offsetIdx := len(args) + 2
    dataArgs := append(args, pageSize, offset)
    rows, err := r.pool.Query(ctx, fmt.Sprintf(`
        SELECT cwe_id, COALESCE(name,''), COALESCE(description,''), 0
        FROM cwe_weaknesses %s
        ORDER BY cwe_id LIMIT $%d OFFSET $%d
    `, whereClause, limitIdx, offsetIdx), dataArgs...)
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

func (r *CWERepo) GetByID(ctx context.Context, cweID string) (*httpdelivery.CWEItem, error) {
    normalized := strings.ToUpper(cweID)
    if !strings.HasPrefix(normalized, "CWE-") {
        normalized = "CWE-" + normalized
    }
    var item httpdelivery.CWEItem
    err := r.pool.QueryRow(ctx,
        `SELECT cwe_id, COALESCE(name,''), COALESCE(description,'') FROM cwe_weaknesses WHERE UPPER(cwe_id)=$1`,
        normalized,
    ).Scan(&item.CWEID, &item.Name, &item.Description)
    if err != nil {
        return nil, fmt.Errorf("cwe_repo.GetByID %s: %w", cweID, err)
    }
    return &item, nil
}
```

### 4. Wire trong embed/server.go

Tìm dòng:
```bash
grep -n "cweH\|CWEHandler\|TODO.*cwe" services/data-service/embed/server.go
```

Thay thế:
```go
// OLD:
var cweH *httpdelivery.CWEHandler // TODO: wire postgres.CWERepo when available

// NEW (thêm import postgres nếu cần):
cweRepo := postgres.NewCWERepo(pgPool)
cweH := httpdelivery.NewCWEHandler(cweRepo)
```

### 5. Build check
```bash
cd services/data-service && go build ./...
```

---

## Verification

```bash
# List CWEs
curl -s "https://c12.openledger.vn/api/v2/cwe?q=injection&page=1" | jq '.total, .items[0].cwe_id'
# PASS nếu total > 0

# Get by ID
curl -s "https://c12.openledger.vn/api/v2/cwe/CWE-79" | jq '.cwe_id, .name'
# PASS nếu cwe_id = "CWE-79"
```
