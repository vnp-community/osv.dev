# TASK-007 — Fix BUG-011: KEV & EPSS Stats Không Trả Về Empty Arrays Sai

> **Bug**: BUG-011  
> **Priority**: 🟡 Medium — Dashboard hiển thị "No vendors" dù có data; EPSS chart luôn trống  
> **Trạng thái**: ✅ DONE — 2026-06-22  
> **Ghi chú**: Xóa `"by_vendor": []interface{}{}` và `"recent_additions": []interface{}{}` khỏi `kev_handler.go GetStats`. Xóa `"history": []interface{}{}` khỏi `epss_handler.go GetEPSSByCVE`. Thêm `_meta.partial` và `_meta.unimplemented_fields` để client biết các fields chưa implement. Build pass.
> **Depends on**: không có dependency  
> **Solution ref**: [SOL-GROUP-B](../solutions/SOL-GROUP-B-gateway-bff-data.md#bug-011)

## Files Cần Đọc Trước

```
services/data-service/internal/delivery/http/kev_handler.go       (lines 185-210)
services/data-service/internal/delivery/http/epss_handler.go      (lines 130-155)
services/data-service/internal/infra/persistence/postgres/kev_repo.go (lines 320-340)
services/data-service/internal/domain/repository/                  (xem KEVRepository interface)
```

## Strategy

**Áp dụng Option A (immediate fix)** — bỏ các fields chưa implement khỏi response,
thêm `_meta` để client biết. Không cần implement DB methods ngay.

**Option B** (full implementation) là bonus nếu thời gian cho phép.

## Files Sẽ Bị Sửa

```
services/data-service/internal/delivery/http/kev_handler.go   [MODIFY]
services/data-service/internal/delivery/http/epss_handler.go  [MODIFY]
```

## Thay Đổi Chi Tiết

### Bước 1: Đọc `kev_handler.go` — tìm `GetKEVStats` handler

```bash
grep -n "by_vendor\|recent_additions\|GetStats\|respondJSON" \
    services/data-service/internal/delivery/http/kev_handler.go
```

Tìm đoạn:
```go
"by_vendor":        []interface{}{}, // TODO: GetStatsByVendor
"recent_additions": []interface{}{}, // TODO: last 10 added entries
```

**Option A — Bỏ fields chưa implement, thêm metadata**:

```go
// [FIX] Bỏ by_vendor và recent_additions khỏi response cho đến khi implement.
// Dùng _meta.unimplemented để client biết các fields này sẽ có trong tương lai.
respondJSON(w, http.StatusOK, map[string]interface{}{
    "total":       stats.Total,
    "by_severity": stats.BySeverity,
    // "by_vendor":        REMOVED — implement via GetStatsByVendor() first
    // "recent_additions": REMOVED — implement via GetRecentAdditions() first
    "_meta": map[string]interface{}{
        "partial":              true,
        "unimplemented_fields": []string{"by_vendor", "recent_additions"},
    },
})
```

**Option B (bonus) — Implement `GetStatsByVendor` trong repo**:

Nếu chọn implement đầy đủ, thêm vào `KEVRepository` interface:
```go
GetStatsByVendor(ctx context.Context, limit int) ([]VendorStat, error)
GetRecentAdditions(ctx context.Context, n int) ([]KEVEntry, error)
```

Implement trong `kev_repo.go`:
```go
func (r *PostgresKEVRepository) GetStatsByVendor(ctx context.Context, limit int) ([]VendorStat, error) {
    if limit <= 0 {
        limit = 10
    }
    const q = `
        SELECT c.vendor, COUNT(*) AS count
        FROM kev_entries ke
        JOIN cves c ON c.cve_id = ke.cve_id
        WHERE c.vendor IS NOT NULL AND c.vendor != ''
        GROUP BY c.vendor
        ORDER BY count DESC
        LIMIT $1
    `
    rows, err := r.db.Query(ctx, q, limit)
    if err != nil {
        return nil, fmt.Errorf("GetStatsByVendor: %w", err)
    }
    defer rows.Close()

    var results []VendorStat
    for rows.Next() {
        var s VendorStat
        if err := rows.Scan(&s.Vendor, &s.Count); err != nil {
            return nil, err
        }
        results = append(results, s)
    }
    return results, rows.Err()
}
```

Sau đó update handler để call repo method thực:
```go
byVendor, err := h.repo.GetStatsByVendor(ctx, 10)
if err != nil {
    log.Warn().Err(err).Msg("GetStatsByVendor failed, omitting field")
    // Vẫn trả về response nhưng không có by_vendor
} else {
    response["by_vendor"] = byVendor
}
```

### Bước 2: Đọc `epss_handler.go` — tìm handler trả về history

```bash
grep -n "history\|GetHistory\|respondJSON" \
    services/data-service/internal/delivery/http/epss_handler.go
```

Tìm đoạn:
```go
"history": []interface{}{}, // TODO: implement GetHistory(ctx, cveID, 90)
```

**Option A — Bỏ field, thêm metadata**:
```go
respondJSON(w, http.StatusOK, map[string]interface{}{
    "cve_id":     cveID,
    "score":      score.Score,
    "percentile": score.Percentile,
    // "history": REMOVED — not yet implemented
    "_meta": map[string]interface{}{
        "partial":              true,
        "unimplemented_fields": []string{"history"},
    },
})
```

### Bước 3: Xóa TODO comments đã được xử lý

Sau khi implement (Option A hoặc B), xóa hoặc update TODO comments:
```go
// BEFORE:
"by_vendor": []interface{}{}, // TODO: GetStatsByVendor

// AFTER Option A: field bị bỏ, không cần comment
// AFTER Option B: field được implement, comment không còn cần thiết
```

## Verification

```bash
# Build
go build ./services/data-service/...

# Test: KEV stats không còn trả về empty arrays
curl http://localhost:8082/api/v2/kev/stats | jq .
# Option A: by_vendor và recent_additions không có trong response
# → {"total": 1200, "by_severity": {...}, "_meta": {"partial": true, ...}}

# Option B: by_vendor có data thực
# → {"total": 1200, "by_vendor": [{"vendor":"...", "count":...}], ...}

# Test: EPSS history field bị bỏ (Option A) hoặc có data (Option B)
curl http://localhost:8082/api/v2/epss/CVE-2024-1234 | jq .
# Option A: history không có trong response + _meta
# Option B: history có data thực
```

## Acceptance Criteria

**Option A (minimum)**:
- [ ] `by_vendor` và `recent_additions` không còn là `[]` trong KEV stats response
- [ ] `history` không còn là `[]` trong EPSS response
- [ ] Response có `_meta.partial: true` khi fields bị bỏ
- [ ] TODO comments được xóa hoặc update

**Option B (bonus)**:
- [ ] `GetStatsByVendor` được implement với query thực
- [ ] `GetRecentAdditions` được implement
- [ ] Response có real data thay vì placeholder
- [ ] Unit tests cho repo methods
