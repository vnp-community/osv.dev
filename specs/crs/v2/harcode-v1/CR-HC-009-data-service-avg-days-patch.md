# CR-HC-009: data-service — AvgDaysToPatch Hardcoded Zero

## Trạng thái: 🟡 Medium

## Vấn đề
File: `services/data-service/internal/infra/persistence/postgres/kev_repo.go:327`

```go
stats.AvgDaysToPatch = 0 // TODO: implement when CVE publish dates available in PG
```

`AvgDaysToPatch` là KPI quan trọng trong KEV (Known Exploited Vulnerabilities) statistics.
Luôn trả về 0 làm cho metric này vô nghĩa với security teams.

## Phân tích

`AvgDaysToPatch` = trung bình số ngày từ CVE publish date đến ngày patch available.

Data cần thiết đã có trong DB:
- `kev_entries.date_added` — ngày thêm vào KEV
- `cves.published_at` hoặc `cves.created_at` — ngày CVE được công bố
- `kev_entries.patch_url` — patch URL (indicator có patch chưa)

## Giải pháp

### 1. SQL Query tính AvgDaysToPatch
```sql
SELECT 
    AVG(
        EXTRACT(EPOCH FROM (k.date_added - c.published_at)) / 86400
    )::INT as avg_days_to_patch
FROM kev_entries k
JOIN cves c ON c.cve_id = k.cve_id
WHERE c.published_at IS NOT NULL
  AND k.date_added > c.published_at;
```

### 2. Implement trong kev_repo.go
```go
func (r *KEVRepo) Stats(ctx context.Context) (*KEVStats, error) {
    stats := &KEVStats{}
    
    // ... existing counts ...
    
    // Tính AvgDaysToPatch từ join với cves table
    err := r.db.QueryRowContext(ctx, `
        SELECT COALESCE(AVG(
            EXTRACT(EPOCH FROM (k.date_added - c.published_at)) / 86400
        )::INT, 0)
        FROM kev_entries k
        JOIN cves c ON c.cve_id = k.cve_id
        WHERE c.published_at IS NOT NULL
          AND k.date_added > c.published_at
    `).Scan(&stats.AvgDaysToPatch)
    if err != nil {
        // non-fatal: fallback to 0 với log warning
        r.log.Warn().Err(err).Msg("kev_repo: failed to compute avg_days_to_patch")
    }
    
    return stats, nil
}
```

### 3. Cần kiểm tra schema CVEs table
```sql
-- Kiểm tra column published_at tồn tại không:
SELECT column_name FROM information_schema.columns 
WHERE table_name = 'cves' AND column_name LIKE 'publish%';

-- Nếu chưa có → thêm:
ALTER TABLE cves ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ;
-- Sau đó populate từ CVSS/NVD data
```

## Files cần thay đổi
- `services/data-service/internal/infra/persistence/postgres/kev_repo.go` — implement SQL
- `services/data-service/migrations/005_cves_published_at.sql` [NEW nếu cần]

## Acceptance Criteria
- [ ] `GET /api/v2/kev/stats` trả `avg_days_to_patch` khác 0 (khi có data)
- [ ] Không có hardcoded `= 0` trong production code
- [ ] Fallback graceful về 0 nếu data không đủ (với log warning)
