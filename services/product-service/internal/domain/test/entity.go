// Package test defines the Test domain entity.
package test

import (
	"time"

	"github.com/google/uuid"
)

// Test represents a single scan run within an Engagement.
type Test struct {
	ID           uuid.UUID
	EngagementID uuid.UUID
	ScanType     string // e.g. "Trivy Scan", "Bandit Scan", "SARIF"
	Title        string
	Description  string
	TargetStart  time.Time
	TargetEnd    *time.Time
	LeadID       *uuid.UUID
	Version      string
	BuildID      string
	CommitHash   string
	BranchTag    string
	Tags         []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// New creates a new Test with required fields.
func New(engagementID uuid.UUID, scanType, title string) *Test {
	return &Test{
		ID:           uuid.New(),
		EngagementID: engagementID,
		ScanType:     scanType,
		Title:        title,
		TargetStart:  time.Now().UTC(),
		Tags:         []string{},
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
}
