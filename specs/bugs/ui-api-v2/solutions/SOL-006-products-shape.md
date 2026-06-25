# SOL-006 — Products: Response Shape Sai (P1)

**Bug**: [BUG-006](../BUG-006-products.md)  
**Service**: `finding-service` (`services/finding-service`)  
**Endpoint**: `GET /api/v1/products`, `GET /api/v1/products/{id}`  
**Lỗi frontend**: `TypeError: Cannot read properties of undefined` — nested field không tồn tại

**Status**: `✅ Implemented` — via [TASK-004](../../tasks/TASK-004-*.md)

---

## Root Cause

Frontend `ProductSecurity` component đọc các nested fields trong product response:
- `product.findings` hoặc `product.finding_counts` (object với keys critical/high/medium/low)
- `product.grade` (string)
- `product.engagements` (array — có thể không có)

Backend trả product nhưng **thiếu các field** này, hoặc trả `null` thay vì `{}` / `[]`.

---

## Giải pháp

### Kiểm tra response hiện tại

```bash
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/products" | jq '.[0]'
```

### Fix trong finding-service — ProductHandler

File: [`services/finding-service/internal/delivery/http/product_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/product_handler.go)

```go
// Đảm bảo response có đầy đủ fields với default safe values

type ProductResponse struct {
    ID              string                 `json:"id"`
    Name            string                 `json:"name"`
    Description     string                 `json:"description"`
    BusinessCriticality string             `json:"business_criticality"`
    Lifecycle       string                 `json:"lifecycle"`
    Origin          string                 `json:"origin"`
    Grade           string                 `json:"grade"`           // A/B/C/D/F
    FindingCounts   map[string]int         `json:"finding_counts"`  // critical/high/medium/low
    ActiveFindings  int                    `json:"active_findings"`
    Members         []MemberResponse       `json:"members"`         // KHÔNG bao giờ nil
    Tags            []string               `json:"tags"`            // KHÔNG bao giờ nil
    CreatedAt       string                 `json:"created_at"`
    UpdatedAt       string                 `json:"updated_at"`
}

func toProductResponse(p *product.Product, grade string, counts map[string]int) *ProductResponse {
    // Defensive defaults
    if counts == nil {
        counts = map[string]int{"Critical": 0, "High": 0, "Medium": 0, "Low": 0}
    }
    
    members := make([]MemberResponse, 0)  // never nil
    tags := make([]string, 0)              // never nil
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
        ActiveFindings:      counts["Critical"] + counts["High"] + counts["Medium"] + counts["Low"],
        Members:             members,
        Tags:                tags,
        CreatedAt:           p.CreatedAt.Format(time.RFC3339),
        UpdatedAt:           p.UpdatedAt.Format(time.RFC3339),
    }
}
```

### Fix ProductHandler.List

```go
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
    products, err := h.repo.List(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list products")
        return
    }

    // FIX: never nil
    if products == nil {
        products = make([]*product.Product, 0)
    }

    responses := make([]*ProductResponse, 0, len(products))
    for _, p := range products {
        // Get grade and finding counts (can be cached)
        grade := h.grader.GetGrade(r.Context(), p.ID)
        counts := h.findingCounts.GetCounts(r.Context(), p.ID)
        responses = append(responses, toProductResponse(p, grade, counts))
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":  responses,  // always array
        "total": len(responses),
    })
}
```

---

## Response Schema Required

```json
// GET /api/v1/products
{
  "data": [
    {
      "id": "uuid",
      "name": "Product Name",
      "grade": "B",
      "finding_counts": {
        "Critical": 0,
        "High": 2,
        "Medium": 5,
        "Low": 1
      },
      "active_findings": 8,
      "members": [],
      "tags": [],
      "created_at": "2026-06-01T00:00:00Z",
      "updated_at": "2026-06-20T00:00:00Z"
    }
  ],
  "total": 1
}
```

---

## Verification

```bash
# Kiểm tra finding_counts không undefined
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/products" | jq '.[0].finding_counts'
# Expected: {"Critical": 0, "High": 0, ...}

# Kiểm tra members là array
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/products" | jq '.[0].members | type'
# Expected: "array"
```
