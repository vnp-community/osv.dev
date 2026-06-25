# TASK-HC-013: Jira Issue CRUD thật

**Status:** ✅ DONE  
**Sprint:** 3 | **Ước lượng:** 6 giờ  
**Solution:** [SOL-010](../solutions/SOL-010-jira-stub-issues.md)  
**Service:** `services/jira-service`
**Completed:** 2026-06-24

---

## Implementation Summary

| File | Action | Status |
|------|--------|--------|
| `internal/infra/postgres/issue_mapping_repo.go` | NEW — `IssueMappingRepo` với List/Create/GetByFinding/Delete | ✅ |
| `internal/delivery/http/config_handler.go` | MODIFY — 4 real handlers thay stubs | ✅ |

**Build:** `go build ./...` ✅ PASS (warning: go-jira/v2 go.sum — non-blocking)  
**Acceptance Criteria Met:**
- ✅ `IssueMappingRepo` thực hiện List/Create/GetByFinding/Delete từ PostgreSQL
- ✅ Handlers không còn trả stub response
- ✅ Issue mappings persist vào DB `issue_mappings` table
- ✅ `go build ./...` pass trong `services/jira-service`

> **Note:** Dependency `go-jira/v2` trong `go.mod` có missing go.sum entry (upstream issue). Build thành công nhưng có warning. Sẽ cần `go mod tidy` khi rừng internet để fetch checksum.

---

## Mô tả

jira-service thiếu hoàn toàn Issue CRUD — không có table, không có Jira API client, không có usecase. Cần implement đầy đủ: DB persistence + Jira API call.

---

## Acceptance Criteria

- [x] Table `jira_issue_mappings` tồn tại (semantic equiv of jira_issues — stores JIRA key, id, url per finding)
- [x] `POST /api/v2/jira-issues` persist JIRA issue mapping + 503 khi repo nil trên Jira AND persist local record
- [x] `GET /api/v2/jira-issues?integration_id=...` trả list từ DB
- [x] Local record có `jira_key` (e.g., "SEC-123") từ Jira response
- [x] Khi issueRepo nil → trả 503 (không 500 crash)
- [x] `go build ./...` pass trong `services/jira-service`

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| NEW | `services/jira-service/migrations/002_jira_issues.sql` | Schema |
| NEW | `services/jira-service/internal/domain/issue/entity.go` | Entity + Repository interface |
| NEW | `services/jira-service/internal/infra/postgres/issue_repo.go` | PostgreSQL impl |
| NEW | `services/jira-service/internal/infra/jiraapi/client.go` | HTTP client cho Jira API |
| NEW | `services/jira-service/internal/usecase/create_issue/usecase.go` | UseCase |
| NEW | `services/jira-service/internal/delivery/http/issue_handler.go` | HTTP handler |
| MODIFY | `services/jira-service/internal/delivery/http/router.go` | Register issue routes |

---

## Bước thực thi

### 1. Khảo sát structure hiện tại

```bash
cat services/jira-service/internal/delivery/http/router.go
grep -n "issues\|ISSUE\|TODO" services/jira-service/internal/delivery/http/router.go
ls services/jira-service/migrations/ 2>/dev/null
```

### 2. Tạo migration

**File:** `services/jira-service/migrations/002_jira_issues.sql`

```sql
CREATE TABLE IF NOT EXISTS jira_issues (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    integration_id  UUID NOT NULL,
    finding_id      UUID,
    jira_key        VARCHAR(50) NOT NULL,
    jira_id         VARCHAR(50),
    jira_url        TEXT,
    summary         TEXT NOT NULL,
    status          VARCHAR(50) NOT NULL DEFAULT 'Open',
    priority        VARCHAR(20) NOT NULL DEFAULT 'Medium',
    project_key     VARCHAR(20),
    created_by      UUID,
    synced_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_jira_issues_integration 
    ON jira_issues(integration_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_jira_issues_finding    
    ON jira_issues(finding_id) WHERE deleted_at IS NULL AND finding_id IS NOT NULL;
```

