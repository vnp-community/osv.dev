// Package jira_create_issue creates a Jira issue from a finding.
package jira_create_issue

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	jiraclient "github.com/osv/notification-service/internal/infra/jira"
	"github.com/osv/notification-service/internal/domain/integration"
)

// Request contains the data needed to create a Jira issue from a finding.
type Request struct {
	FindingID     string
	IntegrationID string
	// Finding details (populated by caller from finding-service)
	Title       string
	Description string
	Severity    string
	CVE         string
	ProductName string
	FindingURL  string
}

// Result holds the created Jira issue details.
type Result struct {
	IssueKey string
	IssueURL string
}

// JiraRepo defines persistence for Jira integrations and issues.
type JiraRepo interface {
	FindByID(ctx context.Context, id uuid.UUID) (*integration.JiraIntegration, error)
	CreateIssue(ctx context.Context, issue *integration.JiraIssue) error
}

// UseCase creates a Jira issue from a finding.
type UseCase struct {
	repo JiraRepo
}

// New creates a new jira_create_issue usecase.
func New(repo JiraRepo) *UseCase {
	return &UseCase{repo: repo}
}

// Execute creates a Jira issue for the given finding.
func (uc *UseCase) Execute(ctx context.Context, req Request) (*Result, error) {
	integrationID, err := uuid.Parse(req.IntegrationID)
	if err != nil {
		return nil, fmt.Errorf("invalid integration ID: %w", err)
	}

	// 1. Fetch Jira integration config from repository
	jiraIntegration, err := uc.repo.FindByID(ctx, integrationID)
	if err != nil {
		return nil, fmt.Errorf("fetch jira integration: %w", err)
	}
	if jiraIntegration == nil {
		return nil, fmt.Errorf("jira integration %s not found", req.IntegrationID)
	}

	// 2. Map finding → Jira issue fields
	priority := severityToPriority(req.Severity)
	summary := fmt.Sprintf("[%s] %s", req.Severity, req.Title)
	if req.CVE != "" {
		summary = fmt.Sprintf("[%s][%s] %s", req.CVE, req.Severity, req.Title)
	}

	labels := []string{"security", "osv", req.Severity}
	if req.CVE != "" {
		labels = append(labels, req.CVE)
	}

	descParts := req.Description
	if req.FindingURL != "" {
		descParts += fmt.Sprintf("\n\nFinding: %s", req.FindingURL)
	}
	if req.ProductName != "" {
		descParts += fmt.Sprintf("\nProduct: %s", req.ProductName)
	}

	// 3. Create Jira client and POST to Jira REST API v3
	client := jiraclient.NewClient(jiraIntegration.ServerURL, jiraIntegration.APIToken, jiraIntegration.APIToken)
	issueKey, issueURL, err := client.CreateIssue(ctx,
		jiraIntegration.ProjectKey,
		jiraIntegration.IssueType,
		summary,
		descParts,
		priority,
		labels,
	)
	if err != nil {
		return nil, fmt.Errorf("create jira issue via API: %w", err)
	}

	// 4. Store JiraIssue record in DB
	findingID, _ := uuid.Parse(req.FindingID)
	issue := &integration.JiraIssue{
		ID:            uuid.New(),
		FindingID:     findingID,
		IntegrationID: integrationID,
		IssueKey:      issueKey,
		IssueURL:      issueURL,
		Status:        "Open",
		SyncedAt:      time.Now().UTC(),
	}
	if err := uc.repo.CreateIssue(ctx, issue); err != nil {
		// Non-fatal: log but don't fail (issue is created in Jira)
		_ = err
	}

	return &Result{IssueKey: issueKey, IssueURL: issueURL}, nil
}

// severityToPriority maps finding severity to Jira priority.
func severityToPriority(severity string) string {
	switch severity {
	case "Critical":
		return "Highest"
	case "High":
		return "High"
	case "Medium":
		return "Medium"
	case "Low":
		return "Low"
	default:
		return "Lowest"
	}
}
