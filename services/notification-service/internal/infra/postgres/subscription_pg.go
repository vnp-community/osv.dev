package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/notification-service/internal/domain/repository"
	entity "github.com/osv/notification-service/internal/domain/subscription"
)

type pgSubscriptionRepository struct{ db *pgxpool.Pool }

func NewSubscriptionRepository(db *pgxpool.Pool) repository.SubscriptionRepository {
	return &pgSubscriptionRepository{db: db}
}

func (r *pgSubscriptionRepository) Save(ctx context.Context, s *entity.AlertSubscription) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO alert_subscriptions (id, owner_id, type, value, min_severity, min_epss, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, s.ID, s.OwnerID, string(s.Type), s.Value, s.MinSeverity, s.MinEPSS, s.IsActive, s.CreatedAt)
	return err
}

func (r *pgSubscriptionRepository) FindByOwner(ctx context.Context, ownerID string) ([]*entity.AlertSubscription, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, owner_id, type, value, min_severity, min_epss, is_active, created_at
		FROM alert_subscriptions
		WHERE owner_id = $1 AND is_active = TRUE
	`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*entity.AlertSubscription
	for rows.Next() {
		var s entity.AlertSubscription
		var typ string
		if err := rows.Scan(
			&s.ID, &s.OwnerID, &typ, &s.Value, &s.MinSeverity, &s.MinEPSS, &s.IsActive, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		s.Type = entity.SubscriptionType(typ)
		subs = append(subs, &s)
	}
	return subs, rows.Err()
}

func (r *pgSubscriptionRepository) FindByVendor(ctx context.Context, vendor string) ([]*entity.AlertSubscription, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, owner_id, type, value, min_severity, min_epss, is_active, created_at
		FROM alert_subscriptions
		WHERE type = 'vendor' AND lower(value) = lower($1) AND is_active = TRUE
	`, vendor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*entity.AlertSubscription
	for rows.Next() {
		var s entity.AlertSubscription
		var typ string
		if err := rows.Scan(
			&s.ID, &s.OwnerID, &typ, &s.Value, &s.MinSeverity, &s.MinEPSS, &s.IsActive, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		s.Type = entity.SubscriptionType(typ)
		subs = append(subs, &s)
	}
	return subs, rows.Err()
}

func (r *pgSubscriptionRepository) Delete(ctx context.Context, id, ownerID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM alert_subscriptions WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
