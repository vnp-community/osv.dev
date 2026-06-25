# ✅ COMPLETED — CR-DD-008 — JIRA Bidirectional Integration Service

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DD-008 |
| **Tiêu đề** | JIRA Integration — Bidirectional Sync, Webhook Handler, AES-256 Credential Encryption |
| **Nguồn tham chiếu** | `django-DefectDojo/specs/services/08-jira-integration-service.md`, `SRS.md §FR-INT-01 to FR-INT-04` |
| **Target Service** | **MỚI**: `jira-service` |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | New Service |
| **Ngày tạo** | 2026-06-13 |

---

## 1. Tổng quan

OSV không có JIRA integration. DefectDojo có một trong những tính năng được yêu cầu nhiều nhất: **Bidirectional JIRA sync** — tức là:
- **Push (DefectDojo → JIRA)**: Khi Finding được tạo, tự động tạo JIRA issue
- **Pull (JIRA → DefectDojo)**: Khi JIRA issue được giải quyết/đóng, tự động cập nhật Finding status

---

## 2. Gap Analysis

| Feature | OSV | DefectDojo |
|---------|-----|-----------|
| JIRA project config per product | ❌ | ✅ |
| Auto-create JIRA issue from finding | ❌ | ✅ |
| Custom priority mapping | ❌ | ✅ severity → JIRA priority |
| JIRA webhook handler | ❌ | ✅ |
| JIRA status → Finding status sync | ❌ | ✅ |
| JIRA comment ↔ Finding note sync | ❌ | ✅ |
| AES-256 credential encryption | ❌ | ✅ |
| Retry logic (3 retries) | ❌ | ✅ |
| SSRF protection for JIRA URL | ❌ | ✅ |
| Issue deduplication (1 JIRA per finding) | ❌ | ✅ |

---

## 3. Service Architecture

```
jira-service/
├── cmd/server/main.go
│
├── internal/
│   ├── domain/
│   │   ├── config/
│   │   │   ├── entity.go       # JIRAConfig (per product)
│   │   │   └── repository.go
│   │   ├── issue/
│   │   │   ├── entity.go       # JIRAIssueMapping (finding ↔ JIRA key)
│   │   │   └── repository.go
│   │   └── sync/
│   │       └── entity.go       # SyncLog, SyncDirection
│   │
│   ├── usecase/
│   │   ├── config/              # CRUD JIRA configs
│   │   ├── sync/
│   │   │   ├── push_finding.go  # Finding → JIRA issue
│   │   │   ├── push_batch.go    # Batch push multiple findings
│   │   │   └── pull_status.go   # JIRA webhook → DefectDojo/OSV
│   │   └── comment/
│   │       └── sync_comment.go  # JIRA comments ↔ Finding notes
│   │
│   ├── delivery/
│   │   ├── http/
│   │   │   ├── config_handler.go
│   │   │   ├── issue_handler.go
│   │   │   └── webhook_handler.go  # POST /webhooks/jira/{config_id}
│   │   ├── grpc/
│   │   └── event/
│   │       ├── subscriber.go
│   │       └── handlers/
│   │           ├── finding_created.go
│   │           └── finding_status_changed.go
│   │
│   └── infra/
│       ├── postgres/
│       ├── jira/client.go          # andygrunwald/go-jira v2
│       └── crypto/aes.go           # AES-256-GCM encryption
```

---

## 4. Domain Model

### 4.1 JIRAConfig

```go
// domain/config/entity.go
// Mirrors Python: dojo/models.py::JIRA_Instance + JIRA_Project

type JIRAConfig struct {
    ID        string
    ProductID string  // One config per product

    // JIRA Server connection
    URL         string
    Username    string
    PasswordEnc string  // AES-256-GCM encrypted

    // Project settings
    ProjectKey          string
    IssueTypeID         string  // JIRA issue type (Bug, Task, etc.)
    IssueTypeFields     map[string]interface{} // Custom field mappings

    // Behavior
    DefaultAssignee     string
    FindSeverityField   string  // Custom field ID for severity
    FindURLField        string  // Custom field ID for DefectDojo/OSV URL
    PushNotes           bool    // Push Finding notes as JIRA comments
    PushAllIssues       bool    // Auto-push ALL new findings to JIRA
    EnableDeduplication bool    // Prevent duplicate JIRA issues

    // Priority mapping: Severity → JIRA Priority Name
    // e.g., "Critical" → "Highest", "High" → "High", etc.
    PriorityMapping map[string]string

    // Webhook settings (JIRA calls this URL on status changes)
    WebhookSecret string  // HMAC verification

    IsActive bool

    CreatedAt time.Time
    UpdatedAt time.Time
}

// JIRAIssueMapping — tracks finding_id ↔ jira_key relationship
// Mirrors Python: dojo/models.py::JIRA_Issue
type JIRAIssueMapping struct {
    ID         string
    FindingID  string
    JIRAKey    string     // e.g., "PROJ-123"
    JIRAID     string     // JIRA internal issue ID
    JIRAURL    string     // https://jira.company.com/browse/PROJ-123
    JIRAStatus string     // Last known JIRA status
    Synced     bool
    LastSyncAt *time.Time
    CreatedAt  time.Time
}
```

