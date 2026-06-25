# TASK-BE-006 — finding-service: Internal Aggregate Endpoints for BFF

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-006 |
| **Service** | `services/finding-service` |
| **Solution Ref** | [SOL-UI-002 §2.4](../solutions/SOL-UI-002-dashboard-bff-sse.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | — |
| **Estimated** | 3h |
| **Status** | ✅ DONE |

---

## Context

Dashboard BFF (TASK-BE-005) cần gọi **internal endpoints** trực tiếp vào finding-service (không qua gateway). Các endpoints này chỉ accessible từ internal Docker network, không có auth middleware.

---

## Goal

Thêm `/internal/*` route group vào finding-service với 4 aggregate endpoints.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/finding-service/internal/adapter/http/internal_handler.go` |
| CREATE | `services/finding-service/internal/usecase/stats_usecase.go` |
| MODIFY | `services/finding-service/internal/adapter/http/router.go` |

---

## Implementation

### File 1: `services/finding-service/internal/usecase/stats_usecase.go`

```go
package usecase

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DashboardStats is the aggregate KPI data for the dashboard
type DashboardStats struct {
	CriticalFindings int     `json:"critical_findings"`
	HighFindings     int     `json:"high_findings"`
	MediumFindings   int     `json:"medium_findings"`
	LowFindings      int     `json:"low_findings"`
	SecurityGrade    string  `json:"security_grade"`
	SecurityScore    int     `json:"security_score"`
	SLACompliance    float64 `json:"sla_compliance"`
	SLAAtRisk        int     `json:"sla_at_risk"`
	SLABreached      int     `json:"sla_breached"`
}

// TrendPoint is one month's finding counts
type TrendPoint struct {
	Month    string `json:"month"` // "2026-01"
	Critical int    `json:"critical"`
	High     int    `json:"high"`
	Medium   int    `json:"medium"`
	Low      int    `json:"low"`
}

// ProductGrade is the security grade for a product
type ProductGrade struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Grade         string `json:"grade"`
	Score         int    `json:"score"`
	CriticalCount int    `json:"critical_count"`
	HighCount     int    `json:"high_count"`
	Trend         string `json:"trend"`
}

// SLABreachItem is an SLA-breached finding
type SLABreachItem struct {
	FindingID   string `json:"finding_id"`
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	ProductName string `json:"product_name"`
	DaysOverdue int    `json:"days_overdue"`
	ExpiresAt   string `json:"expires_at"`
}

// StatsUseCase computes aggregate statistics for the dashboard
type StatsUseCase struct {
	findingRepo FindingRepository
}

func NewStatsUseCase(findingRepo FindingRepository) *StatsUseCase {
	return &StatsUseCase{findingRepo: findingRepo}
}

// GetDashboardStats returns KPI data for the dashboard
func (uc *StatsUseCase) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	var stats DashboardStats
	var mu sync.Mutex
	var wg sync.WaitGroup
	var errs []error

	wg.Add(2)

	// Query 1: Severity counts
	go func() {
		defer wg.Done()
		counts, err := uc.findingRepo.CountBySeverity(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errs = append(errs, err)
			return
		}
		stats.CriticalFindings = counts.Critical
		stats.HighFindings = counts.High
		stats.MediumFindings = counts.Medium
		stats.LowFindings = counts.Low
	}()

	// Query 2: SLA stats
	go func() {
		defer wg.Done()
		sla, err := uc.findingRepo.GetSLASummary(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errs = append(errs, err)
			return
		}
		stats.SLACompliance = sla.CompliancePct
		stats.SLAAtRisk = sla.AtRisk
		stats.SLABreached = sla.Breached
	}()

	wg.Wait()

	// Compute security grade
	stats.SecurityGrade = computeSecurityGrade(stats.CriticalFindings, stats.HighFindings)
	stats.SecurityScore = gradeToScore(stats.SecurityGrade)

	return &stats, nil
}

