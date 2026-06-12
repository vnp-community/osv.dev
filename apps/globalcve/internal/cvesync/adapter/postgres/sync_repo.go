// Package postgres — SyncJob repository implementation.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/globalcve/mono/internal/cvesync/domain/entity"
)

// SyncRepo is the PostgreSQL implementation of SyncJobRepository.
type SyncRepo struct {
	pool *pgxpool.Pool
}

// NewSyncRepo creates a new PostgreSQL sync job repository.
func NewSyncRepo(pool *pgxpool.Pool) *SyncRepo {
	return &SyncRepo{pool: pool}
}

// Create inserts a new sync job record.
func (r *SyncRepo) Create(ctx context.Context, job *entity.SyncJob) error {
	query := `
		INSERT INTO sync_jobs (source, status, started_at, synced, skipped, errors, error_msg)
		VALUES ($1, $2, $3, 0, 0, 0, '')
		RETURNING id`

	err := r.pool.QueryRow(ctx, query, string(job.Source), string(job.Status), job.StartedAt).Scan(&job.ID)
	if err != nil {
		return fmt.Errorf("create sync job: %w", err)
	}
	return nil
}

// UpdateStatus updates the status, stats, and completed_at timestamp of a sync job.
func (r *SyncRepo) UpdateStatus(ctx context.Context, id int64, status entity.SyncStatus, stats entity.SyncStats) error {
	now := time.Now()
	query := `
		UPDATE sync_jobs SET
			status       = $1,
			completed_at = $2,
			synced       = $3,
			skipped      = $4,
			errors       = $5,
			error_msg    = $6
		WHERE id = $7`

	_, err := r.pool.Exec(ctx, query,
		string(status), now,
		stats.Synced, stats.Skipped, stats.Errors, stats.ErrorMsg,
		id,
	)
	return err
}

// GetLatest retrieves the most recent sync job for a source.
func (r *SyncRepo) GetLatest(ctx context.Context, source entity.SourceName) (*entity.SyncJob, error) {
	query := `
		SELECT id, source, status, started_at, completed_at, synced, skipped, errors, error_msg
		FROM sync_jobs
		WHERE source = $1
		ORDER BY started_at DESC
		LIMIT 1`

	row := r.pool.QueryRow(ctx, query, string(source))
	job := &entity.SyncJob{}
	err := row.Scan(
		&job.ID, &job.Source, &job.Status,
		&job.StartedAt, &job.CompletedAt,
		&job.Synced, &job.Skipped, &job.Errors, &job.ErrorMsg,
	)
	if err != nil {
		return nil, fmt.Errorf("get latest sync job: %w", err)
	}
	return job, nil
}

// List retrieves the most recent sync jobs across all sources.
func (r *SyncRepo) List(ctx context.Context, limit int) ([]*entity.SyncJob, error) {
	query := `
		SELECT id, source, status, started_at, completed_at, synced, skipped, errors, error_msg
		FROM sync_jobs
		ORDER BY started_at DESC
		LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list sync jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*entity.SyncJob
	for rows.Next() {
		job := &entity.SyncJob{}
		if err := rows.Scan(
			&job.ID, &job.Source, &job.Status,
			&job.StartedAt, &job.CompletedAt,
			&job.Synced, &job.Skipped, &job.Errors, &job.ErrorMsg,
		); err != nil {
			return nil, fmt.Errorf("scan sync job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}
