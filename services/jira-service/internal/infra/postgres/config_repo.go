package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/jira-service/internal/domain/jiraconfig"
)

type ConfigRepo struct {
	pool *pgxpool.Pool
}

func NewConfigRepo(pool *pgxpool.Pool) *ConfigRepo {
	return &ConfigRepo{pool: pool}
}

// Save upserts a JIRA config. DB column is `api_token` (stores encrypted value).
func (r *ConfigRepo) Save(ctx context.Context, cfg *jiraconfig.JIRAConfig) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO jira_configs (id, product_id, server_url, username, api_token, project_key, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			server_url  = EXCLUDED.server_url,
			username    = EXCLUDED.username,
			api_token   = EXCLUDED.api_token,
			project_key = EXCLUDED.project_key,
			enabled     = EXCLUDED.enabled,
			updated_at  = NOW()
	`, cfg.ID, cfg.ProductID, cfg.URL, cfg.Username, cfg.PasswordEnc, cfg.ProjectKey, cfg.IsActive, cfg.CreatedAt, cfg.UpdatedAt)
	if err != nil {
		return fmt.Errorf("config_repo.Save: %w", err)
	}
	return nil
}

func (r *ConfigRepo) FindByID(ctx context.Context, id uuid.UUID) (*jiraconfig.JIRAConfig, error) {
	cfg := &jiraconfig.JIRAConfig{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, product_id, server_url, username, api_token, project_key, enabled, created_at, updated_at
		FROM jira_configs WHERE id = $1
	`, id).Scan(&cfg.ID, &cfg.ProductID, &cfg.URL, &cfg.Username, &cfg.PasswordEnc, &cfg.ProjectKey, &cfg.IsActive, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (r *ConfigRepo) FindByProduct(ctx context.Context, productID uuid.UUID) (*jiraconfig.JIRAConfig, error) {
	cfg := &jiraconfig.JIRAConfig{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, product_id, server_url, username, api_token, project_key, enabled, created_at, updated_at
		FROM jira_configs WHERE product_id = $1 LIMIT 1
	`, productID).Scan(&cfg.ID, &cfg.ProductID, &cfg.URL, &cfg.Username, &cfg.PasswordEnc, &cfg.ProjectKey, &cfg.IsActive, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (r *ConfigRepo) List(ctx context.Context, limit, offset int) ([]*jiraconfig.JIRAConfig, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM jira_configs`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, product_id, server_url, username, api_token, project_key, enabled, created_at, updated_at
		FROM jira_configs
		ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var cfgs []*jiraconfig.JIRAConfig
	for rows.Next() {
		cfg := &jiraconfig.JIRAConfig{}
		if err := rows.Scan(&cfg.ID, &cfg.ProductID, &cfg.URL, &cfg.Username, &cfg.PasswordEnc, &cfg.ProjectKey, &cfg.IsActive, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
			return nil, 0, err
		}
		cfgs = append(cfgs, cfg)
	}
	return cfgs, total, nil
}

// Delete performs a hard delete of the JIRA configuration by ID.
func (r *ConfigRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM jira_configs WHERE id = $1`, id)
	return err
}
