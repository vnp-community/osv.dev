# SOL-GROUP-D — Finding Service Business Logic Fixes

> **Fixes**: BUG-008, BUG-009, BUG-010  
> **Service**: `finding-service`  
> **Priority**: BUG-010 🔴 High | BUG-008 🟡 Medium | BUG-009 🟡 Medium

---

## BUG-010 — Nil Report Repository Stub (High Priority — Fix First)

### Root Cause

`nilReportRepo` được dùng trong production code khi MinIO chưa được cấu hình.
Behavior không nhất quán: `Save` → lỗi 500, `ListByProduct` → 200 OK với empty list.
User nghĩ reports đã bị xóa trong khi thực ra storage chưa được cấu hình.

### Files Cần Sửa

- `services/finding-service/embedded.go`

### Solution

**Bước 1**: Đổi tên và unify error behavior của stub:

```go
// services/finding-service/embedded.go

// errStorageNotConfigured là lỗi trả về khi MinIO chưa được cấu hình.
// Dùng sentinel error để client có thể phân biệt với lỗi khác.
var errStorageNotConfigured = errors.New(
    "report storage not configured: set MINIO_ENDPOINT env var to enable report features",
)

// notConfiguredReportRepo là stub dùng khi MinIO chưa có.
// [FIX] Tất cả methods đều trả về errStorageNotConfigured — consistent behavior.
// [FIX] Đổi tên từ nilReportRepo → notConfiguredReportRepo để rõ ràng hơn.
type notConfiguredReportRepo struct{}

func (n *notConfiguredReportRepo) Save(_ context.Context, _ *report.Report) error {
    return errStorageNotConfigured
}

func (n *notConfiguredReportRepo) FindByID(_ context.Context, _ uuid.UUID) (*report.Report, error) {
    // [FIX] Trả về errStorageNotConfigured thay vì "report not found"
    // Trước: return nil, fmt.Errorf("report not found")  ← misleading!
    return nil, errStorageNotConfigured
}

func (n *notConfiguredReportRepo) ListByProduct(
    _ context.Context, _ uuid.UUID, _, _ int, _ string,
) ([]*report.Report, int, error) {
    // [FIX] Trả về error thay vì empty list thành công
    // Trước: return []*report.Report{}, 0, nil  ← hides the problem!
    return nil, 0, errStorageNotConfigured
}

func (n *notConfiguredReportRepo) Delete(_ context.Context, _ uuid.UUID) error {
    return errStorageNotConfigured
}
```

**Bước 2**: Config-driven wiring với MinIO:

```go
// services/finding-service/embedded.go — hàm WireEmbedded()

func WireEmbedded(ctx context.Context, ...) error {
    logger := ...

    // [FIX] Config-driven: chỉ dùng stub khi MINIO_ENDPOINT không được set.
    // Log WARN rõ ràng để admin biết reports bị disabled.
    var reportRepo report.Repository = &notConfiguredReportRepo{}
    
    minioEndpoint := os.Getenv("MINIO_ENDPOINT")
    if minioEndpoint != "" {
        minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
        minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
        minioBucket    := config.Str("MINIO_BUCKET", "reports")
        
        realRepo, err := minio.NewReportRepository(ctx, minio.Config{
            Endpoint:  minioEndpoint,
            AccessKey: minioAccessKey,
            SecretKey: minioSecretKey,
            Bucket:    minioBucket,
        })
        if err != nil {
            // Nếu MinIO được config nhưng không thể kết nối → WARN và dùng stub
            logger.Warn().Err(err).
                Str("endpoint", minioEndpoint).
                Msg("MinIO unavailable, report storage disabled — reports will not persist")
        } else {
            reportRepo = realRepo
            logger.Info().
                Str("endpoint", minioEndpoint).
                Str("bucket", minioBucket).
                Msg("MinIO report storage enabled")
        }
    } else {
        logger.Warn().
            Msg("MINIO_ENDPOINT not set — report storage disabled. " +
                "Set MINIO_ENDPOINT to enable report creation and download.")
    }

    // Wire vào handler
    reportHandler := httpdelivery.NewReportHandler(generateUC, reportRepo, templateStore)
    
    return nil
}
```

