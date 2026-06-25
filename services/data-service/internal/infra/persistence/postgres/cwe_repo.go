// Package postgres — cwe_repo.go
// PostgreSQL implementation of CWERepository for data-service.
// Queries CWE data from the cwe_weaknesses table.
package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	httpdelivery "github.com/osv/data-service/internal/delivery/http"
)

// CWERepo implements httpdelivery.CWERepository via PostgreSQL.
type CWERepo struct {
	db *pgxpool.Pool
}

// NewCWERepo creates a CWERepo backed by a pgxpool.
func NewCWERepo(db *pgxpool.Pool) *CWERepo {
	return &CWERepo{db: db}
}

// List returns CWE entries with optional full-text search and pagination.
// [FIX TASK-HC-002] Production implementation — no longer nil.
func (r *CWERepo) List(ctx context.Context, query string, page, pageSize int) ([]httpdelivery.CWEItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Build WHERE clause for optional search
	var whereClause string
	var args []interface{}
	if query != "" {
		whereClause = "WHERE (cwe_id ILIKE $1 OR name ILIKE $1 OR description ILIKE $1)"
		args = append(args, "%"+query+"%")
	}

	// COUNT query
	var total int64
	countSQL := "SELECT COUNT(*) FROM cwe_weaknesses " + whereClause
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("cwe_repo.List count: %w", err)
	}

	// DATA query — no JOIN for related_cve_count (expensive); return 0
	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2
	dataArgs := append(args, pageSize, offset) //nolint:gocritic
	dataSQL := fmt.Sprintf(`
		SELECT cwe_id, COALESCE(name,''), COALESCE(description,''), 0::bigint
		FROM cwe_weaknesses
		%s
		ORDER BY cwe_id
		LIMIT $%d OFFSET $%d
	`, whereClause, limitIdx, offsetIdx)

	rows, err := r.db.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("cwe_repo.List query: %w", err)
	}
	defer rows.Close()

	var items []httpdelivery.CWEItem
	for rows.Next() {
		var item httpdelivery.CWEItem
		if err := rows.Scan(&item.CWEID, &item.Name, &item.Description, &item.RelatedCVECount); err != nil {
			return nil, 0, fmt.Errorf("cwe_repo.List scan: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("cwe_repo.List rows: %w", err)
	}
	return items, total, nil
}

// GetByID returns a single CWE entry by its ID (e.g., "CWE-79" or "79").
// [FIX TASK-HC-002] Production implementation.
func (r *CWERepo) GetByID(ctx context.Context, cweID string) (*httpdelivery.CWEItem, error) {
	// Normalize: accept "79", "CWE-79", "cwe-79"
	normalized := strings.ToUpper(cweID)
	if !strings.HasPrefix(normalized, "CWE-") {
		normalized = "CWE-" + normalized
	}

	var item httpdelivery.CWEItem
	err := r.db.QueryRow(ctx, `
		SELECT cwe_id, COALESCE(name,''), COALESCE(description,'')
		FROM cwe_weaknesses
		WHERE UPPER(cwe_id) = $1
	`, normalized).Scan(&item.CWEID, &item.Name, &item.Description)
	if err != nil {
		return nil, fmt.Errorf("cwe_repo.GetByID %s: %w", cweID, err)
	}
	return &item, nil
}
