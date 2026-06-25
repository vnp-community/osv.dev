// Package postgres provides the PostgreSQL implementation for jira_issue_mappings.
// TASK-HC-013: Replace stub JIRA issue handlers with real DB-backed CRUD.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IssueMappingRecord is the DTO for jira_issue_mappings rows.
type IssueMappingRecord struct {
	ID                   uuid.UUID  `json:"id"`
	FindingID            uuid.UUID  `json:"finding_id"`
	JiraConfigurationID  *uuid.UUID `json:"jira_configuration_id,omitempty"`
	JiraID               string     `json:"jira_id"`
	JiraKey              string     `json:"jira_key"`
	JiraURL              string     `json:"jira_url"`
	JiraStatus           string     `json:"jira_status"`
	JiraPriority         string     `json:"jira_priority"`
	Synced               bool       `json:"synced"`
	LastSyncAt           *time.Time `json:"last_sync_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

// IssueMappingRepo persists and retrieves JIRA issue mappings.
type IssueMappingRepo struct {
	pool *pgxpool.Pool
}

// NewIssueMappingRepo creates a new IssueMappingRepo.
func NewIssueMappingRepo(pool *pgxpool.Pool) *IssueMappingRepo {
	return &IssueMappingRepo{pool: pool}
}

// Save upserts an issue mapping.
func (r *IssueMappingRepo) Save(ctx context.Context, m *IssueMappingRecord) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	m.CreatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO jira_issue_mappings
		  (id, finding_id, jira_configuration_id, jira_id, jira_key, jira_url,
		   jira_status, jira_priority, synced, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (finding_id) DO UPDATE
		  SET jira_id     = EXCLUDED.jira_id,
		      jira_key    = EXCLUDED.jira_key,
		      jira_url    = EXCLUDED.jira_url,
		      jira_status = EXCLUDED.jira_status,
		      synced      = TRUE,
		      last_sync_at = NOW()
	`,
		m.ID, m.FindingID, m.JiraConfigurationID, m.JiraID, m.JiraKey, m.JiraURL,
		m.JiraStatus, m.JiraPriority, m.Synced, m.CreatedAt,
	)
	return err
}

// FindByFindingID retrieves a mapping by finding UUID.
func (r *IssueMappingRepo) FindByFindingID(ctx context.Context, findingID uuid.UUID) (*IssueMappingRecord, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, finding_id, jira_configuration_id, jira_id, jira_key, jira_url,
		       jira_status, jira_priority, synced, last_sync_at, created_at
		FROM jira_issue_mappings WHERE finding_id = $1
	`, findingID)

	m := &IssueMappingRecord{}
	if err := row.Scan(
		&m.ID, &m.FindingID, &m.JiraConfigurationID, &m.JiraID, &m.JiraKey, &m.JiraURL,
		&m.JiraStatus, &m.JiraPriority, &m.Synced, &m.LastSyncAt, &m.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("jira_issue_mappings.FindByFindingID: %w", err)
	}
	return m, nil
}

// FindByID retrieves a mapping by its own UUID.
func (r *IssueMappingRepo) FindByID(ctx context.Context, id uuid.UUID) (*IssueMappingRecord, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, finding_id, jira_configuration_id, jira_id, jira_key, jira_url,
		       jira_status, jira_priority, synced, last_sync_at, created_at
		FROM jira_issue_mappings WHERE id = $1
	`, id)

	m := &IssueMappingRecord{}
	if err := row.Scan(
		&m.ID, &m.FindingID, &m.JiraConfigurationID, &m.JiraID, &m.JiraKey, &m.JiraURL,
		&m.JiraStatus, &m.JiraPriority, &m.Synced, &m.LastSyncAt, &m.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("jira_issue_mappings.FindByID: %w", err)
	}
	return m, nil
}

// List returns all mappings with pagination.
func (r *IssueMappingRepo) List(ctx context.Context, page, pageSize int) ([]*IssueMappingRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM jira_issue_mappings`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, finding_id, jira_configuration_id, jira_id, jira_key, jira_url,
		       jira_status, jira_priority, synced, last_sync_at, created_at
		FROM jira_issue_mappings
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []*IssueMappingRecord
	for rows.Next() {
		m := &IssueMappingRecord{}
		if err := rows.Scan(
			&m.ID, &m.FindingID, &m.JiraConfigurationID, &m.JiraID, &m.JiraKey, &m.JiraURL,
			&m.JiraStatus, &m.JiraPriority, &m.Synced, &m.LastSyncAt, &m.CreatedAt,
		); err != nil {
			continue
		}
		result = append(result, m)
	}
	return result, total, rows.Err()
}

// Delete removes a mapping by ID.
func (r *IssueMappingRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM jira_issue_mappings WHERE id = $1`, id)
	return err
}