**Bước 3**: HTTP handler phải trả về 503 thay vì 500 khi storage chưa configured:

```go
// services/finding-service/internal/delivery/http/report_handler.go

func (h *ReportHandler) CreateReport(w http.ResponseWriter, r *http.Request) {
    // ...
    if err := h.repo.Save(ctx, report); err != nil {
        if errors.Is(err, errStorageNotConfigured) {
            // [FIX] 503 Service Unavailable thay vì 500 Internal Server Error
            http.Error(w, `{"error":"report storage not available","hint":"contact administrator to configure MinIO"}`,
                http.StatusServiceUnavailable)
            return
        }
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    // ...
}

func (h *ReportHandler) ListReports(w http.ResponseWriter, r *http.Request) {
    reports, total, err := h.repo.ListByProduct(ctx, productID, page, limit, sort)
    if err != nil {
        if errors.Is(err, errStorageNotConfigured) {
            // [FIX] Trả về 503 với thông báo rõ ràng thay vì 200 OK với []
            http.Error(w, `{"error":"report storage not available"}`,
                http.StatusServiceUnavailable)
            return
        }
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    // ...
}
```

**Bước 4**: Health endpoint expose trạng thái storage:

```go
// services/finding-service/internal/delivery/http/health_handler.go

type HealthResponse struct {
    Status     string                     `json:"status"`
    Components map[string]ComponentStatus `json:"components"`
}

type ComponentStatus struct {
    Status  string `json:"status"`
    Message string `json:"message,omitempty"`
}

func (h *HealthHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
    components := map[string]ComponentStatus{
        "database": {Status: "ok"},
    }
    
    // Check report storage
    if h.storageConfigured {
        components["report_storage"] = ComponentStatus{Status: "ok"}
    } else {
        components["report_storage"] = ComponentStatus{
            Status:  "not_configured",
            Message: "set MINIO_ENDPOINT to enable report features",
        }
    }
    
    // Overall status
    overallStatus := "ok"
    for _, c := range components {
        if c.Status != "ok" {
            overallStatus = "degraded"
            break
        }
    }
    
    code := http.StatusOK
    if overallStatus == "degraded" {
        code = http.StatusOK  // vẫn 200 — degraded không phải down
    }
    
    respondJSON(w, code, HealthResponse{
        Status:     overallStatus,
        Components: components,
    })
}
```

---

## BUG-008 — Inconsistent Pagination Magic Numbers

### Root Cause

Magic numbers `5`, `20`, `50`, `200` scatter khắp finding-service handlers và repo layer.
`product_handler` default là 50 nhưng `product_repo` clamp về 20 → silently returns 20 items.

### Files Cần Sửa

- `services/finding-service/internal/delivery/http/finding_handler.go`
- `services/finding-service/internal/delivery/http/product_handler.go`
- `services/finding-service/internal/delivery/http/internal_handler.go`
- `services/finding-service/internal/delivery/http/report_handler.go`
- `services/finding-service/internal/infra/postgres/product_repo.go`
- **[NEW]** `services/finding-service/internal/domain/pagination/constants.go`

### Solution

**Bước 1**: Tạo pagination constants package:

