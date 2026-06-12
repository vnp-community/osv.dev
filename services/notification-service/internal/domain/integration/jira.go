package integration

import (
	"time"

	"github.com/google/uuid"
)

// JiraIntegration holds configuration for a Jira project integration
type JiraIntegration struct {
	ID          uuid.UUID
	ProductID   uuid.UUID
	ServerURL   string
	ProjectKey  string
	IssueType   string
	APIToken    string // encrypted at rest
	AutoCreate  bool
	AutoSync    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// JiraIssue maps a finding to a Jira issue
type JiraIssue struct {
	ID            uuid.UUID
	FindingID     uuid.UUID
	IntegrationID uuid.UUID
	IssueKey      string
	IssueURL      string
	Status        string
	SyncedAt      time.Time
}
