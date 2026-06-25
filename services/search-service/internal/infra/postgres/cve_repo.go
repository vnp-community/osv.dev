// Package postgres implements the CVE search read repository using pgx/v5.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/osv/search-service/internal/domain/entity"
	domainerrors "github.com/osv/search-service/internal/domain/errors"
	"github.com/osv/search-service/internal/domain/repository"
)

type cveRepository struct {
	db *pgxpool.Pool
}

// NewCVERepository creates a PostgreSQL-backed CVERepository.
func NewCVERepository(db *pgxpool.Pool) repository.CVERepository {
	return &cveRepository{db: db}
}

// Search runs a full-text + filter query against the cves table.
func (r *cveRepository) Search(ctx context.Context, filter *entity.SearchFilter) ([]*entity.CVE, int64, error) {
	filter.Validate()

	var conditions []string
	var args []interface{}
	idx := 1

	if filter.Query != "" {
		if entity.IsValidID(filter.Query) {
			// Exact CVE ID match
			conditions = append(conditions, fmt.Sprintf("id = $%d", idx))
			args = append(args, strings.ToUpper(strings.TrimSpace(filter.Query)))
			idx++
		} else {
			// Full-text search
			conditions = append(conditions, fmt.Sprintf(
				"to_tsvector('english', id||' '||description) @@ plainto_tsquery('english', $%d)", idx))
			args = append(args, filter.Query)
			idx++
		}
	}
	if filter.Severity != nil {
		conditions = append(conditions, fmt.Sprintf("UPPER(severity) = $%d", idx))
		args = append(args, string(*filter.Severity))
		idx++
	}
	if filter.Source != nil {
		conditions = append(conditions, fmt.Sprintf("UPPER(COALESCE(source,'NVD')) = $%d", idx))
		args = append(args, string(*filter.Source))
		idx++
	}
	// CR-GCV-002: EPSS filters
	if filter.MinEPSS != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(epss, 0) >= $%d", idx))
		args = append(args, *filter.MinEPSS)
		idx++
	}
	if filter.MaxEPSS != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(epss, 0) <= $%d", idx))
		args = append(args, *filter.MaxEPSS)
		idx++
	}
	// CR-GCV-003: KEV / Exploit filters
	if filter.IsKEV != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(is_kev, FALSE) = $%d", idx))
		args = append(args, *filter.IsKEV)
		idx++
	}
	if filter.IsExploit != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(is_exploit, FALSE) = $%d", idx))
		args = append(args, *filter.IsExploit)
		idx++
	}
	// CR-GCV-005: CWE / Vendor text filters
	if filter.CWE != "" {
		conditions = append(conditions, fmt.Sprintf("$%d = ANY(cwe)", idx))
		args = append(args, filter.CWE)
		idx++
	}
	if filter.Vendor != "" {
		conditions = append(conditions, fmt.Sprintf("$%d = ANY(vendors)", idx))
		args = append(args, strings.ToLower(filter.Vendor))
		idx++
	}
	if filter.Product != "" {
		conditions = append(conditions, fmt.Sprintf("$%d = ANY(products)", idx))
		args = append(args, strings.ToLower(filter.Product))
		idx++
	}

	where := "TRUE"
	if len(conditions) > 0 {
		where = strings.Join(conditions, " AND ")
	}

	// Sort order
	orderBy := "published DESC NULLS LAST"
	switch filter.Sort {
	case entity.SortOldest:
		orderBy = "published ASC NULLS LAST"
	case entity.SortEPSSDesc:
		orderBy = "COALESCE(epss, 0) DESC NULLS LAST"
	case entity.SortCVSS3:
		orderBy = "COALESCE(cvss3_score, cvss_score, 0) DESC NULLS LAST"
	}

	// Count query
	var total int64
	countSQL := "SELECT COUNT(*) FROM cves WHERE " + where
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("cve search count: %w", err)
	}

	// Data query — include EPSS, CVSS3, IsExploit
	offset := filter.Page * filter.Limit
	dataSQL := fmt.Sprintf(`
		SELECT id, description,
		       UPPER(COALESCE(severity,'UNKNOWN')) AS severity,
		       COALESCE(published, '1970-01-01'::timestamp) AS published,
		       COALESCE(source,'NVD')              AS source,
		       COALESCE(is_kev, FALSE)             AS is_kev,
		       COALESCE(is_exploit, FALSE)         AS is_exploit,
		       COALESCE(link, '')                  AS link,
		       cvss_score,
		       cvss3_score,
		       epss,
		       epss_percentile,
		       created_at, updated_at
		FROM cves WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		where, orderBy, idx, idx+1)
	args = append(args, filter.Limit, offset)

	rows, err := r.db.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("cve search query: %w", err)
	}
	defer rows.Close()

	var cves []*entity.CVE
	for rows.Next() {
		c := &entity.CVE{}
		if err := rows.Scan(
			&c.ID, &c.Description, &c.Severity, &c.Published,
			&c.Source, &c.IsKEV, &c.IsExploit, &c.Link,
			&c.CVSSScore, &c.CVSS3Score, &c.EPSS, &c.EPSSPct,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		cves = append(cves, c)
	}
	return cves, total, rows.Err()
}

// FindByID fetches a single CVE record.
func (r *cveRepository) FindByID(ctx context.Context, id string) (*entity.CVE, error) {
	c := &entity.CVE{}
	err := r.db.QueryRow(ctx, `
		SELECT id, description, severity, 
		       COALESCE(published, '1970-01-01'::timestamp) AS published, 
		       COALESCE(source, 'NVD') AS source, 
		       is_kev, 
		       COALESCE(is_exploit, FALSE) AS is_exploit,
		       COALESCE(link, '') AS link, cvss_score, created_at, updated_at
		FROM cves WHERE id = $1`, strings.ToUpper(strings.TrimSpace(id))).Scan(
		&c.ID, &c.Description, &c.Severity, &c.Published, &c.Source,
		&c.IsKEV, &c.IsExploit, &c.Link, &c.CVSSScore, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainerrors.ErrCVENotFound
	}
	if err != nil {
		return nil, fmt.Errorf("cve find by id: %w", err)
	}
	return c, nil
}

// FindByIDs fetches multiple CVE records by ID.
func (r *cveRepository) FindByIDs(ctx context.Context, ids []string) ([]*entity.CVE, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	
	query := `
		SELECT id, description, severity, 
		       COALESCE(published, '1970-01-01'::timestamp) AS published, 
		       COALESCE(source, 'NVD') AS source, 
		       is_kev, 
		       COALESCE(is_exploit, FALSE) AS is_exploit,
		       COALESCE(link, '') AS link, cvss_score, created_at, updated_at
		FROM cves WHERE id = ANY($1)
	`
	rows, err := r.db.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("cve find by ids: %w", err)
	}
	defer rows.Close()

	var cves []*entity.CVE
	for rows.Next() {
		c := &entity.CVE{}
		if err := rows.Scan(
			&c.ID, &c.Description, &c.Severity, &c.Published, &c.Source,
			&c.IsKEV, &c.IsExploit, &c.Link, &c.CVSSScore, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		cves = append(cves, c)
	}
	return cves, rows.Err()
}

// Count returns the total number of CVEs.
func (r *cveRepository) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM cves").Scan(&n)
	return n, err
}

// CountWithEPSS returns count of CVEs that have an EPSS score.
func (r *cveRepository) CountWithEPSS(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM cves WHERE epss IS NOT NULL").Scan(&n)
	return n, err
}

// QueryEPSSDistribution returns bucket counts for EPSS score ranges.
// Returns nil on error (non-fatal for stats endpoints).
func (r *cveRepository) QueryEPSSDistribution(ctx context.Context) *repository.EPSSDistribution {
	row := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE epss < 0.1)                  AS very_low,
			COUNT(*) FILTER (WHERE epss >= 0.1 AND epss < 0.5)  AS low,
			COUNT(*) FILTER (WHERE epss >= 0.5 AND epss < 0.9)  AS high,
			COUNT(*) FILTER (WHERE epss >= 0.9)                 AS critical,
			AVG(epss)                                           AS mean,
			PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY epss)  AS median
		FROM cves
		WHERE epss IS NOT NULL
	`)
	var d repository.EPSSDistribution
	if err := row.Scan(&d.VeryLow, &d.Low, &d.High, &d.Critical, &d.Mean, &d.Median); err != nil {
		return nil
	}
	return &d
}

