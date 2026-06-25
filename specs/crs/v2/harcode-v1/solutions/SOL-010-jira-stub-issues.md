# SOL-010: Jira Issue CRUD Real Implementation — jira-service

**CR:** CR-HC-010 | **Priority:** 🟡 Medium | **Sprint:** 3  
**Service:** `services/jira-service`

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-013
**Note:** IssueMappingRepo (PostgreSQL) + 4 real handlers thay stubs
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**Cấu trúc hiện tại:**
```
jira-service/
├── internal/delivery/http/
│   ├── router.go         — có TASK-007 FIX comment cho jira-issues stubs
│   ├── config_handler.go — Jira config CRUD (có thể đã ok)
│   └── webhook_handler.go
├── internal/domain/jiraconfig/entity.go
├── internal/usecase/push_finding.go
└── internal/usecase/sync/pull_status.go
```

**Thiếu hoàn toàn:**
- `jira_issues` entity và repository
- Issue CRUD handlers
- Jira API client (real HTTP calls)

---

## Solution

### Bước 1: Migration

**File mới:** `jira-service/migrations/002_jira_issues.sql`

```sql
CREATE TABLE IF NOT EXISTS jira_issues (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    integration_id  UUID NOT NULL,         -- references jira_configs
    finding_id      UUID,                   -- optional link to finding
    jira_key        VARCHAR(50) NOT NULL,   -- e.g., "SEC-123"
    jira_id         VARCHAR(50),            -- Jira internal ID
    jira_url        TEXT,
    summary         TEXT NOT NULL,
    status          VARCHAR(50) NOT NULL DEFAULT 'Open',
    priority        VARCHAR(20) NOT NULL DEFAULT 'Medium',
    reporter        VARCHAR(255),
    assignee        VARCHAR(255),
    project_key     VARCHAR(20),
    labels          TEXT[],
    created_by      UUID,
    synced_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_jira_issues_integration ON jira_issues(integration_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_jira_issues_finding    ON jira_issues(finding_id)     WHERE deleted_at IS NULL AND finding_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jira_issues_key        ON jira_issues(jira_key)       WHERE deleted_at IS NULL;
```

### Bước 2: Domain entity

**File mới:** `jira-service/internal/domain/issue/entity.go`

```go
package issue

import (
    "time"
    "github.com/google/uuid"
)

// Issue represents a Jira issue linked to a security finding.
type Issue struct {
    ID            uuid.UUID  `json:"id"             db:"id"`
    IntegrationID uuid.UUID  `json:"integration_id" db:"integration_id"`
    FindingID     *uuid.UUID `json:"finding_id"     db:"finding_id"`
    JiraKey       string     `json:"jira_key"       db:"jira_key"`
    JiraID        string     `json:"jira_id"        db:"jira_id"`
    JiraURL       string     `json:"jira_url"       db:"jira_url"`
    Summary       string     `json:"summary"        db:"summary"`
    Status        string     `json:"status"         db:"status"`
    Priority      string     `json:"priority"       db:"priority"`
    ProjectKey    string     `json:"project_key"    db:"project_key"`
    Assignee      string     `json:"assignee"       db:"assignee"`
    CreatedBy     *uuid.UUID `json:"created_by"     db:"created_by"`
    SyncedAt      *time.Time `json:"synced_at"      db:"synced_at"`
    CreatedAt     time.Time  `json:"created_at"     db:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at"     db:"updated_at"`
}