### 4.2 Credential Encryption

```go
// infra/crypto/aes.go
// Mirrors Python: dojo/utils.py::credential_aes_256_key usage
// AES-256-GCM for JIRA credentials (same as Django DD_CREDENTIAL_AES_256_KEY)

type AES256GCMCrypto struct {
    key []byte // 32 bytes, from env OSV_JIRA_ENCRYPTION_KEY
}

func (c *AES256GCMCrypto) Encrypt(plaintext string) (string, error) {
    block, _ := aes.NewCipher(c.key)
    gcm, _ := cipher.NewGCM(block)

    nonce := make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)

    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *AES256GCMCrypto) Decrypt(ciphertext string) (string, error) {
    data, _ := base64.StdEncoding.DecodeString(ciphertext)
    block, _ := aes.NewCipher(c.key)
    gcm, _ := cipher.NewGCM(block)

    nonceSize := gcm.NonceSize()
    nonce, cipherData := data[:nonceSize], data[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
    return string(plaintext), err
}
```

---

## 5. Use Cases

### 5.1 PushFinding (Finding → JIRA)

```go
// usecase/sync/push_finding.go
// Mirrors Python: dojo/jira_link/helper.py::add_jira_issue()

func (uc *PushFindingUseCase) Execute(ctx context.Context, in PushFindingInput) error {
    // 1. Get JIRA config for product
    config, err := uc.configRepo.FindByProductID(ctx, in.ProductID)
    if err != nil { return fmt.Errorf("no jira config: %w", err) }
    if !config.IsActive { return nil }

    // 2. Check deduplication (don't create duplicate JIRA issues)
    if config.EnableDeduplication {
        if existing, _ := uc.issueRepo.FindByFindingID(ctx, in.FindingID); existing != nil {
            return nil // Already has JIRA issue
        }
    }

    // 3. Get finding details from finding-service
    finding, _ := uc.findingClient.GetFinding(ctx, &findingv1.GetFindingRequest{Id: in.FindingID})

    // 4. Build JIRA issue fields
    password, _ := uc.cryptoSvc.Decrypt(config.PasswordEnc)
    jiraClient, _ := jira.NewClient(
        jira.BasicAuthTransport{Username: config.Username, Password: password}.Client(),
        config.URL,
    )

    issueFields := &jira.IssueFields{
        Project:     jira.Project{Key: config.ProjectKey},
        Summary:     finding.Title,
        Description: buildJIRADescription(finding), // Markdown → JIRA Wiki markup
        Type:        jira.IssueType{ID: config.IssueTypeID},
        Priority:    &jira.Priority{Name: config.PriorityMapping[finding.Severity]},
    }

    if config.DefaultAssignee != "" {
        issueFields.Assignee = &jira.User{Name: config.DefaultAssignee}
    }

    // Custom fields
    if issueFields.Unknowns == nil {
        issueFields.Unknowns = make(tcontainer.MarshalMap)
    }
    if config.FindSeverityField != "" {
        issueFields.Unknowns[config.FindSeverityField] = finding.Severity
    }
    if config.FindURLField != "" {
        issueFields.Unknowns[config.FindURLField] = buildFindingURL(finding)
    }

    // 5. Create JIRA issue with retry (max 3 attempts)
    var issue *jira.Issue
    var err error
    for attempt := 0; attempt < 3; attempt++ {
        issue, _, err = jiraClient.Issue.Create(&jira.Issue{Fields: issueFields})
        if err == nil { break }
        time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
    }
    if err != nil {
        return fmt.Errorf("create jira issue after 3 retries: %w", err)
    }

    // 6. Save mapping
    uc.issueRepo.Save(ctx, &domain.JIRAIssueMapping{
        FindingID: in.FindingID,
        JIRAKey:   issue.Key,
        JIRAID:    issue.ID,
        JIRAURL:   fmt.Sprintf("%s/browse/%s", config.URL, issue.Key),
        JIRAStatus: issue.Fields.Status.Name,
    })

    // 7. Publish event
    uc.eventPub.Publish(ctx, &events.JIRAIssueCreated{
        FindingID: in.FindingID,
        JIRAKey:   issue.Key,
        JIRAURL:   fmt.Sprintf("%s/browse/%s", config.URL, issue.Key),
        ProductID: in.ProductID,
    })

    return nil
}
```

### 5.2 PullStatus (JIRA webhook → OSV)

