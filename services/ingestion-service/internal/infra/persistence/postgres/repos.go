// Package postgres implements the CVE write repository and sync job repository.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/osv/ingestion-service/internal/domain/entity"
	domainerrors "github.com/osv/ingestion-service/internal/domain/errors"
	"github.com/osv/ingestion-service/internal/domain/repository"
)

const batchSize = 500

// --- CVE Repository ---

type cveRepository struct{ db *pgxpool.Pool }

// NewCVERepository creates a PostgreSQL CVE write repository.
func NewCVERepository(db *pgxpool.Pool) repository.CVERepository {
	return &cveRepository{db: db}
}

// UpsertBatch inserts or updates CVEs in batches.
func (r *cveRepository) UpsertBatch(ctx context.Context, cves []*entity.CVE) (synced, skipped int, err error) {
	for i := 0; i < len(cves); i += batchSize {
		end := i + batchSize
		if end > len(cves) {
			end = len(cves)
		}
		s, sk, e := r.upsertChunk(ctx, cves[i:end])
		if e != nil {
			return synced, skipped, e
		}
		synced += s
		skipped += sk
	}
	return synced, skipped, nil
}

func (r *cveRepository) upsertChunk(ctx context.Context, chunk []*entity.CVE) (synced, skipped int, err error) {
	if len(chunk) == 0 {
		return 0, 0, nil
	}

	placeholders := make([]string, 0, len(chunk))
	args := make([]interface{}, 0, len(chunk)*9)
	idx := 1
	now := time.Now().UTC()

	for _, c := range chunk {
		placeholders = append(placeholders, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			idx, idx+1, idx+2, idx+3, idx+4, idx+5, idx+6, idx+7, idx+8,
		))
		args = append(args,
			c.ID, c.Description, c.Severity, c.Published, c.Source,
			c.IsKEV, c.Link, c.CVSSScore, now,
		)
		idx += 9
	}

	sql := fmt.Sprintf(`
		INSERT INTO cves
			(id, description, severity, published, source, is_kev, link, cvss_score, created_at, updated_at)
		VALUES %s
		ON CONFLICT (id) DO UPDATE SET
			description = EXCLUDED.description,
			severity    = EXCLUDED.severity,
			published   = COALESCE(EXCLUDED.published, cves.published),
			is_kev      = EXCLUDED.is_kev OR cves.is_kev,
			link        = EXCLUDED.link,
			cvss_score  = COALESCE(EXCLUDED.cvss_score, cves.cvss_score),
			updated_at  = NOW()
		RETURNING (xmax = 0) AS is_insert`,
		strings.Join(placeholders, ","),
	)

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("cve upsert: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var isInsert bool
		if err := rows.Scan(&isInsert); err != nil {
			return synced, skipped, err
		}
		if isInsert {
			synced++
		} else {
			skipped++
		}
	}
	return synced, skipped, rows.Err()
}

// --- Sync Job Repository ---

type syncRepository struct{ db *pgxpool.Pool }

// NewSyncRepository creates a PostgreSQL sync job repository.
func NewSyncRepository(db *pgxpool.Pool) repository.SyncRepository {
	return &syncRepository{db: db}
}

func (r *syncRepository) CreateJob(ctx context.Context, source entity.SourceName) (*entity.SyncJob, error) {
	var job entity.SyncJob
	err := r.db.QueryRow(ctx, `
		INSERT INTO sync_jobs (source, status, started_at)
		VALUES ($1, $2, $3)
		RETURNING id, source, status, started_at`,
		string(source), string(entity.SyncStatusPending), time.Now().UTC(),
	).Scan(&job.ID, &job.Source, &job.Status, &job.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("create sync job: %w", err)
	}
	return &job, nil
}

func (r *syncRepository) UpdateJobStatus(ctx context.Context, jobID int64, status entity.SyncStatus, stats *entity.SyncStats) error {
	now := time.Now().UTC()
	errMsg := ""
	if stats != nil {
		errMsg = stats.ErrorMsg
	}
	_, err := r.db.Exec(ctx, `
		UPDATE sync_jobs SET
			status       = $1,
			completed_at = $2,
			synced       = $3,
			skipped      = $4,
			errors       = $5,
			error_msg    = $6
		WHERE id = $7`,
		string(status), now,
		stats.Synced, stats.Skipped, stats.Errors, errMsg,
		jobID,
	)
	return err
}

func (r *syncRepository) GetLastJob(ctx context.Context, source entity.SourceName) (*entity.SyncJob, error) {
	var job entity.SyncJob
	err := r.db.QueryRow(ctx, `
		SELECT id, source, status, started_at, completed_at, synced, skipped, errors, error_msg
		FROM sync_jobs WHERE source = $1 ORDER BY started_at DESC LIMIT 1`,
		string(source),
	).Scan(&job.ID, &job.Source, &job.Status, &job.StartedAt, &job.CompletedAt,
		&job.Synced, &job.Skipped, &job.Errors, &job.ErrorMsg)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainerrors.ErrJobNotFound
	}
	return &job, err
}

func (r *syncRepository) ListRecentJobs(ctx context.Context, limit int) ([]*entity.SyncJob, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, source, status, started_at, completed_at, synced, skipped, errors, error_msg
		FROM sync_jobs ORDER BY started_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*entity.SyncJob
	for rows.Next() {
		j := &entity.SyncJob{}
		if err := rows.Scan(&j.ID, &j.Source, &j.Status, &j.StartedAt, &j.CompletedAt,
			&j.Synced, &j.Skipped, &j.Errors, &j.ErrorMsg); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
