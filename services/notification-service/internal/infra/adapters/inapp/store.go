// Package inapp — store.go
// InAppStore manages in-app notifications in PostgreSQL.
// S3-NOTIF-01: additive, existing Firestore/SMTP channels unchanged.
package inapp

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Notification is an in-app notification record.
type Notification struct {
	ID        uuid.UUID              `json:"id"`
	UserID    uuid.UUID              `json:"user_id"`
	EventType string                 `json:"event_type"`
	Title     string                 `json:"title"`
	Body      string                 `json:"body,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	IsRead    bool                   `json:"is_read"`
	ReadAt    *time.Time             `json:"read_at,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// Store manages in-app notification persistence.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a new InApp Store.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Create inserts a new in-app notification.
func (s *Store) Create(ctx context.Context, n *Notification) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO inapp_notifications (id, user_id, event_type, title, body, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, n.ID, n.UserID, n.EventType, n.Title, n.Body, n.Payload)
	return err
}

// GetUnread returns unread notifications for a user created after lastSeenID.
// Pass uuid.Nil for lastSeenID to get all unread.
func (s *Store) GetUnread(ctx context.Context, userID uuid.UUID, limit int) ([]*Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, event_type, title, body, is_read, created_at
		FROM inapp_notifications
		WHERE user_id = $1 AND is_read = FALSE
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("inapp.GetUnread: %w", err)
	}
	defer rows.Close()

	var results []*Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.EventType, &n.Title, &n.Body, &n.IsRead, &n.CreatedAt); err != nil {
			continue
		}
		results = append(results, &n)
	}
	return results, rows.Err()
}

// MarkRead marks a single notification as read.
func (s *Store) MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	now := time.Now().UTC()
	tag, err := s.db.Exec(ctx, `
		UPDATE inapp_notifications
		SET is_read = TRUE, read_at = $3
		WHERE id = $1 AND user_id = $2 AND is_read = FALSE
	`, id, userID, now)
	if err != nil {
		return fmt.Errorf("inapp.MarkRead: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("inapp.MarkRead: not found or already read")
	}
	return nil
}

// MarkAllRead marks all unread notifications for a user as read.
func (s *Store) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	now := time.Now().UTC()
	tag, err := s.db.Exec(ctx, `
		UPDATE inapp_notifications
		SET is_read = TRUE, read_at = $2
		WHERE user_id = $1 AND is_read = FALSE
	`, userID, now)
	if err != nil {
		return 0, fmt.Errorf("inapp.MarkAllRead: %w", err)
	}
	return tag.RowsAffected(), nil
}
