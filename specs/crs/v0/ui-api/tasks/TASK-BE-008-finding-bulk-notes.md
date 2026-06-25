# TASK-BE-008 — finding-service: Bulk Ops (reopen/assign) + Notes

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-008 |
| **Service** | `services/finding-service` |
| **Solution Ref** | [SOL-UI-004 §1.3](../solutions/SOL-UI-004-finding-product-reports-admin.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-BE-007 (finding schema, dto helpers) |
| **Estimated** | 3h |

---

## Context

UI Finding Management page cần:
- `POST /api/v1/findings/bulk/reopen` — batch reopen closed findings
- `POST /api/v1/findings/bulk/assign` — batch assign to user
- `POST /api/v1/findings/{id}/notes` — add note/comment to finding
- `GET /api/v1/findings/stats` — aggregated stats for a product

---

## Goal

Thêm bulk handlers, notes handler, stats handler vào finding-service.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/finding-service/db/migrations/005_finding_notes.sql` |
| CREATE | `services/finding-service/internal/adapter/http/bulk_handler.go` |
| CREATE | `services/finding-service/internal/adapter/http/note_handler.go` |
| MODIFY | `services/finding-service/internal/adapter/http/router.go` |

---

## Implementation

### File 1: `services/finding-service/db/migrations/005_finding_notes.sql`

```sql
-- +migrate Up
CREATE TABLE IF NOT EXISTS finding_notes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id  UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    created_by  VARCHAR(255) NOT NULL,  -- user email
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_finding_notes_finding_id ON finding_notes(finding_id);

-- +migrate Down
DROP TABLE IF EXISTS finding_notes;
```

### File 2: `services/finding-service/internal/adapter/http/bulk_handler.go`

```go
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type BulkHandler struct {
	findingRepo FindingRepository
	eventBus    EventBus
}

// POST /v2/findings/bulk_reopen  → also alias POST /findings/bulk/reopen
func (h *BulkHandler) BulkReopen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FindingIDs []string `json:"finding_ids"`
		Comment    string   `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if len(req.FindingIDs) == 0 {
		respondError(w, 400, "VALIDATION_ERROR", "finding_ids is required")
		return
	}
	if len(req.FindingIDs) > 100 {
		respondError(w, 400, "VALIDATION_ERROR", "maximum 100 findings per bulk operation")
		return
	}

	userID := r.Header.Get("X-User-ID")
	success, failed := 0, []string{}

	for _, idStr := range req.FindingIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			failed = append(failed, idStr)
			continue
		}

		finding, err := h.findingRepo.FindByID(r.Context(), id)
		if err != nil {
			failed = append(failed, idStr)
			continue
		}

		// State machine: can only reopen non-active findings
		if !finding.IsMitigated && !finding.FalsePositive && !finding.RiskAccepted {
			failed = append(failed, idStr) // already active
			continue
		}

		finding.IsMitigated = false
		finding.FalsePositive = false
		finding.RiskAccepted = false
		finding.OutOfScope = false
		finding.UpdatedAt = time.Now()

		if err := h.findingRepo.Update(r.Context(), finding); err != nil {
			failed = append(failed, idStr)
			continue
		}

		// Publish NATS event
		h.eventBus.Publish(r.Context(), "finding.status.changed", map[string]interface{}{
			"finding_id": idStr,
			"new_status": "Active",
			"changed_by": userID,
		})

		success++
	}

	respondJSON(w, 200, map[string]interface{}{
		"success_count": success,
		"failed_ids":    failed,
	})
}

// POST /v2/findings/bulk_assign  → also alias POST /findings/bulk/assign
func (h *BulkHandler) BulkAssign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FindingIDs []string `json:"finding_ids"`
		AssignedTo string   `json:"assigned_to"` // user email
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if len(req.FindingIDs) == 0 || req.AssignedTo == "" {
		respondError(w, 400, "VALIDATION_ERROR", "finding_ids and assigned_to are required")
		return
	}

	count, err := h.findingRepo.BulkUpdateAssignee(r.Context(), req.FindingIDs, req.AssignedTo)
	if err != nil {
		respondError(w, 500, "INTERNAL_ERROR", err.Error())
		return
	}

	respondJSON(w, 200, map[string]interface{}{
		"success_count": count,
		"failed_ids":    []string{},
	})
}

// GET /findings/stats
func (h *BulkHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	productID := r.URL.Query().Get("product_id")

	stats, err := h.findingRepo.GetStats(r.Context(), productID)
	if err != nil {
		respondError(w, 500, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, 200, stats)
}
```

### SQL for BulkUpdateAssignee:

```sql
UPDATE findings
SET assigned_to = $2, updated_at = NOW()
WHERE id = ANY($1::uuid[])
RETURNING id;
```

### File 3: `services/finding-service/internal/adapter/http/note_handler.go`

```go
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type NoteHandler struct {
	findingRepo FindingRepository
	noteRepo    NoteRepository
}

// POST /v2/findings/{id}/notes
func (h *NoteHandler) AddNote(w http.ResponseWriter, r *http.Request) {
	findingID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid finding ID")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if req.Content == "" {
		respondError(w, 400, "VALIDATION_ERROR", "content is required")
		return
	}

	// Verify finding exists
	if _, err := h.findingRepo.FindByID(r.Context(), findingID); err != nil {
		respondError(w, 404, "NOT_FOUND", "Finding not found")
		return
	}

	userEmail := r.Header.Get("X-User-Email")
	if userEmail == "" {
		userEmail = r.Header.Get("X-User-ID") // fallback
	}

	note := &FindingNote{
		ID:        uuid.New(),
		FindingID: findingID,
		Content:   req.Content,
		CreatedBy: userEmail,
		CreatedAt: time.Now(),
	}

	if err := h.noteRepo.Create(r.Context(), note); err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Failed to add note")
		return
	}

	respondJSON(w, 201, map[string]interface{}{
		"id":         note.ID,
		"finding_id": findingID,
		"content":    note.Content,
		"created_by": note.CreatedBy,
		"created_at": note.CreatedAt.Format(time.RFC3339),
	})
}

// GET /v2/findings/{id}/notes
func (h *NoteHandler) ListNotes(w http.ResponseWriter, r *http.Request) {
	findingID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid finding ID")
		return
	}

	notes, err := h.noteRepo.ListByFinding(r.Context(), findingID)
	if err != nil {
		respondError(w, 500, "INTERNAL_ERROR", err.Error())
		return
	}

	respondJSON(w, 200, map[string]interface{}{"notes": notes})
}

// Domain type
type FindingNote struct {
	ID        uuid.UUID `json:"id"`
	FindingID uuid.UUID `json:"finding_id"`
	Content   string    `json:"content"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}
