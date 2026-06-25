# ✅ COMPLETED — TASK-DD-029 — JIRA Webhook Handler + Pull Status (JIRA → Finding)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-029 |
| **Service** | `jira-service` |
| **CR** | CR-DD-008 |
| **Phase** | 3 — Integrations |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-DD-028 |
| **Estimated effort** | 1 ngày |

## Context

Implement JIRA webhook receiver (`POST /webhooks/jira/{config_id}`) để nhận status changes từ JIRA và sync ngược về finding-service. Cũng implement NATS subscriber để auto-push findings khi `push_all_issues=true`.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/jira-service/
```

## Files to Create

```
internal/usecase/sync/
└── pull_status.go          # JIRA status → finding state

internal/delivery/
├── http/
│   └── webhook_handler.go  # POST /webhooks/jira/{config_id}
└── event/
    ├── subscriber.go
    └── handlers/
        ├── finding_created.go    # auto-push if push_all_issues
        └── finding_status.go     # sync JIRA status on finding change
```

## Implementation Spec

### `internal/delivery/http/webhook_handler.go`

```go
package http

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "net/http"
    "strings"

    "github.com/go-chi/chi/v5"
)

type WebhookHandler struct {
    configRepo  ConfigRepository
    pullStatus  *sync.PullStatusUseCase
}

func (h *WebhookHandler) RegisterRoutes(r chi.Router) {
    r.Post("/webhooks/jira/{config_id}", h.Handle)
}

// POST /webhooks/jira/{config_id}
// No JWT auth — HMAC signature verification instead
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
    configID := chi.URLParam(r, "config_id")

    // 1. Get config (for webhook secret)
    cfg, err := h.configRepo.FindByID(r.Context(), configID)
    if err != nil || cfg == nil {
        http.Error(w, `{"detail":"Configuration not found"}`, http.StatusNotFound)
        return
    }

    // 2. Read body
    body, _ := io.ReadAll(r.Body)

    // 3. Verify HMAC (X-Hub-Signature header)
    if cfg.WebhookSecret != "" {
        signature := r.Header.Get("X-Hub-Signature")
        if !verifyHMAC(body, cfg.WebhookSecret, signature) {
            http.Error(w, `{"detail":"Invalid signature"}`, http.StatusUnauthorized)
            return
        }
    }

    // 4. Return 202 Accepted immediately (async processing)
    w.WriteHeader(http.StatusAccepted)
    w.Write([]byte(`{"status":"accepted"}`))

    // 5. Process webhook async
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        h.processWebhook(ctx, body, cfg)
    }()
}

// verifyHMAC verifies "sha256=<hex>" signature against body
func verifyHMAC(body []byte, secret, signature string) bool {
    expectedPrefix := "sha256="
    if !strings.HasPrefix(signature, expectedPrefix) {
        return false
    }
    expectedHex := strings.TrimPrefix(signature, expectedPrefix)
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    actual := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(actual), []byte(expectedHex))
}

// JIRAWebhookPayload — JIRA webhook event structure
type JIRAWebhookPayload struct {
    WebhookEvent string      `json:"webhookEvent"`
    Issue        JIRAIssue   `json:"issue"`
    ChangeLog    ChangeLog   `json:"changelog"`
    Comment      *Comment    `json:"comment"`
}

type JIRAIssue struct {
    ID     string   `json:"id"`
    Key    string   `json:"key"`
    Fields struct {
        Status struct {
            Name string `json:"name"`
        } `json:"status"`
        Summary string `json:"summary"`
    } `json:"fields"`
}

type ChangeLog struct {
    Items []ChangeLogItem `json:"items"`
}
type ChangeLogItem struct {
    Field    string `json:"field"`
    FromStr  string `json:"fromString"`
    ToStr    string `json:"toString"`
}

func (h *WebhookHandler) processWebhook(ctx context.Context, body []byte, cfg *JIRAConfig) {
    var payload JIRAWebhookPayload
    if err := json.Unmarshal(body, &payload); err != nil {
        return
    }

    switch payload.WebhookEvent {
    case "jira:issue_updated":
        // Check if status changed
        for _, item := range payload.ChangeLog.Items {
            if item.Field == "status" {
                h.pullStatus.Execute(ctx, &sync.PullStatusInput{
                    JIRAKey:       payload.Issue.Key,
                    JIRAID:        payload.Issue.ID,
                    NewJIRAStatus: item.ToStr,
                })
            }
        }
    case "comment_created", "comment_updated":
        if payload.Comment != nil {
            h.pullStatus.SyncComment(ctx, payload.Issue.Key, payload.Comment)
        }
    }
}
```

### `internal/usecase/sync/pull_status.go`

```go
package sync

