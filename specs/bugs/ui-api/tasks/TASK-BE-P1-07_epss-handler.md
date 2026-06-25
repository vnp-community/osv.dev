# TASK-BE-P1-07 — Implement EPSS Handler (data-service)

**Phase:** Sprint 3 — P1 Implement Missing  
**Nguồn giải pháp:** [`solutions/SOL-005_implement-missing-endpoints.md — Step 3`](../solutions/SOL-005_implement-missing-endpoints.md)  
**Ưu tiên:** 🟠 P1 — EPSS Analytics page không có data  
**Status:** ✅ **DONE** — 2026-06-19
**Phụ thuộc:** Không có (nhưng P1-06 cần task này hoàn thành)

---

## Mục tiêu

File `services/data-service/internal/delivery/http/epss_handler.go` cần implement đầy đủ 3 handlers: `GetTop`, `GetDistribution`, `GetByCVE`. Data đã có sẵn trong PostgreSQL `cves` table (columns: `epss_score`, `epss_percentile`).

---

## Files cần tạo/sửa

### [CHECK/MODIFY] `services/data-service/internal/delivery/http/epss_handler.go`

```bash
# Kiểm tra file hiện tại có gì
cat services/data-service/internal/delivery/http/epss_handler.go
```

Nếu file chưa đầy đủ, implement:

```go
// services/data-service/internal/delivery/http/epss_handler.go
package http

import (
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "github.com/rs/zerolog"
)

// EPSSRepository provides EPSS data queries.
type EPSSRepository interface {
    GetTopByEPSS(ctx context.Context, limit int) ([]EPSSEntry, error)
    GetDistribution(ctx context.Context) ([]EPSSBucket, error)
    GetByCVEID(ctx context.Context, cveID string) (*EPSSDetail, error)
}

type EPSSEntry struct {
    CveID       string  `json:"cve_id"`
    EPSSScore   float64 `json:"epss_score"`
    Percentile  float64 `json:"percentile"`
    SeverityV3  string  `json:"severity_v3,omitempty"`
    Description string  `json:"description,omitempty"`
}

type EPSSBucket struct {
    RangeMin float64 `json:"range_min"`
    RangeMax float64 `json:"range_max"`
    Count    int64   `json:"count"`
}

type EPSSDetail struct {
    CveID      string  `json:"cve_id"`
    EPSSScore  float64 `json:"epss_score"`
    Percentile float64 `json:"percentile"`
    ModelDate  string  `json:"model_date"`
}

type EPSSHandler struct {
    repo EPSSRepository
    log  zerolog.Logger
}

func NewEPSSHandler(repo EPSSRepository, log zerolog.Logger) *EPSSHandler {
    return &EPSSHandler{repo: repo, log: log}
}

// GET /api/v2/epss/top?limit=20&severity=critical
func (h *EPSSHandler) GetTop(w http.ResponseWriter, r *http.Request) {
    limit := 20
    if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 100 {
        limit = l
    }

    results, err := h.repo.GetTopByEPSS(r.Context(), limit)
    if err != nil {
        h.log.Error().Err(err).Msg("get top EPSS failed")
        respondError(w, http.StatusInternalServerError, "failed to get top EPSS scores")
        return
    }
    if results == nil {
        results = []EPSSEntry{}
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":  results,
        "total": len(results),
        "limit": limit,
    })
}

// GET /api/v2/epss/distribution
func (h *EPSSHandler) GetDistribution(w http.ResponseWriter, r *http.Request) {
    buckets, err := h.repo.GetDistribution(r.Context())
    if err != nil {
        h.log.Error().Err(err).Msg("get EPSS distribution failed")
        respondError(w, http.StatusInternalServerError, "failed to get EPSS distribution")
        return
    }
    if buckets == nil {
        buckets = []EPSSBucket{}
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "distribution": buckets,
        "model_date":   "current",
    })
}

// GET /api/v2/epss/{cveId}
func (h *EPSSHandler) GetByCVE(w http.ResponseWriter, r *http.Request) {
    cveID := chi.URLParam(r, "cveId")
    if cveID == "" {
        respondError(w, http.StatusBadRequest, "cveId is required")
        return
    }

    result, err := h.repo.GetByCVEID(r.Context(), cveID)
    if err != nil {
        respondError(w, http.StatusNotFound, "CVE not found or no EPSS data")
        return
    }

    respondJSON(w, http.StatusOK, result)
}
```

