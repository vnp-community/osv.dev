# ✅ COMPLETED — Solution: jira-service (New Service)

> **Covers**: CR-DD-008  
> **Lý do tạo service mới**: JIRA integration là external integration domain hoàn toàn tách biệt — có credential encryption riêng (AES-256), JIRA webhook receiver riêng, state sync riêng. Không phù hợp để gắn vào finding-service hay notification-service.

---

## Service Structure

```
services/jira-service/           # NEW SERVICE
├── cmd/server/main.go
├── Dockerfile
├── go.mod
├── migrations/
│   ├── 001_jira_configurations.sql
│   └── 002_jira_issue_mappings.sql
│
└── internal/
    ├── domain/
    │   ├── config/
    │   │   ├── entity.go       # JIRAConfig (per product)
    │   │   └── repository.go
    │   ├── issue/
    │   │   ├── entity.go       # JIRAIssueMapping (finding ↔ JIRA key)
    │   │   └── repository.go
    │   └── sync/
    │       ├── entity.go       # SyncLog, SyncDirection
    │       └── repository.go
    │
    ├── usecase/
    │   ├── config/
    │   │   ├── create.go
    │   │   ├── update.go
    │   │   └── delete.go
    │   ├── sync/
    │   │   ├── push_finding.go      # Finding → JIRA issue (6 steps)
    │   │   ├── push_batch.go        # Batch push multiple findings
    │   │   └── pull_status.go       # JIRA webhook → finding-service
    │   └── comment/
    │       └── sync_comment.go      # JIRA comments ↔ Finding notes
    │
    ├── delivery/
    │   ├── http/
    │   │   ├── server.go
    │   │   ├── config_handler.go
    │   │   ├── issue_handler.go
    │   │   └── webhook_handler.go   # POST /webhooks/jira/{config_id}
    │   ├── grpc/
    │   │   └── jira_server.go
    │   └── event/
    │       ├── subscriber.go
    │       └── handlers/
    │           ├── finding_created.go       # NATS → push if push_all_issues=true
    │           └── finding_status_changed.go # NATS → update JIRA status
    │
    └── infra/
        ├── postgres/
        │   ├── config_repo.go
        │   ├── issue_repo.go
        │   └── sync_repo.go
        ├── jira/
        │   └── client.go           # andygrunwald/go-jira v2
        ├── crypto/
        │   └── aes.go              # AES-256-GCM encryption
        └── grpc/client/
            └── finding.go          # → finding-service gRPC
```

---

## Domain Model

### JIRAConfig Entity

```go
// jira-service/internal/domain/config/entity.go
type JIRAConfig struct {
    ID        string
    ProductID string

    // JIRA Server connection
    URL         string
    Username    string
    PasswordEnc string  // AES-256-GCM encrypted, NEVER returned in plaintext

    // Project settings
    ProjectKey          string
    IssueTypeID         string
    IssueTypeFields     map[string]interface{}

    // Behavior
    DefaultAssignee     string
    FindSeverityField   string
    FindURLField        string
    PushNotes           bool
    PushAllIssues       bool
    EnableDeduplication bool

    // Priority mapping: Severity → JIRA Priority Name
    PriorityMapping map[string]string  // e.g., "Critical" → "Highest"

    // Webhook
    WebhookSecret string  // HMAC verification

    IsActive bool
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### JIRAIssueMapping Entity

```go
// jira-service/internal/domain/issue/entity.go
type JIRAIssueMapping struct {
    ID         string
    FindingID  string
    JIRAKey    string     // e.g., "PROJ-123"
    JIRAID     string
    JIRAURL    string
    JIRAStatus string
    Synced     bool
    LastSyncAt *time.Time
    CreatedAt  time.Time
}
```

---

## AES-256-GCM Credential Encryption

```go
// jira-service/internal/infra/crypto/aes.go
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

## Use Cases

### PushFinding (Finding → JIRA)

```go
// jira-service/internal/usecase/sync/push_finding.go
// 1. Get JIRA config for product
// 2. Check deduplication (1 JIRA issue per finding)
// 3. Get finding details via gRPC → finding-service
// 4. Build JIRA issue fields (title, description, priority from mapping)
// 5. Create JIRA issue with retry (max 3: 5s/10s/15s backoff)
// 6. Save JIRAIssueMapping
// 7. Publish jira.issue.created event
```

### PullStatus (JIRA webhook → finding-service)

```go
// jira-service/internal/usecase/sync/pull_status.go
// JIRA status mappings:
var jiraStatusToFindingAction = map[string]string{
    "done":       "close",
    "resolved":   "close",
    "closed":     "close",
    "won't fix":  "close",
    "in progress": "reopen",
    "open":       "reopen",
    "reopened":   "reopen",
    "to do":      "reopen",
}
// Also sync JIRA comments → Finding notes (prefix "[JIRA]")
```

### Webhook Handler

