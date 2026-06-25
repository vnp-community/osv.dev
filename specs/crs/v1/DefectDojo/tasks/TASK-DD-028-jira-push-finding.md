# ✅ COMPLETED — TASK-DD-028 — JIRA Push Finding + Credential Encryption + Config CRUD API

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-028 |
| **Service** | `jira-service` |
| **CR** | CR-DD-008 |
| **Phase** | 3 — Integrations |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-027 |
| **Estimated effort** | 1.5 ngày |

## Context

Implement: (1) JIRA config CRUD với credential encryption; (2) `PushFindingUseCase` — tạo JIRA issue từ finding với retry và dedup check; (3) REST API handlers.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/jira-service/
```

## Files to Create

```
internal/usecase/config/
├── create.go
├── update.go
└── delete.go

internal/usecase/sync/
├── push_finding.go
└── push_batch.go

internal/infra/jira/
└── client.go          # go-jira v2 wrapper

internal/infra/grpc/client/
└── finding.go         # gRPC client → finding-service

internal/delivery/http/
├── config_handler.go
└── issue_handler.go
```

## Implementation Spec

### `internal/usecase/config/create.go`

```go
package config

import (
    "context"
    "time"
    "github.com/google/uuid"
    "github.com/osv/services/jira-service/internal/domain/jiraconfig"
    "github.com/osv/services/jira-service/internal/infra/crypto"
)

type CreateJIRAConfigInput struct {
    ProductID           string
    URL                 string
    Username            string
    Password            string  // plaintext — will be encrypted
    ProjectKey          string
    IssueTypeID         string
    DefaultAssignee     string
    FindSeverityField   string
    FindURLField        string
    PushNotes           bool
    PushAllIssues       bool
    EnableDeduplication bool
    PriorityMapping     map[string]string
    WebhookSecret       string
}

type CreateJIRAConfigUseCase struct {
    configRepo jiraconfig.Repository
    crypto     *crypto.AES256GCM
    jiraClient JIRAClientFactory  // to test-connect on create
    eventPub   EventPublisher
}

func (uc *CreateJIRAConfigUseCase) Execute(ctx context.Context, in CreateJIRAConfigInput) (*jiraconfig.JIRAConfig, error) {
    // 1. Encrypt password
    passwordEnc, err := uc.crypto.Encrypt(in.Password)
    if err != nil {
        return nil, err
    }

    // 2. Test connection (try to get project)
    client, err := uc.jiraClient.NewClient(in.URL, in.Username, in.Password)
    if err != nil {
        return nil, fmt.Errorf("cannot connect to JIRA: %w", err)
    }
    if _, _, err := client.Project.Get(in.ProjectKey); err != nil {
        return nil, fmt.Errorf("JIRA project %q not found or not accessible: %w", in.ProjectKey, err)
    }

    cfg := &jiraconfig.JIRAConfig{
        ID:                  uuid.New().String(),
        ProductID:           in.ProductID,
        URL:                 in.URL,
        Username:            in.Username,
        PasswordEnc:         passwordEnc,
        ProjectKey:          in.ProjectKey,
        IssueTypeID:         in.IssueTypeID,
        DefaultAssignee:     in.DefaultAssignee,
        FindSeverityField:   in.FindSeverityField,
        FindURLField:        in.FindURLField,
        PushNotes:           in.PushNotes,
        PushAllIssues:       in.PushAllIssues,
        EnableDeduplication: in.EnableDeduplication,
        PriorityMapping:     in.PriorityMapping,
        WebhookSecret:       in.WebhookSecret,
        IsActive:            true,
        CreatedAt:           time.Now(),
        UpdatedAt:           time.Now(),
    }
    return cfg, uc.configRepo.Save(ctx, cfg)
}
```

### `internal/usecase/sync/push_finding.go`

```go
package sync

import (
    "context"
    "fmt"
    "strings"
    "time"
    "github.com/google/uuid"
    "github.com/andygrunwald/go-jira/v2/cloud"
    "github.com/osv/services/jira-service/internal/domain/jiraconfig"
    "github.com/osv/services/jira-service/internal/domain/issuemapping"
)

type PushFindingInput struct {
    FindingID string
    ProductID string
    Manual    bool  // true = user triggered, false = auto (push_all_issues)
}

type PushFindingUseCase struct {
    configRepo  jiraconfig.Repository
    mappingRepo issuemapping.Repository
    jiraFactory JIRAClientFactory
    findingClient FindingServiceClient
    crypto      Decryptor
    eventPub    EventPublisher
}

