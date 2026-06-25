// Package postgres — search_history_repo.go
// TASK-HC-007: PostgreSQL implementation of SearchHistoryRepository.
// Persists per-user search queries for the /api/v1/search/recent endpoint.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SearchHistoryEntry represents a single search record.
type SearchHistoryEntry struct {
	ID          uuid.UUID `json:"id"           db:"id"`
	UserID      uuid.UUID `json:"user_id"      db:"user_id"`
	Query       string    `json:"query"        db:"query"`
	ResultCount int       `json:"result_count" db:"result_count"`
	SearchType  string    `json:"search_type"  db:"search_type"`
	CreatedAt   time.Time `json:"created_at"   db:"created_at"`
}

// SearchHistoryRepository provides persistence for user search history.
type SearchHistoryRepository interface {
	Save(ctx context.Context, e *SearchHistoryEntry) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*SearchHistoryEntry, error)
}

// SearchHistoryRepo is the PostgreSQL implementation.
type SearchHistoryRepo struct {
	pool *pgxpool.Pool
}

// NewSearchHistoryRepo creates a SearchHistoryRepo.
// [FIX TASK-HC-007] Production implementation — replaces in-memory stub.
func NewSearchHistoryRepo(pool *pgxpool.Pool) *SearchHistoryRepo {
	return &SearchHistoryRepo{pool: pool}
}

// Save inserts a search history entry.
func (r *SearchHistoryRepo) Save(ctx context.Context, e *SearchHistoryEntry) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO search_history (user_id, query, result_count, search_type)
		VALUES ($1, $2, $3, $4)
	`, e.UserID, e.Query, e.ResultCount, e.SearchType)
	if err != nil {
		return fmt.Errorf("search_history.Save: %w", err)
	}
	return nil
}

// ListByUser returns the most recent searches for a user, newest first.
func (r *SearchHistoryRepo) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*SearchHistoryEntry, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, query, result_count, search_type, created_at
		FROM search_history
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("search_history.ListByUser: %w", err)
	}
	defer rows.Close()

	var entries []*SearchHistoryEntry
	for rows.Next() {
		e := &SearchHistoryEntry{}
		if err := rows.Scan(&e.ID, &e.UserID, &e.Query, &e.ResultCount, &e.SearchType, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("search_history.ListByUser scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
