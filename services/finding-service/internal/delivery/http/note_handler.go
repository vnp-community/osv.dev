package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/finding"
)

type NoteRepository interface {
	Create(ctx context.Context, note *FindingNote) error
	ListByFinding(ctx context.Context, findingID uuid.UUID) ([]*FindingNote, error)
}

type NoteHandler struct {
	findingRepo finding.Repository
	noteRepo    NoteRepository
}

func NewNoteHandler(findingRepo finding.Repository, noteRepo NoteRepository) *NoteHandler {
	return &NoteHandler{findingRepo: findingRepo, noteRepo: noteRepo}
}

// POST /v2/findings/{id}/notes
func (h *NoteHandler) AddNote(w http.ResponseWriter, r *http.Request) {
	findingID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, 400, "Invalid finding ID")
		return
	}

	var req struct {
		Content string `json:"content"`
		Note    string `json:"note"` // alias used by seed scripts
		Private bool   `json:"private"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "Invalid request body")
		return
	}
	// Accept "note" as alias for "content"
	if req.Content == "" {
		req.Content = req.Note
	}
	if req.Content == "" {
		respondError(w, 400, "content is required")
		return
	}

	// Verify finding exists
	if _, err := h.findingRepo.FindByID(r.Context(), findingID); err != nil {
		respondError(w, 404, "Finding not found")
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
		Private:   req.Private,
		CreatedBy: userEmail,
		CreatedAt: time.Now(),
	}

	if err := h.noteRepo.Create(r.Context(), note); err != nil {
		respondError(w, 500, "Failed to add note")
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
	findingID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, 400, "Invalid finding ID")
		return
	}

	notes, err := h.noteRepo.ListByFinding(r.Context(), findingID)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}

	respondJSON(w, 200, map[string]interface{}{"notes": notes})
}

// Domain type
type FindingNote struct {
	ID        uuid.UUID `json:"id"`
	FindingID uuid.UUID `json:"finding_id"`
	Content   string    `json:"content"`
	Private   bool      `json:"private"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}
