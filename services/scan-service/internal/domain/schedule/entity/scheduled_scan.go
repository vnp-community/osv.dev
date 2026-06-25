package entity

import (
	"encoding/json"
	"time"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// ScheduledScan is a recurring scan triggered by cron or interval.
type ScheduledScan struct {
	ID              uuid.UUID       `db:"id"`
	UserID          uuid.UUID       `db:"user_id"`
	Targets         []string        `db:"targets"`
	ScanType        string          `db:"scan_type"`
	CronExpr        string          `db:"cron_expr"`        // e.g. "0 2 * * *"
	IntervalMinutes int             `db:"interval_minutes"` // alternative to cron
	Status          string          `db:"status"`           // active|paused|expired
	NextRunAt       time.Time       `db:"next_run_at"`
	LastRunAt       *time.Time      `db:"last_run_at"`
	Options         json.RawMessage `db:"options"`
	CreatedAt       time.Time       `db:"created_at"`
}

// CalculateNextRun computes the next scheduled run time from the given base time.
func (s *ScheduledScan) CalculateNextRun(from time.Time) time.Time {
	if s.CronExpr != "" {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(s.CronExpr)
		if err == nil {
			return schedule.Next(from)
		}
	}
	if s.IntervalMinutes > 0 {
		return from.Add(time.Duration(s.IntervalMinutes) * time.Minute)
	}
	return from.Add(24 * time.Hour) // fallback: daily
}

// ComputeNextRun is an alias matching the task spec.
func (s *ScheduledScan) ComputeNextRun() (time.Time, error) {
    return s.CalculateNextRun(time.Now().UTC()), nil
}

// FrequencyCronMap maps predefined frequency names to cron expressions
var FrequencyCronMap = map[string]string{
    "hourly": "0 * * * *",    // Every hour at :00
    "daily":  "0 2 * * *",    // Daily at 2:00 AM UTC
    "weekly": "0 2 * * 0",    // Sunday at 2:00 AM UTC
}

// SetFrequency sets the cron expression from a named frequency
func (s *ScheduledScan) SetFrequency(frequency string) {
    if cronExpr, ok := FrequencyCronMap[frequency]; ok {
        s.CronExpr = cronExpr
    }
}