// IssueRepository provides persistence for Jira issues.
type IssueRepository interface {
    Create(ctx context.Context, issue *Issue) error
    FindByID(ctx context.Context, id uuid.UUID) (*Issue, error)
    ListByIntegration(ctx context.Context, integrationID uuid.UUID, limit, offset int) ([]*Issue, int, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status string, syncedAt time.Time) error
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### Bước 3: Jira API Client

**File mới:** `jira-service/internal/infra/jiraapi/client.go`

```go
package jiraapi

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type Client struct {
    httpClient *http.Client
    baseURL    string
    username   string
    apiToken   string
}

func NewClient(baseURL, username, apiToken string) *Client {
    return &Client{
        httpClient: &http.Client{Timeout: 30 * time.Second},
        baseURL:    baseURL,
        username:   username,
        apiToken:   apiToken,
    }
}

type CreateIssueRequest struct {
    ProjectKey  string `json:"project_key"`
    Summary     string `json:"summary"`
    Description string `json:"description"`
    IssueType   string `json:"issue_type"` // "Bug", "Task", "Story"
    Priority    string `json:"priority"`
    Labels      []string `json:"labels"`
}

type JiraIssueResponse struct {
    ID   string `json:"id"`
    Key  string `json:"key"`
    Self string `json:"self"`
}

func (c *Client) CreateIssue(ctx context.Context, req CreateIssueRequest) (*JiraIssueResponse, error) {
    body := map[string]interface{}{
        "fields": map[string]interface{}{
            "project":     map[string]string{"key": req.ProjectKey},
            "summary":     req.Summary,
            "description": req.Description,
            "issuetype":   map[string]string{"name": req.IssueType},
            "priority":    map[string]string{"name": req.Priority},
        },
    }

    rawBody, _ := json.Marshal(body)
    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
        c.baseURL+"/rest/api/2/issue", bytes.NewReader(rawBody))
    if err != nil {
        return nil, fmt.Errorf("jira.CreateIssue: create request: %w", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.SetBasicAuth(c.username, c.apiToken)

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("jira.CreateIssue: http: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 300 {
        return nil, fmt.Errorf("jira.CreateIssue: HTTP %d", resp.StatusCode)
    }

    var result JiraIssueResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("jira.CreateIssue: decode: %w", err)
    }
    result.Self = c.baseURL + "/browse/" + result.Key
    return &result, nil
}
```

### Bước 4: CreateIssue UseCase

**File mới:** `jira-service/internal/usecase/create_issue/usecase.go`

```go
package createissue

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/osv/jira-service/internal/domain/issue"
    "github.com/osv/jira-service/internal/infra/jiraapi"
)

type UseCase struct {
    repo      issue.IssueRepository
    jiraClient *jiraapi.Client
}

func New(repo issue.IssueRepository, jiraClient *jiraapi.Client) *UseCase {
    return &UseCase{repo: repo, jiraClient: jiraClient}
}

type Input struct {
    IntegrationID uuid.UUID
    FindingID     *uuid.UUID
    ProjectKey    string
    Summary       string
    Description   string
    Priority      string
    CreatedBy     uuid.UUID
}

func (uc *UseCase) Execute(ctx context.Context, in Input) (*issue.Issue, error) {
    // 1. Create issue in Jira
    jiraResp, err := uc.jiraClient.CreateIssue(ctx, jiraapi.CreateIssueRequest{
        ProjectKey:  in.ProjectKey,
        Summary:     in.Summary,
        Description: in.Description,
        IssueType:   "Bug",
        Priority:    in.Priority,
    })
    if err != nil {
        return nil, fmt.Errorf("createIssue: jira api: %w", err)
    }

    // 2. Persist local record
    iss := &issue.Issue{
        ID:            uuid.New(),
        IntegrationID: in.IntegrationID,
        FindingID:     in.FindingID,
        JiraKey:       jiraResp.Key,
        JiraID:        jiraResp.ID,
        JiraURL:       jiraResp.Self,
        Summary:       in.Summary,
        Status:        "Open",
        Priority:      in.Priority,
        ProjectKey:    in.ProjectKey,
        CreatedBy:     &in.CreatedBy,
    }

    if err := uc.repo.Create(ctx, iss); err != nil {
        return nil, fmt.Errorf("createIssue: persist: %w", err)
    }
    return iss, nil
}
```

### Bước 5: HTTP Handler

**File mới:** `jira-service/internal/delivery/http/issue_handler.go`

```go
func (h *IssueHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req struct {
        IntegrationID string `json:"integration_id"`
        FindingID     string `json:"finding_id"`
        ProjectKey    string `json:"project_key"`
        Summary       string `json:"summary"`
        Description   string `json:"description"`
        Priority      string `json:"priority"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request")
        return
    }

    iss, err := h.createUC.Execute(r.Context(), createissue.Input{...})
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to create issue: "+err.Error())
        return
    }
    writeJSON(w, http.StatusCreated, iss)
}

func (h *IssueHandler) List(w http.ResponseWriter, r *http.Request) {
    integrationID, _ := uuid.Parse(r.URL.Query().Get("integration_id"))
    issues, total, err := h.issueRepo.ListByIntegration(r.Context(), integrationID, 20, 0)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to list issues")
        return
    }
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "issues": issues,
        "total":  total,
    })
}
```

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| NEW | `jira-service/migrations/002_jira_issues.sql` |
| NEW | `jira-service/internal/domain/issue/entity.go` |
| NEW | `jira-service/internal/infra/postgres/issue_repo.go` |
| NEW | `jira-service/internal/infra/jiraapi/client.go` |
| NEW | `jira-service/internal/usecase/create_issue/usecase.go` |
| NEW | `jira-service/internal/delivery/http/issue_handler.go` |
| MODIFY | `jira-service/internal/delivery/http/router.go` — wire issue routes |

---

## Verification

```bash
psql $DATABASE_URL -f jira-service/migrations/002_jira_issues.sql

# Create issue (requires Jira config)
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -d '{"integration_id":"...","project_key":"SEC","summary":"XSS in login form","priority":"High"}' \
  "https://c12.openledger.vn/api/v2/jira-issues"
# Expect: {"id":"uuid","jira_key":"SEC-123","status":"Open",...}

# List issues
curl -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v2/jira-issues?integration_id=..."
# Expect: {"issues":[...],"total":N}
```
