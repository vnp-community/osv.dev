package grading

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
)

// Grade is a letter grade representing a product's security posture.
type Grade string

const (
	GradeA Grade = "A" // Excellent: no high/critical findings
	GradeB Grade = "B" // Good: no critical, <5 high
	GradeC Grade = "C" // Fair: no critical, 5-10 high OR <5 medium
	GradeD Grade = "D" // Poor: ≥1 critical OR ≥10 high
	GradeF Grade = "F" // Failing: ≥3 critical OR ≥20 high
)

// SeverityCount holds finding counts by severity.
type SeverityCount struct {
	Critical int
	High     int
	Medium   int
	Low      int
	Info     int
}

// Total returns total finding count.
func (s *SeverityCount) Total() int {
	return s.Critical + s.High + s.Medium + s.Low + s.Info
}

// ProductGrade is the computed grade for a product.
type ProductGrade struct {
	ProductID      uuid.UUID
	Grade          Grade
	RiskScore      float64   // 0–100 (lower is worse)
	SeverityCount  SeverityCount
	ComputedAt     time.Time
}

// FindingSummaryProvider retrieves finding counts for a product.
type FindingSummaryProvider interface {
	GetActiveSeverityCounts(ctx context.Context, productID uuid.UUID) (*SeverityCount, error)
}

// GradeRepository persists computed grades.
type GradeRepository interface {
	Save(ctx context.Context, g *ProductGrade) error
	GetByProductID(ctx context.Context, productID uuid.UUID) (*ProductGrade, error)
	ListAll(ctx context.Context) ([]*ProductGrade, error)
}

// EventPublisher publishes NATS events.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload map[string]any) error
}

// ComputeGradeUseCase computes and stores product security grades.
type ComputeGradeUseCase struct {
	summaryProvider FindingSummaryProvider
	gradeRepo       GradeRepository
	eventPub        EventPublisher
}

// NewComputeGrade creates a new ComputeGradeUseCase.
func NewComputeGrade(sp FindingSummaryProvider, gr GradeRepository, ep EventPublisher) *ComputeGradeUseCase {
	return &ComputeGradeUseCase{summaryProvider: sp, gradeRepo: gr, eventPub: ep}
}

// Execute computes the security grade for a product.
func (uc *ComputeGradeUseCase) Execute(ctx context.Context, productID uuid.UUID) (*ProductGrade, error) {
	counts, err := uc.summaryProvider.GetActiveSeverityCounts(ctx, productID)
	if err != nil {
		return nil, err
	}

	grade := ComputeGrade(counts)
	riskScore := ComputeRiskScore(counts)

	pg := &ProductGrade{
		ProductID:     productID,
		Grade:         grade,
		RiskScore:     riskScore,
		SeverityCount: *counts,
		ComputedAt:    time.Now().UTC(),
	}

	if err := uc.gradeRepo.Save(ctx, pg); err != nil {
		return nil, err
	}

	_ = uc.eventPub.Publish(ctx, "defectdojo.report.graded", map[string]any{
		"product_id": productID.String(),
		"grade":      string(grade),
		"risk_score": riskScore,
		"critical":   counts.Critical,
		"high":       counts.High,
	})

	return pg, nil
}

// ─── Grading Algorithm ────────────────────────────────────────────────────────

// ComputeGrade assigns a letter grade based on finding severity counts.
// Algorithm mirrors Django DefectDojo system_settings grading:
//   F: critical >= 3 || high >= 20
//   D: critical >= 1 || high >= 10
//   C: high >= 5 || medium >= 10
//   B: high < 5 && critical == 0 && medium < 10
//   A: all findings are low or info only
func ComputeGrade(c *SeverityCount) Grade {
	switch {
	case c.Critical >= 3 || c.High >= 20:
		return GradeF
	case c.Critical >= 1 || c.High >= 10:
		return GradeD
	case c.High >= 5 || c.Medium >= 10:
		return GradeC
	case c.High >= 1 || c.Medium >= 5:
		return GradeB
	default:
		return GradeA
	}
}

// ComputeRiskScore calculates a numeric risk score (0 = perfect, 100 = worst).
// Weights: Critical=10, High=5, Medium=2, Low=0.5
// Score is capped at 100 and inverted so that 100=perfect.
func ComputeRiskScore(c *SeverityCount) float64 {
	raw := float64(c.Critical)*10 + float64(c.High)*5 + float64(c.Medium)*2 + float64(c.Low)*0.5
	// Normalize: score of 100 raw maps to 0 (worst), 0 raw maps to 100 (best)
	normalized := math.Max(0, 100-raw)
	return math.Round(normalized*10) / 10 // 1 decimal place
}
