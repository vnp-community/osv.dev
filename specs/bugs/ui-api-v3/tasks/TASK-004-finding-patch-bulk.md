# TASK-004: Finding-Service — PATCH /findings/{id} & Bulk Close/Reopen

> **Bug**: BUG-006  
> **Solution**: SOL-004  
> **Service**: `services/finding-service`  
> **File chính**: `internal/delivery/http/finding_handler.go`, `internal/delivery/http/bulk_handler.go`, `internal/delivery/http/router.go`  
> **Priority**: 🔴 HIGH  
> **Status**: `[x] DONE`

### Thay Đổi Thực Tế
- **PATCH handler**: Thêm `FindingHandler.PatchFinding()` vào `finding_handler.go` — dùng `json.NewDecoder`, delegate sang `transition` use case
- **BulkClose**: Thêm `BulkHandler.BulkClose()` vào `bulk_handler.go` — dùng `bulkUC.Execute` với operation `"close"`
- **Router**: Đăng ký `r.Patch("/{id}")` và `bulk/close`, `bulk/reopen`, `bulk/assign` (slash path) **trước** `/{id}` wildcard trong `/api/v1/findings` route group

## Phân Tích Thực Tế

**Từ router.go** (`internal/delivery/http/router.go`):
```go
r.Route("/api/v1/findings", func(r chi.Router) {
    r.Get("/", handler.List)
    r.Get("/{id}", handler.Get)
    r.Put("/{id}/close", handler.Close)
    r.Put("/{id}/reopen", handler.Reopen)
    r.Put("/{id}/false-positive", handler.MarkFalsePositive)
    r.Put("/{id}/risk-accepted", handler.AcceptRisk)
    // Notes: đã có (conditional)
})
```

**Thiếu**:
- ❌ `PATCH /api/v1/findings/{id}` — gateway forward tới đây nhưng không có handler
- ❌ `POST /api/v1/findings/bulk/close` — có `POST /bulk_close` (underscore) nhưng gateway forward `bulk/close` (slash)
- ❌ `POST /api/v1/findings/bulk/reopen` — tương tự
- ❌ `POST /api/v1/findings/bulk/assign` — tương tự

**Từ bulk_handler.go**:
```go
r.Post("/bulk_reopen", bulk.BulkReopen)   // underscore
r.Post("/bulk_assign", bulk.BulkAssign)   // underscore
```

**Gateway gọi**: `/bulk/close`, `/bulk/reopen`, `/bulk/assign` (slash)

## Việc Cần Làm

### Bước 1: Kiểm tra FindingHandler hiện có

```bash
grep -n "func.*Handler\|func.*Finding\|Patch\|PATCH\|Update\|Status" \
  services/finding-service/internal/delivery/http/finding_handler.go | head -30
```

### Bước 2: Thêm PATCH handler vào FindingHandler

File: `services/finding-service/internal/delivery/http/finding_handler.go`

```go
// PatchFinding handles PATCH /api/v1/findings/{id}
// Partial update: status transition, assignee, severity
func (h *FindingHandler) PatchFinding(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    findingID, err := uuid.Parse(id)
    if err != nil {
        respondError(w, http.StatusBadRequest, "invalid finding ID")
        return
    }

    var req struct {
        Status   *string `json:"status"`    // "Active", "Mitigated", "FalsePositive", etc.
        Severity *string `json:"severity"`  // optional
        Notes    *string `json:"notes"`     // optional
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    userID := r.Header.Get("X-User-ID")

    f, err := h.findingRepo.GetByID(r.Context(), findingID)
    if err != nil {
        respondError(w, http.StatusNotFound, "finding not found")
        return
    }

    // State transition
    if req.Status != nil {
        newState := finding.State(*req.Status)
        if err := f.TransitionTo(newState, userID); err != nil {
            respondError(w, http.StatusBadRequest, err.Error())
            return
        }
    }

    if err := h.findingRepo.Update(r.Context(), f); err != nil {
        respondError(w, http.StatusInternalServerError, "failed to update finding")
        return
    }

    respondJSON(w, http.StatusOK, f)
}
```

### Bước 3: Fix Bulk Handler routes (underscore → slash)

File: `services/finding-service/internal/delivery/http/router.go`