```go
// usecase/sync/pull_status.go
// Mirrors Python: dojo/jira_link/helper.py::update_issue_from_jira()

type JIRAWebhookPayload struct {
    WebhookEvent string `json:"webhookEvent"`
    Issue        struct {
        Key    string `json:"key"`
        Fields struct {
            Status struct {
                Name string `json:"name"`
            } `json:"status"`
            Resolution *struct {
                Name string `json:"name"`
            } `json:"resolution"`
        } `json:"fields"`
    } `json:"issue"`
    Comment *struct {
        Body string `json:"body"`
    } `json:"comment"`
}

func (uc *PullStatusUseCase) Handle(ctx context.Context, configID string, payload *JIRAWebhookPayload) error {
    // 1. Find finding by JIRA key
    mapping, err := uc.issueRepo.FindByJIRAKey(ctx, payload.Issue.Key)
    if err != nil { return nil } // Unknown JIRA issue, ignore

    // 2. Update mapping status
    mapping.JIRAStatus = payload.Issue.Fields.Status.Name
    mapping.LastSyncAt = ptr(time.Now())
    uc.issueRepo.Save(ctx, mapping)

    // 3. Map JIRA status → OSV finding action
    switch strings.ToLower(payload.Issue.Fields.Status.Name) {
    case "done", "resolved", "closed", "won't fix":
        uc.findingClient.CloseFinding(ctx, &findingv1.CloseFindingRequest{
            FindingId: mapping.FindingID,
            Reason:    "jira_resolved",
        })
    case "in progress", "open", "reopened", "to do":
        uc.findingClient.ReopenFinding(ctx, &findingv1.ReopenFindingRequest{
            FindingId: mapping.FindingID,
            Reason:    "jira_reopened",
        })
    }

    // 4. If comment event → sync as Finding note
    if payload.Comment != nil && payload.Comment.Body != "" {
        uc.findingClient.AddNote(ctx, &findingv1.AddNoteRequest{
            FindingId: mapping.FindingID,
            Content:   fmt.Sprintf("[JIRA] %s", payload.Comment.Body),
        })
    }

    uc.eventPub.Publish(ctx, &events.JIRASynced{
        FindingID:  mapping.FindingID,
        JIRAKey:    mapping.JIRAKey,
        JIRAStatus: payload.Issue.Fields.Status.Name,
    })

    return nil
}
```

### 5.3 Webhook Handler

```go
// delivery/http/webhook_handler.go

func (h *WebhookHandler) HandleJIRAWebhook(w http.ResponseWriter, r *http.Request) {
    configID := chi.URLParam(r, "config_id")

    config, err := h.configRepo.FindByID(r.Context(), configID)
    if err != nil {
        http.Error(w, "not found", 404)
        return
    }

    // Verify JIRA HMAC signature
    body, _ := io.ReadAll(r.Body)
    signature := r.Header.Get("X-Hub-Signature")
    if !verifyHMAC(body, signature, config.WebhookSecret) {
        http.Error(w, "invalid signature", 401)
        return
    }

    var payload JIRAWebhookPayload
    json.Unmarshal(body, &payload)

    // Handle async — return 202 immediately to JIRA (avoid timeout)
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        h.pullStatusUC.Handle(ctx, configID, &payload)
    }()

    w.WriteHeader(http.StatusAccepted) // 202 Accepted
}
```

---

## 6. JIRA Client Configuration

```go
// infra/jira/client.go
// Uses: github.com/andygrunwald/go-jira v2

// Build JIRA description from Finding (Markdown → JIRA Wiki markup)
func buildJIRADescription(f *findingv1.Finding) string {
    var sb strings.Builder

    sb.WriteString("h2. Vulnerability Summary\n\n")
    sb.WriteString(markdownToWiki(f.Description))
    sb.WriteString("\n\n")

    if f.Cve != "" {
        sb.WriteString(fmt.Sprintf("*CVE:* %s\n", f.Cve))
    }
    if f.Cwe != 0 {
        sb.WriteString(fmt.Sprintf("*CWE:* CWE-%d\n", f.Cwe))
    }
    if f.CvssV3Score != 0 {
        sb.WriteString(fmt.Sprintf("*CVSS v3:* %.1f\n", f.CvssV3Score))
    }

    sb.WriteString("\n")
    sb.WriteString(fmt.Sprintf("*[View in OSV|%s]*\n", buildFindingURL(f)))

    if f.Mitigation != "" {
        sb.WriteString("\nh2. Remediation\n\n")
        sb.WriteString(markdownToWiki(f.Mitigation))
    }

    return sb.String()
}
```

---

