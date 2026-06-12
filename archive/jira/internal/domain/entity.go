// Package domain defines JIRA integration domain entities.
package domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// JIRAConfig holds connection and mapping settings for a JIRA instance.
type JIRAConfig struct {
	ID               uuid.UUID
	ProductID        *uuid.UUID        // nil = global config
	URL              string            // e.g., "https://company.atlassian.net"
	Username         string            // JIRA username/email
	APITokenEncrypted string           // AES-256 encrypted — never return raw token
	ProjectKey       string            // e.g., "SEC"
	IssueType        string            // e.g., "Bug"
	DefaultAssigneeID string           // JIRA account ID
	Labels           []string
	IssuePriority    map[string]string // severity → JIRA priority, e.g., {"Critical":"Highest"}
	IsEnabled        bool
	WebhookSecret    string            // HMAC-SHA256 secret for incoming JIRA webhooks
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// JIRAIssueMapping tracks the link between a DefectDojo finding and a JIRA issue.
type JIRAIssueMapping struct {
	ID           uuid.UUID
	FindingID    uuid.UUID
	JIRAConfigID uuid.UUID
	JIRAIssueID  string    // internal JIRA issue ID (numeric)
	JIRAKey      string    // e.g., "SEC-123"
	JIRAStatus   string    // current JIRA workflow status
	JIRAURL      string    // direct URL to issue
	LastSyncedAt time.Time
	CreatedAt    time.Time
}

// JIRAConfigRepository defines persistence for JIRA configs.
type JIRAConfigRepository interface {
	FindForProduct(ctx context.Context, productID uuid.UUID) (*JIRAConfig, error)
	FindGlobal(ctx context.Context) (*JIRAConfig, error)
	FindByID(ctx context.Context, id uuid.UUID) (*JIRAConfig, error)
	Create(ctx context.Context, cfg *JIRAConfig) error
	Update(ctx context.Context, cfg *JIRAConfig) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]*JIRAConfig, error)
}

// JIRAIssueMappingRepository defines persistence for finding→issue mappings.
type JIRAIssueMappingRepository interface {
	Save(ctx context.Context, m *JIRAIssueMapping) error
	FindByFindingID(ctx context.Context, findingID uuid.UUID) (*JIRAIssueMapping, error)
	FindByJIRAKey(ctx context.Context, key string) (*JIRAIssueMapping, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

// MarshalIssuePriority encodes the IssuePriority map to JSON for storage.
func (c *JIRAConfig) MarshalIssuePriority() ([]byte, error) {
	return json.Marshal(c.IssuePriority)
}
