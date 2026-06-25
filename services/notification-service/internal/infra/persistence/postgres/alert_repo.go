// Package postgres — alert_repo.go
// PostgresAlertRepo implements alert.Repository using PostgreSQL.
//
// Stores in-app alerts in the `inapp_alerts` table (migration 005).
// Firestore repos are PRESERVED — this is purely additive.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/osv/notification-service/internal/domain/alert"
)

// PostgresAlertRepo implements alert.Repository using pgxpool.
type PostgresAlertRepo struct {
	db  *pgxpool.Pool
	log zerolog.Logger
}

// NewAlertRepo creates a new PostgresAlertRepo.
func NewAlertRepo(db *pgxpool.Pool, log zerolog.Logger) *PostgresAlertRepo {
	return &PostgresAlertRepo{db: db, log: log}
}

// Compile-time check: PostgresAlertRepo must satisfy alert.Repository.
var _ alert.Repository = (*PostgresAlertRepo)(nil)

// ── alert.Repository implementation ─────────────────────────────────────────

// Create persists a new in-app alert.
func (r *PostgresAlertRepo) Create(ctx context.Context, a *alert.Alert) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO inapp_alerts (user_id, event_type, title, description, url, is_read)
		 VALUES ($1, $2, $3, $4, $5, FALSE)
		 RETURNING id, created_at`,
		a.UserID, a.EventType, a.Title, a.Description, a.URL,
	).Scan(&a.ID, &a.CreatedAt)
}

// ListForUser returns paginated alerts for a user, optionally filtered to unread only.
// Returns (alerts, totalCount, error).
func (r *PostgresAlertRepo) ListForUser(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit, offset int) ([]*alert.Alert, int, error) {
	// Count query
	countQuery := `SELECT COUNT(*) FROM inapp_alerts WHERE user_id = $1`
	args := []interface{}{userID}
	if unreadOnly {
		countQuery += ` AND is_read = FALSE`
	}

	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("alert_repo: count: %w", err)
	}

	// Data query
	dataQuery := `
		SELECT id, user_id, event_type, title, description, url, is_read, created_at
		FROM inapp_alerts
		WHERE user_id = $1`
	if unreadOnly {
		dataQuery += ` AND is_read = FALSE`
	}
	dataQuery += ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, dataQuery, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("alert_repo: list: %w", err)
	}
	defer rows.Close()

	var alerts []*alert.Alert
	for rows.Next() {
		a := &alert.Alert{}
		if err := rows.Scan(&a.ID, &a.UserID, &a.EventType, &a.Title, &a.Description, &a.URL, &a.IsRead, &a.CreatedAt); err != nil {
			r.log.Warn().Err(err).Msg("alert_repo: skip row on scan error")
			continue
		}
		alerts = append(alerts, a)
	}
	return alerts, total, rows.Err()
}

// CountUnread returns the number of unread alerts for a user.
func (r *PostgresAlertRepo) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM inapp_alerts WHERE user_id = $1 AND is_read = FALSE`,
		userID,
	).Scan(&count)
	return count, err
}

// MarkRead marks a single alert as read (guarded by userID for security).
func (r *PostgresAlertRepo) MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE inapp_alerts SET is_read = TRUE, read_at = NOW() WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}

// MarkAllRead marks all unread alerts for a user as read.
func (r *PostgresAlertRepo) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE inapp_alerts SET is_read = TRUE, read_at = NOW() WHERE user_id = $1 AND is_read = FALSE`,
		userID,
	)
	return err
}

// Delete removes an alert (guarded by userID for security)
func (r *PostgresAlertRepo) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM inapp_alerts WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}

// AlertRow is used internally for HTTP-layer scanning.
type AlertRow struct {
	ID         string
	Type       string
	Title      string
	Message    string
	Severity   string
	IsRead     bool
	EntityType string
	EntityID   string
	CreatedAt  time.Time
	ReadAt     *time.Time
}

// ListAlertsByUser returns alert rows for the HTTP layer (avoids import cycle).
func (r *PostgresAlertRepo) ListAlertsByUser(ctx context.Context, userID, isRead string, limit, offset int) ([]*AlertRow, int, int, error) {
	var total, unread int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE is_read = false)
		FROM inapp_alerts WHERE user_id = $1`, userID).Scan(&total, &unread)
	if err != nil {
		return nil, 0, 0, err
	}

	q := `SELECT id, type, title, message, severity, is_read, entity_type, entity_id, created_at, read_at
		  FROM inapp_alerts
		  WHERE user_id = $1`
	var args []interface{}
	args = append(args, userID)

	if isRead != "" {
		q += ` AND is_read = $2`
		args = append(args, isRead == "true")
	}

	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT %d OFFSET %d`, limit, offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	var alerts []*AlertRow
	for rows.Next() {
		a := &AlertRow{}
		var readAt *time.Time
		if err := rows.Scan(&a.ID, &a.Type, &a.Title, &a.Message, &a.Severity, &a.IsRead, &a.EntityType, &a.EntityID, &a.CreatedAt, &readAt); err != nil {
			continue
		}
		a.ReadAt = readAt
		alerts = append(alerts, a)
	}

	return alerts, total, unread, nil
}

// countUnreadByStringID — string-based variant for the HTTP layer.
func (r *PostgresAlertRepo) CountUnreadByStringID(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM inapp_alerts WHERE user_id = $1 AND is_read = FALSE`,
		userID,
	).Scan(&count)
	return count, err
}

// markReadByStringIDs — marks a single alert read using string IDs.
func (r *PostgresAlertRepo) MarkReadByStringIDs(ctx context.Context, alertID, userID string) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE inapp_alerts SET is_read = TRUE, read_at = NOW() WHERE id = $1 AND user_id = $2`,
		alertID, userID,
	)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("alert not found or unauthorized")
	}
	return nil
}

// markAllReadByStringID — marks all alerts read for a user, returning the count updated.
func (r *PostgresAlertRepo) MarkAllReadByStringID(ctx context.Context, userID string) (int, error) {
	cmd, err := r.db.Exec(ctx,
		`UPDATE inapp_alerts SET is_read = TRUE, read_at = NOW() WHERE user_id = $1 AND is_read = FALSE`,
		userID,
	)
	if err != nil {
		return 0, err
	}
	return int(cmd.RowsAffected()), nil
}
