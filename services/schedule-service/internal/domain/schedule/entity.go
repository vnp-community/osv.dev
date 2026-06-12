package entity

import (
	"time"

	"github.com/google/uuid"
)

// ScheduleStatus represents the current state of a schedule.
type ScheduleStatus string

const (
	ScheduleStatusActive   ScheduleStatus = "active"
	ScheduleStatusPaused   ScheduleStatus = "paused"
	ScheduleStatusDisabled ScheduleStatus = "disabled"
)

// ScheduleType defines what kind of scan is triggered.
type ScheduleType string

const (
	ScheduleTypeFullScan        ScheduleType = "full_scan"
	ScheduleTypeIncrementalScan ScheduleType = "incremental_scan"
	ScheduleTypeTargetedScan    ScheduleType = "targeted_scan"
)

// Schedule is the aggregate root for a recurring scan schedule.
type Schedule struct {
	ID          uuid.UUID
	Name        string
	Description string
	CronExpr    string // e.g. "0 2 * * *" — every day at 02:00
	Type        ScheduleType
	TargetIDs   []string // product/asset IDs to scan
	Status      ScheduleStatus
	LastRunAt   *time.Time
	NextRunAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsActive returns true when the schedule should trigger scans.
func (s *Schedule) IsActive() bool {
	return s.Status == ScheduleStatusActive
}

// Pause transitions the schedule to paused state.
func (s *Schedule) Pause() {
	s.Status = ScheduleStatusPaused
	s.UpdatedAt = time.Now()
}

// Resume transitions the schedule back to active state.
func (s *Schedule) Resume() {
	s.Status = ScheduleStatusActive
	s.UpdatedAt = time.Now()
}

// Disable permanently deactivates the schedule.
func (s *Schedule) Disable() {
	s.Status = ScheduleStatusDisabled
	s.UpdatedAt = time.Now()
}

// RecordRun updates last/next run times after a successful trigger.
func (s *Schedule) RecordRun(nextRun time.Time) {
	now := time.Now()
	s.LastRunAt = &now
	s.NextRunAt = &nextRun
	s.UpdatedAt = now
}
