# ✅ COMPLETED — TASK-DD-027 — JIRA Service Bootstrap (New Service)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-027 |
| **Service** | `jira-service` (NEW) |
| **CR** | CR-DD-008 |
| **Phase** | 3 — Integrations |
| **Priority** | 🔴 High |
| **Prerequisites** | — (độc lập) |
| **Estimated effort** | 1 ngày |

## Context

Tạo mới `jira-service` với scaffolding: go module, Dockerfile, migrations, cấu trúc domain (JIRAConfig, JIRAIssueMapping). Config credentials stored AES-256-GCM encrypted.

## Reference

- Solution: [`sol-jira-service.md`](../solutions/sol-jira-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/jira-service/
```

## Files to Create

```
services/jira-service/
├── go.mod
├── Dockerfile
├── .env.example
├── cmd/server/main.go
├── migrations/
│   ├── 001_jira_configurations.sql
│   └── 002_jira_issue_mappings.sql
└── internal/
    ├── config/config.go
    ├── domain/
    │   ├── jiraconfig/
    │   │   ├── entity.go
    │   │   └── repository.go
    │   └── issuemapping/
    │       ├── entity.go
    │       └── repository.go
    └── infra/
        ├── crypto/aes256gcm.go
        └── postgres/db.go
```

## Implementation Spec

### `internal/domain/jiraconfig/entity.go`

```go
package jiraconfig

import "time"

type JIRAConfig struct {
    ID        string
    ProductID string

    // Connection (credentials encrypted at rest)
    URL         string
    Username    string
    PasswordEnc string   // AES-256-GCM encrypted

    // Project
    ProjectKey      string
    IssueTypeID     string
    IssueTypeFields map[string]interface{}

    // Behavior
    DefaultAssignee     string
    FindSeverityField   string
    FindURLField        string
    PushNotes           bool
    PushAllIssues       bool
    EnableDeduplication bool

    // Priority mapping: Severity → JIRA Priority Name
    PriorityMapping map[string]string

    // Webhook verification
    WebhookSecret string

    IsActive  bool
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### `internal/domain/issuemapping/entity.go`

```go
package issuemapping

import "time"

type JIRAIssueMapping struct {
    ID         string
    FindingID  string
    JIRAID     string
    JIRAKey    string     // e.g., "PROJ-123"
    JIRAURL    string
    JIRAStatus string
    Synced     bool
    LastSyncAt *time.Time
    CreatedAt  time.Time
}
```

### `migrations/001_jira_configurations.sql`

```sql
CREATE TABLE IF NOT EXISTS jira_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL UNIQUE,
    url VARCHAR(2048) NOT NULL,
    username VARCHAR(255) NOT NULL,
    password_enc TEXT NOT NULL,
    project_key VARCHAR(50) NOT NULL,
    issue_type_id VARCHAR(50) NOT NULL,
    issue_type_fields JSONB DEFAULT '{}',
    default_assignee VARCHAR(255),
    find_severity_field VARCHAR(255),
    find_url_field VARCHAR(255),
    push_notes BOOLEAN DEFAULT FALSE,
    push_all_issues BOOLEAN DEFAULT FALSE,
    enable_deduplication BOOLEAN DEFAULT TRUE,
    priority_mapping JSONB DEFAULT '{"Critical":"Highest","High":"High","Medium":"Medium","Low":"Low","Info":"Lowest"}',
    webhook_secret VARCHAR(255),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_jira_config_product ON jira_configurations(product_id);
```

### `migrations/002_jira_issue_mappings.sql`

```sql
CREATE TABLE IF NOT EXISTS jira_issue_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL UNIQUE,
    jira_id VARCHAR(100) NOT NULL,
    jira_key VARCHAR(100) NOT NULL,
    jira_url TEXT NOT NULL,
    jira_status VARCHAR(100),
    synced BOOLEAN DEFAULT TRUE,
    last_sync_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_jira_mapping_key ON jira_issue_mappings(jira_key);
CREATE INDEX IF NOT EXISTS idx_jira_mapping_finding ON jira_issue_mappings(finding_id);
```

### `internal/infra/crypto/aes256gcm.go`

Identical to TASK-DD-002 implementation. Copy from finding-service or extract to shared package `pkg/crypto`.

### `.env.example`

```
JIRA_HTTP_PORT=8088
JIRA_GRPC_PORT=9008
JIRA_DATABASE_URL=postgres://jira:jira@localhost:5432/jira_db?sslmode=disable
NATS_ADDRESS=nats://localhost:4222
FINDING_GRPC_ADDR=finding-service:9005
OSV_JIRA_ENCRYPTION_KEY=<base64-encoded-32-byte-key>
```

## go.mod

```
module github.com/osv/services/jira-service

go 1.22

require (
    github.com/andygrunwald/go-jira/v2 v2.0.0
    github.com/go-chi/chi/v5 v5.1.0
    github.com/nats-io/nats.go v1.35.0
    github.com/lib/pq v1.10.9
    google.golang.org/grpc v1.64.0
    github.com/google/uuid v1.6.0
)
```

## Acceptance Criteria

- [x] `go build ./...` thành công
- [x] `docker build -t jira-service:test .` thành công
- [x] Service starts on port 8088 (HTTP) và 9008 (gRPC)
- [x] Migrations chạy thành công
- [x] `jira_configurations` table created với UNIQUE constraint on product_id
- [x] `jira_issue_mappings` table created với UNIQUE constraint on finding_id
- [x] Default priority_mapping seeded (Critical→Highest, etc.)
- [x] `AES256GCM.Encrypt` → `Decrypt` roundtrip works
- [x] `GET /health` → 200

## Implementation Status: ✅ DONE

> `jira-service/internal/domain/jiraconfig/entity.go` — JIRAConfig (URL, credentials encrypted, priority_mapping, webhook_secret)
> `jira-service/internal/domain/issuemapping/entity.go` — JIRAIssueMapping
> `jira-service/migrations/001_jira_configurations.sql` — jira_configurations table + UNIQUE(product_id) + default priority_mapping JSONB
> `jira-service/migrations/002_jira_issue_mappings.sql` — jira_issue_mappings + UNIQUE(finding_id) + 2 indexes
> `.env.example` — JIRA_HTTP_PORT=8088, JIRA_GRPC_PORT=9008, OSV_JIRA_ENCRYPTION_KEY
