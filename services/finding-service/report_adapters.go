package finding

import (
	"context"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/finding"
	"github.com/osv/finding-service/internal/infra/postgres"
	stats_uc "github.com/osv/finding-service/internal/usecase"
	reportuc "github.com/osv/finding-service/internal/usecase/report"
)

type reportFindingAdapter struct {
	repo *postgres.FindingRepo
}

func (a *reportFindingAdapter) ListForReport(ctx context.Context, opts *reportuc.ListOptions) ([]*reportuc.FindingRow, error) {
	var pid *uuid.UUID
	if opts.ProductID != "" {
		if id, err := uuid.Parse(opts.ProductID); err == nil {
			pid = &id
		}
	}
	filter := finding.FindingFilter{
		ProductID:  pid,
		ActiveOnly: opts.ActiveOnly,
	}
	ch, err := a.repo.ListForReport(ctx, filter)
	if err != nil {
		return nil, err
	}

	var rows []*reportuc.FindingRow
	for f := range ch {
		// Filter by engagement if specified
		if opts.EngagementID != nil && f.EngagementID.String() != *opts.EngagementID {
			continue
		}
		// Filter by severity if specified
		if len(opts.Severities) > 0 {
			match := false
			for _, s := range opts.Severities {
				if string(f.Severity) == s {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		status := "Open"
		if f.IsMitigated {
			status = "Mitigated"
		}

		rows = append(rows, &reportuc.FindingRow{
			ID:          f.ID.String(),
			Title:       f.Title,
			Severity:    string(f.Severity),
			Status:      status,
			CVE:         f.CVE,
			FilePath:    f.FilePath,
			Description: f.Description,
			FoundDate:   f.Date,
		})
	}
	return rows, nil
}

type pubInterface interface {
	Publish(ctx context.Context, subject string, msg interface{}) error
}

type reportPublisherAdapter struct {
	pub pubInterface
}

func (a *reportPublisherAdapter) Publish(ctx context.Context, subject string, payload map[string]any) error {
	if a.pub == nil {
		return nil
	}
	return a.pub.Publish(ctx, subject, payload)
}

type statsFindingAdapter struct {
	repo *postgres.FindingRepo
}

// NewStatsFindingAdapter returns an exported adapter that bridges *postgres.FindingRepo
// to the usecase.FindingRepository interface required by StatsUseCase.
func NewStatsFindingAdapter(repo *postgres.FindingRepo) *statsFindingAdapter {
	return &statsFindingAdapter{repo: repo}
}

func (a *statsFindingAdapter) CountBySeverity(ctx context.Context) (*stats_uc.SeverityCounts, error) {
	return a.repo.CountBySeverity(ctx)
}

func (a *statsFindingAdapter) GetSLASummary(ctx context.Context) (*stats_uc.SLASummary, error) {
	data, err := a.repo.GetSLASummary(ctx, "")
	if err != nil {
		return nil, err
	}
	return &stats_uc.SLASummary{
		CompliancePct: data.CompliancePct,
		AtRisk:        data.AtRisk,
		Breached:      data.Breached,
	}, nil
}

func (a *statsFindingAdapter) GetMonthlyTrend(ctx context.Context, interval string) ([]stats_uc.TrendPoint, error) {
	return a.repo.GetMonthlyTrend(ctx, interval)
}

func (a *statsFindingAdapter) GetProductGrades(ctx context.Context) ([]stats_uc.ProductGrade, error) {
	return a.repo.GetProductGrades(ctx)
}

func (a *statsFindingAdapter) GetSLABreaches(ctx context.Context, limit int) ([]stats_uc.SLABreachItem, error) {
	return a.repo.GetSLABreaches(ctx, limit)
}

func (a *statsFindingAdapter) CountActiveByCVEIds(ctx context.Context, cveIds []string) (int, error) {
	return a.repo.CountActiveByCVEIds(ctx, cveIds)
}
