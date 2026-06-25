# BUG-008 — Finding Service: Hardcoded Pagination Defaults Không Nhất Quán

## Metadata
- **ID**: BUG-008
- **Service**: `finding-service`
- **Files**:
  - [`internal/delivery/http/finding_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/finding_handler.go)
  - [`internal/delivery/http/product_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/product_handler.go)
  - [`internal/delivery/http/internal_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/internal_handler.go)
  - [`internal/infra/postgres/product_repo.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/infra/postgres/product_repo.go)
- **Severity**: Medium
- **Category**: Hardcode / Pagination
- **Status**: Open

## Mô tả

Nhiều magic numbers cho pagination được scatter khắp finding-service mà không có
một constant dùng chung:

```go
// finding_handler.go:74 — max limit khi limit > 200
limit = 200

// finding_handler.go:83 — default limit
limit = parseIntParam(r, "limit", 20)

// finding_handler.go:86 — max clamp lại
limit = 200

// product_handler.go:81 — max limit
limit = 200

// product_handler.go:89 — default limit
limit = parseIntParam(r, "limit", 50)  // khác với finding_handler!

// internal_handler.go:70-71 — SLA breaches max limit
if limit <= 0 || limit > 50 {
    limit = 5    // default riêng biệt
}

// report_handler.go:153
limit := intOr(r.URL.Query().Get("limit"), 20)

// product_repo.go:143-144 — DB layer tự set default
if limit <= 0 {
    limit = 20   // mismatch với handler default 50
}
```

### Bảng tóm tắt inconsistency

| Endpoint | Default Limit | Max Limit |
|----------|--------------|-----------|
| `GET /findings` | 20 | 200 |
| `GET /products` | 50 | 200 |
| `GET /reports` | 20 | (none) |
| `GET /sla-breaches` | 5 | 50 |
| `product_repo.ListWithStats` (DB) | 20 | (none) |
| `product_repo.ListProducts` (DB) | 20 | (none) |

**product_handler default là 50 nhưng repo default là 20** — gây bug khi handler
không pass limit và repo silently clamp về 20.

## Tác động

1. API clients nhận số lượng results khác nhau tùy endpoint — gây confused.
2. Frontend phải hardcode pagination constants riêng, có thể lệch với backend.
3. Khi muốn thay đổi max limit cho performance, phải tìm và sửa nhiều nơi.

## Fix Proposal

### Tạo pagination constants package

```go
// finding-service/internal/domain/pagination/constants.go
package pagination

const (
    DefaultPage           = 1
    DefaultFindingLimit   = 20
    DefaultProductLimit   = 20  // FIX: unify with finding
    DefaultReportLimit    = 20
    DefaultSLABreachLimit = 10  // FIX: 5 is too small
    MaxFindingLimit       = 200
    MaxProductLimit       = 200
    MaxSLABreachLimit     = 100
    MaxReportLimit        = 100
)
```

### Dùng constants trong handlers

```go
// finding_handler.go
import "github.com/osv/finding-service/internal/domain/pagination"

limit = parseIntParam(r, "limit", pagination.DefaultFindingLimit)
if limit > pagination.MaxFindingLimit {
    limit = pagination.MaxFindingLimit
}
```

### Bỏ clamping trong repo layer

Repo không nên tự set default — handler có trách nhiệm cung cấp valid limit.

```go
// product_repo.go — REMOVE this:
// if limit <= 0 {
//     limit = 20
// }
// INSTEAD: return error hoặc require caller to validate
```

## Files Affected

| File | Line | Issue |
|------|------|-------|
| [finding_handler.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/finding_handler.go) | 74, 83, 86 | Magic numbers 200, 20 |
| [product_handler.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/product_handler.go) | 81, 89 | Max 200 nhưng default 50 |
| [internal_handler.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/internal_handler.go) | 70-71 | Default 5, max 50 |
| [product_repo.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/infra/postgres/product_repo.go) | 143-144 | Clamping in repo layer |