// Execute pushes a finding to JIRA (6-step pipeline with retry)
func (uc *PushFindingUseCase) Execute(ctx context.Context, in PushFindingInput) (*issuemapping.JIRAIssueMapping, error) {
    // Step 1: Get JIRA config for product
    cfg, err := uc.configRepo.FindByProduct(ctx, in.ProductID)
    if err != nil || cfg == nil {
        return nil, ErrNoJIRAConfig
    }

    // Step 2: Check deduplication
    if cfg.EnableDeduplication {
        existing, _ := uc.mappingRepo.FindByFinding(ctx, in.FindingID)
        if existing != nil {
            return existing, nil // already has a JIRA issue
        }
    }

    // Step 3: Get finding details via gRPC
    finding, err := uc.findingClient.GetFinding(ctx, in.FindingID)
    if err != nil {
        return nil, fmt.Errorf("getting finding: %w", err)
    }

    // Step 4: Build JIRA issue fields
    password, _ := uc.crypto.Decrypt(cfg.PasswordEnc)
    client, _ := uc.jiraFactory.NewClient(cfg.URL, cfg.Username, password)

    priority := cfg.PriorityMapping[finding.Severity]
    if priority == "" { priority = "Medium" }

    description := buildJIRADescription(finding)
    fields := &cloud.IssueFields{
        Project:     cloud.Project{Key: cfg.ProjectKey},
        IssueType:   cloud.IssueType{ID: cfg.IssueTypeID},
        Summary:     fmt.Sprintf("[%s] %s", finding.Severity, finding.Title),
        Description: description,
        Priority:    &cloud.Priority{Name: priority},
    }
    if cfg.DefaultAssignee != "" {
        fields.Assignee = &cloud.User{EmailAddress: cfg.DefaultAssignee}
    }
    if cfg.FindSeverityField != "" {
        // Set custom severity field
    }
    if cfg.FindURLField != "" && finding.URL != "" {
        // Set custom URL field
    }

    // Step 5: Create JIRA issue (retry 3 times: 5s/10s/15s)
    var issue *cloud.Issue
    for attempt := 0; attempt < 3; attempt++ {
        issue, _, err = client.Issue.Create(&cloud.Issue{Fields: fields})
        if err == nil { break }
        time.Sleep(time.Duration(5*(attempt+1)) * time.Second)
    }
    if err != nil {
        return nil, fmt.Errorf("creating JIRA issue after 3 retries: %w", err)
    }

    // Step 6: Save mapping
    mapping := &issuemapping.JIRAIssueMapping{
        ID:        uuid.New().String(),
        FindingID: in.FindingID,
        JIRAID:    issue.ID,
        JIRAKey:   issue.Key,
        JIRAURL:   cfg.URL + "/browse/" + issue.Key,
        JIRAStatus: "Open",
        Synced:    true,
        CreatedAt: time.Now(),
    }
    uc.mappingRepo.Save(ctx, mapping)

    uc.eventPub.Publish(ctx, "jira.issue.created", map[string]any{
        "finding_id": in.FindingID,
        "jira_key":   issue.Key,
        "jira_url":   mapping.JIRAURL,
        "product_id": in.ProductID,
        "_service":   "jira-service",
    })

    return mapping, nil
}

// buildJIRADescription converts finding to JIRA wiki markup
func buildJIRADescription(f *FindingDTO) string {
    var sb strings.Builder
    sb.WriteString("h2. Finding Details\n\n")
    sb.WriteString(fmt.Sprintf("*Severity:* %s\n", f.Severity))
    if f.CVE != "" {
        sb.WriteString(fmt.Sprintf("*CVE:* %s\n", f.CVE))
    }
    if f.CVSSv3Score != nil {
        sb.WriteString(fmt.Sprintf("*CVSS v3:* %.1f\n", *f.CVSSv3Score))
    }
    sb.WriteString("\nh2. Description\n\n")
    sb.WriteString(f.Description)
    if f.Mitigation != "" {
        sb.WriteString("\n\nh2. Mitigation\n\n")
        sb.WriteString(f.Mitigation)
    }
    return sb.String()
}
```

### `internal/delivery/http/config_handler.go`

```go
// Routes:
// GET    /api/v2/jira-configurations
// POST   /api/v2/jira-configurations        (Admin)
// GET    /api/v2/jira-configurations/{id}   (password → "***")
// PUT    /api/v2/jira-configurations/{id}   (Admin)
// DELETE /api/v2/jira-configurations/{id}   (Admin)

// IMPORTANT: password field is NEVER returned in plaintext
// GET response includes: "password": "***"
```

### `internal/delivery/http/issue_handler.go`

```go
// Routes:
// GET    /api/v2/jira-issues                  → list all mappings
// POST   /api/v2/jira-issues                  → manual push finding to JIRA
// GET    /api/v2/jira-issues/{finding_id}     → get JIRA issue for finding
// DELETE /api/v2/jira-issues/{id}             → unlink (delete mapping, NOT JIRA issue)

// POST /api/v2/jira-issues body:
// {"finding_id": "uuid", "product_id": "uuid"}
// Response 201: {"finding_id": "uuid", "jira_key": "PROJ-123", "jira_url": "https://..."}
```

## Acceptance Criteria

- [x] `POST /api/v2/jira-configurations` → password encrypted before save
- [x] `GET /api/v2/jira-configurations/{id}` → response has `"password": "***"`
- [x] `POST /api/v2/jira-issues` → JIRA issue created in configured project
- [x] Duplicate push (same finding_id) → returns existing mapping (dedup)
- [x] `EnableDeduplication=false` → allows multiple issues for same finding
- [x] JIRA API failure → retry 3 times with 5s/10s/15s backoff
- [x] Priority mapping: Critical severity → "Highest" JIRA priority
- [x] NATS `jira.issue.created` event published after successful push
- [x] Description includes: severity, CVE, CVSS score, description, mitigation
- [x] `DELETE /api/v2/jira-issues/{id}` → mapping deleted, JIRA issue NOT deleted

## Implementation Status: ✅ DONE

> `jira-service/internal/usecase/push_finding.go` — PushFindingUseCase: 6-step pipeline (config lookup → dedup check → gRPC finding → build fields → retry 3x → save mapping)
> `jira-service/internal/usecase/sync/pull_status.go` — config CRUD
> `jira-service/internal/delivery/http/webhook_handler.go` — config CRUD handler + issue handler
> Password masked ("***") in GET response; AES-256-GCM encrypted at rest
> buildJIRADescription: severity, CVE, CVSSv3, description, mitigation in JIRA wiki format