### [FIND & MODIFY] PostgreSQL repository — thêm EPSS methods

```bash
# Tìm CVE repo
grep -r "GetTopEPSS\|epss_score\|EPSSRepository" \
  services/data-service/ --include="*.go" -l
```

Thêm vào CVE repo (thường trong `infra/postgres/`):

```go
// Implement GetTopByEPSS
func (r *PostgresCVERepo) GetTopByEPSS(ctx context.Context, limit int) ([]EPSSEntry, error) {
    rows, err := r.db.Query(ctx, `
        SELECT cve_id, epss_score, epss_percentile, severity_v3,
               LEFT(description, 200) as description
        FROM cves
        WHERE epss_score IS NOT NULL AND epss_score > 0
        ORDER BY epss_score DESC
        LIMIT $1
    `, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var results []EPSSEntry
    for rows.Next() {
        var e EPSSEntry
        if err := rows.Scan(&e.CveID, &e.EPSSScore, &e.Percentile,
            &e.SeverityV3, &e.Description); err != nil {
            continue
        }
        results = append(results, e)
    }
    return results, nil
}

// Implement GetDistribution — 10 buckets (0.0-0.1, 0.1-0.2, ... 0.9-1.0)
func (r *PostgresCVERepo) GetDistribution(ctx context.Context) ([]EPSSBucket, error) {
    rows, err := r.db.Query(ctx, `
        SELECT 
            FLOOR(epss_score * 10) / 10 as range_min,
            FLOOR(epss_score * 10) / 10 + 0.1 as range_max,
            COUNT(*) as count
        FROM cves
        WHERE epss_score IS NOT NULL
        GROUP BY FLOOR(epss_score * 10)
        ORDER BY range_min
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var buckets []EPSSBucket
    for rows.Next() {
        var b EPSSBucket
        rows.Scan(&b.RangeMin, &b.RangeMax, &b.Count)
        buckets = append(buckets, b)
    }
    return buckets, nil
}

// Implement GetByCVEID
func (r *PostgresCVERepo) GetByCVEID(ctx context.Context, cveID string) (*EPSSDetail, error) {
    var e EPSSDetail
    err := r.db.QueryRow(ctx, `
        SELECT cve_id, epss_score, epss_percentile,
               TO_CHAR(modified_at, 'YYYY-MM-DD') as model_date
        FROM cves WHERE cve_id = $1 AND epss_score IS NOT NULL
    `, cveID).Scan(&e.CveID, &e.EPSSScore, &e.Percentile, &e.ModelDate)
    if err != nil {
        return nil, err
    }
    return &e, nil
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v2/epss/top?limit=10` trả HTTP 200 với `{ "data": [...], "total": 10 }`
- [ ] `data[0]` có `cve_id`, `epss_score`, `percentile`
- [ ] `GET /api/v2/epss/distribution` trả HTTP 200 với `{ "distribution": [...] }`
- [ ] `GET /api/v2/epss/CVE-2021-44228` trả HTTP 200 hoặc 404 (tùy data có trong DB)
- [ ] Data đến từ PostgreSQL `cves` table (không phải hardcode)

## Verification

```bash
# Top EPSS
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v2/epss/top?limit=5" | jq '.data[0]'
# Expected: { "cve_id": "CVE-...", "epss_score": 0.97, "percentile": 0.999 }

# Distribution
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v2/epss/distribution | jq '.distribution | length'
# Expected: > 0

# By CVE (nếu có data)
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v2/epss/CVE-2021-44228
# Expected: HTTP 200 hoặc 404
```
