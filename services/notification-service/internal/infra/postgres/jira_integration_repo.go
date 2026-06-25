// Package postgres provides PostgreSQL repositories for notification-service integrations.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/osv/notification-service/internal/domain/integration"
)

// JiraIntegrationRepo implements persistence for JiraIntegration and JiraIssue.
type JiraIntegrationRepo struct {
	pool *pgxpool.Pool
}

// NewJiraIntegrationRepo creates a new JiraIntegrationRepo.
func NewJiraIntegrationRepo(pool *pgxpool.Pool) *JiraIntegrationRepo {
	return &JiraIntegrationRepo{pool: pool}
}

// Create inserts a new JiraIntegration.
func (r *JiraIntegrationRepo) Create(ctx context.Context, j *integration.JiraIntegration) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO jira_integrations (id, product_id, server_url, project_key, issue_type, api_token, auto_create, auto_sync, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		j.ID, j.ProductID, j.ServerURL, j.ProjectKey, j.IssueType, j.APIToken,
		j.AutoCreate, j.AutoSync, j.CreatedAt, j.UpdatedAt)
	return err
}

// Update updates a JiraIntegration.
func (r *JiraIntegrationRepo) Update(ctx context.Context, j *integration.JiraIntegration) error {
	j.UpdatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE jira_integrations SET
			server_url=$2, project_key=$3, issue_type=$4, api_token=$5,
			auto_create=$6, auto_sync=$7, updated_at=$8
		WHERE id=$1`,
		j.ID, j.ServerURL, j.ProjectKey, j.IssueType, j.APIToken,
		j.AutoCreate, j.AutoSync, j.UpdatedAt)
	return err
}

// Delete removes a JiraIntegration.
func (r *JiraIntegrationRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM jira_integrations WHERE id=$1`, id)
	return err
}

// FindByID retrieves a JiraIntegration by ID.
func (r *JiraIntegrationRepo) FindByID(ctx context.Context, id uuid.UUID) (*integration.JiraIntegration, error) {
	j := &integration.JiraIntegration{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, product_id, server_url, project_key, issue_type, api_token, auto_create, auto_sync, created_at, updated_at
		FROM jira_integrations WHERE id=$1`, id).Scan(
		&j.ID, &j.ProductID, &j.ServerURL, &j.ProjectKey, &j.IssueType, &j.APIToken,
		&j.AutoCreate, &j.AutoSync, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return j, err
}

// ListByProduct returns all JiraIntegrations for a product.
func (r *JiraIntegrationRepo) ListByProduct(ctx context.Context, productID uuid.UUID) ([]*integration.JiraIntegration, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, product_id, server_url, project_key, issue_type, api_token, auto_create, auto_sync, created_at, updated_at
		FROM jira_integrations WHERE product_id=$1`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*integration.JiraIntegration
	for rows.Next() {
		j := &integration.JiraIntegration{}
		if err := rows.Scan(
			&j.ID, &j.ProductID, &j.ServerURL, &j.ProjectKey, &j.IssueType, &j.APIToken,
			&j.AutoCreate, &j.AutoSync, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, j)
	}
	return results, rows.Err()
}

// CreateIssue stores a JiraIssue link.
func (r *JiraIntegrationRepo) CreateIssue(ctx context.Context, issue *integration.JiraIssue) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO jira_issues (id, finding_id, integration_id, issue_key, issue_url, status, synced_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (finding_id, integration_id) DO UPDATE SET
			issue_key=EXCLUDED.issue_key, issue_url=EXCLUDED.issue_url,
			status=EXCLUDED.status, synced_at=EXCLUDED.synced_at`,
		issue.ID, issue.FindingID, issue.IntegrationID,
		issue.IssueKey, issue.IssueURL, issue.Status, issue.SyncedAt)
	return err
}

// UpdateIssueStatus updates a JiraIssue status and sync timestamp.
func (r *JiraIntegrationRepo) UpdateIssueStatus(ctx context.Context, issueID uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE jira_issues SET status=$2, synced_at=$3 WHERE id=$1`,
		issueID, status, time.Now().UTC())
	return err
}

// ListIssuesNeedingSync returns JiraIssues that have not been synced in the last hour.
func (r *JiraIntegrationRepo) ListIssuesNeedingSync(ctx context.Context) ([]*integration.JiraIssue, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, finding_id, integration_id, issue_key, issue_url, status, synced_at
		FROM jira_issues
		WHERE synced_at < NOW() - INTERVAL '1 hour'
		  AND status NOT IN ('Done', 'Resolved', 'Closed')
		ORDER BY synced_at ASC
		LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []*integration.JiraIssue
	for rows.Next() {
		issue := &integration.JiraIssue{}
		if err := rows.Scan(
			&issue.ID, &issue.FindingID, &issue.IntegrationID,
			&issue.IssueKey, &issue.IssueURL, &issue.Status, &issue.SyncedAt); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}
