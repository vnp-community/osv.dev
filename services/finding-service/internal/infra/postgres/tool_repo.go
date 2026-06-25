package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/tool"
)

// ToolConfigRepo implements tool.ToolConfigurationRepository using pgxpool.
type ToolConfigRepo struct {
	pool *pgxpool.Pool
}

// NewToolConfigRepo creates a new ToolConfigRepo.
func NewToolConfigRepo(pool *pgxpool.Pool) *ToolConfigRepo {
	return &ToolConfigRepo{pool: pool}
}

// Save inserts or updates a ToolConfiguration (upsert by id).
func (r *ToolConfigRepo) Save(ctx context.Context, tc *tool.ToolConfiguration) error {
	tc.UpdatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tool_configurations (id, name, description, tool_type, url, auth_type, username, password_enc, api_key_enc, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			tool_type = EXCLUDED.tool_type,
			url = EXCLUDED.url,
			auth_type = EXCLUDED.auth_type,
			username = EXCLUDED.username,
			password_enc = EXCLUDED.password_enc,
			api_key_enc = EXCLUDED.api_key_enc,
			updated_at = EXCLUDED.updated_at
	`, tc.ID, tc.Name, tc.Description, tc.ToolType, tc.URL,
		string(tc.AuthType), tc.Username, tc.PasswordEnc, tc.APIKeyEnc,
		tc.CreatedAt, tc.UpdatedAt)
	return err
}

// FindByID finds a ToolConfiguration by ID.
func (r *ToolConfigRepo) FindByID(ctx context.Context, id uuid.UUID) (*tool.ToolConfiguration, error) {
	tc := &tool.ToolConfiguration{}
	var authTypeStr string
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, description, tool_type, url, auth_type, username, password_enc, api_key_enc, created_at, updated_at
		FROM tool_configurations WHERE id=$1
	`, id).Scan(
		&tc.ID, &tc.Name, &tc.Description, &tc.ToolType, &tc.URL,
		&authTypeStr, &tc.Username, &tc.PasswordEnc, &tc.APIKeyEnc,
		&tc.CreatedAt, &tc.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("tool configuration not found: %w", err)
	}
	tc.AuthType = tool.AuthType(authTypeStr)
	return tc, nil
}

// List returns all ToolConfigurations.
func (r *ToolConfigRepo) List(ctx context.Context) ([]*tool.ToolConfiguration, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, description, tool_type, url, auth_type, username, password_enc, api_key_enc, created_at, updated_at
		FROM tool_configurations ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []*tool.ToolConfiguration
	for rows.Next() {
		tc := &tool.ToolConfiguration{}
		var authTypeStr string
		if err := rows.Scan(
			&tc.ID, &tc.Name, &tc.Description, &tc.ToolType, &tc.URL,
			&authTypeStr, &tc.Username, &tc.PasswordEnc, &tc.APIKeyEnc,
			&tc.CreatedAt, &tc.UpdatedAt,
		); err != nil {
			return nil, err
		}
		tc.AuthType = tool.AuthType(authTypeStr)
		tools = append(tools, tc)
	}
	return tools, rows.Err()
}

// Delete removes a ToolConfiguration by ID.
func (r *ToolConfigRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM tool_configurations WHERE id=$1`, id)
	return err
}
