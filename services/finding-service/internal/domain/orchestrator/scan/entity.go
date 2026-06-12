// Package scan defines the import pipeline domain for the DefectDojo scan orchestrator.
package scan

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidTransition = errors.New("invalid status transition")

// ImportStatus represents the lifecycle stage of a scan import operation.
type ImportStatus string

const (
	ImportStatusPending       ImportStatus = "pending"
	ImportStatusParsing       ImportStatus = "parsing"
	ImportStatusDeduplicating ImportStatus = "deduplicating"
	ImportStatusSaving        ImportStatus = "saving"
	ImportStatusCompleted     ImportStatus = "completed"
	ImportStatusFailed        ImportStatus = "failed"
)

// validTransitions defines which status transitions are allowed.
var validTransitions = map[ImportStatus][]ImportStatus{
	ImportStatusPending:       {ImportStatusParsing, ImportStatusFailed},
	ImportStatusParsing:       {ImportStatusDeduplicating, ImportStatusFailed},
	ImportStatusDeduplicating: {ImportStatusSaving, ImportStatusFailed},
	ImportStatusSaving:        {ImportStatusCompleted, ImportStatusFailed},
	ImportStatusCompleted:     {},
	ImportStatusFailed:        {ImportStatusPending}, // allow retry
}

// CanTransitionTo returns true if the status transition is valid.
func (s ImportStatus) CanTransitionTo(next ImportStatus) bool {
	for _, allowed := range validTransitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// ScanImport represents one scan result import pipeline run.
type ScanImport struct {
	ID           uuid.UUID
	TestID       uuid.UUID
	EngagementID uuid.UUID
	ProductID    uuid.UUID
	ScanType     string
	Status       ImportStatus
	FileKey      *string       // MinIO object key
	Options      ImportOptions
	Result       *ImportResult
	ErrorMsg     string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Transition moves to the next status, returning ErrInvalidTransition if not allowed.
func (si *ScanImport) Transition(next ImportStatus) error {
	if !si.Status.CanTransitionTo(next) {
		return ErrInvalidTransition
	}
	si.Status = next
	si.UpdatedAt = time.Now().UTC()
	return nil
}

// ImportOptions holds all configuration parameters for the import pipeline.
type ImportOptions struct {
	// Auto-create context
	AutoCreateContext bool
	ProductName       string
	ProductTypeName   string
	EngagementName    string
	TestTitle         string

	// Import behavior
	Active                       bool
	Verified                     bool
	MinimumSeverity              string
	CloseOldFindings             bool
	CloseOldFindingsProductScope bool
	DoNotReactivate              bool
	DeduplicationOnEngagement    bool

	// CI/CD metadata
	Version    string
	BuildID    string
	CommitHash string
	BranchTag  string
	Service    string

	// Finding grouping
	GroupBy                           string
	CreateFindingGroupsForAllFindings bool

	// Tags
	Tags                 []string
	ApplyTagsToFindings  bool
	ApplyTagsToEndpoints bool

	// Additional endpoints
	EndpointsToAdd []string

	// Auth
	RequestorUserID string
}

// ImportResult holds the final statistics from a completed import.
type ImportResult struct {
	TotalFindings  int
	NewFindings    int
	ClosedFindings int
	Reactivated    int
	Untouched      int
	FindingIDs     []string
}
