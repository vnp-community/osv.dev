# CR-HC-010: jira-service — Issue List là Stub, không lưu DB

## Trạng thái: 🟡 Medium

## Vấn đề
File: `services/jira-service/internal/delivery/http/router.go:48`

```go
// TASK-007 FIX: jira-issues stubs
```

Jira issue tracking là feature quan trọng cho enterprise security teams nhưng đang là stub.
`jira_issues` table đã tồn tại trong DB nhưng handler không lưu data.

## Phân tích kiến trúc hiện tại

```
jira-service/
├── internal/delivery/http/
│   ├── router.go         — routes wiring
│   ├── config_handler.go — Jira config (có thể đã implemented)
│   └── issue_handler.go  — stub?
├── internal/usecase/     — chưa rõ có implement không
└── internal/infra/       — repo tồn tại không?
```

## Giải pháp

### 1. Domain Entity
```go
type JiraIssue struct {
    ID          uuid.UUID  `db:"id"`
    FindingID   uuid.UUID  `db:"finding_id"`
    JiraKey     string     `db:"jira_key"`     // e.g., "SEC-123"
    JiraURL     string     `db:"jira_url"`
    Summary     string     `db:"summary"`
    Status      string     `db:"status"`       // "Open", "In Progress", "Done"
    Priority    string     `db:"priority"`
    CreatedBy   uuid.UUID  `db:"created_by"`
    CreatedAt   time.Time  `db:"created_at"`
    UpdatedAt   time.Time  `db:"updated_at"`
    SyncedAt    *time.Time `db:"synced_at"`    // last sync from Jira API
}
```

### 2. Repository interface
```go
type JiraIssueRepository interface {
    Create(ctx context.Context, issue *JiraIssue) error
    FindByID(ctx context.Context, id uuid.UUID) (*JiraIssue, error)
    FindByFinding(ctx context.Context, findingID uuid.UUID) ([]*JiraIssue, error)
    ListByIntegration(ctx context.Context, integrationID uuid.UUID, limit, offset int) ([]*JiraIssue, int, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### 3. UseCases
```go
// CreateIssueUseCase: POST finding → Jira API → save local record
type CreateIssueUseCase struct {
    jiraClient JiraAPIClient        // calls real Jira REST API
    issueRepo  JiraIssueRepository  // saves local copy
    configRepo JiraConfigRepository // reads Jira credentials
}

func (uc *CreateIssueUseCase) Execute(ctx context.Context, in CreateIssueInput) (*JiraIssue, error) {
    config, err := uc.configRepo.GetByProductID(ctx, in.ProductID)
    if err != nil {
        return nil, fmt.Errorf("jira config not found: %w", err)
    }
    // Call Jira API to create issue
    jiraIssue, err := uc.jiraClient.CreateIssue(ctx, config, in.Summary, in.Description, in.Priority)
    if err != nil {
        return nil, fmt.Errorf("jira api: %w", err)
    }
    // Save to local DB
    local := &JiraIssue{
        FindingID: in.FindingID,
        JiraKey:   jiraIssue.Key,
        JiraURL:   jiraIssue.URL,
        Summary:   in.Summary,
        Status:    "Open",
        CreatedBy: in.UserID,
    }
    if err := uc.issueRepo.Create(ctx, local); err != nil {
        return nil, fmt.Errorf("save issue: %w", err)
    }
    return local, nil
}
```

### 4. Wire trong embedded.go / router.go
```go
jiraClient := jiraapi.NewClient(httpClient)
issueRepo := postgresrepo.NewJiraIssueRepo(pool)
createIssueUC := createissue.NewUseCase(jiraClient, issueRepo, configRepo)
issueH := issuehandler.NewHandler(createIssueUC, issueRepo, logger)
```

## Files cần thay đổi
- `services/jira-service/internal/domain/issue.go` [NEW]
- `services/jira-service/internal/usecase/create_issue/usecase.go` [NEW]
- `services/jira-service/internal/usecase/sync_issues/usecase.go` [NEW]
- `services/jira-service/internal/infra/postgres/issue_repo.go` [NEW]
- `services/jira-service/internal/infra/jiraapi/client.go` [NEW]
- `services/jira-service/internal/delivery/http/issue_handler.go` — implement
- `services/jira-service/internal/delivery/http/router.go` — wire handlers

## Database
Sử dụng `jira_issues` table đã tồn tại trong DB.

## Acceptance Criteria
- [ ] `POST /api/v1/jira/issues` → tạo issue trên Jira real + lưu vào `jira_issues` table
- [ ] `GET /api/v1/jira/issues?finding_id=...` → trả issues từ DB
- [ ] `PUT /api/v1/jira/issues/{id}/sync` → sync status từ Jira API về DB
