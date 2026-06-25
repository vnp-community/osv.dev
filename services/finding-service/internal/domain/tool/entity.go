// Package tool defines the ToolConfiguration domain entity for external tool credentials.
package tool

import (
	"time"

	"github.com/google/uuid"
)

// AuthType describes the authentication mechanism for a tool.
type AuthType string

const (
	AuthTypeAPIKey    AuthType = "api_key"
	AuthTypeHTTPBasic AuthType = "http_basic"
	AuthTypeSSH       AuthType = "ssh"
	AuthTypeBearer    AuthType = "bearer"
)

// IsValid returns true if the AuthType is a recognized value.
func (a AuthType) IsValid() bool {
	switch a {
	case AuthTypeAPIKey, AuthTypeHTTPBasic, AuthTypeSSH, AuthTypeBearer:
		return true
	}
	return false
}

// ToolConfiguration represents external tool credentials (build server, SCM, etc.).
// Passwords and API keys are stored AES-256-GCM encrypted.
type ToolConfiguration struct {
	ID          uuid.UUID
	Name        string
	Description string
	ToolType    string   // "GitHub"|"GitLab"|"Jira"|"Slack"|"SonarQube"|"Jenkins"|...
	URL         string
	AuthType    AuthType
	Username    string
	PasswordEnc string // AES-256-GCM encrypted — never return in plaintext
	APIKeyEnc   string // AES-256-GCM encrypted — never return in plaintext
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// New creates a new ToolConfiguration.
func New(name, toolType, url string, authType AuthType) *ToolConfiguration {
	now := time.Now().UTC()
	return &ToolConfiguration{
		ID:        uuid.New(),
		Name:      name,
		ToolType:  toolType,
		URL:       url,
		AuthType:  authType,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
