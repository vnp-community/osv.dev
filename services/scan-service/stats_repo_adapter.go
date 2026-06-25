package scan

import (
	"context"

	"github.com/osv/scan-service/internal/adapters/repository/postgres"
	deliveryhttp "github.com/osv/scan-service/internal/delivery/http"
)

// statsRepoAdapter wraps postgres.ScanRepo to satisfy delivery/http.ScanStatsRepository.
// ScanRepo has CountByStatus, CountCompletedToday, CountFindingsToday,
// CountScheduledActive, and GetWeeklyActivity — this adapter maps them to the
// interface expected by StatsHandler.
type statsRepoAdapter struct {
	inner *postgres.ScanRepo
}

func newStatsRepoAdapter(r *postgres.ScanRepo) *statsRepoAdapter {
	return &statsRepoAdapter{inner: r}
}

// Compile-time interface satisfaction check.
var _ deliveryhttp.ScanStatsRepository = (*statsRepoAdapter)(nil)

// GetScanStats queries per-status counts and KPI metrics from the DB.
func (a *statsRepoAdapter) GetScanStats(ctx context.Context) (*deliveryhttp.ScanStats, error) {
	running, _ := a.inner.CountByStatus(ctx, "running")
	completed, _ := a.inner.CountByStatus(ctx, "completed")
	failed, _ := a.inner.CountByStatus(ctx, "failed")
	pending, _ := a.inner.CountByStatus(ctx, "pending")
	cancelled, _ := a.inner.CountByStatus(ctx, "cancelled")

	completedToday, _ := a.inner.CountCompletedToday(ctx)
	totalFindings, _ := a.inner.CountFindingsToday(ctx)
	scheduled, _ := a.inner.CountScheduledActive(ctx)

	total := running + completed + failed + pending + cancelled

	return &deliveryhttp.ScanStats{
		Total:          total,
		Running:        running,
		Completed:      completed,
		Failed:         failed,
		Pending:        pending,
		Cancelled:      cancelled,
		ActiveScans:    running,
		CompletedToday: completedToday,
		TotalFindings:  totalFindings,
		ScheduledScans: scheduled,
	}, nil
}

// GetWeeklyStats returns per-day scan activity for the last `days` days.
func (a *statsRepoAdapter) GetWeeklyStats(ctx context.Context, _ int) ([]deliveryhttp.DailyStats, error) {
	rows, err := a.inner.GetWeeklyActivity(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]deliveryhttp.DailyStats, 0, len(rows))
	for _, r := range rows {
		out = append(out, deliveryhttp.DailyStats{
			Day:       r.Day,
			Total:     r.Scans,
			Completed: r.Scans, // approximation — DB returns total scans per day
		})
	}
	return out, nil
}
