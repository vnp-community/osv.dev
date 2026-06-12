package jira_create_issue

// Request contains the data needed to create a Jira issue from a finding
type Request struct {
	FindingID     string
	IntegrationID string
}

// Result holds the created Jira issue details
type Result struct {
	IssueKey string
	IssueURL string
}

// UseCase creates a Jira issue from a finding
type UseCase struct{}

// New creates a new jira_create_issue usecase
func New() *UseCase { return &UseCase{} }

// Execute creates a Jira issue for the given finding
func (uc *UseCase) Execute(req Request) (*Result, error) {
	// 1. Fetch finding details from finding-service (via gRPC)
	// 2. Fetch Jira integration config from repository
	// 3. Map finding → Jira issue fields (summary, description, priority, labels)
	// 4. POST to Jira REST API v3
	// 5. Store JiraIssue record in DB
	return nil, nil
}
