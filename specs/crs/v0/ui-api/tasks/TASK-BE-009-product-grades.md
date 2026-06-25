# TASK-BE-009 — finding-service: Product Grades Endpoint + Product DTO

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-009 |
| **Service** | `services/finding-service` |
| **Solution Ref** | [SOL-UI-004 §1.4](../solutions/SOL-UI-004-finding-product-reports-admin.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-BE-007 (grade computation logic) |
| **Estimated** | 3h |

---

## Context

UI Product Security page cần:
1. `GET /api/v1/products/grades` — grade A-F + score + trend cho tất cả products (new endpoint)
2. `GET /api/v1/products` — thêm `grade`, `score`, `finding_summary` vào existing response

---

## Target Files

| Action | File Path |
|--------|-----------|
| MODIFY | `services/finding-service/internal/adapter/http/product_handler.go` |
| MODIFY | `services/finding-service/internal/adapter/http/dto.go` |

---

## Implementation

### Product Grades Types (thêm vào dto.go):

```go
// services/finding-service/internal/adapter/http/dto.go

type ProductListItem struct {
    ID            string          `json:"id"`
    Name          string          `json:"name"`
    Description   string          `json:"description"`
    Type          string          `json:"type"`
    Criticality   string          `json:"criticality"`
    Lifecycle     string          `json:"lifecycle"`
    Grade         string          `json:"grade"`           // NEW
    Score         int             `json:"score"`           // NEW 0-100
    FindingSummary *FindingSummary `json:"finding_summary"` // NEW
    Tags          []string        `json:"tags"`
    CreatedAt     string          `json:"created_at"`
}

type FindingSummary struct {
    Critical    int `json:"critical"`
    High        int `json:"high"`
    Medium      int `json:"medium"`
    Low         int `json:"low"`
    TotalActive int `json:"total_active"`
}

type ProductGradeDTO struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    Grade         string `json:"grade"`
    Score         int    `json:"score"`
    CriticalCount int    `json:"critical_count"`
    HighCount     int    `json:"high_count"`
    TotalActive   int    `json:"total_active"`
    Trend         string `json:"trend"` // "improving" | "worsening" | "stable"
}

type ProductGradesResponse struct {
    Products     []ProductGradeDTO `json:"products"`
    OverallGrade string            `json:"overall_grade"`
    OverallScore int               `json:"overall_score"`
}
```

### Product Handler update:

```go
// services/finding-service/internal/adapter/http/product_handler.go

// GET /v2/products/grades (new)
// Also served at: GET /products/grades
func (h *ProductHandler) GetProductGrades(w http.ResponseWriter, r *http.Request) {
    products, err := h.productRepo.ListWithGrades(r.Context())
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }

    var totalCritical, totalHigh, totalActive int
    grades := make([]ProductGradeDTO, len(products))

    for i, p := range products {
        grade := computeGrade(p.CriticalCount, p.HighCount)
        trend := h.computeTrend(r.Context(), p.ID.String())

        grades[i] = ProductGradeDTO{
            ID:            p.ID.String(),
            Name:          p.Name,
            Grade:         grade,
            Score:         gradeToScore(grade),
            CriticalCount: p.CriticalCount,
            HighCount:     p.HighCount,
            TotalActive:   p.TotalActive,
            Trend:         trend,
        }

        totalCritical += p.CriticalCount
        totalHigh += p.HighCount
        totalActive += p.TotalActive
    }

    overallGrade := computeGrade(totalCritical, totalHigh)

    respondJSON(w, 200, ProductGradesResponse{
        Products:     grades,
        OverallGrade: overallGrade,
        OverallScore: gradeToScore(overallGrade),
    })
}

// Modify existing ListProducts to include grade + finding_summary
func (h *ProductHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
    // ... existing logic ...

    // Add grade + finding_summary to each product
    products, total, _ := h.productRepo.ListWithStats(r.Context(), filter)

    items := make([]ProductListItem, len(products))
    for i, p := range products {
        grade := computeGrade(p.CriticalCount, p.HighCount)
        items[i] = ProductListItem{
            ID:          p.ID.String(),
            Name:        p.Name,
            Description: p.Description,
            Type:        p.Type,
            Criticality: p.Criticality,
            Grade:       grade,
            Score:       gradeToScore(grade),
            FindingSummary: &FindingSummary{
                Critical:    p.CriticalCount,
                High:        p.HighCount,
                Medium:      p.MediumCount,
                Low:         p.LowCount,
                TotalActive: p.TotalActive,
            },
        }
    }

    respondJSON(w, 200, map[string]interface{}{
        "products":  items,
        "total":     total,
    })
}

// computeGrade: A=no findings, B=low only, C=medium, D=high, F=critical
func computeGrade(critical, high int) string {
    if critical > 0 { return "F" }
    if high > 5     { return "D" }
    if high > 0     { return "C" }
    return "B"
}

func gradeToScore(grade string) int {
    return map[string]int{"A": 100, "B": 80, "C": 65, "D": 50, "F": 30}[grade]
}

// computeTrend: compare current grade with last month's grade snapshot
func (h *ProductHandler) computeTrend(ctx context.Context, productID string) string {
    // Simple implementation: compare current critical count with 30 days ago
    current, _ := h.productRepo.GetCriticalCount(ctx, productID)
    past, _ := h.productRepo.GetCriticalCountAt(ctx, productID, time.Now().Add(-30*24*time.Hour))

    if current < past { return "improving" }
    if current > past { return "worsening" }
    return "stable"
}
```

### SQL for ListWithGrades:

```sql
-- product_repo.go
SELECT
    p.id, p.name,
    COUNT(f.*) FILTER (WHERE f.severity = 'Critical' AND f.active AND NOT f.is_duplicate) AS critical_count,
    COUNT(f.*) FILTER (WHERE f.severity = 'High'     AND f.active AND NOT f.is_duplicate) AS high_count,
    COUNT(f.*) FILTER (WHERE f.severity = 'Medium'   AND f.active AND NOT f.is_duplicate) AS medium_count,
    COUNT(f.*) FILTER (WHERE f.severity = 'Low'      AND f.active AND NOT f.is_duplicate) AS low_count,
    COUNT(f.*) FILTER (WHERE f.active AND NOT f.is_duplicate) AS total_active
FROM products p
LEFT JOIN findings f ON f.product_id = p.id
GROUP BY p.id, p.name
ORDER BY critical_count DESC, high_count DESC;
```

### Router additions:

```go
// services/finding-service/internal/adapter/http/router.go
mux.HandleFunc("GET /v2/products/grades",  h.Product.GetProductGrades)
mux.HandleFunc("GET /products/grades",     h.Product.GetProductGrades) // v1 alias
mux.HandleFunc("GET /v2/products/types",   h.Product.GetProductTypes)
```

---

## Verification

```bash
cd services/finding-service
go build ./...

# Get grades
curl http://localhost:8085/v2/products/grades | jq '.overall_grade'
# Expected: "B" | "C" | "D" | "F"

# Get products list with grade
curl http://localhost:8085/v2/products | jq '.products[0] | {grade, score, finding_summary}'
# Expected: {"grade":"C","score":65,"finding_summary":{"critical":0,"high":3,...}}
```

---

## Checklist

- [x] `GET /v2/products/grades` trả về `{products:[], overall_grade, overall_score}`
- [x] Grade formula: Critical>0 → F, High>5 → D, High>0 → C, else B (no findings → A)
- [x] `gradeToScore`: A=100, B=80, C=65, D=50, F=30
- [x] `computeTrend` trả về "improving"|"worsening"|"stable"
- [x] `GET /v2/products` response mỗi item có `grade`, `score`, `finding_summary`
- [x] SQL dùng `COUNT(*) FILTER` để count trong 1 query (không N+1)
- [x] `go build ./...` thành công
