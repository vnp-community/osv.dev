# SOL-003 — Findings API 500: Debug & Fix (P0 — CRITICAL)

**Bug**: [BUG-003](../BUG-003-findings.md)  
**Service**: `finding-service` (`services/finding-service`)  
**Endpoint**: `GET /api/v1/findings?page=1&pageSize=50`  
**HTTP Error**: `500 Internal Server Error`

**Status**: `✅ Implemented` — via [TASK-001](../../tasks/TASK-001-*.md)

---

## Root Cause Analysis

### Routing (Gateway)

Route đã đúng trong [`apps/osv/internal/gateway/router.go:153`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L153):

```go
mux.Handle("GET /api/v1/findings", protected(proxy.Forward("finding-service:8085")))
```

### Handler

Handler trong [`finding_handler.go:67`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/finding_handler.go#L67) gọi:

```go
res, err := h.repo.List(r.Context(), filter)
if err != nil {
    respondError(w, http.StatusInternalServerError, "failed to list findings")
    return
}
```

### SQL Query — Vấn đề nghi ngờ

Query trong [`finding_repo.go:151`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/infra/postgres/finding_repo.go#L151) có `LEFT JOIN jira_issues` và `LEFT JOIN jira_configs`. Các table này có thể chưa tồn tại hoặc chưa có dữ liệu khi database mới setup.

```sql
FROM findings f
JOIN products p ON p.id = f.product_id    -- ← INNER JOIN: fails nếu không có product
LEFT JOIN jira_issues ji ON ji.finding_id = f.id
LEFT JOIN jira_configs jc ON jc.product_id = f.product_id
```

**Các nguyên nhân có thể gây 500:**

1. **Table `jira_issues` hoặc `jira_configs` không tồn tại** (schema `osv_jira` chưa init)
2. **`findings` thiếu column** như `asset_ip`, `asset_hostname`, `epss_score`, `is_kev` (schema mới thêm nhưng migration chưa chạy)
3. **`products` không tồn tại** — `INNER JOIN products` sẽ fail nếu bảng products trống hoặc không tồn tại

---

## Giải pháp

### Bước 1: Kiểm tra logs finding-service

```bash
# Trong container/server
docker logs finding-service --tail 50 | grep -i "error\|panic\|500"

# Hoặc check database connection
docker exec finding-service cat /app/logs/app.log | grep "List\|findings\|query"
```

### Bước 2: Kiểm tra migration đã chạy chưa

```bash
# Kiểm tra tất cả columns tồn tại
docker exec -it postgres psql -U postgres -d osv -c "
  SELECT column_name, data_type
  FROM information_schema.columns
  WHERE table_schema = 'osv_finding'
    AND table_name = 'findings'
  ORDER BY ordinal_position;
"

# Kiểm tra tables jira_issues, jira_configs tồn tại
docker exec -it postgres psql -U postgres -d osv -c "
  SELECT table_name FROM information_schema.tables
  WHERE table_schema = 'osv_finding'
  ORDER BY table_name;
"
```

### Bước 3: Fix SQL Query — Sửa INNER JOIN thành LEFT JOIN

File: [`services/finding-service/internal/infra/postgres/finding_repo.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/infra/postgres/finding_repo.go#L168)

```go
// TRƯỚC (có thể crash nếu products thiếu)
FROM findings f
JOIN products p ON p.id = f.product_id    // INNER JOIN

// SAU (an toàn hơn)
FROM findings f
LEFT JOIN products p ON p.id = f.product_id   // LEFT JOIN
LEFT JOIN jira_issues ji ON ji.finding_id = f.id
LEFT JOIN jira_configs jc ON jc.product_id = f.product_id
```

Thêm null-safe cho product_name trong scan:

```go
// Thêm vào scan: &fm.ProductName → dùng *string hoặc sql.NullString
var productName sql.NullString
err := rows.Scan(
    // ... other fields ...
    &productName,   // thay vì &fm.ProductName trực tiếp
    // ...
)
fm.ProductName = productName.String  // "" nếu NULL
```

### Bước 4: Thêm defensive check trong handler

File: [`services/finding-service/internal/delivery/http/finding_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/finding_handler.go#L108)

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

// Defensive: đảm bảo Findings không nil
if res == nil {
    res = &finding.FindingListResult{Findings: []*finding.FindingWithMeta{}}
}
if res.Findings == nil {
    res.Findings = []*finding.FindingWithMeta{}
}
```

### Bước 5: Đảm bảo response trả array không null

File: [`services/finding-service/internal/delivery/http/finding_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/finding_handler.go#L115)

```go
// TRƯỚC
responses := make([]FindingListItem, 0, len(res.Findings))

// Trả về response — Findings luôn là array (không phải null)
respondJSON(w, http.StatusOK, &FindingListResponse{
    Findings: responses,  // ← `responses` là []FindingListItem, không bao giờ nil
    // ...
})
```

`make([]FindingListItem, 0, ...)` đã đúng — tạo empty slice không phải nil, JSON sẽ serialize thành `[]` thay vì `null`.

---

## Verification

```bash
# Test endpoint sau khi fix
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/findings?page=1&pageSize=50"

# Expected: 200 OK với
# {
#   "findings": [],    ← array (kể cả empty)
#   "total": 0,
#   ...
# }
```

---

## Files cần sửa

| File | Thay đổi |
|------|----------|
| [`finding_repo.go:168`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/infra/postgres/finding_repo.go#L168) | `JOIN products` → `LEFT JOIN products` |
| [`finding_handler.go:108`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/finding_handler.go#L108) | Thêm error logging chi tiết + nil guard |
| `migrations/` | Kiểm tra và chạy migration còn thiếu |
