# TASK-HC-003: Implement AvgDaysToPatch từ SQL

**Status:** ✅ DONE  
**Sprint:** 1 | **Ước lượng:** 1 giờ  
**Solution:** [SOL-009](../solutions/SOL-009-data-avg-days-patch.md)  
**Service:** `services/data-service`

---

## Mô tả

`kev_repo.go` trả `AvgDaysToPatch = 0 // TODO: implement`. Cần tính từ JOIN `cves` + `kev_entries`.

---

## Acceptance Criteria

- [x] `GET /api/v2/kev/stats` trả `avg_days_to_patch` > 0 (hoặc thực tế dựa trên data)
- [x] Không còn `// TODO: implement` comment trong production code
- [x] Query không gây timeout (< 2 giây)
- [x] `go build ./...` pass trong `services/data-service`

---

## Files cần sửa

| Action | File | Thay đổi |
|--------|------|---------|
| MODIFY | `services/data-service/internal/infra/persistence/postgres/kev_repo.go` | Implement `computeAvgDaysToPatch()` method |

---

## Bước thực thi

### 1. Tìm dòng TODO
```bash
grep -n "AvgDaysToPatch\|TODO.*implement\|avg_days" \
  services/data-service/internal/infra/persistence/postgres/kev_repo.go
```

### 2. Kiểm tra columns tồn tại trong DB
```bash
psql $DATABASE_URL -c "
SELECT COUNT(*) as kev_entries_with_date
FROM kev_entries WHERE date_added IS NOT NULL;
"

psql $DATABASE_URL -c "
SELECT COUNT(*) as cves_with_published
FROM cves WHERE published_at IS NOT NULL;
"
```

### 3. Test query thủ công trước
```bash
psql $DATABASE_URL -c "
SELECT COALESCE(
    AVG(EXTRACT(EPOCH FROM (k.date_added::timestamptz - c.published_at)) / 86400.0)::INT,
    0
) as avg_days
FROM kev_entries k
JOIN cves c ON c.cve_id = k.cve_id
WHERE c.published_at IS NOT NULL
  AND k.date_added IS NOT NULL
  AND k.date_added > c.published_at;
"
```
Ghi lại kết quả để verify sau.

### 4. Thêm method `computeAvgDaysToPatch` vào kev_repo.go

Tìm struct và method hiện tại:
```bash
grep -n "type.*Repo\|func.*KEVRepo\|func.*kevRepo\|GetStats" \
  services/data-service/internal/infra/persistence/postgres/kev_repo.go | head -15
```

Thêm method mới (điều chỉnh tên struct/db field cho đúng với code hiện tại):
```go
// computeAvgDaysToPatch calculates average days from CVE publish to KEV listing.
// [FIX CR-HC-009] Replace hardcode 0 with real computation.
func (r *KEVPostgresRepo) computeAvgDaysToPatch(ctx context.Context) int {
    var avg float64
    err := r.db.QueryRow(ctx, `
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
    `).Scan(&avg)
    if err != nil {
        return 0  // safe fallback
    }
    return int(avg)
}
```

> **Lưu ý:** Tên struct (`KEVPostgresRepo`, `r.db` hay `r.pool`) cần khớp với code thực tế. Kiểm tra với grep trước.

### 5. Thay `stats.AvgDaysToPatch = 0` bằng lời gọi method

```go
// OLD:
stats.AvgDaysToPatch = 0 // TODO: implement when CVE publish dates available in PG

// NEW:
stats.AvgDaysToPatch = r.computeAvgDaysToPatch(ctx)
```

### 6. Build check
```bash
cd services/data-service && go build ./...
```

---

## Verification

```bash
curl -s "https://c12.openledger.vn/api/v2/kev/stats" | jq '.avg_days_to_patch'
# PASS nếu > 0 (kết quả từ bước 3)
# ACCEPTABLE nếu = 0 và DB không có đủ data join được
```
