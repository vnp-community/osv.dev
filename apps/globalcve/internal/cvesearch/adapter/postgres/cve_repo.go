// Package postgres — CVE read repository for the search service.
package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/globalcve/mono/internal/cvesearch/domain/entity"
)

// CVEReadRepo is the PostgreSQL implementation of CVERepository (read-only).
type CVEReadRepo struct {
	pool *pgxpool.Pool
}

// NewCVEReadRepo creates a new PostgreSQL CVE read repository.
func NewCVEReadRepo(pool *pgxpool.Pool) *CVEReadRepo {
	return &CVEReadRepo{pool: pool}
}

// Search queries CVEs with dynamic filter, sort, and pagination.
func (r *CVEReadRepo) Search(ctx context.Context, filter *entity.SearchFilter) ([]*entity.CVE, int64, error) {
	filter.Validate()

	where, args := buildWhereClause(filter)
	orderBy := buildOrderBy(filter.Sort)
	offset := filter.Page * filter.Limit
	argIdx := len(args) + 1

	// Count query
	countQuery := `SELECT COUNT(*) FROM cves` + where
	var total int64
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count cves: %w", err)
	}

	// Data query
	query := fmt.Sprintf(`
		SELECT id, description, summary, severity, published, modified, source,
		       is_kev, link, cvss_score, cvss3_score, epss, epss_percentile,
		       vendors, products, cwe, references, created_at, updated_at
		FROM cves
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`,
		where, orderBy, argIdx, argIdx+1,
	)
	args = append(args, filter.Limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search cves: %w", err)
	}
	defer rows.Close()

	cves, err := scanCVERows(rows)
	if err != nil {
		return nil, 0, err
	}
	return cves, total, nil
}

// FindByID retrieves a single CVE by ID.
func (r *CVEReadRepo) FindByID(ctx context.Context, id string) (*entity.CVE, error) {
	query := `
		SELECT id, description, summary, severity, published, modified, source,
		       is_kev, link, cvss_score, cvss3_score, epss, epss_percentile,
		       vendors, products, cwe, references, created_at, updated_at
		FROM cves WHERE id = $1`

	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("find cve by id: %w", err)
	}
	defer rows.Close()

	cves, err := scanCVERows(rows)
	if err != nil {
		return nil, err
	}
	if len(cves) == 0 {
		return nil, fmt.Errorf("CVE not found: %s", id)
	}
	return cves[0], nil
}

// Count returns the total number of CVEs.
func (r *CVEReadRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM cves`).Scan(&count)
	return count, err
}

// SearchSemantic performs pgvector cosine similarity search.
func (r *CVEReadRepo) SearchSemantic(ctx context.Context, embedding []float32, limit int, threshold float64) ([]*entity.CVE, error) {
	query := `
		SELECT id, description, summary, severity, published, modified, source,
		       is_kev, link, cvss_score, cvss3_score, epss, epss_percentile,
		       vendors, products, cwe, references, created_at, updated_at,
		       1 - (embedding <=> $1) AS similarity
		FROM cves
		WHERE embedding IS NOT NULL
		  AND 1 - (embedding <=> $1) >= $2
		ORDER BY embedding <=> $1
		LIMIT $3`

	rows, err := r.pool.Query(ctx, query, pgvectorSlice(embedding), threshold, limit)
	if err != nil {
		return nil, fmt.Errorf("semantic search: %w", err)
	}
	defer rows.Close()

	return scanCVERows(rows)
}

// --- helpers ---

func buildWhereClause(filter *entity.SearchFilter) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	idx := 1

	if filter.Query != "" {
		if entity.IsValidID(filter.Query) {
			// Exact CVE ID match
			conditions = append(conditions, fmt.Sprintf("id = $%d", idx))
			args = append(args, filter.Query)
			idx++
		} else {
			// Full-text search using GIN index
			conditions = append(conditions, fmt.Sprintf(
				"to_tsvector('english', id || ' ' || description || ' ' || summary) @@ plainto_tsquery('english', $%d)",
				idx,
			))
			args = append(args, filter.Query)
			idx++
		}
	}

	if filter.Severity != nil {
		conditions = append(conditions, fmt.Sprintf("severity = $%d", idx))
		args = append(args, string(*filter.Severity))
		idx++
	}

	if filter.Source != nil {
		conditions = append(conditions, fmt.Sprintf("source = $%d", idx))
		args = append(args, string(*filter.Source))
		idx++
	}

	if filter.IsKEV != nil && *filter.IsKEV {
		conditions = append(conditions, "is_kev = TRUE")
	}

	if filter.MinEPSS != nil {
		conditions = append(conditions, fmt.Sprintf("epss >= $%d", idx))
		args = append(args, *filter.MinEPSS)
		idx++
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func buildOrderBy(sort entity.SortOrder) string {
	switch sort {
	case entity.SortOldest:
		return "published ASC NULLS LAST"
	case entity.SortCVSS:
		return "cvss3_score DESC NULLS LAST"
	case entity.SortEPSS:
		return "epss DESC NULLS LAST"
	default: // SortNewest
		return "published DESC NULLS LAST"
	}
}

func scanCVERows(rows pgx.Rows) ([]*entity.CVE, error) {
	var cves []*entity.CVE
	for rows.Next() {
		cve := &entity.CVE{}
		var (
			pub, mod  time.Time
			createdAt, updatedAt time.Time
		)
		err := rows.Scan(
			&cve.ID, &cve.Description, &cve.Summary, &cve.Severity,
			&pub, &mod, &cve.Source,
			&cve.IsKEV, &cve.Link,
			&cve.CVSSScore, &cve.CVSS3Score,
			&cve.EPSS, &cve.EPSSPercentile,
			&cve.Vendors, &cve.Products, &cve.CWE, &cve.References,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan cve row: %w", err)
		}
		cve.Published = pub
		cve.Modified = mod
		cve.CreatedAt = createdAt
		cve.UpdatedAt = updatedAt
		cves = append(cves, cve)
	}
	return cves, rows.Err()
}

// pgvectorSlice converts []float32 to a format pgx can send as a vector.
func pgvectorSlice(v []float32) interface{} {
	// pgx handles []float32 → vector type via pgvector-go or raw []float32
	return v
}
