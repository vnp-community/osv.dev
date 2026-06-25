# TASK-GCV-006 — EPSS Filter + Sort in Search Service

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-006 |
| **Service** | `search-service` |
| **CR** | CR-GCV-002 |
| **Phase** | 1 — Core Pipeline |
| **Priority** | 🔴 High |
| **Prerequisites** | — |

## Context

`search-service` hiện có `SearchCVEs` handler với params `query`, `severity`, `sort`, `page`, `limit`. Task này thêm `min_epss` filter và `sort=epss_desc` option. Cũng cần đảm bảo EPSS fields có trong response entity và DB columns tồn tại.

## Reference

- Solution: [SOL-GCV-002](../solutions/SOL-GCV-002-epss-integration.md)
- CR: [CR-GCV-002](../CR-GCV-002-epss-integration.md)

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/usecase/cvesearch/
        (đọc cấu trúc usecase, tìm Request struct và usecase Execute)
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/delivery/http/search_handler.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/migrations/XXXX_add_epss_index.sql
```

**Lưu ý**: Đọc toàn bộ `search-service/internal/usecase/cvesearch/` để xác định tên file chính xác (có thể là `usecase.go`, `request.go`, `response.go`).

## Implementation Spec

### Request struct — ADD MinEPSS

Tìm Request struct trong usecase/cvesearch, ADD fields:

```go
type Request struct {
    // --- EXISTING fields (giữ nguyên) ---
    Query    string
    Severity string
    Source   string
    Sort     string   // "newest" | "oldest" | "severity_desc" | "epss_desc" (NEW value)
    Page     int
    Limit    int

    // --- NEW ---
    MinEPSS *float64  // filter: only CVEs with epss >= MinEPSS
    MaxEPSS *float64  // optional upper bound (rarely used)
}
```

### UseCase Execute — ADD filter + sort logic

Trong `Execute` method, thêm filter/sort EPSS:

```go
// Sau khi build base query, thêm:

// EPSS filter
if req.MinEPSS != nil {
    query = query.Where("epss >= ?", *req.MinEPSS)
    // hoặc dùng raw SQL nếu không dùng query builder:
    // conditions = append(conditions, fmt.Sprintf("epss >= %f", *req.MinEPSS))
}

// Sort EPSS descending
switch req.Sort {
case "epss_desc":
    query = query.OrderBy("epss DESC NULLS LAST")
// giữ nguyên các case hiện có: "newest", "oldest", "severity_desc"
}
```

### HTTP Handler — Parse min_epss param

Trong `SearchCVEs` handler, thêm:

```go
func (h *Handler) SearchCVEs(w http.ResponseWriter, r *http.Request) {
    req := &search.Request{
        // --- existing ---
        Query:    r.URL.Query().Get("query"),
        Severity: strings.ToUpper(r.URL.Query().Get("severity")),
        Source:   strings.ToUpper(r.URL.Query().Get("source")),
        Sort:     r.URL.Query().Get("sort"),
        Page:     parseInt(r.URL.Query().Get("page"), 0),
        Limit:    parseInt(r.URL.Query().Get("limit"), 50),
    }

    // --- NEW: EPSS filter ---
    if minEPSS := r.URL.Query().Get("min_epss"); minEPSS != "" {
        if v, err := strconv.ParseFloat(minEPSS, 64); err == nil && v >= 0 && v <= 1 {
            req.MinEPSS = &v
        }
    }

    // --- existing execute ---
    resp, err := h.searchUC.Execute(r.Context(), req)
    // ...
}
```

### Migration SQL

```sql
-- Ensure EPSS columns exist in search-service DB
-- (data-service may already have these, but search-service may use same or separate DB)
ALTER TABLE cves ADD COLUMN IF NOT EXISTS epss NUMERIC(8,7) DEFAULT NULL;
ALTER TABLE cves ADD COLUMN IF NOT EXISTS epss_percentile NUMERIC(8,7) DEFAULT NULL;
ALTER TABLE cves ADD COLUMN IF NOT EXISTS epss_updated_at TIMESTAMPTZ DEFAULT NULL;

-- Index for EPSS filtering and sorting
CREATE INDEX IF NOT EXISTS idx_cves_epss ON cves(epss DESC NULLS LAST)
    WHERE epss IS NOT NULL;
```

## Acceptance Criteria

- [x] `GET /api/v2/cves?min_epss=0.5` → chỉ trả CVEs có `epss >= 0.5`
- [x] `GET /api/v2/cves?min_epss=invalid` → ignored (không 400, không panic)
- [x] `GET /api/v2/cves?sort=epss_desc` → sorted by EPSS descending, nulls last
- [x] `GET /api/v2/cves?min_epss=0.9&sort=epss_desc` → high-risk CVEs sorted
- [x] Response mỗi CVE có `epss` và `epss_percentile` trong JSON (nếu có giá trị)
- [x] `sort=epss_desc` kết hợp với `severity=CRITICAL` → cả hai filter áp dụng cùng lúc
- [x] Migration chạy không error trên DB sạch (IF NOT EXISTS)
- [x] `go build ./...` pass không lỗi
