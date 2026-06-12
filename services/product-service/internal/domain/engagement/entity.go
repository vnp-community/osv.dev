// Package engagement defines the Engagement domain entity.
package engagement

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNameRequired    = errors.New("engagement name is required")
	ErrProductRequired = errors.New("product ID is required")
)

// Status represents the lifecycle state of an engagement.
type Status string

const (
	StatusNotStarted Status = "Not Started"
	StatusInProgress Status = "In Progress"
	StatusOnHold     Status = "On Hold"
	StatusCompleted  Status = "Completed"
	StatusCancelled  Status = "Cancelled"
)

// Type distinguishes manual from automated CI/CD engagements.
type Type string

const (
	TypeInteractive Type = "Interactive"
	TypeCICD        Type = "CI/CD"
)

// Engagement represents a testing event within a Product.
type Engagement struct {
	ID                        uuid.UUID
	ProductID                 uuid.UUID
	Name                      string
	Description               string
	LeadID                    *uuid.UUID
	EngagementType            Type
	Status                    Status
	StartDate                 time.Time
	EndDate                   *time.Time
	Version                   string
	BuildID                   string
	CommitHash                string
	BranchTag                 string
	SourceCodeManagementURI   string
	DeduplicationOnEngagement bool
	Tags                      []string
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

// New creates and validates a new Engagement.
func New(productID uuid.UUID, name string, engType Type) (*Engagement, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if productID == uuid.Nil {
		return nil, ErrProductRequired
	}
	return &Engagement{
		ID:             uuid.New(),
		ProductID:      productID,
		Name:           name,
		EngagementType: engType,
		Status:         StatusInProgress,
		StartDate:      time.Now().UTC(),
		Tags:           []string{},
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}, nil
}

// Close marks the engagement as Completed and sets EndDate to now.
func (e *Engagement) Close() {
	e.Status = StatusCompleted
	now := time.Now().UTC()
	e.EndDate = &now
	e.UpdatedAt = now
}
