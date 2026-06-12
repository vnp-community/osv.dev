// Package finding defines the core domain entity for vulnerability findings.
package finding

import (
	"time"

	"github.com/google/uuid"
)

// Severity represents the vulnerability severity level.
type Severity string

const (
	SeverityCritical Severity = "Critical"
	SeverityHigh     Severity = "High"
	SeverityMedium   Severity = "Medium"
	SeverityLow      Severity = "Low"
	SeverityInfo     Severity = "Info"
)

// Numerical returns the numeric equivalent of the severity (4=Critical, 0=Info).
func (s Severity) Numerical() int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 0
	}
}

// Finding is the central domain entity representing one vulnerability finding.
type Finding struct {
	ID                uuid.UUID
	Title             string
	Description       string
	Mitigation        string
	Impact            string
	References        string
	Severity          Severity
	NumericalSeverity int
	CVE               string
	CWE               int
	VulnIDFromTool    string
	CVSSv3            string
	CVSSv3Score       *float64
	CVSSv4            string
	CVSSv4Score       *float64

	// Status flags — CurrentState() derives the logical state from these.
	Active        bool
	Verified      bool
	FalsePositive bool
	Duplicate     bool
	OutOfScope    bool
	IsMitigated   bool
	RiskAccepted  bool

	// Timestamps
	Date               time.Time
	MitigatedAt        *time.Time
	MitigatedByID      *uuid.UUID
	LastReviewed       *time.Time
	LastStatusUpdate   *time.Time
	SLAExpirationDate  *time.Time

	// Context
	TestID             uuid.UUID
	EngagementID       uuid.UUID
	ProductID          uuid.UUID
	DuplicateFindingID *uuid.UUID
	FindingGroupID     *uuid.UUID

	// Location
	ComponentName    string
	ComponentVersion string
	Service          string
	FilePath         string
	LineNumber       int

	// Deduplication
	HashCode string

	// Tags
	Tags         []string
	InheritedTags []string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// New creates a new Finding with required fields and computed NumericalSeverity.
func New(title string, severity Severity, testID, engagementID, productID uuid.UUID) *Finding {
	now := time.Now().UTC()
	return &Finding{
		ID:                uuid.New(),
		Title:             title,
		Severity:          severity,
		NumericalSeverity: severity.Numerical(),
		Active:            true,
		Date:              now,
		TestID:            testID,
		EngagementID:      engagementID,
		ProductID:         productID,
		Tags:              []string{},
		InheritedTags:     []string{},
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}
