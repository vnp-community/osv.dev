# TASK-001 — finding-service: Fix `INNER JOIN` → `LEFT JOIN` (Findings 500)

**Bug**: [BUG-003](../BUG-003-findings.md)  
**Solution**: [SOL-003](../solutions/SOL-003-findings-500.md)  
**Priority**: 🔴 P0  
**Effort**: ~15 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/findings` trả `500 Internal Server Error`. Root cause là SQL query trong `finding_repo.go` dùng `INNER JOIN products` — nếu product bị xoá hoặc schema mới chưa có data, query sẽ trả 0 rows nhưng không crash. Vấn đề thực sự là LEFT JOIN với `jira_issues`/`jira_configs` ở schema `osv_jira` có thể chưa tồn tại khi deploy mới.

---

## File cần sửa

**File 1**: [`services/finding-service/internal/infra/postgres/finding_repo.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/infra/postgres/finding_repo.go)

**File 2**: [`services/finding-service/internal/delivery/http/finding_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/finding_handler.go)

---

## Thay đổi 1 — finding_repo.go (dòng ~168)

**Tìm** (trong hàm `List`, phần FROM clause của CTE):

```sql
    FROM findings f
    JOIN products p ON p.id = f.product_id
    LEFT JOIN jira_issues ji ON ji.finding_id = f.id
    LEFT JOIN jira_configs jc ON jc.product_id = f.product_id
```

**Thay bằng**:

```sql
    FROM findings f
    LEFT JOIN products p ON p.id = f.product_id
    LEFT JOIN jira_issues ji ON ji.finding_id = f.id
    LEFT JOIN jira_configs jc ON jc.product_id = f.product_id
```

Đồng thời sửa scan `product_name` để chấp nhận NULL:

**Tìm** trong phần `rows.Scan(...)`:

```go
            &fm.ProductName, &fm.JiraIssueKey, &fm.JiraURL,
```

**Thay bằng**:

```go
            &fm.ProductName, &fm.JiraIssueKey, &fm.JiraURL,
            // (ProductName là *string hoặc sql.NullString để chấp nhận NULL)
```

Nếu `fm.ProductName` là `string` (không phải pointer), đổi sang `*string` trong struct `FindingWithMeta`:

```go
// domain/finding/finding.go hoặc tương đương
type FindingWithMeta struct {
    *Finding
    ProductName  string  `json:"product_name"` // đổi sang:
    ProductName  *string `json:"product_name,omitempty"`
    JiraIssueKey *string `json:"jira_issue_key,omitempty"`
    JiraURL      *string `json:"jira_url,omitempty"`
}
```

---

## Thay đổi 2 — finding_handler.go (dòng ~108)

**Tìm**:

```go
    res, err := h.repo.List(r.Context(), filter)
    if err != nil {
        h.log.Error().Err(err).Msg("FindingHandler.List")
        respondError(w, http.StatusInternalServerError, "failed to list findings")
        return
    }
```

**Thay bằng**:

```go
    res, err := h.repo.List(r.Context(), filter)
    if err != nil {
        h.log.Error().Err(err).
            Str("product_id", r.URL.Query().Get("product_id")).
            Str("severity", r.URL.Query().Get("severity")).
            Msg("FindingHandler.List: repo error")
        respondError(w, http.StatusInternalServerError, "failed to list findings")
        return
    }

    // Defensive nil guard
    if res == nil {
        res = &finding.FindingListResult{Findings: []*finding.FindingWithMeta{}}
    }
    if res.Findings == nil {
        res.Findings = []*finding.FindingWithMeta{}
    }
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/findings` trả `200 OK` với `findings: []` thay vì `500`
- [ ] Log của finding-service hiển thị query thành công (không có stack trace)
- [ ] Response `findings` luôn là array (kể cả empty)
- [ ] Không có TypeScript hoặc compile error mới

---

## Verify

```bash
# Build finding-service
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service
go build ./...

# Test endpoint
curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/findings?page=1&pageSize=20" \
  | jq '{status: .status, findings_type: (.findings | type)}'
# Expected: {"findings_type": "array"}
```