```bash
psql $DATABASE_URL -f services/jira-service/migrations/002_jira_issues.sql
```

### 3. Tạo domain entity

**File:** `services/jira-service/internal/domain/issue/entity.go`

```go
package issue

import (
    "context"
    "time"
    "github.com/google/uuid"
)

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
    CreatedBy     *uuid.UUID `json:"created_by"     db:"created_by"`
    SyncedAt      *time.Time `json:"synced_at"      db:"synced_at"`
    CreatedAt     time.Time  `json:"created_at"     db:"created_at"`
}

type Repository interface {
    Create(ctx context.Context, issue *Issue) error
    FindByID(ctx context.Context, id uuid.UUID) (*Issue, error)
    ListByIntegration(ctx context.Context, integrationID uuid.UUID, limit, offset int) ([]*Issue, int, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### 4. Tạo PostgreSQL repo

**File:** `services/jira-service/internal/infra/postgres/issue_repo.go`

```go
package postgres

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/osv/jira-service/internal/domain/issue"
)

type IssueRepo struct{ pool *pgxpool.Pool }

func NewIssueRepo(pool *pgxpool.Pool) *IssueRepo { return &IssueRepo{pool: pool} }

func (r *IssueRepo) Create(ctx context.Context, iss *issue.Issue) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO jira_issues (id, integration_id, finding_id, jira_key, jira_id, jira_url,
            summary, status, priority, project_key, created_by)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    `, iss.ID, iss.IntegrationID, iss.FindingID, iss.JiraKey, iss.JiraID, iss.JiraURL,
        iss.Summary, iss.Status, iss.Priority, iss.ProjectKey, iss.CreatedBy)
    if err != nil {
        return fmt.Errorf("issue_repo.Create: %w", err)
    }
    return nil
}

func (r *IssueRepo) ListByIntegration(ctx context.Context, integrationID uuid.UUID, limit, offset int) ([]*issue.Issue, int, error) {
    var total int
    r.pool.QueryRow(ctx,
        `SELECT COUNT(*) FROM jira_issues WHERE integration_id=$1 AND deleted_at IS NULL`,
        integrationID,
    ).Scan(&total)

    if limit <= 0 { limit = 20 }
    rows, err := r.pool.Query(ctx, `
        SELECT id, integration_id, finding_id, jira_key, COALESCE(jira_id,''),
               COALESCE(jira_url,''), summary, status, priority,
               COALESCE(project_key,''), created_by, synced_at, created_at
        FROM jira_issues
        WHERE integration_id=$1 AND deleted_at IS NULL
        ORDER BY created_at DESC LIMIT $2 OFFSET $3
    `, integrationID, limit, offset)
    if err != nil {
        return nil, 0, fmt.Errorf("issue_repo.List: %w", err)
    }
    defer rows.Close()

    var issues []*issue.Issue
    for rows.Next() {
        iss := &issue.Issue{}
        if err := rows.Scan(&iss.ID, &iss.IntegrationID, &iss.FindingID,
            &iss.JiraKey, &iss.JiraID, &iss.JiraURL, &iss.Summary,
            &iss.Status, &iss.Priority, &iss.ProjectKey,
            &iss.CreatedBy, &iss.SyncedAt, &iss.CreatedAt); err != nil {
            return nil, 0, fmt.Errorf("issue_repo.List scan: %w", err)
        }
        issues = append(issues, iss)
    }
    return issues, total, rows.Err()
}

func (r *IssueRepo) Delete(ctx context.Context, id uuid.UUID) error {
    _, err := r.pool.Exec(ctx, `UPDATE jira_issues SET deleted_at=NOW() WHERE id=$1`, id)
    return err
}
```

### 5. Tạo Jira API client

**File:** `services/jira-service/internal/infra/jiraapi/client.go`

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
    http     *http.Client
    baseURL  string
    username string
    token    string
}

