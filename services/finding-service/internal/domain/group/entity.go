// Package group defines the FindingGroup domain entity for logical grouping of findings.
package group

import (
	"time"

	"github.com/google/uuid"
)

// FindingGroup represents a logical collection of related findings (e.g., same CVE in many places).
type FindingGroup struct {
	ID           uuid.UUID
	Name         string
	ProductID    uuid.UUID  // scoped to product
	JIRAIssueKey string     // optional JIRA issue link
	FindingCount int        // denormalized count, updated on membership change
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// New creates a new FindingGroup.
func New(productID uuid.UUID, name string) *FindingGroup {
	now := time.Now().UTC()
	return &FindingGroup{
		ID:        uuid.New(),
		Name:      name,
		ProductID: productID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
