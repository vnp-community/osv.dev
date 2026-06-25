// Package scoring chứa business logic tính grade và security score cho products.
// Single source of truth — không hardcode magic numbers trong handler layer.
package scoring

// GradeThresholds định nghĩa ngưỡng cho từng letter grade.
// Được inject vào handler — không hardcode trong handler layer.
type GradeThresholds struct {
	// CriticalForF: số critical findings >= để bị grade F. Default: 1.
	CriticalForF int
	// HighForD: số high findings > để bị grade D. Default: 5.
	HighForD int
	// MediumForC: số medium findings >= để bị grade C. Default: 10.
	MediumForC int
}

// DefaultGradeThresholds trả về thresholds mặc định của hệ thống.
// Single source of truth — không để magic numbers ở nơi khác.
func DefaultGradeThresholds() GradeThresholds {
	return GradeThresholds{
		CriticalForF: 1,
		HighForD:     5,
		MediumForC:   10,
	}
}

// gradeScores là package-level map — khởi tạo 1 lần, không tạo lại mỗi call.
// [FIX BUG-009] Thay anonymous map trong gradeToScore() tạo lại mỗi invocation.
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
//	F: critical >= thresholds.CriticalForF
//	D: high > thresholds.HighForD
//	C: high > 0, hoặc medium >= thresholds.MediumForC
//	B: medium > 0 (nhưng < MediumForC)
//	A: tất cả bằng 0  ← [FIX BUG-009] bây giờ có thể đạt được
//
// [FIX BUG-009] Grade "A" trả về khi 0 critical + 0 high + 0 medium.
// [FIX BUG-009] Medium severity được tính vào (trước: bị bỏ qua hoàn toàn).
// [FIX BUG-009] Thresholds là tham số — không hardcode magic number 5.
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
// [FIX BUG-009] Dùng package-level var — không tạo map mới mỗi call.
func GradeToScore(grade string) int {
	if score, ok := gradeScores[grade]; ok {
		return score
	}
	return 0
}
