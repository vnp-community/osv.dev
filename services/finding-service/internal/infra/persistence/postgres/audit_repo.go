// Package postgres provides the PostgreSQL repository for audit events.
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	auditdomain "github.com/defectdojo/finding-service/internal/domain/audit"
)

// AuditRepo implements auditdomain.Repository using PostgreSQL (partitioned table).
type AuditRepo struct {
	pool *pgxpool.Pool
}

// NewAuditRepo creates a new AuditRepo.
func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

// Save persists an AuditEvent to the audit_events partitioned table.
func (r *AuditRepo) Save(ctx context.Context, e *auditdomain.AuditEvent) error {
	const q = `
		INSERT INTO audit_events
			(id, event_type, entity_type, entity_id, actor_id, old_state, new_state, occurred_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id, occurred_at) DO NOTHING`

	_, err := r.pool.Exec(ctx, q,
		e.ID, e.EventType, e.EntityType, e.EntityID,
		e.ActorID, e.OldState, e.NewState, e.OccurredAt, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("audit_repo.Save: %w", err)
	}
	return nil
}

// List queries audit events with optional filters, returning a page and total count.
func (r *AuditRepo) List(ctx context.Context, f auditdomain.AuditFilter) ([]*auditdomain.AuditEvent, int, error) {
	// Build dynamic WHERE clause
	where := "WHERE 1=1"
	args := []any{}
	idx := 1

	if f.EntityType != "" {
		where += fmt.Sprintf(" AND entity_type = $%d", idx)
		args = append(args, f.EntityType)
		idx++
	}
	if f.EntityID != "" {
		where += fmt.Sprintf(" AND entity_id = $%d", idx)
		args = append(args, f.EntityID)
		idx++
	}
	if f.ActorID != "" {
		where += fmt.Sprintf(" AND actor_id = $%d", idx)
		args = append(args, f.ActorID)
		idx++
	}
	if f.EventType != "" {
		where += fmt.Sprintf(" AND event_type = $%d", idx)
		args = append(args, f.EventType)
		idx++
	}
	if f.FromTime != nil {
		where += fmt.Sprintf(" AND occurred_at >= $%d", idx)
		args = append(args, *f.FromTime)
		idx++
	}
	if f.ToTime != nil {
		where += fmt.Sprintf(" AND occurred_at <= $%d", idx)
		args = append(args, *f.ToTime)
		idx++
	}

	// Count
	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_events "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("audit_repo.List count: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := f.Offset

	q := fmt.Sprintf(`
		SELECT id, event_type, entity_type, entity_id, actor_id, old_state, new_state, occurred_at, created_at
		FROM audit_events %s
		ORDER BY occurred_at DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("audit_repo.List query: %w", err)
	}
	defer rows.Close()

	var events []*auditdomain.AuditEvent
	for rows.Next() {
		e := &auditdomain.AuditEvent{}
		if err := rows.Scan(
			&e.ID, &e.EventType, &e.EntityType, &e.EntityID,
			&e.ActorID, &e.OldState, &e.NewState, &e.OccurredAt, &e.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("audit_repo.List scan: %w", err)
		}
		events = append(events, e)
	}
	return events, total, nil
}

// FindByID retrieves a single audit event by its UUID.
func (r *AuditRepo) FindByID(ctx context.Context, id uuid.UUID) (*auditdomain.AuditEvent, error) {
	const q = `
		SELECT id, event_type, entity_type, entity_id, actor_id, old_state, new_state, occurred_at, created_at
		FROM audit_events WHERE id = $1 LIMIT 1`

	e := &auditdomain.AuditEvent{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&e.ID, &e.EventType, &e.EntityType, &e.EntityID,
		&e.ActorID, &e.OldState, &e.NewState, &e.OccurredAt, &e.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("audit_repo.FindByID: %w", err)
	}
	return e, nil
}
