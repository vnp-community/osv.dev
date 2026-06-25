// Package postgres implements the KEV repository interface using pgx/v5.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	entity "github.com/osv/data-service/internal/domain/kev"
	domainerrors "github.com/osv/data-service/internal/domain/errors"
	"github.com/osv/data-service/internal/domain/repository"
)

type kevRepository struct {
	db *pgxpool.Pool
}

// NewKEVRepository creates a PostgreSQL-backed KEVRepository.
func NewKEVRepository(db *pgxpool.Pool) repository.KEVRepository {
	return &kevRepository{db: db}
}

// UpsertBatch inserts or updates KEV entries in batches of 200.
func (r *kevRepository) UpsertBatch(ctx context.Context, entries []*entity.KEVEntry) (inserted, updated int, err error) {
	const batchSize = 200
	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}
		ins, upd, e := r.upsertChunk(ctx, entries[i:end])
		if e != nil {
			return inserted, updated, e
		}
		inserted += ins
		updated += upd
	}
	return inserted, updated, nil
}

func (r *kevRepository) upsertChunk(ctx context.Context, chunk []*entity.KEVEntry) (inserted, updated int, error error) {
	if len(chunk) == 0 {
		return 0, 0, nil
	}

	// Build multi-row VALUES clause (12 fields per row).
	placeholders := make([]string, 0, len(chunk))
	args := make([]interface{}, 0, len(chunk)*12)
	idx := 1
	now := time.Now().UTC()

	for _, e := range chunk {
		placeholders = append(placeholders, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			idx, idx+1, idx+2, idx+3, idx+4, idx+5, idx+6, idx+7, idx+8, idx+9, idx+10, idx+11,
		))
		args = append(args,
			e.CVEID, e.VendorProject, e.Product, e.VulnerabilityName,
			e.DateAdded, e.DueDate, e.Notes, now,
			e.ShortDescription, e.RequiredAction, e.IsKnownRansomware, e.KnownRansomwareCampaignUse,
		)
		idx += 12
	}

	sql := fmt.Sprintf(`
		INSERT INTO kev_entries
			(cve_id, vendor_project, product, vulnerability_name,
			 date_added, due_date, notes, created_at,
			 short_description, required_action, is_known_ransomware, ransomware_campaign_use)
		VALUES %s
		ON CONFLICT (cve_id) DO UPDATE SET
			vendor_project           = EXCLUDED.vendor_project,
			product                  = EXCLUDED.product,
			vulnerability_name       = EXCLUDED.vulnerability_name,
			due_date                 = EXCLUDED.due_date,
			notes                    = EXCLUDED.notes,
			short_description        = EXCLUDED.short_description,
			required_action          = EXCLUDED.required_action,
			is_known_ransomware      = EXCLUDED.is_known_ransomware,
			ransomware_campaign_use  = EXCLUDED.ransomware_campaign_use,
			updated_at               = NOW()
		RETURNING
			(xmax = 0) AS is_insert`,
		strings.Join(placeholders, ","),
	)

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("kev upsert: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var isInsert bool
		if err := rows.Scan(&isInsert); err != nil {
			return inserted, updated, err
		}
		if isInsert {
			inserted++
		} else {
			updated++
		}
	}
	return inserted, updated, rows.Err()
}

// FindByCVEID fetches a single KEV entry.
func (r *kevRepository) FindByCVEID(ctx context.Context, cveID string) (*entity.KEVEntry, error) {
	row := r.db.QueryRow(ctx, `
		SELECT cve_id, vendor_project, product, vulnerability_name,
		       date_added, due_date, notes, created_at, updated_at,
		       COALESCE(short_description,'') AS short_description,
		       COALESCE(required_action,'')   AS required_action,
		       COALESCE(is_known_ransomware, FALSE) AS is_known_ransomware,
		       COALESCE(ransomware_campaign_use,'') AS ransomware_campaign_use
		FROM kev_entries WHERE cve_id = $1`, cveID)

	e := &entity.KEVEntry{}
	err := row.Scan(
		&e.CVEID, &e.VendorProject, &e.Product, &e.VulnerabilityName,
		&e.DateAdded, &e.DueDate, &e.Notes, &e.CreatedAt, &e.UpdatedAt,
		&e.ShortDescription, &e.RequiredAction, &e.IsKnownRansomware, &e.KnownRansomwareCampaignUse,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainerrors.ErrKEVNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("kev find by id: %w", err)
	}
	return e, nil
}

