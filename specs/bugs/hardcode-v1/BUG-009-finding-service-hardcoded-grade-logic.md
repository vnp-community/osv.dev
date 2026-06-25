# BUG-009 — Finding Service: Hardcoded Grade Score Map và Grade Thresholds

## Metadata
- **ID**: BUG-009
- **Service**: `finding-service`
- **File**: [`internal/delivery/http/product_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/product_handler.go)
- **Lines**: 186–201
- **Severity**: Medium
- **Category**: Hardcode / Business Logic
- **Status**: Open

## Mô tả

Logic tính grade và score được hardcode trực tiếp trong handler layer:

```go
// computeGrade: A=no findings, B=low only, C=medium, D=high, F=critical
func computeGrade(critical, high int) string {
    if critical > 0 {
        return "F"
    }
    if high > 5 {    // MAGIC NUMBER: 5
        return "D"
    }
    if high > 0 {
        return "C"
    }
    return "B"       // "A" never returned!
}

func gradeToScore(grade string) int {
    return map[string]int{"A": 100, "B": 80, "C": 65, "D": 50, "F": 30}[grade]
    // Anonymous map created on EVERY call
}
```

### Các vấn đề:

1. **Grade "A" không bao giờ được trả về**: `computeGrade` chỉ trả về B, C, D, F —
   grade "A" (100 điểm) không thể đạt được dù product không có finding nào.
   Điều này mâu thuẫn với comment `"A=no findings"`.

2. **Magic number `5`**: Ngưỡng `high > 5` cho grade D không có tài liệu giải thích
   tại sao chọn 5. Nếu business quyết định thay đổi ngưỡng này, phải sửa code.

3. **Anonymous map tạo mới mỗi call**: `gradeToScore` tạo một `map[string]int` mới
   mỗi lần được gọi — kém hiệu quả. Nên dùng `switch` hoặc package-level var.

4. **Không có medium severity trong computeGrade**: Chỉ xét `critical` và `high`,
   bỏ qua `medium` và `low`. Một product với 100 medium findings vẫn nhận grade "B".

## Tác động

1. **Bug thực sự**: Product với 0 finding vẫn nhận grade "B" thay vì "A" — score
   là 80 thay vì 100. Scorecard dashboard sẽ không bao giờ hiển thị grade A.
2. Performance: `gradeToScore` tạo map mới trong vòng lặp `for _, p := range products`.
3. Business logic bị lock vào code — không thể tune threshold mà không deploy lại.

## Fix Proposal

```go
// product_handler.go — sửa grade logic
const (
    gradeThresholdDHigh     = 5   // high findings để xuống D
    gradeThresholdCHigh     = 1   // high findings để xuống C
    gradeThresholdCMedium   = 10  // medium findings để xuống C (new)
)

// Package-level map — tạo 1 lần
var gradeScores = map[string]int{
    "A": 100,
    "B": 80,
    "C": 65,
    "D": 50,
    "F": 30,
}

func computeGrade(critical, high, medium int) string {
    if critical > 0 {
        return "F"
    }
    if high > gradeThresholdDHigh {
        return "D"
    }
    if high >= gradeThresholdCHigh || medium >= gradeThresholdCMedium {
        return "C"
    }
    if medium > 0 || high > 0 {
        return "B"
    }
    return "A"  // FIX: grade A is now achievable
}

func gradeToScore(grade string) int {
    if score, ok := gradeScores[grade]; ok {
        return score
    }
    return 0
}
```

### Lý tưởng: tách thành domain layer có thể config

```go
// domain/scoring/grading.go
type GradeConfig struct {
    HighForD    int
    MediumForC  int
}

func DefaultGradeConfig() GradeConfig {
    return GradeConfig{HighForD: 5, MediumForC: 10}
}
```

## References

- [product_handler.go L186-201](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/product_handler.go#L186-L201)
- [product_handler.go L105-119 (caller)](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/product_handler.go#L105-L119)
