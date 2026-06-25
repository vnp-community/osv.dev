// Package alertrepo — adapter.go
// Provides an AlertRepository implementation for the HTTP delivery layer.
// Bridges infra/persistence/postgres to delivery/http.AlertRepository interface.
package alertrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	deliveryhttp "github.com/osv/notification-service/internal/delivery/http"
)

// PostgresAlertAdapter implements delivery/http.AlertRepository using pgxpool directly.
// This avoids the circular dependency between delivery/http and infra/persistence/postgres.
type PostgresAlertAdapter struct {
	db *pgxpool.Pool
}

// New creates a new PostgresAlertAdapter.
func New(db *pgxpool.Pool) *PostgresAlertAdapter {
	return &PostgresAlertAdapter{db: db}
}

// Compile-time check.
var _ deliveryhttp.AlertRepository = (*PostgresAlertAdapter)(nil)

// ListByUser returns paginated alerts for a user.
func (a *PostgresAlertAdapter) ListByUser(ctx context.Context, userID, isRead string, limit, offset int) ([]*deliveryhttp.Alert, int, int, error) {
	var total, unread int
	err := a.db.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE is_read = false)
		FROM inapp_alerts WHERE user_id = $1`, userID).Scan(&total, &unread)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("alert_adapter: count: %w", err)
	}

	q := `SELECT id, COALESCE(type, event_type, ''), title, COALESCE(message, description, ''),
		         COALESCE(severity, 'Info'), is_read,
		         COALESCE(entity_type, ''), COALESCE(entity_id, ''),
		         created_at, read_at
		  FROM inapp_alerts
		  WHERE user_id = $1`
	args := []interface{}{userID}

	if isRead != "" {
		q += ` AND is_read = $2`
		args = append(args, isRead == "true")
	}
	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT %d OFFSET %d`, limit, offset)

	rows, err := a.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("alert_adapter: query: %w", err)
	}
	defer rows.Close()

	var alerts []*deliveryhttp.Alert
	for rows.Next() {
		al := &deliveryhttp.Alert{}
		var idStr string
		var readAt *time.Time
		if err := rows.Scan(&idStr, &al.Type, &al.Title, &al.Message,
			&al.Severity, &al.IsRead, &al.EntityType, &al.EntityID,
			&al.CreatedAt, &readAt); err != nil {
			continue
		}
		al.ReadAt = readAt
		if uid, err := uuid.Parse(idStr); err == nil {
			al.ID = uid
		}
		alerts = append(alerts, al)
	}
	if alerts == nil {
		alerts = []*deliveryhttp.Alert{}
	}
	return alerts, total, unread, nil
}

// CountUnread returns the number of unread alerts for a user.
func (a *PostgresAlertAdapter) CountUnread(ctx context.Context, userID string) (int, error) {
	var count int
	err := a.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM inapp_alerts WHERE user_id = $1 AND is_read = FALSE`,
		userID,
	).Scan(&count)
	return count, err
}

// MarkRead marks a single alert as read.
func (a *PostgresAlertAdapter) MarkRead(ctx context.Context, alertID, userID string) error {
	cmd, err := a.db.Exec(ctx,
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

// MarkAllRead marks all alerts as read for a user, returning the count updated.
func (a *PostgresAlertAdapter) MarkAllRead(ctx context.Context, userID string) (int, error) {
	cmd, err := a.db.Exec(ctx,
		`UPDATE inapp_alerts SET is_read = TRUE, read_at = NOW() WHERE user_id = $1 AND is_read = FALSE`,
		userID,
	)
	if err != nil {
		return 0, err
	}
	return int(cmd.RowsAffected()), nil
}