// List returns paginated KEV entries matching the filter.
func (r *kevRepository) List(ctx context.Context, filter *entity.KEVFilter) ([]*entity.KEVEntry, int64, error) {
	filter.Validate()

	var conditions []string
	var args []interface{}
	idx := 1

	if filter.Query != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(cve_id ILIKE $%d OR vendor_project ILIKE $%d OR vulnerability_name ILIKE $%d)",
			idx, idx+1, idx+2,
		))
		q := "%" + filter.Query + "%"
		args = append(args, q, q, q)
		idx += 3
	}
	if filter.VendorProject != "" {
		conditions = append(conditions, fmt.Sprintf("vendor_project ILIKE $%d", idx))
		args = append(args, "%"+filter.VendorProject+"%")
		idx++
	}
	if filter.Since != nil {
		conditions = append(conditions, fmt.Sprintf("date_added >= $%d", idx))
		args = append(args, *filter.Since)
		idx++
	}
	// CR-GCV-007: Ransomware filter
	if filter.IsRansomware != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(is_known_ransomware, FALSE) = $%d", idx))
		args = append(args, *filter.IsRansomware)
		idx++
	}

	where := "TRUE"
	if len(conditions) > 0 {
		where = strings.Join(conditions, " AND ")
	}

	// Count
	var total int64
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM kev_entries WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("kev list count: %w", err)
	}

	// Data
	offset := filter.Page * filter.Limit
	dataSQL := fmt.Sprintf(`
		SELECT cve_id, vendor_project, product, vulnerability_name,
		       date_added, due_date, notes, created_at, updated_at,
		       COALESCE(short_description,'')         AS short_description,
		       COALESCE(required_action,'')           AS required_action,
		       COALESCE(is_known_ransomware, FALSE)   AS is_known_ransomware,
		       COALESCE(ransomware_campaign_use,'')   AS ransomware_campaign_use
		FROM kev_entries WHERE %s
		ORDER BY date_added DESC NULLS LAST
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, filter.Limit, offset)

	rows, err := r.db.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("kev list query: %w", err)
	}
	defer rows.Close()

	var entries []*entity.KEVEntry
	for rows.Next() {
		e := &entity.KEVEntry{}
		if err := rows.Scan(
			&e.CVEID, &e.VendorProject, &e.Product, &e.VulnerabilityName,
			&e.DateAdded, &e.DueDate, &e.Notes, &e.CreatedAt, &e.UpdatedAt,
			&e.ShortDescription, &e.RequiredAction, &e.IsKnownRansomware, &e.KnownRansomwareCampaignUse,
		); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

// CheckMany returns a map of cveID → isKEV for the given slice.
func (r *kevRepository) CheckMany(ctx context.Context, cveIDs []string) (map[string]bool, error) {
	result := make(map[string]bool, len(cveIDs))
	for _, id := range cveIDs {
		result[id] = false
	}

	rows, err := r.db.Query(ctx,
		"SELECT cve_id FROM kev_entries WHERE cve_id = ANY($1::text[])",
		cveIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("kev check many: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, rows.Err()
}

// GetAllIDs returns all CVE IDs in the catalog.
func (r *kevRepository) GetAllIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, "SELECT cve_id FROM kev_entries ORDER BY cve_id")
	if err != nil {
		return nil, fmt.Errorf("kev get all ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Count returns total entries.
func (r *kevRepository) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM kev_entries").Scan(&n)
	return n, err
}

// Stats returns catalog statistics including extended CR-GCV-007 fields.
func (r *kevRepository) Stats(ctx context.Context) (*entity.KEVStats, error) {
	var stats entity.KEVStats

	// Basic counts
	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE date_added >= NOW() - INTERVAL '7 days') AS last7d,
			COUNT(*) FILTER (WHERE date_added >= NOW() - INTERVAL '30 days') AS last30d,
			COUNT(*) FILTER (WHERE COALESCE(is_known_ransomware, FALSE) = TRUE) AS total_ransomware
		FROM kev_entries
	`).Scan(&stats.Total, &stats.AddedLast7d, &stats.AddedLast30d, &stats.TotalRansomware)
	if err != nil {
		return nil, fmt.Errorf("kev stats: %w", err)
	}

	// Top 10 vendors by KEV count
	vendorRows, err := r.db.Query(ctx, `
		SELECT vendor_project, COUNT(*) as count
		FROM kev_entries
		GROUP BY vendor_project
		ORDER BY count DESC
		LIMIT 10
	`)
	if err == nil {
		defer vendorRows.Close()
		for vendorRows.Next() {
			var vc entity.VendorCount
			if vendorRows.Scan(&vc.Vendor, &vc.Count) == nil {
				stats.TopVendors = append(stats.TopVendors, vc)
			}
		}
	}

	// Entries by month (last 12 months)
	monthRows, err := r.db.Query(ctx, `
		SELECT to_char(date_added, 'YYYY-MM') AS month, COUNT(*) AS count
		FROM kev_entries
		WHERE date_added >= NOW() - INTERVAL '12 months'
		  AND date_added IS NOT NULL
		GROUP BY month
		ORDER BY month ASC
	`)
	if err == nil {
		defer monthRows.Close()
		for monthRows.Next() {
			var mc entity.MonthCount
			if monthRows.Scan(&mc.Month, &mc.Count) == nil {
				stats.ByMonth = append(stats.ByMonth, mc)
			}
		}
	}

	// Average days from CVE publication to CISA KEV addition.
	// [FIX TASK-HC-003] Compute from JOIN kev_entries × cves (PostgreSQL cves table).
	// Falls back to 0 gracefully when data is unavailable.
	var avgDays float64
	avgErr := r.db.QueryRow(ctx, `
		SELECT COALESCE(
			AVG(
				EXTRACT(EPOCH FROM (k.date_added::timestamptz - c.published_at)) / 86400.0
			)::INT,
			0
		)
		FROM kev_entries k
		JOIN cves c ON c.cve_id = k.cve_id
		WHERE c.published_at IS NOT NULL
		  AND k.date_added IS NOT NULL
		  AND k.date_added > c.published_at
	`).Scan(&avgDays)
	if avgErr == nil {
		stats.AvgDaysToPatch = avgDays
	}
	// If avgErr != nil (e.g., table missing or no join data), stats.AvgDaysToPatch stays 0 (safe default)

	return &stats, nil
}

// DeleteByIDs removes KEV entries whose CVE ID is NOT in the keepIDs list.
// Used by diff-based sync to prune entries CISA has removed.
func (r *kevRepository) DeleteByIDs(ctx context.Context, keepIDs []string) (deleted int, err error) {
	if len(keepIDs) == 0 {
		return 0, nil // safety: never delete all if keepIDs is empty
	}

	tag, err := r.db.Exec(ctx,
		"DELETE FROM kev_entries WHERE cve_id != ALL($1::text[])",
		keepIDs2pgArray(keepIDs),
	)
	if err != nil {
		return 0, fmt.Errorf("kev delete stale: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// keepIDs2pgArray converts a string slice to a pgx-compatible array.
func keepIDs2pgArray(ids []string) interface{} {
	// pgx/v5 handles []string directly for ANY() queries
	return ids
}