```go
// services/finding-service/internal/domain/pagination/constants.go

// Package pagination định nghĩa tất cả constants liên quan đến phân trang.
// Đây là single source of truth — không để magic numbers trong handlers hay repo.
package pagination

const (
    // DefaultFindingLimit là số lượng findings trả về mặc định khi client không chỉ định.
    DefaultFindingLimit = 20

    // DefaultProductLimit là số lượng products trả về mặc định.
    // [FIX] Thống nhất 20 thay vì 50 (product_handler) để nhất quán với finding.
    DefaultProductLimit = 20

    // DefaultReportLimit là số lượng reports trả về mặc định.
    DefaultReportLimit = 20

    // DefaultSLABreachLimit là số lượng SLA breaches trả về mặc định.
    // [FIX] Tăng từ 5 lên 10 — 5 quá nhỏ cho production.
    DefaultSLABreachLimit = 10

    // MaxFindingLimit là giới hạn trên cho findings pagination.
    MaxFindingLimit = 200

    // MaxProductLimit là giới hạn trên cho products pagination.
    MaxProductLimit = 200

    // MaxReportLimit là giới hạn trên cho reports pagination.
    MaxReportLimit = 100

    // MaxSLABreachLimit là giới hạn trên cho SLA breach list.
    MaxSLABreachLimit = 100

    // DefaultPage là trang mặc định khi client không chỉ định.
    DefaultPage = 1
)

// Clamp đảm bảo limit nằm trong khoảng [1, max].
// Trả về defaultVal nếu limit <= 0.
func Clamp(limit, defaultVal, max int) int {
    if limit <= 0 {
        return defaultVal
    }
    if limit > max {
        return max
    }
    return limit
}
```

**Bước 2**: Dùng constants trong `finding_handler.go`:

```go
// services/finding-service/internal/delivery/http/finding_handler.go

import "github.com/osv/finding-service/internal/domain/pagination"

func (h *FindingHandler) ListFindings(w http.ResponseWriter, r *http.Request) {
    // [FIX] Dùng constants thay vì magic numbers
    limit := parseIntParam(r, "limit", pagination.DefaultFindingLimit)
    limit = pagination.Clamp(limit, pagination.DefaultFindingLimit, pagination.MaxFindingLimit)
    // Trước: if limit > 200 { limit = 200 }  ← magic number

    page := parseIntParam(r, "page", pagination.DefaultPage)
    // ...
}
```

**Bước 3**: Dùng constants trong `product_handler.go`:

```go
// services/finding-service/internal/delivery/http/product_handler.go

import "github.com/osv/finding-service/internal/domain/pagination"

func (h *ProductHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
    // [FIX] Đổi từ default 50 → 20, dùng constant
    // Trước: limit = parseIntParam(r, "limit", 50)
    limit := parseIntParam(r, "limit", pagination.DefaultProductLimit)
    limit = pagination.Clamp(limit, pagination.DefaultProductLimit, pagination.MaxProductLimit)
    // ...
}
```

**Bước 4**: Dùng constants trong `internal_handler.go`:

```go
// services/finding-service/internal/delivery/http/internal_handler.go

import "github.com/osv/finding-service/internal/domain/pagination"

func (h *InternalHandler) ListSLABreaches(w http.ResponseWriter, r *http.Request) {
    // [FIX] Dùng constant thay vì magic number 5/50
    limit := parseIntParam(r, "limit", pagination.DefaultSLABreachLimit)
    limit = pagination.Clamp(limit, pagination.DefaultSLABreachLimit, pagination.MaxSLABreachLimit)
    // Trước: if limit <= 0 || limit > 50 { limit = 5 }
    // ...
}
```

**Bước 5**: Xóa clamping trong repo layer:

```go
// services/finding-service/internal/infra/postgres/product_repo.go

func (r *PostgresProductRepo) ListWithStats(ctx context.Context, filter ProductFilter) ([]ProductWithStats, int, error) {
    // [FIX] REMOVE: clamping trong repo layer
    // Trước:
    //   if filter.Limit <= 0 {
    //       filter.Limit = 20  ← repo không nên set default
    //   }
    //
    // Sau: handler có trách nhiệm validate limit trước khi gọi repo.
    // Nếu limit <= 0 được pass vào repo, là lỗi của caller.
    if filter.Limit <= 0 {
        // Return error thay vì silently clamp
        return nil, 0, fmt.Errorf("product_repo.ListWithStats: limit must be > 0, got %d", filter.Limit)
    }
    
    // ... query ...
}
```

---

