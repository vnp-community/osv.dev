// Package postgres — CVE write repository implementation for the sync service.
package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/globalcve/mono/internal/cvesearch/domain/entity"
	"github.com/globalcve/mono/internal/cvesync/domain/repository"
)

// CVERepo is the PostgreSQL implementation of CVEWriteRepository.
type CVERepo struct {
	pool *pgxpool.Pool
}

// NewCVERepo creates a new PostgreSQL CVE write repository.
func NewCVERepo(pool *pgxpool.Pool) *CVERepo {
	return &CVERepo{pool: pool}
}

// UpsertBatch inserts or updates a batch of CVEs in PostgreSQL.
// Uses ON CONFLICT (id) DO UPDATE to handle duplicates.
func (r *CVERepo) UpsertBatch(ctx context.Context, cves []*entity.CVE) (inserted, updated int, err error) {
	if len(cves) == 0 {
		return 0, 0, nil
	}

	const batchSize = 500
	for i := 0; i < len(cves); i += batchSize {
		end := i + batchSize
		if end > len(cves) {
			end = len(cves)
		}
		ins, upd, err := r.upsertChunk(ctx, cves[i:end])
		if err != nil {
			return inserted, updated, err
		}
		inserted += ins
		updated += upd
	}
	return inserted, updated, nil
}

func (r *CVERepo) upsertChunk(ctx context.Context, cves []*entity.CVE) (inserted, updated int, err error) {
	// Build multi-row INSERT ... ON CONFLICT
	const cols = `id, description, summary, severity, published, modified, source,
		is_kev, link, cvss_score, cvss3_score, cvss_vector, cvss3_vector,
		epss, epss_percentile, vendors, products, cwe, references, updated_at`

	placeholders := make([]string, 0, len(cves))
	args := make([]interface{}, 0, len(cves)*20)
	idx := 1

	for _, cve := range cves {
		sev := string(cve.Severity)
		if sev == "" {
			sev = "UNKNOWN"
		}
		src := string(cve.Source)
		if src == "" {
			src = "NVD"
		}

		placeholders = append(placeholders, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,NOW())",
			idx, idx+1, idx+2, idx+3, idx+4, idx+5, idx+6,
			idx+7, idx+8, idx+9, idx+10, idx+11, idx+12,
			idx+13, idx+14, idx+15, idx+16, idx+17, idx+18,
		))
		idx += 19

		args = append(args,
			cve.ID,
			cve.Description,
			cve.Summary,
			sev,
			nullableTime(cve.Published),
			nullableTime(cve.Modified),
			src,
			cve.IsKEV,
			cve.Link,
			cve.CVSSScore,
			cve.CVSS3Score,
			cve.CVSSVector,
			cve.CVSS3Vector,
			cve.EPSS,
			cve.EPSSPercentile,
			pgTextArray(cve.Vendors),
			pgTextArray(cve.Products),
			pgTextArray(cve.CWE),
			pgTextArray(cve.References),
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO cves (%s)
		VALUES %s
		ON CONFLICT (id) DO UPDATE SET
			description     = EXCLUDED.description,
			summary         = EXCLUDED.summary,
			severity        = EXCLUDED.severity,
			published       = COALESCE(EXCLUDED.published, cves.published),
			modified        = COALESCE(EXCLUDED.modified, cves.modified),
			source          = EXCLUDED.source,
			link            = COALESCE(NULLIF(EXCLUDED.link,''), cves.link),
			cvss_score      = COALESCE(EXCLUDED.cvss_score, cves.cvss_score),
			cvss3_score     = COALESCE(EXCLUDED.cvss3_score, cves.cvss3_score),
			cvss_vector     = COALESCE(NULLIF(EXCLUDED.cvss_vector,''), cves.cvss_vector),
			cvss3_vector    = COALESCE(NULLIF(EXCLUDED.cvss3_vector,''), cves.cvss3_vector),
			epss            = COALESCE(EXCLUDED.epss, cves.epss),
			epss_percentile = COALESCE(EXCLUDED.epss_percentile, cves.epss_percentile),
			vendors         = CASE WHEN array_length(EXCLUDED.vendors, 1) > 0 THEN EXCLUDED.vendors ELSE cves.vendors END,
			products        = CASE WHEN array_length(EXCLUDED.products, 1) > 0 THEN EXCLUDED.products ELSE cves.products END,
			cwe             = CASE WHEN array_length(EXCLUDED.cwe, 1) > 0 THEN EXCLUDED.cwe ELSE cves.cwe END,
			references      = CASE WHEN array_length(EXCLUDED.references, 1) > 0 THEN EXCLUDED.references ELSE cves.references END,
			updated_at      = NOW()
		RETURNING (xmax = 0) AS is_insert`,
		cols, strings.Join(placeholders, ","),
	)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("upsert chunk: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var isInsert bool
		if err := rows.Scan(&isInsert); err != nil {
			continue
		}
		if isInsert {
			inserted++
		} else {
			updated++
		}
	}
	return inserted, updated, rows.Err()
}

// MarkKEV updates the is_kev flag for a list of CVE IDs.
func (r *CVERepo) MarkKEV(ctx context.Context, ids []string, isKEV bool) error {
	if len(ids) == 0 {
		return nil
	}

	// Build parameterized IN clause
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = isKEV
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}

	query := fmt.Sprintf(
		`UPDATE cves SET is_kev = $1, updated_at = NOW() WHERE id IN (%s)`,
		strings.Join(placeholders, ","),
	)

	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

// UpdateEPSS updates EPSS scores for a batch of CVE IDs.
func (r *CVERepo) UpdateEPSS(ctx context.Context, updates []repository.EPSSUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	// Use a temporary table approach for bulk update
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, u := range updates {
		_, err := tx.Exec(ctx,
			`UPDATE cves SET epss = $1, epss_percentile = $2, updated_at = NOW() WHERE id = $3`,
			u.Score, u.Percentile, u.CVEID,
		)
		if err != nil {
			return fmt.Errorf("update epss for %s: %w", u.CVEID, err)
		}
	}

	return tx.Commit(ctx)
}

// --- helpers ---

func nullableTime(t interface{}) interface{} {
	return t
}

func pgTextArray(s []string) interface{} {
	if len(s) == 0 {
		return []string{}
	}
	return s
}
