package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type NotifPreference struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

type NotifPrefRepository interface {
	GetPreferences(ctx context.Context, userID string) ([]NotifPreference, error)
	UpdatePreferences(ctx context.Context, userID string, updates []struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}) ([]NotifPreference, error)
}

type PostgresNotifPrefRepo struct {
	db *pgxpool.Pool
}

func NewPostgresNotifPrefRepo(db *pgxpool.Pool) *PostgresNotifPrefRepo {
	return &PostgresNotifPrefRepo{db: db}
}

func (r *PostgresNotifPrefRepo) GetPreferences(ctx context.Context, userID string) ([]NotifPreference, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id::text, event_type as id, label, description, enabled
        FROM auth.notification_preferences
        WHERE user_id = $1
        ORDER BY label
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefs []NotifPreference
	for rows.Next() {
		var p NotifPreference
		var rawID string
		// the query aliases event_type as id, we map the text cast of id to rawID and use event_type (id) to NotifPreference.ID
		if err := rows.Scan(&rawID, &p.ID, &p.Label, &p.Description, &p.Enabled); err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}
	return prefs, nil
}

func (r *PostgresNotifPrefRepo) UpdatePreferences(ctx context.Context, userID string, updates []struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}) ([]NotifPreference, error) {
	// Simple update in loop since it's just a few preferences per user
	for _, up := range updates {
		_, err := r.db.Exec(ctx, `
			UPDATE auth.notification_preferences 
			SET enabled = $3, updated_at = NOW() 
			WHERE user_id = $1 AND event_type = $2`,
			userID, up.ID, up.Enabled)
		if err != nil {
			return nil, err
		}
	}
	return r.GetPreferences(ctx, userID)
}