## BUG-009 — Hardcoded Grade Logic (Grade "A" Unreachable)

### Root Cause

`computeGrade()` trong `product_handler.go` không có case nào trả về `"A"`.
Product với 0 findings nhận grade `"B"` (80 điểm) thay vì `"A"` (100 điểm).
Map `gradeScores` được tạo mới mỗi lần gọi `gradeToScore`.

### Files Cần Sửa

- `services/finding-service/internal/delivery/http/product_handler.go`
- **[NEW]** `services/finding-service/internal/domain/scoring/grading.go`

### Solution

**Bước 1**: Tách business logic ra domain layer:

```go
// [NEW] services/finding-service/internal/domain/scoring/grading.go

// Package scoring chứa business logic tính grade và score cho products.
package scoring

// GradeThresholds định nghĩa ngưỡng cho từng grade.
// Được cấu hình tại runtime — không hardcode magic numbers.
type GradeThresholds struct {
    // CriticalForF: số critical findings để bị grade F
    CriticalForF int // default: 1 (bất kỳ critical nào = F)

    // HighForD: số high findings (> HighForD) để bị grade D
    HighForD int // default: 5

    // MediumForC: số medium findings (>= MediumForC) để bị grade C
    MediumForC int // default: 10
}

// DefaultGradeThresholds trả về thresholds mặc định của hệ thống.
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

// ComputeGrade tính grade dựa trên số lượng findings theo severity.
//
// Logic (từ tốt đến xấu):
//   A: 0 critical, 0 high, 0 medium  → perfect score
//   B: 0 critical, 0-highForD high, medium < mediumForC
//   C: 0 critical, 0-highForD high, medium >= mediumForC; hoặc high > 0
//   D: 0 critical, high > highForD
//   F: critical >= criticalForF
//
// [FIX] Grade "A" bây giờ có thể đạt được (trước: không bao giờ trả về "A").
// [FIX] Medium severity được tính vào grade (trước: bị bỏ qua hoàn toàn).
// [FIX] Thresholds được document rõ ràng (trước: magic number 5 không giải thích).
func ComputeGrade(critical, high, medium int, thresholds GradeThresholds) string {
    if critical >= thresholds.CriticalForF {
        return "F"
    }
    if high > thresholds.HighForD {
        return "D"
    }
    if high > 0 || medium >= thresholds.MediumForC {
        return "C"
    }
    if medium > 0 {
        return "B"
    }
    return "A" // [FIX] Bây giờ có thể đạt được: 0 critical, 0 high, 0 medium
}

// GradeToScore chuyển grade letter sang numeric score.
// [FIX] Dùng package-level var thay vì tạo map mới mỗi call.
func GradeToScore(grade string) int {
    if score, ok := gradeScores[grade]; ok {
        return score
    }
    return 0 // unknown grade
}
```

**Bước 2**: Cập nhật `product_handler.go` dùng domain scoring:

```go
// services/finding-service/internal/delivery/http/product_handler.go

import "github.com/osv/finding-service/internal/domain/scoring"

type ProductHandler struct {
    // ...
    gradeThresholds scoring.GradeThresholds  // [ADD] injected, not hardcoded
}

func NewProductHandler(..., thresholds scoring.GradeThresholds) *ProductHandler {
    return &ProductHandler{
        // ...
        gradeThresholds: thresholds,
    }
}

// ListProductsWithGrades trả về products với grade và score.
func (h *ProductHandler) ListProductsWithGrades(w http.ResponseWriter, r *http.Request) {
    // ...
    for _, p := range products {
        // [FIX] Dùng domain scoring package thay vì inline functions
        grade := scoring.ComputeGrade(
            p.CriticalCount,
            p.HighCount,
            p.MediumCount,        // [FIX] Medium bây giờ được tính
            h.gradeThresholds,
        )
        score := scoring.GradeToScore(grade)
        
        // Append to response
        result = append(result, ProductGradeResponse{
            ProductID: p.ID,
            Grade:     grade,
            Score:     score,
        })
    }
    // ...
}
```

