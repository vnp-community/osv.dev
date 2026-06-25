package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/scan-service/internal/domain/importdomain"
)

type TestImportRepo struct {
	pool *pgxpool.Pool
}

func NewTestImportRepo(pool *pgxpool.Pool) *TestImportRepo {
	return &TestImportRepo{pool: pool}
}

func (r *TestImportRepo) Save(ctx context.Context, h *importdomain.TestImport) error {
	q := `INSERT INTO test_imports (
		id, test_id, import_type, version, branch_tag, build_id, commit_hash,
		new_findings, closed_findings, reactivated, untouched,
		scan_file_key, import_settings, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
	_, err := r.pool.Exec(ctx, q,
		h.ID, h.TestID, h.ImportType, h.Version, h.BranchTag, h.BuildID, h.CommitHash,
		h.NewFindings, h.ClosedFindings, h.Reactivated, h.Untouched,
		h.ScanFileKey, h.ImportSettings, h.CreatedAt,
	)
	return err
}

func (r *TestImportRepo) FindByID(ctx context.Context, id string) (*importdomain.TestImport, error) {
	q := `SELECT id, test_id, import_type, version, branch_tag, build_id, commit_hash,
		new_findings, closed_findings, reactivated, untouched,
		scan_file_key, created_at
	FROM test_imports WHERE id = $1`
	var h importdomain.TestImport
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&h.ID, &h.TestID, &h.ImportType, &h.Version, &h.BranchTag, &h.BuildID, &h.CommitHash,
		&h.NewFindings, &h.ClosedFindings, &h.Reactivated, &h.Untouched,
		&h.ScanFileKey, &h.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *TestImportRepo) ListByTest(ctx context.Context, testID string) ([]*importdomain.TestImport, error) {
	q := `SELECT id, test_id, import_type, version, branch_tag, build_id, commit_hash,
		new_findings, closed_findings, reactivated, untouched,
		scan_file_key, created_at
	FROM test_imports WHERE test_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, testID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*importdomain.TestImport
	for rows.Next() {
		var h importdomain.TestImport
		if err := rows.Scan(
			&h.ID, &h.TestID, &h.ImportType, &h.Version, &h.BranchTag, &h.BuildID, &h.CommitHash,
			&h.NewFindings, &h.ClosedFindings, &h.Reactivated, &h.Untouched,
			&h.ScanFileKey, &h.CreatedAt,
		); err == nil {
			result = append(result, &h)
		}
	}
	return result, nil
}
