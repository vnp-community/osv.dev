# TASK-006 — Fix BUG-009: Grade "A" Unreachable & Domain Scoring Package

> **Bug**: BUG-009  
> **Priority**: 🟡 Medium — Product scorecard luôn sai; grade "A" không bao giờ hiển thị  
> **Depends on**: không có dependency  
> **Solution ref**: [SOL-GROUP-D](../solutions/SOL-GROUP-D-finding-service-logic.md#bug-009)  
> **Trạng thái**: ✅ DONE — 2026-06-22  
> **Ghi chú**: Tạo `internal/domain/scoring/grading.go` với `ComputeGrade(critical, high, medium, thresholds)` — medium severity giờ được tính. Grade "A" bây giờ có thể đạt được (0 critical + 0 high + 0 medium). `GradeThresholds` inject vào handler. Xóa `computeGrade`/`gradeToScore` inline khỏi `product_handler.go`. Build pass.

## Files Cần Đọc Trước

```
services/finding-service/internal/delivery/http/product_handler.go  (lines 180-210)
services/finding-service/internal/domain/product/                    (xem entity)
services/finding-service/go.mod                                      (module name)
```

## Files Sẽ Được Tạo / Sửa

```
services/finding-service/internal/domain/scoring/grading.go  [NEW]
services/finding-service/internal/delivery/http/product_handler.go  [MODIFY]
services/finding-service/embedded.go                          [MODIFY — inject GradeThresholds]
```

## Thay Đổi Chi Tiết

### Bước 1: Đọc `product_handler.go` thực tế

```bash
grep -n "computeGrade\|gradeToScore\|Grade\|Score" \
    services/finding-service/internal/delivery/http/product_handler.go
```

Xác định:
- Tên hàm chính xác (`computeGrade`, `gradeToScore`)  
- Signature: có nhận `medium int` không?  
- Caller tại dòng nào (cần sửa call site thêm `medium` arg)

### Bước 2: Tạo `internal/domain/scoring/grading.go` [NEW]

Tạo file mới trong package `scoring`:

```go
// Package scoring chứa business logic tính grade và security score cho products.
package scoring

// GradeThresholds định nghĩa ngưỡng cho từng letter grade.
// Được inject vào handler — không hardcode trong handler layer.
type GradeThresholds struct {
    // CriticalForF: số critical findings (>=) để bị grade F. Default: 1.
    CriticalForF int
    // HighForD: số high findings (>) để bị grade D. Default: 5.
    HighForD int
    // MediumForC: số medium findings (>=) để bị grade C. Default: 10.
    MediumForC int
}

// DefaultGradeThresholds trả về thresholds mặc định của hệ thống.
// Là single source of truth — không để magic numbers ở nơi khác.
func DefaultGradeThresholds() GradeThresholds {
    return GradeThresholds{
        CriticalForF: 1,
        HighForD:     5,
        MediumForC:   10,
    }
}

// gradeScores là package-level map — tạo 1 lần, không tạo lại mỗi call.
// [FIX] Thay thế anonymous map trong gradeToScore() tạo lại mỗi invocation.
var gradeScores = map[string]int{
    "A": 100,
    "B": 80,
    "C": 65,
    "D": 50,
    "F": 30,
}

// ComputeGrade tính grade dựa trên finding counts theo severity.
//
// Thứ tự ưu tiên (từ xấu đến tốt):
//
//   F: critical >= thresholds.CriticalForF
//   D: high > thresholds.HighForD
//   C: high > 0, hoặc medium >= thresholds.MediumForC
//   B: medium > 0 (nhưng < MediumForC)
//   A: tất cả bằng 0  ← [FIX] bây giờ có thể đạt được
//
// [FIX] Grade "A" bây giờ trả về khi 0 critical + 0 high + 0 medium.
// [FIX] Medium severity được tính vào (trước: bị bỏ qua hoàn toàn).
// [FIX] Thresholds là tham số — không hardcode magic number 5.
func ComputeGrade(critical, high, medium int, t GradeThresholds) string {
    if critical >= t.CriticalForF {
        return "F"
    }
    if high > t.HighForD {
        return "D"
    }
    if high > 0 || medium >= t.MediumForC {
        return "C"
    }
    if medium > 0 {
        return "B"
    }
    return "A"
}

// GradeToScore chuyển grade letter sang numeric score (0–100).
// [FIX] Dùng package-level var — không tạo map mới mỗi call.
func GradeToScore(grade string) int {
    if score, ok := gradeScores[grade]; ok {
        return score
    }
    return 0
}
```

### Bước 3: Sửa `product_handler.go`

**Xóa** các hàm inline `computeGrade` và `gradeToScore` (hoặc `computeProductGrade` etc. — đọc tên thực tế).

**Thêm** field `gradeThresholds` vào handler struct:
```go
type ProductHandler struct {
    // ... existing fields ...
    gradeThresholds scoring.GradeThresholds  // [ADD]
}
```

**Cập nhật** constructor để nhận thresholds:
```go
func NewProductHandler(..., thresholds scoring.GradeThresholds) *ProductHandler {
    return &ProductHandler{
        // ...
        gradeThresholds: thresholds,
    }
}
```

**Thay** call sites dùng inline functions bằng domain package:
```go
// Thay:
grade := computeGrade(p.CriticalCount, p.HighCount)
score := gradeToScore(grade)

// Bằng:
grade := scoring.ComputeGrade(p.CriticalCount, p.HighCount, p.MediumCount, h.gradeThresholds)
score := scoring.GradeToScore(grade)
```

> **Nếu `MediumCount` chưa có trong data struct**: Đọc query thực tế — nếu DB trả về medium count,
> đảm bảo nó được scan vào struct. Nếu chưa có, thêm field và update query.

### Bước 4: Sửa `embedded.go` — inject thresholds

```go
import "github.com/<module>/finding-service/internal/domain/scoring"

func WireEmbedded(...) error {
    // Default thresholds — có thể override qua env vars nếu cần
    gradeThresholds := scoring.DefaultGradeThresholds()

    productHandler := httpdelivery.NewProductHandler(..., gradeThresholds)
    // ...
}
```

### Bước 5: Import

Thêm import `scoring` package vào `product_handler.go`:
```go
import (
    // ...
    "github.com/<module>/finding-service/internal/domain/scoring"
)
```

Lấy đúng module path từ `services/finding-service/go.mod`.

## Verification

```bash
# Build
go build ./services/finding-service/...

# Unit test grading logic
cat > /tmp/grade_test.go << 'EOF'
package scoring_test

import (
    "testing"
    "github.com/.../finding-service/internal/domain/scoring"
)

func TestComputeGrade(t *testing.T) {
    thresholds := scoring.DefaultGradeThresholds()
    
    // [FIX] Grade A bây giờ đạt được
    if grade := scoring.ComputeGrade(0, 0, 0, thresholds); grade != "A" {
        t.Errorf("want A, got %s", grade)
    }
    // Grade B: 1 medium finding
    if grade := scoring.ComputeGrade(0, 0, 1, thresholds); grade != "B" {
        t.Errorf("want B, got %s", grade)
    }
    // Grade F: any critical
    if grade := scoring.ComputeGrade(1, 0, 0, thresholds); grade != "F" {
        t.Errorf("want F, got %s", grade)
    }
}
EOF
go test ./services/finding-service/internal/domain/scoring/...

# API test: product với 0 findings nhận grade A
curl http://localhost:8085/api/v1/products/{empty-product-id}/grade
# → {"grade":"A","score":100}
```

## Acceptance Criteria

- [ ] `internal/domain/scoring/grading.go` được tạo với 3 functions
- [ ] Inline `computeGrade`/`gradeToScore` bị xóa khỏi `product_handler.go`
- [ ] `ComputeGrade(0, 0, 0, DefaultGradeThresholds())` trả về `"A"`
- [ ] `gradeToScore` không còn tạo anonymous map mỗi call
- [ ] Medium severity được tính vào grading logic
- [ ] `go build ./services/finding-service/...` thành công