**Bước 3**: Khởi tạo với default thresholds trong `embedded.go`:

```go
// services/finding-service/embedded.go

import "github.com/osv/finding-service/internal/domain/scoring"

func WireEmbedded(ctx context.Context, ...) error {
    // [ADD] Grading thresholds — có thể override qua env vars nếu cần
    gradeThresholds := scoring.DefaultGradeThresholds()
    
    // Optional: load từ env vars cho flexibility
    if v := config.Int("GRADE_HIGH_THRESHOLD_D", 0); v > 0 {
        gradeThresholds.HighForD = v
    }
    if v := config.Int("GRADE_MEDIUM_THRESHOLD_C", 0); v > 0 {
        gradeThresholds.MediumForC = v
    }
    
    productHandler := httpdelivery.NewProductHandler(..., gradeThresholds)
    // ...
}
```

### Grade Logic So Sánh

| Scenario | Before (BUG) | After (FIX) |
|----------|-------------|-------------|
| 0 critical, 0 high, 0 medium | `"B"` (score 80) ❌ | `"A"` (score 100) ✅ |
| 0 critical, 0 high, 5 medium | `"B"` (score 80) ❌ | `"B"` (score 80) ✅ |
| 0 critical, 0 high, 10 medium | `"B"` (score 80) ❌ | `"C"` (score 65) ✅ |
| 0 critical, 3 high | `"C"` (score 65) ✅ | `"C"` (score 65) ✅ |
| 0 critical, 6 high | `"D"` (score 50) ✅ | `"D"` (score 50) ✅ |
| 1+ critical | `"F"` (score 30) ✅ | `"F"` (score 30) ✅ |

---

## Tóm Tắt Thay Đổi

| Bug | File | Thay Đổi Chính |
|-----|------|----------------|
| BUG-010 | `finding-service/embedded.go` | Rename stub; consistent errors; config-driven MinIO wiring |
| BUG-010 | `report_handler.go` | 503 khi storage unconfigured; error trong ListReports |
| BUG-010 | `health_handler.go` | Expose `report_storage` status trong health check |
| BUG-008 | `domain/pagination/constants.go` | **[NEW]** Tập trung tất cả pagination constants |
| BUG-008 | `finding_handler.go` | Dùng `pagination.Clamp()` + constants |
| BUG-008 | `product_handler.go` | Unify default từ 50 → 20; dùng constants |
| BUG-008 | `internal_handler.go` | Default SLA breach từ 5 → 10; dùng constants |
| BUG-008 | `product_repo.go` | Xóa clamping; return error nếu limit <= 0 |
| BUG-009 | `domain/scoring/grading.go` | **[NEW]** Domain scoring package; fix "A" unreachable |
| BUG-009 | `product_handler.go` | Inject `GradeThresholds`; dùng `scoring.ComputeGrade()` |

## Test Verification

```bash
# BUG-010: Verify consistent errors khi MinIO không configured
unset MINIO_ENDPOINT
curl -X POST http://localhost:8085/api/v1/reports -H "Content-Type: application/json" -d '{...}'
# → 503 Service Unavailable: "report storage not available"

curl http://localhost:8085/api/v1/reports?product_id=xxx
# → [FIX] 503 Service Unavailable (trước: 200 OK với [])

curl http://localhost:8085/health
# → {"status":"degraded","components":{"report_storage":{"status":"not_configured"}}}

# BUG-009: Verify grade A đạt được
# Tạo product với 0 findings, kiểm tra grade
curl http://localhost:8085/api/v1/products/{id}/grade
# → {"grade":"A","score":100}  (trước: {"grade":"B","score":80})

# BUG-008: Verify pagination defaults nhất quán
curl "http://localhost:8085/api/v1/findings" | jq '.pagination.limit'
# → 20
curl "http://localhost:8085/api/v1/products" | jq '.pagination.limit'
# → 20  (trước: 50, không nhất quán)
```
