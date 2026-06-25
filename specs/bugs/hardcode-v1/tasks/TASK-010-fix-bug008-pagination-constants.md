# TASK-010 — Fix BUG-008: Pagination Constants Package & Unify Magic Numbers

> **Bug**: BUG-008  
> **Priority**: 🟡 Medium — API contract instability; frontend phải hardcode pagination riêng  
> **Depends on**: không có dependency  
> **Solution ref**: [SOL-GROUP-D](../solutions/SOL-GROUP-D-finding-service-logic.md#bug-008)

## Files Cần Đọc Trước

```
services/finding-service/internal/delivery/http/finding_handler.go   (lines 70-95)
services/finding-service/internal/delivery/http/product_handler.go   (lines 78-95)
services/finding-service/internal/delivery/http/internal_handler.go  (lines 65-75)
services/finding-service/internal/delivery/http/report_handler.go    (lines 148-158)
services/finding-service/internal/infra/postgres/product_repo.go     (lines 138-150)
services/finding-service/go.mod                                       (module name)
```

## Files Sẽ Được Tạo / Sửa

```
services/finding-service/internal/domain/pagination/constants.go  [NEW]
services/finding-service/internal/delivery/http/finding_handler.go [MODIFY]
services/finding-service/internal/delivery/http/product_handler.go [MODIFY]
services/finding-service/internal/delivery/http/internal_handler.go [MODIFY]
services/finding-service/internal/delivery/http/report_handler.go  [MODIFY]
services/finding-service/internal/infra/postgres/product_repo.go   [MODIFY]
```

## Thay Đổi Chi Tiết

### Bước 1: Đọc tất cả files để map exact magic numbers

```bash
grep -n "200\|limit.*20\|limit.*50\|limit.*5\b" \
    services/finding-service/internal/delivery/http/finding_handler.go \
    services/finding-service/internal/delivery/http/product_handler.go \
    services/finding-service/internal/delivery/http/internal_handler.go \
    services/finding-service/internal/delivery/http/report_handler.go \
    services/finding-service/internal/infra/postgres/product_repo.go
```

### Bước 2: Tạo `internal/domain/pagination/constants.go` [NEW]

> **Chú ý**: `services/shared/pkg/pagination` đã tồn tại. Kiểm tra nội dung trước:
> ```bash
> ls services/shared/pkg/pagination/
> cat services/shared/pkg/pagination/*.go
> ```
> Nếu shared/pagination đã có constants phù hợp, dùng nó thay vì tạo mới trong finding-service.
> Nếu chưa đủ, tạo file mới trong finding-service/internal/domain/pagination/.

```go
// Package pagination định nghĩa pagination constants cho finding-service.
// Single source of truth — không để magic numbers trong handlers hay repo.
package pagination

const (
    // DefaultFindingLimit là số findings mặc định mỗi trang.
    DefaultFindingLimit = 20

    // DefaultProductLimit là số products mặc định mỗi trang.
    // [FIX] Thống nhất về 20 — trước đây là 50, không nhất quán với finding.
    DefaultProductLimit = 20

    // DefaultReportLimit là số reports mặc định mỗi trang.
    DefaultReportLimit = 20

    // DefaultSLABreachLimit là số SLA breaches mặc định.
    // [FIX] Tăng từ 5 lên 10 — 5 quá nhỏ cho production use.
    DefaultSLABreachLimit = 10

    // MaxFindingLimit là giới hạn trên cho findings query.
    MaxFindingLimit = 200

    // MaxProductLimit là giới hạn trên cho products query.
    MaxProductLimit = 200

    // MaxReportLimit là giới hạn trên cho reports query.
    MaxReportLimit = 100

    // MaxSLABreachLimit là giới hạn trên cho SLA breach list.
    MaxSLABreachLimit = 100

    // DefaultPage là trang mặc định.
    DefaultPage = 1
)

// Clamp đảm bảo n nằm trong [1, max].
// Trả về defaultVal nếu n <= 0.
func Clamp(n, defaultVal, max int) int {
    if n <= 0 {
        return defaultVal
    }
    if n > max {
        return max
    }
    return n
}
```

### Bước 3: Sửa `finding_handler.go`

Tìm các magic numbers (đọc file thực tế trước):
```go
// Pattern có thể xuất hiện:
if limit > 200 { limit = 200 }         // ← MaxFindingLimit
limit = parseIntParam(r, "limit", 20)  // ← DefaultFindingLimit
```

Thay bằng:
```go
import "github.com/<module>/finding-service/internal/domain/pagination"

limit := parseIntParam(r, "limit", pagination.DefaultFindingLimit)
limit = pagination.Clamp(limit, pagination.DefaultFindingLimit, pagination.MaxFindingLimit)
```

### Bước 4: Sửa `product_handler.go`

Tìm:
```go
limit = parseIntParam(r, "limit", 50)  // ← [FIX] thay bằng DefaultProductLimit (20)
if limit > 200 { limit = 200 }
```

Thay bằng:
```go
limit := parseIntParam(r, "limit", pagination.DefaultProductLimit)  // 20, không còn 50
limit = pagination.Clamp(limit, pagination.DefaultProductLimit, pagination.MaxProductLimit)
```

### Bước 5: Sửa `internal_handler.go`

Tìm pattern SLA breach limit:
```go
if limit <= 0 || limit > 50 {
    limit = 5  // ← [FIX] DefaultSLABreachLimit (10) và MaxSLABreachLimit (100)
}
```

Thay bằng:
```go
limit = pagination.Clamp(limit, pagination.DefaultSLABreachLimit, pagination.MaxSLABreachLimit)
```

### Bước 6: Sửa `report_handler.go`

Tương tự, thay magic `20` bằng `pagination.DefaultReportLimit`.

### Bước 7: Sửa `product_repo.go` — Xóa clamping trong repo layer

Tìm:
```go
if limit <= 0 {
    limit = 20  // ← clamping trong repo layer là sai
}
```

**Xóa** clamping này. Repo không nên tự set default — handler có trách nhiệm.
Nếu lo ngại về backward compat, thêm assert thay vì silent clamp:
```go
// Sau khi xóa clamping, thêm guard:
if limit <= 0 {
    return nil, 0, fmt.Errorf("product_repo: limit must be > 0, got %d", limit)
}
```

## Kiểm Tra Trước Khi Sửa

```bash
# Kiểm tra shared/pkg/pagination đã có gì
cat services/shared/pkg/pagination/*.go 2>/dev/null || echo "no pagination in shared"

# Map tất cả magic numbers
grep -rn "limit.*=.*20\b\|limit.*=.*50\b\|limit.*=.*5\b\|> 200\|> 50" \
    services/finding-service/internal/
```

## Verification

```bash
# Build
go build ./services/finding-service/...

# Test: finding default limit là 20
curl "http://localhost:8085/api/v1/findings" | jq '.pagination.limit // .limit'
# → 20

# Test: product default limit là 20 (không còn 50)
curl "http://localhost:8085/api/v1/products" | jq '.pagination.limit // .limit'  
# → 20 (trước: 50)

# Test: max limit được enforce
curl "http://localhost:8085/api/v1/findings?limit=9999" | jq '.pagination.limit // .limit'
# → 200 (clamped to MaxFindingLimit)
```

## Acceptance Criteria

- [ ] `internal/domain/pagination/constants.go` được tạo với tất cả constants và `Clamp()`
- [ ] Tất cả magic numbers `20`, `50`, `5`, `200` trong handlers được thay bằng constants
- [ ] `product_handler` default là `20` (không còn `50`)
- [ ] SLA breach default là `10` (không còn `5`)
- [ ] Clamping bị xóa khỏi `product_repo.go`
- [ ] `go build ./services/finding-service/...` thành công
