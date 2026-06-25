# TASK-BE-P0-03 — Fix Findings List/Stats 500

**Phase:** Sprint 1 — P0 Unblock  
**Nguồn giải pháp:** [`solutions/SOL-003_fix-findings-sla-500.md`](../solutions/SOL-003_fix-findings-sla-500.md)  
**Ưu tiên:** 🔴 P0 — Blocking (trang Findings hoàn toàn không load)  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19

---

## Mục tiêu

Fix `GET /api/v1/findings` trả 500 `"failed to list findings"` và `GET /api/v1/findings/stats` trả 400.

---

## Điều tra trước khi code

```bash
# 1. Xem logs finding-service để biết lỗi cụ thể
docker logs osv-backend-finding-service-1 --tail 200 | grep -i "error\|finding\|sql\|panic"

# 2. Kiểm tra bảng findings có tồn tại không
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "SELECT COUNT(*) FROM findings;"

# 3. Tìm handler file
grep -r "ListFindings\|/api/v1/findings\|failed to list findings" \
  services/finding-service/ --include="*.go" -l

# 4. Test trực tiếp (bypass gateway)
docker exec osv-backend-gateway-1 \
  curl "http://finding-service:8085/api/v1/findings?page=1&page_size=10"
```

---

## Files cần sửa

### [FIND & MODIFY] finding-service list handler

```bash
# Tìm handler
grep -r "failed to list findings" services/finding-service/ --include="*.go"
```

**Fix 1: Query param parsing** — handler có thể dùng `pageSize` thay vì `page_size`:

```go
// TRƯỚC (chỉ nhận camelCase — không đúng spec):
pageSize := r.URL.Query().Get("pageSize")

// SAU (nhận cả hai):
pageSize := r.URL.Query().Get("page_size")
if pageSize == "" {
    pageSize = r.URL.Query().Get("pageSize")
}
page := r.URL.Query().Get("page")
if page == "" {
    page = "1"
}
```

**Fix 2: SQL query** — kiểm tra và fix JOIN nếu cần:

```go
// Nếu SQL query fail vì JOIN với bảng chưa có data:
// Tìm query trong infra/postgres/finding_repo.go
grep -r "FROM findings\|JOIN.*engagements\|JOIN.*tests" \
  services/finding-service/ --include="*.go"
```

Nếu query đang JOIN bắt buộc với engagements/tests (empty tables), đổi sang LEFT JOIN:
```sql
-- TRƯỚC (INNER JOIN → fail khi bảng rỗng):
SELECT f.* FROM findings f
JOIN tests t ON f.test_id = t.id
JOIN engagements e ON t.engagement_id = e.id

-- SAU (LEFT JOIN — không fail khi rỗng):
SELECT f.* FROM findings f
LEFT JOIN tests t ON f.test_id = t.id
LEFT JOIN engagements e ON t.engagement_id = e.id
WHERE ($1 = '' OR f.status = $1)
ORDER BY f.created_at DESC
LIMIT $2 OFFSET $3
```

**Fix 3: Response format** — wrap đúng theo spec:

```go
func (h *FindingHandler) ListFindings(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    page, _     := strconv.Atoi(q.Get("page"))
    if page < 1 { page = 1 }
    pageSize, _ := strconv.Atoi(q.Get("page_size"))
    if pageSize < 1 { pageSize = 20 }
    if pageSize > 100 { pageSize = 100 }

    filter := FindingFilter{
        Status:   q.Get("status"),
        Severity: q.Get("severity"),
        Page:     page,
        PageSize: pageSize,
    }

    findings, total, err := h.findingRepo.List(r.Context(), filter)
    if err != nil {
        log.Error().Err(err).Msg("list findings failed")
        respondError(w, http.StatusInternalServerError, "failed to list findings")
        return
    }

    // Đếm theo severity (cho stats widget)
    bySeverity := map[string]int{
        "Critical": 0, "High": 0, "Medium": 0, "Low": 0, "Info": 0,
    }
    for _, f := range findings {
        bySeverity[f.Severity]++
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "findings":    findings,
        "total":       total,
        "page":        page,
        "page_size":   pageSize,
        "by_severity": bySeverity,
    })
}
```

### [FIND & MODIFY] findings/stats handler — fix 400 error

```bash
# Tìm stats handler
grep -r "stats\|/findings/stats" services/finding-service/ --include="*.go" -l
```

`GET /api/v1/findings/stats` trả 400 — có thể endpoint yêu cầu required param. Fix để params là optional:

```go
// stats handler — không yêu cầu bất kỳ param bắt buộc nào
func (h *FindingHandler) GetStats(w http.ResponseWriter, r *http.Request) {
    // product_id là optional
    productID := r.URL.Query().Get("product_id")

    var stats FindingStats
    var err error
    if productID != "" {
        stats, err = h.findingRepo.StatsByProduct(r.Context(), productID)
    } else {
        stats, err = h.findingRepo.StatsGlobal(r.Context())
    }

    if err != nil {
        log.Error().Err(err).Msg("get stats failed")
        respondError(w, http.StatusInternalServerError, "failed to get stats")
        return
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "by_severity": stats.BySeverity,
        "by_status":   stats.ByStatus,
        "sla_stats": map[string]interface{}{
            "breached": stats.SLABreached,
            "at_risk":  stats.SLAAtRisk,
            "ok":       stats.SLAOK,
        },
        "total_active": stats.TotalActive,
    })
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/findings?page=1&page_size=10` trả HTTP 200 (không còn 500)
- [ ] Response có `{ "findings": [], "total": 0, "page": 1, "page_size": 10 }`
- [ ] `GET /api/v1/findings?status=active` hoạt động
- [ ] `GET /api/v1/findings/stats` trả HTTP 200 (không còn 400)
- [ ] `GET /api/v1/findings/stats?product_id=<uuid>` cũng hoạt động

## Verification

```bash
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/findings?page=1&page_size=10"
# Expected: HTTP 200

curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/findings/stats
# Expected: HTTP 200 (không phải 400)
```
