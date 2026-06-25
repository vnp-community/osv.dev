// Package finding_usecase — note.go
// AddNoteUseCase and ListNotesUseCase implement finding note management.
package finding_usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/note"
)

// ErrEmptyNote is returned when note content is empty.
var ErrEmptyNote = errors.New("note content is required")

// ─── AddNote ─────────────────────────────────────────────────────────────────

// AddNoteInput is the request for adding a note to a finding.
type AddNoteInput struct {
	FindingID uuid.UUID
	AuthorID  uuid.UUID
	Content   string
	IsPrivate bool
	NoteType  string
}

// AddNoteUseCase adds an analyst note to a finding.
type AddNoteUseCase struct {
	noteRepo note.Repository
}

// NewAddNote creates a new AddNoteUseCase.
func NewAddNote(repo note.Repository) *AddNoteUseCase {
	return &AddNoteUseCase{noteRepo: repo}
}

// Execute creates and saves a new finding note.
func (uc *AddNoteUseCase) Execute(ctx context.Context, in AddNoteInput) (*note.FindingNote, error) {
	if in.Content == "" {
		return nil, ErrEmptyNote
	}
	n := note.New(in.FindingID, in.AuthorID, in.Content, in.IsPrivate)
	n.NoteType = in.NoteType
	if err := uc.noteRepo.Save(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

// ─── ListNotes ────────────────────────────────────────────────────────────────

// ListNotesUseCase lists notes for a finding.
type ListNotesUseCase struct {
	noteRepo note.Repository
}

// NewListNotes creates a new ListNotesUseCase.
func NewListNotes(repo note.Repository) *ListNotesUseCase {
	return &ListNotesUseCase{noteRepo: repo}
}

// Execute returns all non-private notes for a finding.
// If requestorID matches the author, private notes are also returned.
func (uc *ListNotesUseCase) Execute(ctx context.Context, findingID, requestorID uuid.UUID) ([]*note.FindingNote, error) {
	notes, err := uc.noteRepo.ListByFinding(ctx, findingID)
	if err != nil {
		return nil, err
	}
	// Filter private notes: only visible to author
	visible := make([]*note.FindingNote, 0, len(notes))
	for _, n := range notes {
		if !n.IsPrivate || n.AuthorID == requestorID {
			visible = append(visible, n)
		}
	}
	return visible, nil
}
