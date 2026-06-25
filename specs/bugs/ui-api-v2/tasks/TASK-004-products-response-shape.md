# TASK-004 — finding-service: Thêm `finding_counts` + `grade` vào Product response

**Bug**: [BUG-006](../BUG-006-products.md)  
**Solution**: [SOL-006](../solutions/SOL-006-products-shape.md)  
**Priority**: 🟠 P1  
**Effort**: ~20 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/products` thiếu các fields mà frontend `ProductSecurity` component cần: `finding_counts`, `grade`, `members`, `tags`. Component cố đọc `product.finding_counts.Critical` → `undefined.Critical` → crash.

---

## File cần sửa

**File**: [`services/finding-service/internal/delivery/http/product_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/product_handler.go)

---

## Thay đổi — ProductResponse struct

**Tìm** struct response của product (tên có thể là `ProductResponse`, `productDTO`, v.v.):

```go
type ProductResponse struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    // ... (chỉ có basic fields)
}
```

**Thêm các fields** còn thiếu:

```go
type ProductResponse struct {
    ID                  string         `json:"id"`
    Name                string         `json:"name"`
    Description         string         `json:"description"`
    BusinessCriticality string         `json:"business_criticality"`
    Lifecycle           string         `json:"lifecycle"`
    Grade               string         `json:"grade"`           // "A"|"B"|"C"|"D"|"F"
    FindingCounts       map[string]int `json:"finding_counts"`  // {"Critical":0,"High":2,...}
    ActiveFindings      int            `json:"active_findings"`
    Members             []interface{}  `json:"members"`         // never nil
    Tags                []string       `json:"tags"`            // never nil
    CreatedAt           string         `json:"created_at"`
    UpdatedAt           string         `json:"updated_at"`
}
```

---

## Thay đổi — toProductResponse helper

**Tìm** hoặc **tạo** helper function `toProductResponse`:

```go
func toProductResponse(p *product.Product) *ProductResponse {
    // Default finding_counts (safe defaults)
    counts := map[string]int{
        "Critical": 0,
        "High":     0,
        "Medium":   0,
        "Low":      0,
    }

    // Default grade
    grade := "A"

    // Defensive: never nil arrays
    members := make([]interface{}, 0)
    tags := make([]string, 0)
    if p.Tags != nil {
        tags = p.Tags
    }

    return &ProductResponse{
        ID:                  p.ID.String(),
        Name:                p.Name,
        Description:         p.Description,
        BusinessCriticality: p.BusinessCriticality,
        Lifecycle:           p.Lifecycle,
        Grade:               grade,
        FindingCounts:       counts,
        ActiveFindings:      0,
        Members:             members,
        Tags:                tags,
        CreatedAt:           p.CreatedAt.Format(time.RFC3339),
        UpdatedAt:           p.UpdatedAt.Format(time.RFC3339),
    }
}
```

> **Nâng cao** (optional): Nếu finding-service có thể query counts, fetch counts thực tế qua `r.pool.QueryRow()` trong handler và populate `FindingCounts` + `Grade` thực sự.

---

## Thay đổi — Handler List

**Tìm** phần handler trả response:

```go
respondJSON(w, http.StatusOK, products)
```

**Thay bằng**:

```go
// never nil
if products == nil {
    products = make([]*product.Product, 0)
}

responses := make([]*ProductResponse, 0, len(products))
for _, p := range products {
    responses = append(responses, toProductResponse(p))
}

respondJSON(w, http.StatusOK, map[string]interface{}{
    "data":  responses,
    "total": len(responses),
})
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/products` có field `finding_counts` (không `undefined`)
- [ ] `finding_counts` là object với keys `Critical`, `High`, `Medium`, `Low`
- [ ] `members` và `tags` là `[]` (không `null`)
- [ ] `grade` có giá trị string (mặc định `"A"`)
- [ ] `go build ./...` trong finding-service không có lỗi

---

## Verify

```bash
curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/products" | jq '
  .data[0] | {
    has_finding_counts: (has("finding_counts")),
    finding_counts_type: (.finding_counts | type),
    members_type: (.members | type),
    tags_type: (.tags | type),
    grade: .grade
  }'
# Expected:
# {
#   "has_finding_counts": true,
#   "finding_counts_type": "object",
#   "members_type": "array",
#   "tags_type": "array",
#   "grade": "A"
# }
```
