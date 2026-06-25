package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// statsProvider is the minimal interface the stats handler needs.
// Implemented by *postgres.ScanRepo.
type statsProvider interface {
	CountByStatus(ctx context.Context, status string) (int, error)
	CountCompletedToday(ctx context.Context) (int, error)
	CountFindingsToday(ctx context.Context) (int, error)
	CountScheduledActive(ctx context.Context) (int, error)
	GetWeeklyActivity(ctx context.Context) ([]weeklyRow, error)
}

// weeklyRow is the data shape returned by the repo.
// Use a local alias so the handler package has no import cycle.
type weeklyRow struct {
	Day      string
	Scans    int
	Findings int
}

// StatsHandler serves GET /api/v1/scans/stats and GET /api/v1/scans/stats/weekly.
type StatsHandler struct {
	repo statsProvider
}

// ScanStats is the JSON shape returned by GET /api/v1/scans/stats.
type ScanStats struct {
	ActiveScans    int `json:"active_scans"`
	CompletedToday int `json:"completed_today"`
	TotalFindings  int `json:"total_findings"`
	ScheduledScans int `json:"scheduled_scans"`
}

// WeeklyActivity is one element of the array for GET /api/v1/scans/stats/weekly.
type WeeklyActivity struct {
	Day      string `json:"day"`
	Scans    int    `json:"scans"`
	Findings int    `json:"findings"`
}

// GetStats handles GET /api/v1/scans/stats
// Returns KPI counts: active scans, completed today, findings today, scheduled.
// When repo is nil (not yet wired), returns zeros instead of crashing.
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var active, completed, findings, scheduled int
	if h.repo != nil {
		active, _ = h.repo.CountByStatus(ctx, "running")
		completed, _ = h.repo.CountCompletedToday(ctx)
		findings, _ = h.repo.CountFindingsToday(ctx)
		scheduled, _ = h.repo.CountScheduledActive(ctx)
	}

	writeScanStatsJSON(w, http.StatusOK, ScanStats{
		ActiveScans:    active,
		CompletedToday: completed,
		TotalFindings:  findings,
		ScheduledScans: scheduled,
	})
}

// GetWeeklyActivity handles GET /api/v1/scans/stats/weekly
// Returns an array of exactly 7 elements (Mon-Sun relative to today).
func (h *StatsHandler) GetWeeklyActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.repo == nil {
		// Graceful empty response — 7 days of zeros
		writeScanStatsJSON(w, http.StatusOK, buildEmptyWeekly())
		return
	}

	rows, err := h.repo.GetWeeklyActivity(ctx)
	if err != nil {
		writeScanStatsJSON(w, http.StatusOK, buildEmptyWeekly())
		return
	}

	result := make([]WeeklyActivity, len(rows))
	for i, row := range rows {
		result[i] = WeeklyActivity{Day: row.Day, Scans: row.Scans, Findings: row.Findings}
	}
	writeScanStatsJSON(w, http.StatusOK, result)
}

// buildEmptyWeekly returns 7 zero-filled WeeklyActivity entries for the past 7 days.
func buildEmptyWeekly() []WeeklyActivity {
	now := time.Now().UTC()
	result := make([]WeeklyActivity, 7)
	for i := 0; i < 7; i++ {
		result[i] = WeeklyActivity{
			Day:      now.AddDate(0, 0, -(6 - i)).Format("Mon"),
			Scans:    0,
			Findings: 0,
		}
	}
	return result
}

func writeScanStatsJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
