# TASK-DD-005 — Finding Bulk Ops, Notes, Groups, CVSS REST API

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-005 |
| **Service** | `finding-service` |
| **CR** | CR-DD-004 |
| **Phase** | 1 — Foundation |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-004 |
| **Estimated effort** | 1.5 ngày |

## Context

Implement Bulk Operations (close/reopen/delete/tag nhiều findings cùng lúc), Finding Notes (comments), Finding Groups (logical grouping), và REST API cho state machine transitions. Cũng thêm REST endpoints `/api/v2/findings/{id}/close`, `/api/v2/findings/{id}/reopen`, etc.

## Reference

- Solution: [`sol-finding-service.md § CR-DD-004`](../solutions/sol-finding-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/
```

## Files to Create

```
internal/domain/note/
├── entity.go
└── repository.go

internal/domain/group/
├── entity.go
└── repository.go

internal/usecase/finding/
├── bulk.go
├── note_add.go
└── note_list.go

internal/delivery/http/
├── finding_state_handler.go    # close/reopen/accept-risk/false-positive
├── finding_bulk_handler.go
└── finding_note_handler.go

internal/infra/postgres/
├── note_repo.go
└── group_repo.go
```

## Implementation Spec

### `internal/domain/note/entity.go`

```go
package note

import "time"

type FindingNote struct {
    ID        string
    FindingID string
    AuthorID  string
    Content   string
    EditCount int
    IsPrivate bool
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### `internal/domain/note/repository.go`

```go
package note

import "context"

type FindingNoteRepository interface {
    Save(ctx context.Context, note *FindingNote) error
    FindByID(ctx context.Context, id string) (*FindingNote, error)
    ListByFinding(ctx context.Context, findingID string) ([]*FindingNote, error)
    Delete(ctx context.Context, id string) error
}
```

### `internal/domain/group/entity.go`

```go
package group

import "time"

type FindingGroup struct {
    ID           string
    Name         string
    TestID       string
    JIRAIssueKey string  // optional JIRA link
    FindingIDs   []string
    CreatedAt    time.Time
}
```

### `internal/usecase/finding/bulk.go`

```go
package finding

import (
    "context"
    "fmt"
    "log/slog"
    "github.com/osv/services/finding-service/internal/domain/finding"
)

type BulkOperation string
const (
    BulkClose          BulkOperation = "close"
    BulkReopen         BulkOperation = "reopen"
    BulkMarkFP         BulkOperation = "false_positive"
    BulkMarkOOS        BulkOperation = "out_of_scope"
    BulkDelete         BulkOperation = "delete"
    BulkAddTags        BulkOperation = "add_tags"
    BulkRemoveTags     BulkOperation = "remove_tags"
    BulkSetSeverity    BulkOperation = "set_severity"
)

type BulkUpdateInput struct {
    FindingIDs  []string
    Operation   BulkOperation
    Tags        []string  // for add_tags/remove_tags
    Severity    string    // for set_severity
    RequesterID string
    ProductID   string
}

type BulkUpdateResult struct {
    Updated int
    Failed  int
    Errors  []string
}

type BulkUpdateFindingsUseCase struct {
    findingRepo finding.Repository
    closeUC     *CloseFindingUseCase
    reopenUC    *ReopenFindingUseCase
    markFPUC    *MarkFalsePositiveUseCase
    eventPub    EventPublisher
}

func (uc *BulkUpdateFindingsUseCase) Execute(ctx context.Context, in BulkUpdateInput) (*BulkUpdateResult, error) {
    if len(in.FindingIDs) == 0 {
        return &BulkUpdateResult{}, nil
    }
    if len(in.FindingIDs) > 1000 {
        return nil, fmt.Errorf("bulk operation limited to 1000 findings, got %d", len(in.FindingIDs))
    }

    result := &BulkUpdateResult{}

    for _, fid := range in.FindingIDs {
        var err error
        switch in.Operation {
        case BulkClose:
            err = uc.closeUC.Execute(ctx, CloseFindingInput{FindingID: fid, RequesterID: in.RequesterID})
        case BulkReopen:
            err = uc.reopenUC.Execute(ctx, ReopenFindingInput{FindingID: fid, RequesterID: in.RequesterID})
        case BulkMarkFP:
            err = uc.markFPUC.Execute(ctx, MarkFalsePositiveInput{FindingID: fid, RequesterID: in.RequesterID})
        case BulkDelete:
            err = uc.findingRepo.Delete(ctx, fid)
        case BulkAddTags, BulkRemoveTags, BulkSetSeverity:
            f, ferr := uc.findingRepo.FindByID(ctx, fid)
            if ferr != nil {
                err = ferr
                break
            }
            switch in.Operation {
            case BulkAddTags:
                f.Tags = appendUnique(f.Tags, in.Tags...)
            case BulkRemoveTags:
                f.Tags = removeAll(f.Tags, in.Tags...)
            case BulkSetSeverity:
                f.Severity = in.Severity
            }
            err = uc.findingRepo.Save(ctx, f)
        }

        if err != nil {
            result.Failed++
            result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", fid, err))
            slog.ErrorContext(ctx, "bulk operation failed for finding", "finding_id", fid, "error", err)
        } else {
            result.Updated++
        }
    }

    uc.eventPub.Publish(ctx, "finding.bulk_updated", map[string]any{
        "finding_ids": in.FindingIDs,
        "operation":   string(in.Operation),
        "product_id":  in.ProductID,
        "updated":     result.Updated,
        "_service":    "finding-service",
    })

    return result, nil
}
```

### `internal/usecase/finding/note_add.go`

```go
package finding

import (
    "context"
    "time"
    "github.com/google/uuid"
    "github.com/osv/services/finding-service/internal/domain/note"
)

type AddNoteInput struct {
    FindingID string
    AuthorID  string
    Content   string
    IsPrivate bool
}

type AddNoteUseCase struct {
    noteRepo note.FindingNoteRepository
}

func (uc *AddNoteUseCase) Execute(ctx context.Context, in AddNoteInput) (*note.FindingNote, error) {
    if in.Content == "" {
        return nil, ErrEmptyNote
    }
    n := &note.FindingNote{
        ID:        uuid.New().String(),
        FindingID: in.FindingID,
        AuthorID:  in.AuthorID,
        Content:   in.Content,
        IsPrivate: in.IsPrivate,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
    if err := uc.noteRepo.Save(ctx, n); err != nil {
        return nil, err
    }
    return n, nil
}
```

### `internal/delivery/http/finding_state_handler.go`

```go
package http

import (
    "encoding/json"
    "errors"
    "net/http"
    "github.com/go-chi/chi/v5"
    "github.com/osv/services/finding-service/internal/domain/finding"
    findinguc "github.com/osv/services/finding-service/internal/usecase/finding"
)

type FindingStateHandler struct {
    closeUC   *findinguc.CloseFindingUseCase
    reopenUC  *findinguc.ReopenFindingUseCase
    markFPUC  *findinguc.MarkFalsePositiveUseCase
    undoFPUC  *findinguc.UndoFalsePositiveUseCase
    oosUC     *findinguc.MarkOutOfScopeUseCase
    riskUC    *findinguc.AcceptRiskUseCase
}

func (h *FindingStateHandler) RegisterRoutes(r chi.Router) {
    r.Post("/api/v2/findings/{id}/close", h.Close)
    r.Post("/api/v2/findings/{id}/reopen", h.Reopen)
    r.Post("/api/v2/findings/{id}/false-positive", h.MarkFP)
    r.Post("/api/v2/findings/{id}/undo-false-positive", h.UndoFP)
    r.Post("/api/v2/findings/{id}/out-of-scope", h.MarkOOS)
    r.Post("/api/v2/findings/{id}/accept-risk", h.AcceptRisk)
}

func (h *FindingStateHandler) Close(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    userID := r.Header.Get("X-User-ID")

    if err := h.closeUC.Execute(r.Context(), findinguc.CloseFindingInput{
        FindingID: id, RequesterID: userID,
    }); err != nil {
        if errors.Is(err, finding.ErrInvalidTransition) {
            writeError(w, http.StatusConflict, "Finding cannot be closed from its current state")
            return
        }
        writeError(w, http.StatusInternalServerError, "Failed to close finding")
        return
    }
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "mitigated"})
}

func (h *FindingStateHandler) Reopen(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    userID := r.Header.Get("X-User-ID")

    if err := h.reopenUC.Execute(r.Context(), findinguc.ReopenFindingInput{
        FindingID: id, RequesterID: userID,
    }); err != nil {
        if errors.Is(err, finding.ErrInvalidTransition) {
            writeError(w, http.StatusConflict, "Finding cannot be reopened from its current state")
            return
        }
        writeError(w, http.StatusInternalServerError, "Failed to reopen finding")
        return
    }
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "active"})
}
```

### `internal/delivery/http/finding_bulk_handler.go`

```go
package http

// POST /api/v2/findings/bulk
// Request:
// {
//   "finding_ids": ["uuid1", "uuid2", ...],
//   "operation": "close",   // close|reopen|false_positive|delete|add_tags|remove_tags
//   "tags": ["tag1"],       // only for add_tags/remove_tags
//   "severity": "High"      // only for set_severity
// }
// Response: {"updated": 5, "failed": 0, "errors": []}

type BulkHandler struct {
    bulkUC *findinguc.BulkUpdateFindingsUseCase
}

func (h *BulkHandler) RegisterRoutes(r chi.Router) {
    r.Post("/api/v2/findings/bulk", h.BulkUpdate)
}
```

## REST Endpoints Summary

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v2/findings/{id}/close` | Active → Mitigated |
| POST | `/api/v2/findings/{id}/reopen` | Mitigated/FP/OOS/RA → Active |
| POST | `/api/v2/findings/{id}/false-positive` | Active → FalsePositive |
| POST | `/api/v2/findings/{id}/accept-risk` | Active → RiskAccepted |
| POST | `/api/v2/findings/{id}/out-of-scope` | Active → OutOfScope |
| GET | `/api/v2/findings/{id}/notes` | List finding notes |
| POST | `/api/v2/findings/{id}/notes` | Add note |
| GET | `/api/v2/finding-groups` | List groups |
| POST | `/api/v2/finding-groups` | Create group |
| POST | `/api/v2/findings/bulk` | Bulk operation |
| GET | `/api/v2/findings/severity_count` | Aggregate severity counts |

## Acceptance Criteria

- [x] `POST /api/v2/findings/{id}/close` → Active finding → 200, status=mitigated
- [x] `POST /api/v2/findings/{id}/close` → Mitigated finding → 409 Conflict
- [x] `POST /api/v2/findings/{id}/reopen` → Mitigated → 200, status=active
- [x] `POST /api/v2/findings/bulk` với 100 findings, op=close → all closed
- [x] Bulk > 1000 findings → 400 error
- [x] `POST /api/v2/findings/{id}/notes` → note created, content saved
- [x] `POST /api/v2/findings/{id}/notes` với `is_private=true` → note là private
- [x] Bulk operation publishes `finding.bulk_updated` NATS event
- [x] `GET /api/v2/findings/severity_count?product_id=X` → counts object _(implemented)_

## Implementation Status: ✅ DONE

> `internal/delivery/http/bulk_handler.go` — BulkUpdate endpoint (close/reopen/delete/tag/severity)
> `internal/delivery/http/note_handler.go` — ListNotes + AddNote endpoints
> `internal/usecase/finding/bulk.go` — BulkUpdateFindingsUseCase (limit 1000, NATS event)
> `internal/usecase/finding/note.go` — AddNoteUseCase, ListNotesUseCase