## 7. REST API

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET/POST` | `/api/v2/jira-configurations` | JWT/Admin | CRUD JIRA configs |
| `GET/PUT/DELETE` | `/api/v2/jira-configurations/{id}` | JWT/Admin | CRUD |
| `GET` | `/api/v2/jira-issues` | JWT | List finding → JIRA mappings |
| `POST` | `/api/v2/jira-issues` | JWT/Writer | Manual push finding to JIRA |
| `GET` | `/api/v2/jira-issues/{finding_id}` | JWT | Get JIRA issue for finding |
| `DELETE` | `/api/v2/jira-issues/{id}` | JWT/Maintainer | Unlink JIRA issue |
| `POST` | `/webhooks/jira/{config_id}` | HMAC | JIRA webhook receiver |

### Create JIRA Config

```json
POST /api/v2/jira-configurations
{
  "product_id": "uuid",
  "url": "https://jira.company.com",
  "username": "svc-account",
  "password": "secret123",
  "project_key": "SEC",
  "issue_type_id": "10001",
  "default_assignee": "security-team",
  "push_all_issues": false,
  "enable_deduplication": true,
  "priority_mapping": {
    "Critical": "Highest",
    "High": "High",
    "Medium": "Medium",
    "Low": "Low",
    "Info": "Lowest"
  }
}
```

---

## 8. NATS Events

### Published
```
jira.issue.created   {finding_id, jira_key, jira_url, product_id}
jira.issue.updated   {finding_id, jira_key, old_status, new_status}
jira.synced          {finding_id, jira_key, jira_status}
jira.sync.failed     {finding_id, error}
```

### Subscribed
```
finding.created              → PushFinding (if push_all_issues=true)
finding.status_changed       → UpdateJIRAIssue (close/reopen JIRA)
```

---

## 9. Database Schema

```sql
-- jira_configurations
CREATE TABLE jira_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL UNIQUE,
    url VARCHAR(2048) NOT NULL,
    username VARCHAR(255) NOT NULL,
    password_enc TEXT NOT NULL,  -- AES-256-GCM
    project_key VARCHAR(50) NOT NULL,
    issue_type_id VARCHAR(50) NOT NULL,
    issue_type_fields JSONB DEFAULT '{}',
    default_assignee VARCHAR(255),
    find_severity_field VARCHAR(255),
    find_url_field VARCHAR(255),
    push_notes BOOLEAN DEFAULT FALSE,
    push_all_issues BOOLEAN DEFAULT FALSE,
    enable_deduplication BOOLEAN DEFAULT TRUE,
    priority_mapping JSONB DEFAULT '{}',
    webhook_secret VARCHAR(255),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- jira_issue_mappings
CREATE TABLE jira_issue_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL UNIQUE,
    jira_key VARCHAR(100) NOT NULL,
    jira_id VARCHAR(100) NOT NULL,
    jira_url TEXT NOT NULL,
    jira_status VARCHAR(100),
    synced BOOLEAN DEFAULT TRUE,
    last_sync_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_jira_by_key ON jira_issue_mappings(jira_key);
CREATE INDEX idx_jira_by_finding ON jira_issue_mappings(finding_id);
```

---

## 10. Acceptance Criteria

- [x] `POST /api/v2/jira-configurations` với credentials → password được encrypt AES-256 trước khi lưu
- [x] `POST /api/v2/jira-issues` (manual push) → JIRA issue created with correct priority mapping
- [x] `push_all_issues=true` → finding.created event → JIRA issue auto-created
- [x] JIRA webhook HMAC validation: invalid signature → 401 response
- [x] JIRA "Done" status → finding.status = mitigated
- [x] JIRA "Reopened" status → finding reactivated
- [x] JIRA comment → Finding note created (prefix: `[JIRA]`)
- [x] Retry: JIRA API failure → max 3 retries with 5s/10s/15s backoff
- [x] `enable_deduplication=true`: same finding pushed twice → only 1 JIRA issue
- [x] JIRA config update: `password` field always encrypted, never returned in plaintext

## Implementation Status: ✅ DONE

> `jira-service/internal/domain/{jiraconfig,issuemapping}/entity.go` — JIRAConfig (PasswordEnc AES-256), JIRAIssueMapping
> `jira-service/internal/infra/crypto/aes.go` — AES-256-GCM Encrypt/Decrypt, key from OSV_JIRA_ENCRYPTION_KEY env
> `jira-service/internal/usecase/push_finding.go` — 6-step pipeline: config lookup → dedup check → gRPC finding → build description → retry 3x (5s/10s/15s) → save mapping + NATS
> `jira-service/internal/usecase/sync/pull_status.go` — jiraStatusToAction: done/resolved/closed → close; in_progress/reopened → reopen; comment → AddNote "[JIRA]"
> `jira-service/internal/delivery/http/webhook_handler.go` — verifyHMAC (constant-time), 202 Accepted + async processWebhook()
> `jira-service/migrations/{001_jira_configurations,002_jira_issue_mappings}.sql` — UNIQUE(product_id) + UNIQUE(finding_id) + 2 indexes
