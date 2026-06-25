// Package http — stats_handler.go (scan-service)
// Implements GET /api/v1/scans/stats and GET /api/v1/scans/stats/weekly.
// When ScanStatsRepository is nil, returns safe zero-value defaults.
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// ScanStats represents overall scan status counts.
// CR-008: includes both raw counts and KPI aliases expected by frontend.
type ScanStats struct {
	// Raw status counts
	Total     int `json:"total"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Pending   int `json:"pending"`
	Cancelled int `json:"cancelled"`

	// CR-008 KPI fields — aliases used by dashboard widgets
	ActiveScans    int `json:"active_scans"`    // = Running
	CompletedToday int `json:"completed_today"` // = Completed (approximation when repo is nil)
	TotalFindings  int `json:"total_findings"`  // populated from repo if available
	ScheduledScans int `json:"scheduled_scans"` // = Pending
}

// DailyStats represents daily scan breakdown.
// CR-008 spec: `day` values must be: "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun".
type DailyStats struct {
	Day       string `json:"day"`       // "Mon".."Sun"
	Date      string `json:"date"`      // "2026-06-19"
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
}

// ScanStatsRepository is the interface for scan statistics queries.
// Implemented by postgres repo or a no-op stub.
type ScanStatsRepository interface {
	GetScanStats(ctx context.Context) (*ScanStats, error)
	GetWeeklyStats(ctx context.Context, days int) ([]DailyStats, error)
}

// StatsHandler handles scan statistics endpoints.
type StatsHandler struct {
	repo ScanStatsRepository // may be nil → returns empty defaults
	log  zerolog.Logger
}

// NewStatsHandler creates a new StatsHandler. repo may be nil for graceful stub mode.
func NewStatsHandler(repo ScanStatsRepository, log zerolog.Logger) *StatsHandler {
	return &StatsHandler{repo: repo, log: log}
}

// HandleStats handles GET /api/v1/scans/stats
// Returns zero-value ScanStats when repo is nil (not yet wired).
func (h *StatsHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		// Graceful degradation: DB not wired yet — return zeroed KPI response
		writeScanJSON(w, http.StatusOK, &ScanStats{
			Total:          0,
			ActiveScans:    0,
			CompletedToday: 0,
			TotalFindings:  0,
			ScheduledScans: 0,
		})
		return
	}

	stats, err := h.repo.GetScanStats(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("StatsHandler.HandleStats")
		writeScanJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get scan stats"})
		return
	}
	if stats == nil {
		stats = &ScanStats{}
	}
	// Ensure KPI aliases are populated from raw counts
	if stats.ActiveScans == 0 {
		stats.ActiveScans = stats.Running
	}
	if stats.CompletedToday == 0 {
		stats.CompletedToday = stats.Completed
	}
	if stats.ScheduledScans == 0 {
		stats.ScheduledScans = stats.Pending
	}
	writeScanJSON(w, http.StatusOK, stats)
}


// HandleWeeklyStats handles GET /api/v1/scans/stats/weekly
// Returns empty data array when repo is nil.
func (h *StatsHandler) HandleWeeklyStats(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		// Graceful degradation: return 7-day array with zero values
		// CR-008 spec: response must be array with 7 items, day values Mon..Sun
		writeScanJSON(w, http.StatusOK, h.weeklyArray(nil))
		return
	}

	weekly, err := h.repo.GetWeeklyStats(r.Context(), 7)
	if err != nil {
		h.log.Error().Err(err).Msg("StatsHandler.HandleWeeklyStats")
		writeScanJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get weekly stats"})
		return
	}
	// FIX: CR-008 spec — return direct array, not wrapped {data:[]}
	// Populate Day field from date string
	writeScanJSON(w, http.StatusOK, h.weeklyArray(weekly))
}

// weeklyArray returns 7-item array with Mon..Sun day labels.
// If db slice is nil or < 7 items, fills with zeros for missing days.
func (h *StatsHandler) weeklyArray(db []DailyStats) []DailyStats {
	// Build a map date → stats from DB data
	dbMap := make(map[string]DailyStats, len(db))
	for _, d := range db {
		dbMap[d.Date] = d
	}

	now := time.Now().UTC()
	result := make([]DailyStats, 7)
	for i := 6; i >= 0; i-- {
		day := now.AddDate(0, 0, -(6 - i))
		dateStr := day.Format("2006-01-02")
		dayName := day.Weekday().String()[:3] // "Mon", "Tue", ...
		if s, ok := dbMap[dateStr]; ok {
			s.Day = dayName
			s.Date = dateStr
			result[i] = s
		} else {
			result[i] = DailyStats{Day: dayName, Date: dateStr}
		}
	}
	return result
}
