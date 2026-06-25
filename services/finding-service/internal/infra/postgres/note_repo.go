package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/finding-service/internal/delivery/http"
)

type NoteRepo struct {
	pool *pgxpool.Pool
}

func NewNoteRepo(pool *pgxpool.Pool) *NoteRepo {
	return &NoteRepo{pool: pool}
}

func (r *NoteRepo) Create(ctx context.Context, note *http.FindingNote) error {
	// Parse author_id as UUID (fallback to nil uuid if not parseable)
	authorID, err := uuid.Parse(note.CreatedBy)
	if err != nil {
		authorID = uuid.Nil
	}
	now := time.Now().UTC()
	_, err = r.pool.Exec(ctx,
		`INSERT INTO finding_notes (id, finding_id, note, author_id, private, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		note.ID, note.FindingID, note.Content, authorID, note.Private, now, now)
	return err
}

func (r *NoteRepo) ListByFinding(ctx context.Context, findingID uuid.UUID) ([]*http.FindingNote, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, finding_id, note, author_id::text, private, created_at
		 FROM finding_notes WHERE finding_id = $1
		 ORDER BY created_at DESC`,
		findingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*http.FindingNote
	for rows.Next() {
		n := &http.FindingNote{}
		if err := rows.Scan(&n.ID, &n.FindingID, &n.Content, &n.CreatedBy, &n.Private, &n.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, rows.Err()
}
