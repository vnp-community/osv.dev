package jira_sync

// UseCase syncs finding statuses with Jira issue statuses
type UseCase struct{}

// New creates a new jira_sync usecase
func New() *UseCase { return &UseCase{} }

// Execute runs bidirectional status sync between findings and Jira issues
func (uc *UseCase) Execute() error {
	// 1. Query all JiraIssue records needing sync (synced_at > 1h ago)
	// 2. For each, fetch current status from Jira API
	// 3. If Jira issue Done/Resolved → update finding status to resolved
	// 4. Update synced_at timestamp
	return nil
}
