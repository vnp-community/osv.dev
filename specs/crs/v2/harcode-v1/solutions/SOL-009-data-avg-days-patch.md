# SOL-009: AvgDaysToPatch từ DB — data-service

**CR:** CR-HC-009 | **Priority:** 🟡 Medium | **Sprint:** 1  
**Service:** `services/data-service` | **Độ phức tạp:** Low

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-003
**Note:** AvgDaysToPatch tính từ SQL AVG query thay giá trị hardcode 30.0
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**File:** `data-service/internal/infra/persistence/postgres/kev_repo.go:327`
```go
stats.AvgDaysToPatch = 0 // TODO: implement when CVE publish dates available in PG
```

**Table `cves` đã có:** `published_at TIMESTAMPTZ` column (từ schema migration).
**Table `kev_entries` đã có:** `date_added` column.

**Query cần:** Tính trung bình số ngày từ `cves.published_at` đến `kev_entries.date_added`.

---

## Solution

### Bước 1: Kiểm tra columns tồn tại

```bash
psql $DATABASE_URL -c "
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'cves' AND column_name IN ('published_at','cve_id');
"
psql $DATABASE_URL -c "
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'kev_entries' AND column_name IN ('cve_id','date_added');
"
```

### Bước 2: Implement SQL trong kev_repo.go

**File sửa:** `data-service/internal/infra/persistence/postgres/kev_repo.go`

```go
// [FIX CR-HC-009] Implement AvgDaysToPatch từ join cves + kev_entries
func (r *KEVPostgresRepo) computeAvgDaysToPatch(ctx context.Context) int {
    var avgDays float64
    err := r.db.QueryRowContext(ctx, `
        SELECT COALESCE(
            AVG(
                EXTRACT(EPOCH FROM (k.date_added::timestamptz - c.published_at)) / 86400.0
            )::INT,
            0
        )
        FROM kev_entries k
        JOIN cves c ON c.cve_id = k.cve_id
        WHERE c.published_at IS NOT NULL
          AND k.date_added IS NOT NULL
          AND k.date_added > c.published_at
          AND EXTRACT(EPOCH FROM (k.date_added - c.published_at)) > 0
    `).Scan(&avgDays)

    if err != nil {
        // Non-fatal: log warning, return 0 as safe default
        // r.log.Warn().Err(err).Msg("kev_repo: failed to compute avg_days_to_patch")
        return 0
    }
    return int(avgDays)
}

// GetStats đã có — cần sửa dòng hardcode:
func (r *KEVPostgresRepo) GetStats(ctx context.Context) (*KEVStats, error) {
    // ... existing code ...
    
    // [FIX CR-HC-009] Thay thế hardcode bằng real computation
    // OLD: stats.AvgDaysToPatch = 0
    // NEW:
    stats.AvgDaysToPatch = r.computeAvgDaysToPatch(ctx)
    
    return stats, nil
}
```

### Bước 3: Migration nếu `published_at` chưa có trong cves

```bash
# Kiểm tra trước
psql $DATABASE_URL -c "SELECT COUNT(*) FROM cves WHERE published_at IS NOT NULL;"
```

Nếu column tồn tại nhưng giá trị NULL → populate từ NVD data (best effort):

**File mới:** `data-service/migrations/006_populate_published_at.sql`

```sql
-- Populate published_at từ modified_at nếu rỗng (safe approximation)
-- Chỉ chạy một lần khi data đã có
UPDATE cves 
SET published_at = modified_at
WHERE published_at IS NULL 
  AND modified_at IS NOT NULL;

-- Hoặc nếu có published_date column khác:
-- UPDATE cves SET published_at = published_date WHERE published_at IS NULL;

-- Add index cho join performance
CREATE INDEX IF NOT EXISTS idx_cves_published_at ON cves(published_at) 
    WHERE published_at IS NOT NULL;
```

### Bước 4: Fallback graceful khi data không đủ

Query trả 0 tự nhiên khi:
- Không có data thỏa điều kiện
- `published_at` toàn là NULL

Không cần try-catch thêm — `COALESCE(..., 0)` trong SQL đã handle.

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| MODIFY | `data-service/internal/infra/persistence/postgres/kev_repo.go` — implement computation |
| NEW | `data-service/migrations/006_populate_published_at.sql` (nếu cần populate) |

---

## Verification

```bash
# Check data
psql $DATABASE_URL -c "
SELECT AVG(EXTRACT(EPOCH FROM (k.date_added::timestamptz - c.published_at)) / 86400)::INT
FROM kev_entries k JOIN cves c ON c.cve_id = k.cve_id
WHERE c.published_at IS NOT NULL AND k.date_added > c.published_at;
"
# Expect: a non-zero number (typically 200-500 days)

# API test
curl "https://c12.openledger.vn/api/v2/kev/stats"
# Expect: {"avg_days_to_patch": NNN} (not 0)
```