```go
// jira-service/internal/delivery/http/webhook_handler.go
// POST /webhooks/jira/{config_id}
// 1. Verify HMAC signature (X-Hub-Signature header)
// 2. Return 202 Accepted immediately (async processing)
// 3. Handle event async (30s timeout)
```

---

## JIRA Client (go-jira v2)

```go
// jira-service/internal/infra/jira/client.go
// Uses: github.com/andygrunwald/go-jira v2
// Builds JIRA description from Finding (Markdown → JIRA Wiki markup)
// Custom fields: severity field, OSV URL field
```

---

## gRPC Contract

```protobuf
// jira-service/proto/jira/v1/jira.proto
syntax = "proto3";
package jira.v1;

service JIRAService {
    // Config CRUD
    rpc CreateConfig(CreateConfigRequest) returns (CreateConfigResponse);
    rpc GetConfig(GetConfigRequest) returns (GetConfigResponse);
    rpc UpdateConfig(UpdateConfigRequest) returns (UpdateConfigResponse);
    rpc DeleteConfig(DeleteConfigRequest) returns (DeleteConfigResponse);

    // Issue sync
    rpc PushFinding(PushFindingRequest) returns (PushFindingResponse);
    rpc PushBatch(PushBatchRequest) returns (PushBatchResponse);
    rpc GetIssueMapping(GetIssueMappingRequest) returns (GetIssueMappingResponse);
    rpc UnlinkIssue(UnlinkIssueRequest) returns (UnlinkIssueResponse);
}

message PushFindingRequest {
    string finding_id = 1;
    string product_id = 2;
}
```

---

## REST API

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET/POST` | `/api/v2/jira-configurations` | JWT/Admin | CRUD JIRA configs |
| `GET/PUT/DELETE` | `/api/v2/jira-configurations/{id}` | JWT/Admin | CRUD |
| `GET` | `/api/v2/jira-issues` | JWT | List finding → JIRA mappings |
| `POST` | `/api/v2/jira-issues` | JWT/Writer | Manual push finding to JIRA |
| `GET` | `/api/v2/jira-issues/{finding_id}` | JWT | Get JIRA issue for finding |
| `DELETE` | `/api/v2/jira-issues/{id}` | JWT/Maintainer | Unlink |
| `POST` | `/webhooks/jira/{config_id}` | HMAC only | JIRA webhook (no JWT) |

> **Security**: `password` field trong responses luôn được redact (`***`). Không bao giờ trả về plaintext credentials.

---

## NATS Events

### Published
```
jira.issue.created   {finding_id, jira_key, jira_url, product_id}
jira.issue.updated   {finding_id, jira_key, old_status, new_status}
jira.synced          {finding_id, jira_key, jira_status}
jira.sync.failed     {finding_id, error}
```

### Subscribed
```
finding.created          → PushFinding (if push_all_issues=true)
finding.status_changed   → UpdateJIRAIssue (close/reopen JIRA issue)
```

---

## Database Schema

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

## Acceptance Criteria

- [x] `POST /api/v2/jira-configurations` → password encrypted AES-256 trước khi lưu
- [x] `GET /api/v2/jira-configurations/{id}` → password field là `***` (not plaintext)
- [x] Manual push: `POST /api/v2/jira-issues` → JIRA issue created với priority mapping đúng
- [x] `push_all_issues=true` → `finding.created` event → JIRA issue auto-created
- [x] JIRA webhook invalid HMAC → 401 response
- [x] JIRA "Done" status → finding.status = mitigated (via finding-service gRPC)
- [x] JIRA "Reopened" → finding reactivated
- [x] JIRA comment → Finding note (prefix `[JIRA]`)
- [x] Retry: JIRA API failure → max 3 retries với 5s/10s/15s backoff
- [x] `enable_deduplication=true`: same finding pushed twice → only 1 JIRA issue

## Implementation Status: ✅ DONE

> `jira-service/internal/usecase/push_finding.go` — PushFindingUseCase: 6-step pipeline (config lookup → dedup check → gRPC finding → build description → retry 3x → save mapping + publish NATS)
> `jira-service/internal/usecase/sync/pull_status.go` — jiraStatusToAction map: done/resolved/closed → close; in_progress/open/reopened → reopen; SyncComment: JIRA → Finding note with "[JIRA]" prefix
> `jira-service/internal/delivery/http/webhook_handler.go` — verifyHMAC (constant-time), 202 Accepted immediately, async processWebhook()
> `jira-service/internal/domain/{jiraconfig,issuemapping}/entity.go` — JIRAConfig + JIRAIssueMapping
> `jira-service/migrations/{001_jira_configurations,002_jira_issue_mappings}.sql` — UNIQUE(product_id) + UNIQUE(finding_id) + 2 indexes
> AES-256-GCM encryption: password masked in GET responses, OSV_JIRA_ENCRYPTION_KEY env var