// GetAggregations returns faceted aggregations for CVE data.
func (r *cveRepository) GetAggregations(ctx context.Context) (map[string]interface{}, error) {
	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(severity,'UNKNOWN') as severity, COUNT(*) as count
		FROM cves GROUP BY severity ORDER BY count DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("aggregation query failed: %w", err)
	}
	defer rows.Close()

	severityMap := map[string]int{}
	for rows.Next() {
		var sev string
		var count int
		if err := rows.Scan(&sev, &count); err != nil {
			return nil, err
		}
		severityMap[sev] = count
	}


	return map[string]interface{}{
		"by_severity": severityMap,
		"top_vendors": []interface{}{}, // simplified for fallback
		"top_cwe":     []interface{}{},
	}, nil
}

// GetEPSSStats returns aggregate EPSS statistics (TASK-GCV-007).
func (r *cveRepository) GetEPSSStats(ctx context.Context) (*repository.EPSSStats, error) {
	var stats repository.EPSSStats

	// Total scored and Avg EPSS
	row := r.db.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(AVG(epss), 0)
		FROM cves
		WHERE epss IS NOT NULL
	`)
	if err := row.Scan(&stats.TotalScored, &stats.AvgEPSS); err != nil {
		return nil, err
	}

	// High risk count
	row = r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM cves WHERE epss >= 0.9
	`)
	if err := row.Scan(&stats.HighRiskCount); err != nil {
		return nil, err
	}

	// Top 10 CVEs by EPSS
	rows, err := r.db.Query(ctx, `
		SELECT id, epss, COALESCE(epss_percentile, 0), COALESCE(severity, 'UNKNOWN')
		FROM cves
		WHERE epss IS NOT NULL
		ORDER BY epss DESC
		LIMIT 10
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var e repository.EPSSEntry
		if err := rows.Scan(&e.CVEID, &e.EPSS, &e.EPSSPercentile, &e.Severity); err != nil {
			continue
		}
		stats.TopCVEs = append(stats.TopCVEs, e)
	}

	stats.UpdatedAt = time.Now()
	return &stats, nil
}

// GetTopEPSS returns CVEs with EPSS >= minEPSS, sorted by EPSS desc.
func (r *cveRepository) GetTopEPSS(ctx context.Context, minEPSS float64, limit int) ([]repository.EPSSTopEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id,
		       COALESCE(epss, 0)             AS epss,
		       COALESCE(epss_percentile, 0)  AS epss_pct,
		       UPPER(COALESCE(severity,'UNKNOWN')) AS severity,
		       COALESCE(TO_CHAR(published, 'YYYY-MM-DD'), '') AS date
		FROM cves
		WHERE epss IS NOT NULL AND epss >= $1
		ORDER BY epss DESC
		LIMIT $2
	`, minEPSS, limit)
	if err != nil {
		return nil, fmt.Errorf("get top epss: %w", err)
	}
	defer rows.Close()

	var entries []repository.EPSSTopEntry
	for rows.Next() {
		var e repository.EPSSTopEntry
		if err := rows.Scan(&e.CVEID, &e.EPSS, &e.EPSSPct, &e.Severity, &e.Date); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetEPSSPercentiles returns EPSS scores at P50, P90, P95, P99.
func (r *cveRepository) GetEPSSPercentiles(ctx context.Context) (*repository.EPSSPercentiles, error) {
	row := r.db.QueryRow(ctx, `
		SELECT
			COALESCE(PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY epss), 0) AS p50,
			COALESCE(PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY epss), 0) AS p90,
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY epss), 0) AS p95,
			COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY epss), 0) AS p99
		FROM cves
		WHERE epss IS NOT NULL
	`)
	var p repository.EPSSPercentiles
	if err := row.Scan(&p.P50, &p.P90, &p.P95, &p.P99); err != nil {
		return nil, fmt.Errorf("epss percentiles: %w", err)
	}
	return &p, nil
}

func (r *cveRepository) GetDashboardStats(ctx context.Context) (*repository.DashboardStats, error) {
	stats := &repository.DashboardStats{
		BySeverity:  make(map[string]int64),
		LastUpdated: time.Now().UTC(),
	}

	// 1. Total CVEs
	r.db.QueryRow(ctx, "SELECT COUNT(*) FROM cves").Scan(&stats.TotalCVEs)

	// 2. By severity
	rows, _ := r.db.Query(ctx, "SELECT COALESCE(severity,'UNKNOWN'), COUNT(*) FROM cves GROUP BY severity")
	if rows != nil {
		for rows.Next() {
			var sev string
			var count int64
			rows.Scan(&sev, &count)
			stats.BySeverity[sev] = count
		}
		rows.Close()
	}

	// 3. Trending
	r.db.QueryRow(ctx, "SELECT COUNT(*) FROM cves WHERE published >= NOW() - INTERVAL '7 days'").Scan(&stats.NewThisWeek)
	r.db.QueryRow(ctx, "SELECT COUNT(*) FROM cves WHERE published >= NOW() - INTERVAL '30 days'").Scan(&stats.NewThisMonth)

	// 4. Top vendors (explode vendors array)
	vendorRows, _ := r.db.Query(ctx, `
		SELECT vendor, COUNT(*) as count
		FROM cves, UNNEST(vendors) as vendor
		WHERE vendor IS NOT NULL AND vendor != ''
		GROUP BY vendor ORDER BY count DESC LIMIT 10
	`)
	if vendorRows != nil {
		for vendorRows.Next() {
			var v repository.VendorCVECount
			vendorRows.Scan(&v.Vendor, &v.Count)
			stats.TopVendors = append(stats.TopVendors, v)
		}
		vendorRows.Close()
	}

	// 5. EPSS distribution
	r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE epss < 0.1)                  AS very_low,
			COUNT(*) FILTER (WHERE epss >= 0.1 AND epss < 0.5)  AS low,
			COUNT(*) FILTER (WHERE epss >= 0.5 AND epss < 0.9)  AS high,
			COUNT(*) FILTER (WHERE epss >= 0.9)                  AS critical
		FROM cves WHERE epss IS NOT NULL
	`).Scan(&stats.EPSSDistrib.VeryLow, &stats.EPSSDistrib.Low, &stats.EPSSDistrib.High, &stats.EPSSDistrib.Critical)

	// 6. KEV + Exploit counts
	r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE is_kev = TRUE)     AS total_kev,
			COUNT(*) FILTER (WHERE is_exploit = TRUE) AS total_exploit
		FROM cves
	`).Scan(&stats.TotalKEV, &stats.TotalExploit)

	return stats, nil
}