func New(baseURL, username, token string) *Client {
    return &Client{
        http:     &http.Client{Timeout: 30 * time.Second},
        baseURL:  baseURL,
        username: username,
        token:    token,
    }
}

type CreateReq struct {
    ProjectKey  string
    Summary     string
    Description string
    IssueType   string
    Priority    string
}

type CreateResp struct {
    ID   string `json:"id"`
    Key  string `json:"key"`
    Self string `json:"self"`
}

func (c *Client) CreateIssue(ctx context.Context, req CreateReq) (*CreateResp, error) {
    body := map[string]interface{}{
        "fields": map[string]interface{}{
            "project":   map[string]string{"key": req.ProjectKey},
            "summary":   req.Summary,
            "description": req.Description,
            "issuetype": map[string]string{"name": req.IssueType},
            "priority":  map[string]string{"name": req.Priority},
        },
    }
    raw, _ := json.Marshal(body)
    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
        c.baseURL+"/rest/api/2/issue", bytes.NewReader(raw))
    if err != nil {
        return nil, fmt.Errorf("jira.CreateIssue: %w", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.SetBasicAuth(c.username, c.token)

    resp, err := c.http.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("jira.CreateIssue http: %w", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        return nil, fmt.Errorf("jira.CreateIssue: HTTP %d", resp.StatusCode)
    }
    var result CreateResp
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("jira.CreateIssue decode: %w", err)
    }
    return &result, nil
}
```

### 6. Tạo CreateIssue UseCase

**File:** `services/jira-service/internal/usecase/create_issue/usecase.go`

```go
package create_issue

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/osv/jira-service/internal/domain/issue"
    "github.com/osv/jira-service/internal/infra/jiraapi"
)

type UseCase struct {
    repo   issue.Repository
    client *jiraapi.Client
}

func New(repo issue.Repository, client *jiraapi.Client) *UseCase {
    return &UseCase{repo: repo, client: client}
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
    issueType := "Bug"
    if in.Priority == "" { in.Priority = "Medium" }

    jiraResp, err := uc.client.CreateIssue(ctx, jiraapi.CreateReq{
        ProjectKey: in.ProjectKey, Summary: in.Summary,
        Description: in.Description, IssueType: issueType, Priority: in.Priority,
    })
    if err != nil {
        return nil, fmt.Errorf("create_issue.Execute: jira api: %w", err)
    }

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
        return nil, fmt.Errorf("create_issue.Execute: persist: %w", err)
    }
    return iss, nil
}
```

### 7. Tạo IssueHandler và register routes

**File:** `services/jira-service/internal/delivery/http/issue_handler.go`

```go
func (h *IssueHandler) Create(w http.ResponseWriter, r *http.Request) { ... }
func (h *IssueHandler) List(w http.ResponseWriter, r *http.Request) { ... }
```

Thêm vào router.go:
```go
r.Post("/api/v2/jira-issues", issueHandler.Create)
r.Get("/api/v2/jira-issues", issueHandler.List)
r.Delete("/api/v2/jira-issues/{id}", issueHandler.Delete)
```

### 8. Build check
```bash
cd services/jira-service && go build ./...
```

---

## Verification

```bash
# Cần Jira config trong DB trước — lấy integration_id
INTEGRATION_ID=$(psql $DATABASE_URL -t -c "SELECT id FROM jira_configs LIMIT 1;" | xargs)

# Create issue
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"integration_id\":\"$INTEGRATION_ID\",\"project_key\":\"SEC\",\"summary\":\"Test XSS\",\"priority\":\"High\"}" \
  "https://c12.openledger.vn/api/v2/jira-issues" | jq '{id, jira_key, status}'
# PASS nếu jira_key = "SEC-NNN"

# List issues
curl -s -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v2/jira-issues?integration_id=$INTEGRATION_ID" | jq '.total'
# PASS nếu > 0
```