import (
    "context"
    "strings"
)

// jiraStatusToAction maps JIRA status names → finding action
var jiraStatusToAction = map[string]string{
    "done":        "close",
    "resolved":    "close",
    "closed":      "close",
    "won't fix":   "close",
    "in progress": "reopen",
    "open":        "reopen",
    "reopened":    "reopen",
    "to do":       "reopen",
}

type PullStatusInput struct {
    JIRAKey       string
    JIRAID        string
    NewJIRAStatus string
}

type PullStatusUseCase struct {
    mappingRepo   issuemapping.Repository
    findingClient FindingServiceClient
    eventPub      EventPublisher
}

func (uc *PullStatusUseCase) Execute(ctx context.Context, in *PullStatusInput) error {
    // 1. Find finding by JIRA key
    mapping, err := uc.mappingRepo.FindByJIRAKey(ctx, in.JIRAKey)
    if err != nil || mapping == nil {
        return nil // no mapping found — ignore
    }

    // 2. Update mapping status
    mapping.JIRAStatus = in.NewJIRAStatus
    uc.mappingRepo.Save(ctx, mapping)

    // 3. Determine action
    action := jiraStatusToAction[strings.ToLower(in.NewJIRAStatus)]
    if action == "" {
        return nil // unknown status — no action
    }

    // 4. Call finding-service gRPC to update finding state
    switch action {
    case "close":
        uc.findingClient.CloseFinding(ctx, mapping.FindingID)
    case "reopen":
        uc.findingClient.ReopenFinding(ctx, mapping.FindingID)
    }

    uc.eventPub.Publish(ctx, "jira.synced", map[string]any{
        "finding_id": mapping.FindingID,
        "jira_key":   in.JIRAKey,
        "jira_status": in.NewJIRAStatus,
        "action":     action,
        "_service":   "jira-service",
    })
    return nil
}

// SyncComment syncs JIRA comment to finding note
func (uc *PullStatusUseCase) SyncComment(ctx context.Context, jiraKey string, comment *Comment) {
    mapping, _ := uc.mappingRepo.FindByJIRAKey(ctx, jiraKey)
    if mapping == nil { return }

    // Add "[JIRA]" prefix to distinguish JIRA-originated notes
    content := fmt.Sprintf("[JIRA] %s", comment.Body)
    uc.findingClient.AddFindingNote(ctx, mapping.FindingID, "system", content)
}
```

### `internal/delivery/event/handlers/finding_created.go`

```go
package handlers

// Subscribe finding.created → auto-push if config has push_all_issues=true

func (h *FindingCreatedHandler) Handle(msg *nats.Msg) {
    var event struct {
        FindingID string `json:"finding_id"`  // Note: batch_created gives array
        ProductID string `json:"product_id"`
    }
    json.Unmarshal(msg.Data, &event)

    ctx := context.Background()
    cfg, _ := h.configRepo.FindByProduct(ctx, event.ProductID)
    if cfg == nil || !cfg.PushAllIssues {
        return // auto-push not enabled
    }

    h.pushFindingUC.Execute(ctx, &sync.PushFindingInput{
        FindingID: event.FindingID,
        ProductID: event.ProductID,
        Manual:    false,
    })
}
```

## Acceptance Criteria

- [x] `POST /webhooks/jira/{config_id}` với invalid HMAC → 401
- [x] `POST /webhooks/jira/{config_id}` với valid HMAC → 202 Accepted immediately
- [x] JIRA "Done" status webhook → finding closed via finding-service gRPC
- [x] JIRA "Reopened" status → finding reactivated
- [x] JIRA comment webhook → finding note added with "[JIRA]" prefix
- [x] `finding.created` event with `push_all_issues=true` config → auto-push to JIRA
- [x] `finding.created` event with `push_all_issues=false` → no auto-push
- [x] Webhook processing async (202 returned before JIRA processing completes)
- [x] NATS `jira.synced` event published after status sync
- [x] Unknown JIRA status → no action, no error

## Implementation Status: ✅ DONE

> `jira-service/internal/delivery/http/webhook_handler.go` — verifyHMAC (sha256= prefix, constant-time), 202 Accepted, async processWebhook()
> `jira-service/internal/usecase/sync/pull_status.go` — jiraStatusToAction map (done/resolved/closed → close, in_progress/open/reopened → reopen), SyncComment
> NATS `jira.synced` published after each status sync; unknown status silently ignored
> Auto-push: FindingCreatedHandler checks push_all_issues flag
