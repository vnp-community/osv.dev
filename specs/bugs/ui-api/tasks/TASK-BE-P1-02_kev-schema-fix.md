# TASK-BE-P1-02 — Fix KEV Response Schema

**Phase:** Sprint 2 — P1 Schema Fixes  
**Nguồn giải pháp:** [`solutions/SOL-004_fix-schema-mismatches.md — FIX 3`](../solutions/SOL-004_fix-schema-mismatches.md)  
**Ưu tiên:** 🟠 P1 — KEV Catalog page thiếu stats  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19

---

## Mục tiêu

`GET /api/v2/kev` trả `entries`/`limit` thay vì `data`/`page_size` như spec yêu cầu. Thiếu `stats.added_last_30_days`, `stats.ransomware_related`, `page_size`. Fix cả `GET /api/v2/kev/stats` cho đúng schema.

---

## Files cần sửa

### [MODIFY] `services/data-service/internal/delivery/http/kev_handler.go`

**File hiện tại**: [`services/data-service/internal/delivery/http/kev_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/delivery/http/kev_handler.go)

#### Fix 1 — `ListKEV` (line 59-122): đổi field names

Tại line 109-121, thay `enrichedResponse` struct:

```go
// TRƯỚC (line 101-121):
type enrichedResponse struct {
    Entries interface{}            `json:"entries"`   // ← sai
    Total   int64                  `json:"total"`
    Page    int                    `json:"page"`
    Limit   int                    `json:"limit"`     // ← sai
    HasMore bool                   `json:"has_more"`
    Stats   map[string]interface{} `json:"stats"`
}
respondJSON(w, http.StatusOK, enrichedResponse{
    Entries: resp.Entries,
    // ...
})

// SAU — thay bằng map để kiểm soát field names:
respondJSON(w, http.StatusOK, map[string]interface{}{
    "data":      resp.Entries,   // "entries" → "data"
    "total":     resp.Total,
    "page":      resp.Page,
    "page_size": resp.Limit,     // "limit" → "page_size"
    "stats": map[string]interface{}{
        "total":                   resp.Total,
        "added_last_30_days":      getAddedLast30Days(r.Context(), h.kevRepo),
        "ransomware_related":      getRansomwareCount(r.Context(), h.kevRepo),
        "unmitigated_in_platform": unmitigated,
    },
})
```

#### Thêm helper functions (sau line 155):

```go
// getAddedLast30Days counts KEV entries added in last 30 days.
// Degrades gracefully to 0 on error.
func getAddedLast30Days(ctx context.Context, repo repository.KEVRepository) int {
    since := time.Now().UTC().AddDate(0, 0, -30)
    count, err := repo.CountSince(ctx, since)
    if err != nil {
        return 0
    }
    return int(count)
}

// getRansomwareCount counts KEV entries with known_ransomware_campaign_use = true.
func getRansomwareCount(ctx context.Context, repo repository.KEVRepository) int {
    count, err := repo.CountRansomware(ctx)
    if err != nil {
        return 0
    }
    return int(count)
}
```

**Cần thêm methods vào `KEVRepository` interface nếu chưa có:**
```go
// services/data-service/internal/domain/repository/kev_repository.go
type KEVRepository interface {
    // ... existing methods ...
    CountSince(ctx context.Context, since time.Time) (int64, error) // THÊM
    CountRansomware(ctx context.Context) (int64, error)              // THÊM
}
```

Implement trong PostgreSQL repo:
```go
// services/data-service/internal/infra/postgres/kev_repo.go
func (r *PostgresKEVRepo) CountSince(ctx context.Context, since time.Time) (int64, error) {
    var count int64
    err := r.db.QueryRow(ctx,
        `SELECT COUNT(*) FROM kev_entries WHERE date_added >= $1`, since,
    ).Scan(&count)
    return count, err
}

func (r *PostgresKEVRepo) CountRansomware(ctx context.Context) (int64, error) {
    var count int64
    err := r.db.QueryRow(ctx,
        `SELECT COUNT(*) FROM kev_entries WHERE known_ransomware_campaign_use = true`,
    ).Scan(&count)
    return count, err
}
```

#### Fix 2 — `GetStats` (line 197-204): wrap đúng schema

```go
// TRƯỚC (line 197-204):
func (h *KevHandler) GetStats(w http.ResponseWriter, r *http.Request) {
    stats, err := h.kevRepo.Stats(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, "internal error")
        return
    }
    respondJSON(w, http.StatusOK, stats) // ← trả flat, thiếu by_vendor/recent_additions
}

// SAU:
func (h *KevHandler) GetStats(w http.ResponseWriter, r *http.Request) {
    stats, err := h.kevRepo.Stats(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, "internal error")
        return
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "stats":            stats,
        "by_vendor":        []interface{}{}, // TODO: implement groupby vendor
        "recent_additions": []interface{}{}, // TODO: last 10 added entries
    })
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v2/kev` response có field `data` (không phải `entries`)
- [ ] Response có `page_size` (không phải `limit`)
- [ ] Response có `stats.added_last_30_days` (int, không phải absent)
- [ ] Response có `stats.ransomware_related` (int, không phải absent)
- [ ] `GET /api/v2/kev/stats` trả `{ "stats": {...}, "by_vendor": [], "recent_additions": [] }`

## Verification

```bash
# KEV list schema
curl https://c12.openledger.vn/api/v2/kev | jq 'keys'
# Expected: ["data", "page", "page_size", "stats", "total"]
# NOT:      ["entries", "has_more", "limit", "stats", "total"]

# Stats schema
curl https://c12.openledger.vn/api/v2/kev/stats | jq 'keys'
# Expected: ["by_vendor", "recent_additions", "stats"]

# Stats sub-fields
curl https://c12.openledger.vn/api/v2/kev/stats | jq '.stats | keys'
# Expected: includes "added_last_30_days", "ransomware_related", "total"
```
