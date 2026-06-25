# BUG-011 — Data Service: Hardcoded Partial Responses cho KEV và EPSS Stats

## Metadata
- **ID**: BUG-011
- **Service**: `data-service`
- **Files**:
  - [`internal/delivery/http/kev_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/delivery/http/kev_handler.go)
  - [`internal/delivery/http/epss_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/delivery/http/epss_handler.go)
- **Lines**: kev_handler L199-200, epss_handler L142
- **Severity**: Medium
- **Category**: Hardcode / Incomplete Data
- **Status**: Open

## Mô tả

Hai endpoints trả về response với các fields được hardcode thành empty arrays, thay vì
thực sự query data:

### 1. KEV Stats — GET /api/v2/kev/stats

```go
// kev_handler.go:199-200
"by_vendor":        []interface{}{}, // TODO: GetStatsByVendor
"recent_additions": []interface{}{}, // TODO: last 10 added entries
```

Response trả về:
```json
{
  "total": 1200,
  "by_vendor": [],           // always empty
  "recent_additions": []     // always empty
}
```

### 2. EPSS History — GET /api/v2/epss/{cve_id}

```go
// epss_handler.go:142
"history": []interface{}{}, // TODO: implement GetHistory(ctx, cveID, 90)
```

Response trả về:
```json
{
  "cve_id": "CVE-2024-XXXX",
  "score": 0.95,
  "history": []   // always empty — no historical trend data
}
```

## Tác động

1. **UI bugs**: Dashboard hiển thị "No vendors" trong KEV stats mặc dù có data.
   EPSS chart không hiển thị trend vì history luôn empty.
2. **Client confusion**: API trả về 200 OK với empty arrays — client không biết
   là chức năng chưa được implement hay thực sự không có data.
3. **SLA metric error**: KEV stats được dùng để tính SLA metrics trong sla-service.
   Empty `by_vendor` ảnh hưởng đến dashboard aggregation.

## Fix Proposal

### Trả về 501 Not Implemented cho sub-fields chưa implement

```go
// OPTION A: Omit unimplemented fields và document clearly
respondJSON(w, http.StatusOK, map[string]interface{}{
    "total":      stats.Total,
    "by_severity": stats.BySeverity,
    // "by_vendor": không include cho đến khi implement
    // "recent_additions": không include
})

// OPTION B: Implement GetStatsByVendor trong repo
stats, err := h.kevRepo.GetStats(ctx)
// Nếu bao gồm by_vendor, phải query thực sự
```

### Implement missing repo methods

```go
// kev_repository.go
type KEVRepository interface {
    ...
    GetStatsByVendor(ctx context.Context, limit int) ([]VendorStat, error)
    GetRecentAdditions(ctx context.Context, n int) ([]KEVEntry, error)
    GetEPSSHistory(ctx context.Context, cveID string, days int) ([]EPSSPoint, error)
}
```

### Nếu chưa implement: trả về cấu trúc rõ ràng

```go
respondJSON(w, http.StatusOK, map[string]interface{}{
    "total":            stats.Total,
    "by_vendor":        nil,    // null thay vì []
    "_unimplemented":   []string{"by_vendor", "recent_additions"},
})
```

## References

- [kev_handler.go L199-200](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/delivery/http/kev_handler.go#L199-L200)
- [epss_handler.go L142](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/delivery/http/epss_handler.go#L142)
- [data-service/internal/infra/persistence/postgres/kev_repo.go L327](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/infra/persistence/postgres/kev_repo.go#L327)
