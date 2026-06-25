// Package postgres implements AuditRepository using PostgreSQL.
// Schema: audit_events (id, event_type, entity_type, entity_id, actor_id, old_state, new_state, occurred_at, created_at)
package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	httpdelivery "github.com/osv/audit-service/internal/delivery/http"
)

// AuditRepo implements httpdelivery.AuditRepository via pgxpool.
type AuditRepo struct {
	db *pgxpool.Pool
}

// NewAuditRepo creates a new AuditRepo.
func NewAuditRepo(db *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{db: db}
}

var _ httpdelivery.AuditRepository = (*AuditRepo)(nil)

// List returns paginated audit events with optional filters.
// Maps actual DB columns to the AuditEvent domain struct.
func (r *AuditRepo) List(ctx context.Context, filter httpdelivery.AuditFilter, page, pageSize int) ([]*httpdelivery.AuditEvent, int64, error) {
	var conds []string
	var args []interface{}
	idx := 1

	// Filter by actor_id (maps to UserID in filter)
	if filter.UserID != "" {
		conds = append(conds, fmt.Sprintf("actor_id = $%d", idx))
		args = append(args, filter.UserID)
		idx++
	}
	// Filter by event_type (maps to Action in filter)
	if filter.Action != "" {
		conds = append(conds, fmt.Sprintf("event_type ILIKE $%d", idx))
		args = append(args, "%"+filter.Action+"%")
		idx++
	}
	// Filter by entity_type
	if filter.EntityType != "" {
		conds = append(conds, fmt.Sprintf("entity_type = $%d", idx))
		args = append(args, filter.EntityType)
		idx++
	}
	// Filter by entity_id
	if filter.EntityID != "" {
		conds = append(conds, fmt.Sprintf("entity_id = $%d", idx))
		args = append(args, filter.EntityID)
		idx++
	}
	// Date range filters
	if filter.DateFrom != nil {
		conds = append(conds, fmt.Sprintf("occurred_at >= $%d", idx))
		args = append(args, filter.DateFrom)
		idx++
	}
	if filter.DateTo != nil {
		conds = append(conds, fmt.Sprintf("occurred_at <= $%d", idx))
		args = append(args, filter.DateTo)
		idx++
	}
	// Full-text search across event_type and entity_type
	if filter.Search != "" {
		conds = append(conds, fmt.Sprintf(
			"(event_type ILIKE $%d OR entity_type ILIKE $%d OR actor_id::text ILIKE $%d)",
			idx, idx, idx,
		))
		args = append(args, "%"+filter.Search+"%")
		idx++
	}

	where := "WHERE 1=1"
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	// Count query
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_events %s", where)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit_repo: count: %w", err)
	}

	// Data query — map actual columns to AuditEvent fields
	dataQuery := fmt.Sprintf(`
		SELECT
			id::text,
			COALESCE(actor_id, '') AS user_id,
			COALESCE(actor_id, '') AS user_email,
			event_type AS action,
			entity_type,
			COALESCE(entity_id, '') AS entity_id,
			old_state AS old_value,
			new_state AS new_value,
			'' AS ip_address,
			'' AS user_agent,
			occurred_at
		FROM audit_events %s
		ORDER BY occurred_at DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)

	pageArgs := append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(ctx, dataQuery, pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("audit_repo: list: %w", err)
	}
	defer rows.Close()

	var events []*httpdelivery.AuditEvent
	for rows.Next() {
		e := &httpdelivery.AuditEvent{}
		var oldValue, newValue interface{}
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.UserEmail, &e.Action,
			&e.EntityType, &e.EntityID,
			&oldValue, &newValue,
			&e.IPAddress, &e.UserAgent,
			&e.CreatedAt,
		); err != nil {
			continue
		}
		e.OldValue = oldValue
		e.NewValue = newValue
		events = append(events, e)
	}
	return events, total, rows.Err()
}
