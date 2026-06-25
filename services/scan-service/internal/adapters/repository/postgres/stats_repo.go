package postgres

import (
	"context"
	"time"
)

// WeeklyActivityRow holds data for one day in the weekly activity chart.
type WeeklyActivityRow struct {
	Day      string
	Scans    int
	Findings int
}

// CountByStatus returns the number of scans with the given status.
func (r *ScanRepo) CountByStatus(ctx context.Context, status string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM scan.scans WHERE status = $1`, status,
	).Scan(&count)
	return count, err
}

// CountCompletedToday returns scans completed since UTC midnight today.
func (r *ScanRepo) CountCompletedToday(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM scan.scans
		WHERE status = 'completed'
		  AND completed_at >= CURRENT_DATE
		  AND completed_at <  CURRENT_DATE + INTERVAL '1 day'
	`).Scan(&count)
	return count, err
}

// CountFindingsToday returns the sum of finding_count for scans completed today.
func (r *ScanRepo) CountFindingsToday(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(finding_count), 0) FROM scan.scans
		WHERE status = 'completed'
		  AND completed_at >= CURRENT_DATE
		  AND completed_at <  CURRENT_DATE + INTERVAL '1 day'
	`).Scan(&count)
	return count, err
}

// CountScheduledActive returns the number of active scheduled scans.
// Falls back to counting pending scans if the scheduled_scans table does not exist.
func (r *ScanRepo) CountScheduledActive(ctx context.Context) (int, error) {
	var count int
	// Try scheduled_scans table first
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM scan.scheduled_scans WHERE enabled = true
	`).Scan(&count)
	if err != nil {
		// Fallback: pending scans
		r.db.QueryRow(ctx, `SELECT COUNT(*) FROM scan.scans WHERE status = 'pending'`).Scan(&count) //nolint:errcheck
	}
	return count, nil
}

// GetWeeklyActivity returns scan and finding counts per day for the past 7 days.
// Always returns exactly 7 elements, filling zeros for days with no data.
func (r *ScanRepo) GetWeeklyActivity(ctx context.Context) ([]WeeklyActivityRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			to_char(date_trunc('day', started_at AT TIME ZONE 'UTC'), 'Dy') AS day_abbr,
			COUNT(*) FILTER (WHERE status = 'completed')                    AS scans,
			COALESCE(SUM(finding_count), 0)                                 AS findings
		FROM scan.scans
		WHERE started_at >= NOW() - INTERVAL '7 days'
		GROUP BY date_trunc('day', started_at AT TIME ZONE 'UTC')
		ORDER BY date_trunc('day', started_at AT TIME ZONE 'UTC')
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Collect DB data by abbreviated weekday name (Mon, Tue, ...)
	dbData := map[string]WeeklyActivityRow{}
	for rows.Next() {
		var row WeeklyActivityRow
		if err := rows.Scan(&row.Day, &row.Scans, &row.Findings); err != nil {
			continue
		}
		dbData[row.Day] = row
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build 7-element slice starting from 6 days ago → today, ensuring all slots filled
	result := make([]WeeklyActivityRow, 7)
	now := time.Now().UTC()
	for i := 0; i < 7; i++ {
		day := now.AddDate(0, 0, -(6 - i))
		dayStr := day.Format("Mon") // e.g. "Mon", "Tue"
		if row, ok := dbData[dayStr]; ok {
			result[i] = row
		} else {
			result[i] = WeeklyActivityRow{Day: dayStr, Scans: 0, Findings: 0}
		}
	}
	return result, nil
}
