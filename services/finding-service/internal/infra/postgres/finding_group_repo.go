// Package postgres provides the PostgreSQL implementation for FindingGroupRepository.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/osv/finding-service/internal/domain/group"
)

// FindingGroupRepo implements group.Repository backed by PostgreSQL.
type FindingGroupRepo struct {
	pool *pgxpool.Pool
}

// NewFindingGroupRepo creates a new FindingGroupRepo.
func NewFindingGroupRepo(pool *pgxpool.Pool) group.Repository {
	return &FindingGroupRepo{pool: pool}
}

// Save inserts or updates a FindingGroup.
func (r *FindingGroupRepo) Save(ctx context.Context, g *group.FindingGroup) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO finding_groups (id, name, product_id, jira_issue_key, finding_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			name=EXCLUDED.name, jira_issue_key=EXCLUDED.jira_issue_key,
			finding_count=EXCLUDED.finding_count, updated_at=EXCLUDED.updated_at`,
		g.ID, g.Name, g.ProductID, g.JIRAIssueKey, g.FindingCount,
		g.CreatedAt, g.UpdatedAt,
	)
	return err
}

// FindByID retrieves a FindingGroup by ID.
func (r *FindingGroupRepo) FindByID(ctx context.Context, id uuid.UUID) (*group.FindingGroup, error) {
	g := &group.FindingGroup{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, product_id, jira_issue_key, finding_count, created_at, updated_at
		FROM finding_groups WHERE id = $1`, id).Scan(
		&g.ID, &g.Name, &g.ProductID, &g.JIRAIssueKey, &g.FindingCount,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return g, err
}

// ListByProduct returns all FindingGroups for a product.
func (r *FindingGroupRepo) ListByProduct(ctx context.Context, productID uuid.UUID) ([]*group.FindingGroup, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, product_id, jira_issue_key, finding_count, created_at, updated_at
		FROM finding_groups WHERE product_id = $1 ORDER BY created_at DESC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*group.FindingGroup
	for rows.Next() {
		g := &group.FindingGroup{}
		if err := rows.Scan(
			&g.ID, &g.Name, &g.ProductID, &g.JIRAIssueKey, &g.FindingCount,
			&g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// Delete removes a FindingGroup by ID.
func (r *FindingGroupRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM finding_groups WHERE id = $1`, id)
	return err
}

// IncrementCount atomically adjusts the finding_count by delta.
func (r *FindingGroupRepo) IncrementCount(ctx context.Context, groupID uuid.UUID, delta int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE finding_groups
		SET finding_count = finding_count + $1, updated_at = $2
		WHERE id = $3`, delta, time.Now().UTC(), groupID)
	return err
}
