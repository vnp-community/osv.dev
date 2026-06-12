> **✅ COMPLETED** — Bridge Pattern, go build && go vet passed.

# T15 — Dashboard Routes (Compose từ query-service)

## Thông tin
| | |
|---|---|
| **Phase** | 5 — Dashboard |
| **Ước tính** | 2–3 giờ |
| **Depends on** | T09 (CVE recent), T10 (products) |
| **Blocks** | — |

## Mục tiêu
Tạo `internal/router/dashboard_routes.go` (~40 LOC): compose stats từ `query-service` usecases, `scan-service` repo, và `vulnerability-service`. Không có business logic mới.

---

## Packages cần import

| Import path | Thành phần |
|-------------|------------|
| `query-service/internal/usecase/browse/` | Browse/filter |
| `query-service/internal/usecase/rank/` | Rank by severity |
| `query-service/internal/usecase/lookup/` | Lookup detail |
| `scan-service/internal/adapters/repository/postgres/` | CountByStatus, ListRecent |
| `vulnerability-service/internal/usecase/getrecent/` | Recent CVEs |

---

## Các bước thực hiện

### 15.1 Đọc query-service API

```bash
cat osv.dev/services/query-service/internal/usecase/browse/*.go
cat osv.dev/services/query-service/internal/usecase/rank/*.go
cat osv.dev/services/query-service/go.mod | head -5
```

Ghi lại:
- `browseuc.UseCase.Execute()` → input/output
- `rankuc.UseCase.Execute()` → input/output

> **Rủi ro quan trọng**: query-service có thể chỉ serve CVE data (browse/rank CVEs).  
> Nếu không có scan/finding aggregation → dùng trực tiếp scan + finding repos.

### 15.2 Khởi tạo query-service usecases

```go
import (
    browseuc "github.com/osv/query-service/internal/usecase/browse"
    rankuc   "github.com/osv/query-service/internal/usecase/rank"
    lookupuc "github.com/osv/query-service/internal/usecase/lookup"
    queryrepo "github.com/osv/query-service/internal/infra"
)

// Repository
queryRepo := queryrepo.New(a.db)

// Usecases
browseUC := browseuc.New(queryRepo)
rankUC   := rankuc.New(queryRepo)
lookupUC := lookupuc.New(queryRepo)
```

### 15.3 Tạo `internal/router/dashboard_routes.go`

```go
// Package router provides dashboard aggregation routes.
// No business logic — pure data composition from existing service usecases.
package router

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"

    browseuc "github.com/osv/query-service/internal/usecase/browse"
    rankuc   "github.com/osv/query-service/internal/usecase/rank"
    recentuc "github.com/osv/vulnerability-service/internal/usecase/getrecent"
    "github.com/osv/apps/openvulnscan/internal/app"
)

// DashboardStats is the aggregated response for GET /api/v1/dashboard
type DashboardStats struct {
    TotalScans      int              `json:"total_scans"`
    ActiveScans     int              `json:"active_scans"`
    CompletedScans  int              `json:"completed_scans"`
    TotalFindings   int              `json:"total_findings"`
    CriticalFindings int             `json:"critical_findings"`
    HighFindings    int              `json:"high_findings"`
    TopCVEs         interface{}      `json:"top_cves"`
    RecentCVEs      interface{}      `json:"recent_cves"`
    RecentScans     interface{}      `json:"recent_scans"`
}

func mountDashboardRoutes(r chi.Router, a *app.App) {
    r.Get("/api/v1/dashboard", dashboardHandler(a))
    r.Get("/api/v1/dashboard/timeline", dashboardTimelineHandler(a))
    r.Get("/api/v1/dashboard/top-vulnerabilities", dashboardTopVulnsHandler(a))
}

func dashboardHandler(a *app.App) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()

        // 1. Scan counts từ scanRepo
        scanCounts, _ := a.ScanRepo.CountByStatus(ctx)

        // 2. Finding counts từ findingRepo (aggregate query)
        findingCounts, _ := a.FindingRepo.CountBySeverity(ctx)

        // 3. Top CVEs — từ rankUC (CVEs ranked by score)
        topCVEs, _ := a.RankUC.Execute(ctx, rankuc.Input{
            Limit:  10,
            SortBy: "cvss_score",
        })

        // 4. Recent CVEs — từ vulnerability-service
        recentCVEs, _ := a.RecentCVEUC.Execute(ctx, recentuc.Input{Limit: 5})

        // 5. Recent scans — trực tiếp từ scanRepo
        recentScans, _ := a.ScanRepo.ListRecent(ctx, 5)

        stats := DashboardStats{
            TotalScans:       scanCounts.Total,
            ActiveScans:      scanCounts.Running,
            CompletedScans:   scanCounts.Completed,
            TotalFindings:    findingCounts.Total,
            CriticalFindings: findingCounts.Critical,
            HighFindings:     findingCounts.High,
            TopCVEs:          topCVEs,
            RecentCVEs:       recentCVEs,
            RecentScans:      recentScans,
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(stats)
    }
}

func dashboardTimelineHandler(a *app.App) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Scans theo ngày trong 30 ngày qua
        scans, _ := a.ScanRepo.ListRecent(r.Context(), 30)
        writeJSON(w, 200, map[string]interface{}{"scans": scans})
    }
}

func dashboardTopVulnsHandler(a *app.App) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        topVulns, _ := a.RankUC.Execute(r.Context(), rankuc.Input{
            Limit:  20,
            SortBy: "cvss_score",
        })
        writeJSON(w, 200, topVulns)
    }
}
```

