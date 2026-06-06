package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/defectdojo/product-management/internal/domain/engagement"
)

// EngagementRepo implements repository.EngagementRepository.
type EngagementRepo struct {
	pool *pgxpool.Pool
}

func NewEngagementRepo(pool *pgxpool.Pool) *EngagementRepo {
	return &EngagementRepo{pool: pool}
}

func (r *EngagementRepo) Create(ctx context.Context, e *engagement.Engagement) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO engagements
			(id, product_id, name, description, lead_id, engagement_type, status,
			 start_date, version, build_id, commit_hash, branch_tag,
			 source_code_management_uri, deduplication_on_engagement, tags, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		e.ID, e.ProductID, e.Name, e.Description, e.LeadID, e.EngagementType, e.Status,
		e.StartDate, e.Version, e.BuildID, e.CommitHash, e.BranchTag,
		e.SourceCodeManagementURI, e.DeduplicationOnEngagement, e.Tags, e.CreatedAt, e.UpdatedAt,
	)
	return err
}

func (r *EngagementRepo) FindByID(ctx context.Context, id uuid.UUID) (*engagement.Engagement, error) {
	e := &engagement.Engagement{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, product_id, name, description, lead_id, engagement_type, status,
			start_date, end_date, version, build_id, commit_hash, branch_tag,
			deduplication_on_engagement, tags, created_at, updated_at
		FROM engagements WHERE id = $1`, id).Scan(
		&e.ID, &e.ProductID, &e.Name, &e.Description, &e.LeadID, &e.EngagementType, &e.Status,
		&e.StartDate, &e.EndDate, &e.Version, &e.BuildID, &e.CommitHash, &e.BranchTag,
		&e.DeduplicationOnEngagement, &e.Tags, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("engagement not found: %w", err)
	}
	return e, nil
}

func (r *EngagementRepo) FindByNameAndProduct(ctx context.Context, name string, productID uuid.UUID) (*engagement.Engagement, error) {
	e := &engagement.Engagement{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, product_id, name, description, engagement_type, status,
			start_date, deduplication_on_engagement, tags, created_at, updated_at
		FROM engagements WHERE name = $1 AND product_id = $2`, name, productID).Scan(
		&e.ID, &e.ProductID, &e.Name, &e.Description, &e.EngagementType, &e.Status,
		&e.StartDate, &e.DeduplicationOnEngagement, &e.Tags, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("engagement not found: %w", err)
	}
	return e, nil
}

func (r *EngagementRepo) List(ctx context.Context, productID uuid.UUID, limit, offset int) ([]*engagement.Engagement, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM engagements WHERE product_id=$1`, productID).Scan(&total)

	rows, err := r.pool.Query(ctx, `
		SELECT id, product_id, name, status, engagement_type, start_date, created_at, updated_at
		FROM engagements WHERE product_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		productID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var engs []*engagement.Engagement
	for rows.Next() {
		e := &engagement.Engagement{}
		if err := rows.Scan(&e.ID, &e.ProductID, &e.Name, &e.Status, &e.EngagementType, &e.StartDate, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, 0, err
		}
		engs = append(engs, e)
	}
	return engs, total, rows.Err()
}

func (r *EngagementRepo) Update(ctx context.Context, e *engagement.Engagement) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE engagements SET
			name=$2, description=$3, status=$4, end_date=$5,
			version=$6, build_id=$7, commit_hash=$8, branch_tag=$9,
			updated_at=NOW()
		WHERE id=$1`,
		e.ID, e.Name, e.Description, e.Status, e.EndDate,
		e.Version, e.BuildID, e.CommitHash, e.BranchTag,
	)
	return err
}
