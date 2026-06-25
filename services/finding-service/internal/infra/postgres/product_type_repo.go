// Package postgres provides PostgreSQL implementations for finding-service repositories.
// This file contains: ProductTypeRepo, EngagementRepo, TestRepo
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/osv/finding-service/internal/domain/engagement"
	"github.com/osv/finding-service/internal/domain/product_type"
	"github.com/osv/finding-service/internal/domain/repository"
	testp "github.com/osv/finding-service/internal/domain/test"
)

// ─── ProductTypeRepo ──────────────────────────────────────────────────────────

type ProductTypeRepo struct {
	pool *pgxpool.Pool
}

func NewProductTypeRepo(pool *pgxpool.Pool) repository.ProductTypeRepository {
	return &ProductTypeRepo{pool: pool}
}

func (r *ProductTypeRepo) Create(ctx context.Context, pt *product_type.ProductType) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO product_types (id, name, description, critical_product, key_product,
			enable_full_risk_acceptance, enable_simple_risk_acceptance, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO NOTHING`,
		pt.ID, pt.Name, pt.Description, pt.CriticalProduct, pt.KeyProduct,
		pt.EnableFullRiskAcceptance, pt.EnableSimpleRiskAcceptance,
		pt.Tags, pt.CreatedAt, pt.UpdatedAt,
	)
	return err
}

func (r *ProductTypeRepo) FindByID(ctx context.Context, id uuid.UUID) (*product_type.ProductType, error) {
	pt := &product_type.ProductType{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, description, critical_product, key_product,
			enable_full_risk_acceptance, enable_simple_risk_acceptance, tags, created_at, updated_at
		FROM product_types WHERE id = $1`, id).Scan(
		&pt.ID, &pt.Name, &pt.Description, &pt.CriticalProduct, &pt.KeyProduct,
		&pt.EnableFullRiskAcceptance, &pt.EnableSimpleRiskAcceptance,
		&pt.Tags, &pt.CreatedAt, &pt.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return pt, err
}

func (r *ProductTypeRepo) FindByName(ctx context.Context, name string) (*product_type.ProductType, error) {
	pt := &product_type.ProductType{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, description, critical_product, key_product,
			enable_full_risk_acceptance, enable_simple_risk_acceptance, tags, created_at, updated_at
		FROM product_types WHERE name = $1`, name).Scan(
		&pt.ID, &pt.Name, &pt.Description, &pt.CriticalProduct, &pt.KeyProduct,
		&pt.EnableFullRiskAcceptance, &pt.EnableSimpleRiskAcceptance,
		&pt.Tags, &pt.CreatedAt, &pt.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return pt, err
}

// ─── EngagementRepo ───────────────────────────────────────────────────────────

type EngagementRepo struct {
	pool *pgxpool.Pool
}

func NewEngagementRepo(pool *pgxpool.Pool) repository.EngagementRepository {
	return &EngagementRepo{pool: pool}
}

const engagementColumns = `
	id, product_id, name, description, engagement_type, status,
	start_date, end_date, lead_id, build_id, commit_hash, branch_tag,
	source_code_management_uri, deduplication_on_engagement, tags,
	created_at, updated_at`

func scanEngagement(row pgx.Row) (*engagement.Engagement, error) {
	e := &engagement.Engagement{}
	var engType, status string
	err := row.Scan(
		&e.ID, &e.ProductID, &e.Name, &e.Description,
		&engType, &status,
		&e.StartDate, &e.EndDate, &e.LeadID,
		&e.BuildID, &e.CommitHash, &e.BranchTag,
		&e.SourceCodeManagementURI, &e.DeduplicationOnEngagement,
		&e.Tags, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	e.EngagementType = engagement.Type(engType)
	e.Status = engagement.Status(status)
	return e, nil
}

func (r *EngagementRepo) Create(ctx context.Context, eng *engagement.Engagement) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO engagements (id, product_id, name, description, engagement_type, status,
			start_date, end_date, lead_id, build_id, commit_hash, branch_tag,
			source_code_management_uri, deduplication_on_engagement, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		eng.ID, eng.ProductID, eng.Name, eng.Description,
		string(eng.EngagementType), string(eng.Status),
		eng.StartDate, eng.EndDate, eng.LeadID,
		eng.BuildID, eng.CommitHash, eng.BranchTag,
		eng.SourceCodeManagementURI, eng.DeduplicationOnEngagement,
		eng.Tags, eng.CreatedAt, eng.UpdatedAt,
	)
	return err
}

func (r *EngagementRepo) FindByID(ctx context.Context, id uuid.UUID) (*engagement.Engagement, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+engagementColumns+` FROM engagements WHERE id = $1`, id)
	e, err := scanEngagement(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return e, err
}

func (r *EngagementRepo) FindByNameAndProduct(ctx context.Context, name string, productID uuid.UUID) (*engagement.Engagement, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+engagementColumns+` FROM engagements WHERE name = $1 AND product_id = $2`, name, productID)
	e, err := scanEngagement(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return e, err
}

func (r *EngagementRepo) Update(ctx context.Context, eng *engagement.Engagement) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE engagements SET
			name=$1, description=$2, engagement_type=$3, status=$4,
			start_date=$5, end_date=$6, lead_id=$7, build_id=$8, commit_hash=$9,
			branch_tag=$10, source_code_management_uri=$11, deduplication_on_engagement=$12,
			tags=$13, updated_at=$14
		WHERE id=$15`,
		eng.Name, eng.Description, string(eng.EngagementType), string(eng.Status),
		eng.StartDate, eng.EndDate, eng.LeadID, eng.BuildID, eng.CommitHash,
		eng.BranchTag, eng.SourceCodeManagementURI, eng.DeduplicationOnEngagement,
		eng.Tags, time.Now().UTC(), eng.ID,
	)
	return err
}