**Tìm** (trong block v1 bulk hoặc v2):
```go
r.Post("/bulk_reopen", bulk.BulkReopen)
r.Post("/bulk_assign", bulk.BulkAssign)
```

**Thêm aliases với slash path** (KHÔNG xóa underscore — có thể có clients cũ):
```go
// Trong /api/v1/findings route group — PHẢI trước /{id}
// Gateway calls: /bulk/close, /bulk/reopen, /bulk/assign
r.Post("/bulk/close",  bulk.BulkClose)    // THÊM MỚI (gateway path)
r.Post("/bulk/reopen", bulk.BulkReopen)   // THÊM MỚI (gateway path)
r.Post("/bulk/assign", bulk.BulkAssign)   // THÊM MỚI (gateway path)

// Giữ aliases cũ để không break existing integrations
r.Post("/bulk_reopen", bulk.BulkReopen)   // Giữ nguyên
r.Post("/bulk_assign", bulk.BulkAssign)   // Giữ nguyên
```

### Bước 4: Kiểm tra BulkHandler có BulkClose không

```bash
grep -n "func.*Bulk\|BulkClose\|BulkReopen\|BulkAssign" \
  services/finding-service/internal/delivery/http/bulk_handler.go | head -20
```

Nếu thiếu `BulkClose`:

```go
// services/finding-service/internal/delivery/http/bulk_handler.go

// BulkClose handles POST /api/v1/findings/bulk/close
func (h *BulkHandler) BulkClose(w http.ResponseWriter, r *http.Request) {
    var req struct {
        FindingIDs []string `json:"finding_ids"`
        Reason     string   `json:"reason,omitempty"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    if len(req.FindingIDs) == 0 {
        respondError(w, http.StatusBadRequest, "finding_ids required")
        return
    }

    userID := r.Header.Get("X-User-ID")
    
    ids := parseUUIDs(req.FindingIDs)
    updated, err := h.usecase.BulkTransition(r.Context(), ids, finding.StateMitigated, userID)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    respondJSON(w, http.StatusOK, map[string]int{"updated": updated})
}
```

### Bước 5: Register PATCH trong router

File: `services/finding-service/internal/delivery/http/router.go`

```go
r.Route("/api/v1/findings", func(r chi.Router) {
    r.Get("/", handler.List)
    r.Get("/stats", handler.Stats)  // nếu có

    // THÊM bulk routes TRƯỚC /{id}:
    r.Post("/bulk/close",  bulk.BulkClose)
    r.Post("/bulk/reopen", bulk.BulkReopen)
    r.Post("/bulk/assign", bulk.BulkAssign)

    r.Get("/{id}", handler.Get)
    r.PATCH("/{id}", handler.PatchFinding)  // THÊM MỚI

    // Giữ nguyên các routes cũ:
    r.Put("/{id}/close", handler.Close)
    r.Put("/{id}/reopen", handler.Reopen)
    r.Put("/{id}/false-positive", handler.MarkFalsePositive)
    r.Put("/{id}/risk-accepted", handler.AcceptRisk)

    if note != nil {
        r.Get("/{id}/notes", note.ListNotes)
        r.Post("/{id}/notes", note.AddNote)
    }
})
```

### Bước 6: Build & Test

```bash
cd services/finding-service && go build ./...
```

**Test**:
```bash
TOKEN="your_jwt_token"
BASE="https://c12.openledger.vn"

# Test PATCH
curl -s -X PATCH "$BASE/api/v1/findings/FINDING_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": "Mitigated"}' | jq .

# Test bulk close
curl -s -X POST "$BASE/api/v1/findings/bulk/close" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"finding_ids": ["ID1", "ID2"]}' | jq .

# Test notes GET (đã có, verify)
curl -s "$BASE/api/v1/findings/FINDING_ID/notes" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

## Acceptance Criteria

- [x] `PATCH /api/v1/findings/{id}` → `200 OK`
- [x] `POST /api/v1/findings/bulk/close` → `200 OK` với `{updated: N}`
- [x] `POST /api/v1/findings/bulk/reopen` → `200 OK` với `{updated: N}`
- [x] `POST /api/v1/findings/bulk/assign` → `200 OK` với `{updated: N}`
- [x] `GET /api/v1/findings/{id}/notes` → `200 OK` (verify không bị regression)
- [x] `go build ./...` không lỗi