// GetRiskTrend returns monthly risk trend data
func (uc *StatsUseCase) GetRiskTrend(ctx context.Context, period string) ([]TrendPoint, error) {
	var interval string
	switch period {
	case "90d":
		interval = "90 days"
	case "1y":
		interval = "1 year"
	default:
		interval = "30 days"
	}
	return uc.findingRepo.GetMonthlyTrend(ctx, interval)
}

// GetProductGrades returns security grades for all products
func (uc *StatsUseCase) GetProductGrades(ctx context.Context) ([]ProductGrade, error) {
	return uc.findingRepo.GetProductGrades(ctx)
}

// GetSLABreaches returns top SLA-breached findings
func (uc *StatsUseCase) GetSLABreaches(ctx context.Context, limit int) ([]SLABreachItem, error) {
	return uc.findingRepo.GetSLABreaches(ctx, limit)
}

// computeSecurityGrade computes A-F grade from finding counts
func computeSecurityGrade(critical, high int) string {
	if critical > 0 {
		return "F"
	}
	if high > 5 {
		return "D"
	}
	if high > 0 {
		return "C"
	}
	return "B"
}

func gradeToScore(grade string) int {
	switch grade {
	case "A":
		return 95
	case "B":
		return 80
	case "C":
		return 65
	case "D":
		return 50
	default: // F
		return 30
	}
}
```

### SQL queries needed (add to finding repository):

```sql
-- CountBySeverity: active non-duplicate findings
SELECT
    COUNT(*) FILTER (WHERE severity = 'Critical') AS critical,
    COUNT(*) FILTER (WHERE severity = 'High')     AS high,
    COUNT(*) FILTER (WHERE severity = 'Medium')   AS medium,
    COUNT(*) FILTER (WHERE severity = 'Low')      AS low
FROM findings
WHERE active = true AND is_duplicate = false;

-- GetSLASummary
SELECT
    ROUND(
        100.0 * COUNT(*) FILTER (WHERE sla_expiration_date > NOW() OR sla_expiration_date IS NULL)
        / NULLIF(COUNT(*), 0), 2
    ) AS compliance_pct,
    COUNT(*) FILTER (WHERE sla_expiration_date < NOW()) AS breached,
    COUNT(*) FILTER (WHERE sla_expiration_date BETWEEN NOW() AND NOW() + INTERVAL '7 days') AS at_risk
FROM findings
WHERE active = true AND is_duplicate = false;

-- GetMonthlyTrend (parameterized interval)
SELECT
    TO_CHAR(DATE_TRUNC('month', created_at), 'YYYY-MM') AS month,
    COUNT(*) FILTER (WHERE severity = 'Critical') AS critical,
    COUNT(*) FILTER (WHERE severity = 'High')     AS high,
    COUNT(*) FILTER (WHERE severity = 'Medium')   AS medium,
    COUNT(*) FILTER (WHERE severity = 'Low')      AS low
FROM findings
WHERE created_at >= NOW() - $1::interval
GROUP BY month
ORDER BY month ASC;

-- GetProductGrades
SELECT
    p.id, p.name,
    COUNT(f.*) FILTER (WHERE f.severity = 'Critical' AND f.active AND NOT f.is_duplicate) AS critical_count,
    COUNT(f.*) FILTER (WHERE f.severity = 'High' AND f.active AND NOT f.is_duplicate) AS high_count
FROM products p
LEFT JOIN findings f ON f.product_id = p.id
GROUP BY p.id, p.name
ORDER BY critical_count DESC, high_count DESC;

-- GetSLABreaches top N
SELECT
    f.id AS finding_id,
    f.title,
    f.severity,
    p.name AS product_name,
    EXTRACT(DAY FROM NOW() - f.sla_expiration_date)::INT AS days_overdue,
    f.sla_expiration_date AS expires_at
FROM findings f
JOIN products p ON p.id = f.product_id
WHERE f.active = true
  AND f.is_duplicate = false
  AND f.sla_expiration_date < NOW()
ORDER BY f.sla_expiration_date ASC
LIMIT $1;
```

### File 2: `services/finding-service/internal/adapter/http/internal_handler.go`

```go
package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
)

// InternalHandler serves /internal/* endpoints for BFF consumption
// These endpoints MUST NOT be exposed to the public internet
// (Gateway must not route external traffic to these paths)
type InternalHandler struct {
	statsUC StatsUseCase
}