func (r *EngagementRepo) ListByProduct(ctx context.Context, productID uuid.UUID) ([]*engagement.Engagement, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+engagementColumns+` FROM engagements WHERE product_id = $1 ORDER BY created_at DESC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var engagements []*engagement.Engagement
	for rows.Next() {
		e, err := scanEngagement(rows)
		if err != nil {
			return nil, err
		}
		engagements = append(engagements, e)
	}
	return engagements, rows.Err()
}

func (r *EngagementRepo) ListExpiredOpen(ctx context.Context, today time.Time) ([]*engagement.Engagement, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+engagementColumns+` FROM engagements
		WHERE status IN ('In Progress', 'Not Started')
		AND end_date IS NOT NULL AND end_date < $1
		ORDER BY end_date ASC`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var engagements []*engagement.Engagement
	for rows.Next() {
		e, err := scanEngagement(rows)
		if err != nil {
			return nil, err
		}
		engagements = append(engagements, e)
	}
	return engagements, rows.Err()
}

// ─── TestRepo ─────────────────────────────────────────────────────────────────

type TestRepo struct {
	pool *pgxpool.Pool
}

func NewTestRepo(pool *pgxpool.Pool) repository.TestRepository {
	return &TestRepo{pool: pool}
}

const testColumns = `
	id, engagement_id, scan_type, title, description, target_start, target_end,
	lead_id, version, build_id, commit_hash, branch_tag, percent_complete, tags,
	created_at, updated_at`

func scanTest(row pgx.Row) (*testp.Test, error) {
	t := &testp.Test{}
	err := row.Scan(
		&t.ID, &t.EngagementID, &t.ScanType, &t.Title, &t.Description,
		&t.TargetStart, &t.TargetEnd, &t.LeadID,
		&t.Version, &t.BuildID, &t.CommitHash, &t.BranchTag,
		&t.PercentComplete, &t.Tags, &t.CreatedAt, &t.UpdatedAt,
	)
	return t, err
}

func (r *TestRepo) Create(ctx context.Context, t *testp.Test) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tests (id, engagement_id, scan_type, title, description, target_start, target_end,
			lead_id, version, build_id, commit_hash, branch_tag, percent_complete, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		t.ID, t.EngagementID, t.ScanType, t.Title, t.Description,
		t.TargetStart, t.TargetEnd, t.LeadID,
		t.Version, t.BuildID, t.CommitHash, t.BranchTag,
		t.PercentComplete, t.Tags, t.CreatedAt, t.UpdatedAt,
	)
	return err
}

func (r *TestRepo) FindByID(ctx context.Context, id uuid.UUID) (*testp.Test, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+testColumns+` FROM tests WHERE id = $1`, id)
	t, err := scanTest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

func (r *TestRepo) ListByEngagement(ctx context.Context, engagementID uuid.UUID) ([]*testp.Test, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+testColumns+` FROM tests WHERE engagement_id = $1 ORDER BY created_at DESC`, engagementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []*testp.Test
	for rows.Next() {
		t, err := scanTest(rows)
		if err != nil {
			return nil, err
		}
		tests = append(tests, t)
	}
	return tests, rows.Err()
}

func (r *TestRepo) FindByEngagementAndType(ctx context.Context, engagementID uuid.UUID, scanType string) (*testp.Test, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+testColumns+` FROM tests
		WHERE engagement_id = $1 AND scan_type = $2
		ORDER BY created_at DESC LIMIT 1`, engagementID, scanType)
	t, err := scanTest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

func (r *TestRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM tests WHERE id = $1`, id)
	return err
}
