package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/finding-service/internal/domain/report"
)

// ReportRepo implements report.Repository using pgxpool.
type ReportRepo struct {
	pool *pgxpool.Pool
}

func NewReportRepo(pool *pgxpool.Pool) *ReportRepo {
	return &ReportRepo{pool: pool}
}

func (r *ReportRepo) Save(ctx context.Context, rep *report.Report) error {
	q := `
		INSERT INTO reports (
			id, product_id, title, format, status, engagement_id, test_id,
			severities, active_only, storage_key, file_size_bytes, generated_by,
			generated_at, expires_at, error_message, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		) ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			storage_key = EXCLUDED.storage_key,
			file_size_bytes = EXCLUDED.file_size_bytes,
			generated_at = EXCLUDED.generated_at,
			expires_at = EXCLUDED.expires_at,
			error_message = EXCLUDED.error_message
	`
	_, err := r.pool.Exec(ctx, q,
		rep.ID, rep.ProductID, rep.Title, rep.Format, rep.Status, rep.EngagementID, rep.TestID,
		rep.Severities, rep.ActiveOnly, rep.StorageKey, rep.FileSizeBytes, rep.GeneratedBy,
		rep.GeneratedAt, rep.ExpiresAt, rep.ErrorMessage, rep.CreatedAt,
	)
	return err
}

func (r *ReportRepo) FindByID(ctx context.Context, id string) (*report.Report, error) {
	q := `
		SELECT id, product_id, title, format, status, engagement_id, test_id,
			severities, active_only, storage_key, file_size_bytes, generated_by,
			generated_at, expires_at, error_message, created_at
		FROM reports WHERE id = $1
	`
	rep := &report.Report{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&rep.ID, &rep.ProductID, &rep.Title, &rep.Format, &rep.Status, &rep.EngagementID, &rep.TestID,
		&rep.Severities, &rep.ActiveOnly, &rep.StorageKey, &rep.FileSizeBytes, &rep.GeneratedBy,
		&rep.GeneratedAt, &rep.ExpiresAt, &rep.ErrorMessage, &rep.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("report not found: %w", err)
	}
	return rep, nil
}

func (r *ReportRepo) ListByProduct(ctx context.Context, productID, userID string, limit, offset int) ([]*report.Report, int, error) {
	var total int
	var countErr error
	if productID != "" {
		countErr = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports WHERE product_id::text = $1`, productID).Scan(&total)
	} else {
		countErr = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports`).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, countErr
	}

	var q string
	var args []interface{}
	if productID != "" {
		q = `
			SELECT id, product_id, title, format, status, engagement_id, test_id,
				severities, active_only, storage_key, file_size_bytes, generated_by,
				generated_at, expires_at, error_message, created_at
			FROM reports
			WHERE product_id::text = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{productID, limit, offset}
	} else {
		q = `
			SELECT id, product_id, title, format, status, engagement_id, test_id,
				severities, active_only, storage_key, file_size_bytes, generated_by,
				generated_at, expires_at, error_message, created_at
			FROM reports
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`
		args = []interface{}{limit, offset}
	}
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []*report.Report
	for rows.Next() {
		rep := &report.Report{}
		if err := rows.Scan(
			&rep.ID, &rep.ProductID, &rep.Title, &rep.Format, &rep.Status, &rep.EngagementID, &rep.TestID,
			&rep.Severities, &rep.ActiveOnly, &rep.StorageKey, &rep.FileSizeBytes, &rep.GeneratedBy,
			&rep.GeneratedAt, &rep.ExpiresAt, &rep.ErrorMessage, &rep.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		reports = append(reports, rep)
	}
	return reports, total, rows.Err()
}

func (r *ReportRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM reports WHERE id = $1`, id)
	return err
}

func (r *ReportRepo) DeleteExpired(ctx context.Context) (int, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM reports WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
