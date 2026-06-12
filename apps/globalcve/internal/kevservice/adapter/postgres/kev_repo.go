// Package postgres — KEV repository implementation.
package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/globalcve/mono/internal/kevservice/domain/entity"
)

// KEVRepo is the PostgreSQL implementation of KEVRepository.
type KEVRepo struct {
	pool *pgxpool.Pool
}

// NewKEVRepo creates a new PostgreSQL KEV repository.
func NewKEVRepo(pool *pgxpool.Pool) *KEVRepo {
	return &KEVRepo{pool: pool}
}

// UpsertBatch inserts or updates KEV entries in batches of 100.
func (r *KEVRepo) UpsertBatch(ctx context.Context, entries []*entity.KEVEntry) (inserted, updated int, err error) {
	const chunkSize = 100
	for i := 0; i < len(entries); i += chunkSize {
		end := i + chunkSize
		if end > len(entries) {
			end = len(entries)
		}
		ins, upd, err := r.upsertChunk(ctx, entries[i:end])
		if err != nil {
			return inserted, updated, err
		}
		inserted += ins
		updated += upd
	}
	return inserted, updated, nil
}

func (r *KEVRepo) upsertChunk(ctx context.Context, entries []*entity.KEVEntry) (inserted, updated int, err error) {
	const cols = `cve_id, vendor_project, product, vulnerability_name, short_description,
		required_action, date_added, due_date, known_ransomware, notes, updated_at`

	placeholders := make([]string, 0, len(entries))
	args := make([]interface{}, 0, len(entries)*11)
	idx := 1

	for _, e := range entries {
		placeholders = append(placeholders, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,NOW())",
			idx, idx+1, idx+2, idx+3, idx+4, idx+5, idx+6, idx+7, idx+8, idx+9,
		))
		idx += 10

		var dateAdded, dueDatePtr interface{}
		if !e.DateAdded.IsZero() {
			dateAdded = e.DateAdded
		}
		if !e.DueDate.IsZero() {
			dueDatePtr = e.DueDate
		}

		args = append(args,
			e.CVEID, e.VendorProject, e.Product, e.VulnerabilityName,
			e.ShortDescription, e.RequiredAction, dateAdded, dueDatePtr,
			e.KnownRansomware, e.Notes,
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO kev_entries (%s)
		VALUES %s
		ON CONFLICT (cve_id) DO UPDATE SET
			vendor_project     = EXCLUDED.vendor_project,
			product            = EXCLUDED.product,
			vulnerability_name = EXCLUDED.vulnerability_name,
			short_description  = EXCLUDED.short_description,
			required_action    = EXCLUDED.required_action,
			date_added         = COALESCE(EXCLUDED.date_added, kev_entries.date_added),
			due_date           = COALESCE(EXCLUDED.due_date, kev_entries.due_date),
			known_ransomware   = EXCLUDED.known_ransomware,
			notes              = EXCLUDED.notes,
			updated_at         = NOW()
		RETURNING (xmax = 0) AS is_insert`,
		cols, strings.Join(placeholders, ","),
	)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("upsert kev chunk: %w", err)
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

// FindByCVEID retrieves a KEV entry by CVE ID.
func (r *KEVRepo) FindByCVEID(ctx context.Context, cveID string) (*entity.KEVEntry, error) {
	query := `
		SELECT cve_id, vendor_project, product, vulnerability_name, short_description,
		       required_action, date_added, due_date, known_ransomware, notes, created_at, updated_at
		FROM kev_entries WHERE cve_id = $1`

	row := r.pool.QueryRow(ctx, query, cveID)
	e := &entity.KEVEntry{}
	err := row.Scan(
		&e.CVEID, &e.VendorProject, &e.Product, &e.VulnerabilityName, &e.ShortDescription,
		&e.RequiredAction, &e.DateAdded, &e.DueDate, &e.KnownRansomware, &e.Notes,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("find kev by id: %w", err)
	}
	return e, nil
}

// List retrieves KEV entries with optional filtering and pagination.
func (r *KEVRepo) List(ctx context.Context, filter *entity.KEVFilter) ([]*entity.KEVEntry, int64, error) {
	filter.Validate()

	var where []string
	var args []interface{}
	idx := 1

	if filter.VendorProject != "" {
		where = append(where, fmt.Sprintf("vendor_project ILIKE $%d", idx))
		args = append(args, "%"+filter.VendorProject+"%")
		idx++
	}
	if filter.Since != nil {
		where = append(where, fmt.Sprintf("date_added >= $%d", idx))
		args = append(args, *filter.Since)
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	// Count
	var total int64
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM kev_entries"+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Paginate
	offset := filter.Page * filter.Limit
	query := fmt.Sprintf(`
		SELECT cve_id, vendor_project, product, vulnerability_name, short_description,
		       required_action, date_added, due_date, known_ransomware, notes, created_at, updated_at
		FROM kev_entries %s
		ORDER BY date_added DESC NULLS LAST
		LIMIT $%d OFFSET $%d`,
		whereClause, idx, idx+1,
	)
	args = append(args, filter.Limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []*entity.KEVEntry
	for rows.Next() {
		e := &entity.KEVEntry{}
		if err := rows.Scan(
			&e.CVEID, &e.VendorProject, &e.Product, &e.VulnerabilityName, &e.ShortDescription,
			&e.RequiredAction, &e.DateAdded, &e.DueDate, &e.KnownRansomware, &e.Notes,
			&e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

// CheckMany checks which CVE IDs are in the KEV catalog.
func (r *KEVRepo) CheckMany(ctx context.Context, cveIDs []string) (map[string]bool, error) {
	if len(cveIDs) == 0 {
		return map[string]bool{}, nil
	}

	placeholders := make([]string, len(cveIDs))
	args := make([]interface{}, len(cveIDs))
	for i, id := range cveIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`SELECT cve_id FROM kev_entries WHERE cve_id IN (%s)`,
		strings.Join(placeholders, ","))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool, len(cveIDs))
	for _, id := range cveIDs {
		result[id] = false
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			result[id] = true
		}
	}
	return result, rows.Err()
}

// GetAllIDs returns all CVE IDs in the KEV catalog.
func (r *KEVRepo) GetAllIDs(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT cve_id FROM kev_entries ORDER BY date_added DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// Count returns the total number of KEV entries.
func (r *KEVRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM kev_entries`).Scan(&count)
	return count, err
}

// Stats returns KEV catalog statistics.
func (r *KEVRepo) Stats(ctx context.Context) (*entity.KEVStats, error) {
	var stats entity.KEVStats
	var lastSync *time.Time

	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE date_added >= NOW() - '7 days'::interval) AS added_last_7_days,
			COUNT(*) FILTER (WHERE date_added >= NOW() - '30 days'::interval) AS added_last_30_days,
			MAX(updated_at) AS last_sync
		FROM kev_entries`,
	).Scan(&stats.Total, &stats.AddedLast7d, &stats.AddedLast30d, &lastSync)
	if err != nil {
		return nil, err
	}
	if lastSync != nil {
		stats.LastSyncAt = *lastSync
	}
	return &stats, nil
}
