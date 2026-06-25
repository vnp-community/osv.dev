// Package note defines the FindingNote domain entity for analyst comments on findings.
package note

import (
	"time"

	"github.com/google/uuid"
)

// FindingNote represents an analyst note attached to a finding, engagement, or test.
type FindingNote struct {
	ID        uuid.UUID
	FindingID uuid.UUID  // FK to findings.id
	AuthorID  uuid.UUID  // User who wrote the note
	Content   string
	NoteType  string    // optional type label
	EditCount int
	IsPrivate bool     // private notes only visible to author + managers
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New creates a new FindingNote.
func New(findingID, authorID uuid.UUID, content string, isPrivate bool) *FindingNote {
	now := time.Now().UTC()
	return &FindingNote{
		ID:        uuid.New(),
		FindingID: findingID,
		AuthorID:  authorID,
		Content:   content,
		IsPrivate: isPrivate,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