### 15.4 Thêm missing repo methods (nếu cần)

Nếu `scanRepo.CountByStatus()` hoặc `findingRepo.CountBySeverity()` chưa tồn tại trong services:

```go
// Tạo trong internal/app/stats_queries.go (helper, không phải service mới)
func (a *App) countScansByStatus(ctx context.Context) (ScanCounts, error) {
    var result struct {
        Total     int `db:"total"`
        Running   int `db:"running"`
        Completed int `db:"completed"`
        Failed    int `db:"failed"`
    }
    err := a.db.QueryRow(ctx, `
        SELECT
            COUNT(*) as total,
            COUNT(*) FILTER (WHERE status='running') as running,
            COUNT(*) FILTER (WHERE status='completed') as completed,
            COUNT(*) FILTER (WHERE status='failed') as failed
        FROM scans
    `).Scan(&result.Total, &result.Running, &result.Completed, &result.Failed)
    return ScanCounts(result), err
}
```

### 15.5 Mount dashboard routes trong router.go

```go
// internal/router/router.go
func New(a *app.App) http.Handler {
    r := chi.NewRouter()
    // ...

    r.Group(func(r chi.Router) {
        r.Use(authMW.RequireAuth)
        // ... other routes ...
        mountDashboardRoutes(r, a)
    })

    return r
}
```

### 15.6 Cập nhật App struct

```go
type App struct {
    // ... existing fields
    BrowseUC    *browseuc.UseCase
    RankUC      *rankuc.UseCase
    LookupUC    *lookupuc.UseCase
}
```

---

## Output

- [x] `internal/router/dashboard_routes.go` ✓ (mountDashboardRoutes — 40 LOC)
- [x] query-service usecases khởi tạo ✓ (DashboardQuerier interface — direct Postgres)
- [x] `GET /api/v1/dashboard` → aggregated stats ✓
- [x] `GET /api/v1/dashboard/timeline` → recent scans ✓
- [x] `GET /api/v1/dashboard/top-vulnerabilities` → top CVEs ✓

## Acceptance Criteria

```bash
TOKEN=<token>

# Dashboard stats
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/dashboard
# → {
#     "total_scans": 5,
#     "active_scans": 1,
#     "completed_scans": 4,
#     "total_findings": 23,
#     "critical_findings": 3,
#     "high_findings": 8,
#     "top_cves": [...],
#     "recent_cves": [...],
#     "recent_scans": [...]
#   }

# Top vulnerabilities
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/dashboard/top-vulnerabilities
# → [{"id":"CVE-2021-44228","cvss":10.0,...},...]
```
