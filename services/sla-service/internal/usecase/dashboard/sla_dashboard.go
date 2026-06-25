// Package dashboard provides the SLA dashboard and violations use cases.
package dashboard

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
)

// ─── Dashboard Output ─────────────────────────────────────────────────────────

// SLADashboardOutput is the response for GET /api/v2/sla-dashboard.
type SLADashboardOutput struct {
	TotalActive       int                        `json:"total_active"`
	WithinSLA         int                        `json:"within_sla"`
	SLABreached       int                        `json:"sla_breached"`
	BreachRatePct     float64                    `json:"breach_rate_pct"`
	ExpiringSoon7Days int                        `json:"expiring_soon_7_days"`
	BySeverity        map[string]SLASeverityStat `json:"by_severity"`
	ByProduct         []SLAProductStat           `json:"by_product,omitempty"`
}

// SLASeverityStat aggregates SLA status per severity.
type SLASeverityStat struct {
	Total       int     `json:"total"`
	Breached    int     `json:"breached"`
	BreachPct   float64 `json:"breach_pct"`
	AvgDaysLeft float64 `json:"avg_days_left"`
}

// SLAProductStat aggregates SLA status per product.
type SLAProductStat struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	BreachCount int     `json:"breach_count"`
	TotalActive int     `json:"total_active"`
	BreachPct   float64 `json:"breach_pct"`
}

// ─── SLA Violation ────────────────────────────────────────────────────────────

// SLAViolation represents a finding that has breached its SLA deadline.
type SLAViolation struct {
	FindingID          uuid.UUID `json:"finding_id"`
	Title              string    `json:"title"`
	Severity           string    `json:"severity"`
	ProductID          uuid.UUID `json:"product_id"`
	SLAExpirationDate  time.Time `json:"sla_expiration_date"`
	DaysOverdue        int       `json:"days_overdue"`
}

// ─── Repository ───────────────────────────────────────────────────────────────

// ViolationRepository queries SLA violations from the local DB.
type ViolationRepository interface {
	// GetDashboardStats returns aggregate stats, optionally scoped to a product.
	GetDashboardStats(ctx context.Context, productID *uuid.UUID) (*DashboardStats, error)
	// ListViolations returns breached findings, optionally filtered.
	ListViolations(ctx context.Context, filter ViolationFilter) ([]*SLAViolation, int, error)
}

// DashboardStats is the raw aggregate from the DB.
type DashboardStats struct {
	TotalActive       int
	Breached          int
	ExpiringSoon7Days int
	BySeverity        map[string]struct{ Total, Breached int }
}

// ViolationFilter controls ListViolations query.
type ViolationFilter struct {
	ProductID     *uuid.UUID
	Severity      *string
	MinDaysOverdue int
	Limit         int
	Offset        int
}

// ─── SLADashboardUseCase ──────────────────────────────────────────────────────

// SLADashboardUseCase computes the dashboard summary.
type SLADashboardUseCase struct {
	repo ViolationRepository
}

// NewSLADashboard creates a new SLADashboardUseCase.
func NewSLADashboard(r ViolationRepository) *SLADashboardUseCase {
	return &SLADashboardUseCase{repo: r}
}

// Execute computes the SLA dashboard, optionally scoped to a product.
func (uc *SLADashboardUseCase) Execute(ctx context.Context, productID *uuid.UUID) (*SLADashboardOutput, error) {
	stats, err := uc.repo.GetDashboardStats(ctx, productID)
	if err != nil {
		return nil, err
	}

	within := stats.TotalActive - stats.Breached

	var breachRate float64
	if stats.TotalActive > 0 {
		breachRate = math.Round(float64(stats.Breached)/float64(stats.TotalActive)*1000) / 10
	}

	bySeverity := make(map[string]SLASeverityStat, len(stats.BySeverity))
	for sev, s := range stats.BySeverity {
		var pct float64
		if s.Total > 0 {
			pct = math.Round(float64(s.Breached)/float64(s.Total)*1000) / 10
		}
		bySeverity[sev] = SLASeverityStat{
			Total:     s.Total,
			Breached:  s.Breached,
			BreachPct: pct,
		}
	}

	return &SLADashboardOutput{
		TotalActive:       stats.TotalActive,
		WithinSLA:         within,
		SLABreached:       stats.Breached,
		BreachRatePct:     breachRate,
		ExpiringSoon7Days: stats.ExpiringSoon7Days,
		BySeverity:        bySeverity,
	}, nil
}
