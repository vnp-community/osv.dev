// Package postgres — epss_repo.go
// PostgreSQL implementation of EPSSRepository.
// Queries EPSS data from the `cves` table (columns: epss_score, epss_percentile).
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	httpdelivery "github.com/osv/data-service/internal/delivery/http"
)

// EPSSRepo implements httpdelivery.EPSSRepository via PostgreSQL.
type EPSSRepo struct {
	db *pgxpool.Pool
}

// NewEPSSRepo creates a new EPSSRepo.
func NewEPSSRepo(db *pgxpool.Pool) *EPSSRepo {
	return &EPSSRepo{db: db}
}

// GetTopByEPSS returns the top CVEs ordered by EPSS score descending.
// minEPSS filters out entries below the threshold (0 = no filter).
func (r *EPSSRepo) GetTopByEPSS(ctx context.Context, limit int, minEPSS float64) ([]httpdelivery.EPSSEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			cve_id,
			COALESCE(epss_score, 0)       AS epss_score,
			COALESCE(epss_percentile, 0)  AS epss_percentile,
			COALESCE(severity_v3, '')      AS severity,
			COALESCE(is_kev, false)        AS is_kev,
			cvss_v3_score
		FROM cves
		WHERE epss_score IS NOT NULL
		  AND epss_score >= $1
		ORDER BY epss_score DESC
		LIMIT $2
	`, minEPSS, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []httpdelivery.EPSSEntry
	for rows.Next() {
		var e httpdelivery.EPSSEntry
		if err := rows.Scan(
			&e.CVEID, &e.EPSSScore, &e.EPSSPercentile,
			&e.Severity, &e.IsKEV, &e.CVSSv3Score,
		); err != nil {
			continue
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// GetEPSSDistribution returns EPSS score bucketed into 10 ranges (0.0–0.1, …, 0.9–1.0).
func (r *EPSSRepo) GetEPSSDistribution(ctx context.Context) ([]httpdelivery.EPSSBucket, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			CONCAT(
				FLOOR(epss_score * 10)::int * 10, '-',
				(FLOOR(epss_score * 10)::int * 10 + 10), '%'
			) AS range,
			COUNT(*) AS count
		FROM cves
		WHERE epss_score IS NOT NULL
		GROUP BY FLOOR(epss_score * 10)
		ORDER BY FLOOR(epss_score * 10)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []httpdelivery.EPSSBucket
	for rows.Next() {
		var b httpdelivery.EPSSBucket
		if err := rows.Scan(&b.Range, &b.Count); err != nil {
			continue
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}