// GET /internal/stats
func (h *InternalHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.statsUC.GetDashboardStats(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("failed to get dashboard stats")
		http.Error(w, `{"error":"INTERNAL_ERROR"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// GET /internal/risk-trend?period=30d
func (h *InternalHandler) GetRiskTrend(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	trend, err := h.statsUC.GetRiskTrend(r.Context(), period)
	if err != nil {
		log.Error().Err(err).Msg("failed to get risk trend")
		http.Error(w, `{"error":"INTERNAL_ERROR"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trend)
}

// GET /internal/product-grades
func (h *InternalHandler) GetProductGrades(w http.ResponseWriter, r *http.Request) {
	grades, err := h.statsUC.GetProductGrades(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("failed to get product grades")
		http.Error(w, `{"error":"INTERNAL_ERROR"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(grades)
}

// GET /internal/sla-breaches?limit=5
func (h *InternalHandler) GetSLABreaches(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 5
	}

	breaches, err := h.statsUC.GetSLABreaches(r.Context(), limit)
	if err != nil {
		log.Error().Err(err).Msg("failed to get SLA breaches")
		http.Error(w, `{"error":"INTERNAL_ERROR"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(breaches)
}

// POST /internal/findings/count-by-cve-ids  (for data-service KEV cross-count)
func (h *InternalHandler) CountByCVEIds(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CVEIds []string `json:"cve_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"VALIDATION_ERROR"}`, 400)
		return
	}

	count, err := h.statsUC.CountActiveByCVEIds(r.Context(), req.CVEIds)
	if err != nil {
		http.Error(w, `{"error":"INTERNAL_ERROR"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"count": count})
}
```

### Router additions:

```go
// services/finding-service/internal/adapter/http/router.go

// Internal routes — NO auth middleware, internal network only
internalMux := http.NewServeMux()
internalMux.HandleFunc("GET /internal/stats",              h.Internal.GetStats)
internalMux.HandleFunc("GET /internal/risk-trend",         h.Internal.GetRiskTrend)
internalMux.HandleFunc("GET /internal/product-grades",     h.Internal.GetProductGrades)
internalMux.HandleFunc("GET /internal/sla-breaches",       h.Internal.GetSLABreaches)
internalMux.HandleFunc("POST /internal/findings/count-by-cve-ids", h.Internal.CountByCVEIds)

// Mount at same mux
mux.Handle("/internal/", internalMux)
```

---

## Verification

```bash
cd services/finding-service
go build ./...

# With service running:
curl http://localhost:8085/internal/stats | jq .
# Expected: {"critical_findings":N,"high_findings":N,...,"security_grade":"B"}

curl "http://localhost:8085/internal/risk-trend?period=30d" | jq 'length'
# Expected: 1-12 (monthly data points)

curl http://localhost:8085/internal/product-grades | jq '.[0].grade'
# Expected: "A" | "B" | "C" | "D" | "F"

curl "http://localhost:8085/internal/sla-breaches?limit=5" | jq '.[0].days_overdue'
# Expected: number > 0
```

---

## Checklist

- [x] `stats_usecase.go` có 4 methods: GetDashboardStats, GetRiskTrend, GetProductGrades, GetSLABreaches
- [x] `computeSecurityGrade` logic: Critical > 0 → F, High > 5 → D, High > 0 → C, else B
- [x] `internal_handler.go` có 5 handlers tất cả không dùng auth middleware
- [x] SQL queries parameterized (không string concat)
- [x] `/internal/` routes mount trong router nhưng KHÔNG exposed qua gateway
- [x] `go build ./...` thành công
- [x] `GetRiskTrend` trả về array của `{"month":"YYYY-MM","critical":N,...}` sorted ASC

## Notes for AI

- `/internal/*` paths KHÔNG nên có trong gateway router — chỉ internal services mới gọi
- Nếu DB không có `sla_expiration_date` column, check schema của `findings` table trước
- `CountActiveByCVEIds` thêm vào `StatsUseCase` nếu chưa có
