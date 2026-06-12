package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	testp "github.com/defectdojo/product-management/internal/domain/test"
)

// TestRepo implements repository.TestRepository.
type TestRepo struct {
	pool *pgxpool.Pool
}

func NewTestRepo(pool *pgxpool.Pool) *TestRepo {
	return &TestRepo{pool: pool}
}

func (r *TestRepo) Create(ctx context.Context, t *testp.Test) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tests
			(id, engagement_id, scan_type, title, description, target_start,
			 version, build_id, commit_hash, branch_tag, tags, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		t.ID, t.EngagementID, t.ScanType, t.Title, t.Description, t.TargetStart,
		t.Version, t.BuildID, t.CommitHash, t.BranchTag, t.Tags, t.CreatedAt, t.UpdatedAt,
	)
	return err
}

func (r *TestRepo) FindByID(ctx context.Context, id uuid.UUID) (*testp.Test, error) {
	t := &testp.Test{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, engagement_id, scan_type, title, description,
			target_start, target_end, version, build_id, commit_hash, branch_tag,
			tags, created_at, updated_at
		FROM tests WHERE id=$1`, id).Scan(
		&t.ID, &t.EngagementID, &t.ScanType, &t.Title, &t.Description,
		&t.TargetStart, &t.TargetEnd, &t.Version, &t.BuildID, &t.CommitHash, &t.BranchTag,
		&t.Tags, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("test not found: %w", err)
	}
	return t, nil
}

func (r *TestRepo) FindByEngagementAndType(ctx context.Context, engagementID uuid.UUID, scanType string) (*testp.Test, error) {
	t := &testp.Test{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, engagement_id, scan_type, title, target_start, tags, created_at, updated_at
		FROM tests WHERE engagement_id=$1 AND scan_type=$2 LIMIT 1`,
		engagementID, scanType).Scan(
		&t.ID, &t.EngagementID, &t.ScanType, &t.Title, &t.TargetStart, &t.Tags, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("test not found: %w", err)
	}
	return t, nil
}

func (r *TestRepo) List(ctx context.Context, engagementID uuid.UUID) ([]*testp.Test, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, engagement_id, scan_type, title, target_start, created_at
		FROM tests WHERE engagement_id=$1 ORDER BY created_at DESC`, engagementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tests []*testp.Test
	for rows.Next() {
		t := &testp.Test{}
		if err := rows.Scan(&t.ID, &t.EngagementID, &t.ScanType, &t.Title, &t.TargetStart, &t.CreatedAt); err != nil {
			return nil, err
		}
		tests = append(tests, t)
	}
	return tests, rows.Err()
}

func (r *TestRepo) Update(ctx context.Context, t *testp.Test) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tests SET
			title=$2, description=$3, target_end=$4, updated_at=NOW()
		WHERE id=$1`, t.ID, t.Title, t.Description, t.TargetEnd)
	return err
}
