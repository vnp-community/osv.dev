package usecase

import (
	"context"
	"sync"
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

type SeverityCounts struct {
	Critical int
	High     int
	Medium   int
	Low      int
}

type SLASummary struct {
	CompliancePct float64
	AtRisk        int
	Breached      int
}

type FindingRepository interface {
	CountBySeverity(ctx context.Context) (*SeverityCounts, error)
	GetSLASummary(ctx context.Context) (*SLASummary, error)
	GetMonthlyTrend(ctx context.Context, interval string) ([]TrendPoint, error)
	GetProductGrades(ctx context.Context) ([]ProductGrade, error)
	GetSLABreaches(ctx context.Context, limit int) ([]SLABreachItem, error)
	CountActiveByCVEIds(ctx context.Context, cveIds []string) (int, error)
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

// CountActiveByCVEIds returns the count of active findings that map to any of the given CVE IDs
func (uc *StatsUseCase) CountActiveByCVEIds(ctx context.Context, cveIds []string) (int, error) {
	return uc.findingRepo.CountActiveByCVEIds(ctx, cveIds)
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
