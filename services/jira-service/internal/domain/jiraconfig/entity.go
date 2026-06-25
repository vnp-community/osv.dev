// Package jiraconfig defines the JIRA configuration domain entity.
package jiraconfig

import (
	"time"

	"github.com/google/uuid"
)

// JIRAConfig holds the configuration for connecting a product to a JIRA project.
// Credentials are stored encrypted with AES-256-GCM.
type JIRAConfig struct {
	ID        uuid.UUID
	ProductID uuid.UUID

	// Connection — credentials encrypted at rest
	URL         string
	Username    string
	PasswordEnc string // AES-256-GCM encrypted API token

	// Project configuration
	ProjectKey      string
	IssueTypeID     string
	IssueTypeFields map[string]interface{}

	// Behavior settings
	DefaultAssignee     string
	FindSeverityField   string
	FindURLField        string
	PushNotes           bool
	PushAllIssues       bool
	EnableDeduplication bool

	// Priority mapping: DefectDojo Severity → JIRA Priority Name
	// Default: Critical→Highest, High→High, Medium→Medium, Low→Low, Info→Lowest
	PriorityMapping map[string]string

	// Webhook signature verification
	WebhookSecret string

	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New creates a new JIRAConfig with default priority mapping.
func New(productID uuid.UUID, url, username, passwordEnc, projectKey string) *JIRAConfig {
	now := time.Now().UTC()
	return &JIRAConfig{
		ID:        uuid.New(),
		ProductID: productID,
		URL:       url,
		Username:  username,
		PasswordEnc: passwordEnc,
		ProjectKey: projectKey,
		PriorityMapping: map[string]string{
			"Critical": "Highest",
			"High":     "High",
			"Medium":   "Medium",
			"Low":      "Low",
			"Info":     "Lowest",
		},
		EnableDeduplication: true,
		IsActive:            true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

// JIRAPriority maps a DefectDojo severity to a JIRA priority name.
func (c *JIRAConfig) JIRAPriority(severity string) string {
	if p, ok := c.PriorityMapping[severity]; ok {
		return p
	}
	return "Medium"
}