```

### Router additions:

```go
// services/finding-service/internal/adapter/http/router.go

// Bulk operations
mux.HandleFunc("POST /v2/findings/bulk_reopen",   h.Bulk.BulkReopen)
mux.HandleFunc("POST /v2/findings/bulk_assign",   h.Bulk.BulkAssign)
// v1 aliases (gateway maps /api/v1/findings/bulk/reopen → /v2/findings/bulk_reopen)
// OR add explicit v1 paths:
mux.HandleFunc("POST /findings/bulk/reopen",      h.Bulk.BulkReopen)
mux.HandleFunc("POST /findings/bulk/assign",      h.Bulk.BulkAssign)

// Stats
mux.HandleFunc("GET /findings/stats",             h.Bulk.GetStats)

// Notes
mux.HandleFunc("POST /v2/findings/{id}/notes",    h.Note.AddNote)
mux.HandleFunc("GET  /v2/findings/{id}/notes",    h.Note.ListNotes)
```

---

## Verification

```bash
cd services/finding-service
go build ./...

# Bulk reopen
curl -X POST http://localhost:8085/findings/bulk/reopen \
  -H "X-User-ID: $USER_ID" \
  -H "Content-Type: application/json" \
  -d '{"finding_ids":["uuid1","uuid2"],"comment":"Reopening for review"}' | jq .

# Add note
curl -X POST "http://localhost:8085/v2/findings/$FINDING_ID/notes" \
  -H "X-User-ID: $USER_ID" \
  -H "X-User-Email: user@osv.local" \
  -H "Content-Type: application/json" \
  -d '{"content":"Verified false positive after investigation"}' | jq .
# Expected: {"id":"...","content":"Verified...","created_by":"user@osv.local"}
```

---

## Checklist

- [x] `005_finding_notes.sql` tạo `finding_notes` table với FK và index
- [x] `BulkReopen` limit 100 findings per request, skip already-active findings
- [x] `BulkReopen` publish NATS event `finding.status.changed` mỗi reopen
- [x] `BulkAssign` dùng SQL `ANY($1::uuid[])` để update trong 1 query
- [x] `AddNote` kiểm tra finding exists trước khi tạo note
- [x] `AddNote` lấy `created_by` từ `X-User-Email` header
- [x] `go build ./...` thành công
