// Package jira_sync syncs finding statuses with Jira issue statuses.
package jira_sync

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/osv/notification-service/internal/domain/integration"
	jiraclient "github.com/osv/notification-service/internal/infra/jira"
)

// JiraRepo defines persistence for Jira integrations and issues.
type JiraRepo interface {
	FindByID(ctx context.Context, id uuid.UUID) (*integration.JiraIntegration, error)
	ListIssuesNeedingSync(ctx context.Context) ([]*integration.JiraIssue, error)
	UpdateIssueStatus(ctx context.Context, issueID uuid.UUID, status string) error
}

// UseCase syncs finding statuses with Jira issue statuses.
type UseCase struct {
	repo JiraRepo
}

// New creates a new jira_sync usecase.
func New(repo JiraRepo) *UseCase {
	return &UseCase{repo: repo}
}

// Execute runs bidirectional status sync between findings and Jira issues.
func (uc *UseCase) Execute(ctx context.Context) error {
	// 1. Query all JiraIssue records needing sync
	issues, err := uc.repo.ListIssuesNeedingSync(ctx)
	if err != nil {
		return fmt.Errorf("list issues needing sync: %w", err)
	}

	// 2. For each, fetch current status from Jira API
	for _, issue := range issues {
		jiraIntegration, err := uc.repo.FindByID(ctx, issue.IntegrationID)
		if err != nil || jiraIntegration == nil {
			continue
		}

		client := jiraclient.NewClient(jiraIntegration.ServerURL, jiraIntegration.APIToken, jiraIntegration.APIToken)
		jiraStatus, err := client.GetIssueStatus(ctx, issue.IssueKey)
		if err != nil {
			continue // Best-effort: skip this issue, retry next cycle
		}

		// 3. Update stored status if changed
		if jiraStatus != issue.Status {
			if err := uc.repo.UpdateIssueStatus(ctx, issue.ID, jiraStatus); err != nil {
				continue
			}
		}
	}

	return nil
}
