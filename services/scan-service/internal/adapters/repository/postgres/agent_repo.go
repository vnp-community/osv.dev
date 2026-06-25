package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/google/uuid"
	agententity "github.com/osv/scan-service/internal/domain/agent/entity"
	agentrepo "github.com/osv/scan-service/internal/domain/agent/repository"
)

// AgentRepo implements agentrepo.AgentRepository using pgx/v5.
// Agents are stored in the scan.agents table.
type AgentRepo struct{ db *pgxpool.Pool }

// NewAgentRepo creates an AgentRepo backed by the given pgx pool.
func NewAgentRepo(db *pgxpool.Pool) *AgentRepo { return &AgentRepo{db: db} }

// Compile-time interface check.
var _ agentrepo.AgentRepository = (*AgentRepo)(nil)

// Create inserts a new agent record.
func (r *AgentRepo) Create(ctx context.Context, a *agententity.Agent) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	now := time.Now().UTC()
	a.CreatedAt = now
	a.UpdatedAt = now

	_, err := r.db.Exec(ctx, `
		INSERT INTO scan.agents
		  (id, name, hostname, ip_address, os, agent_version, api_key_id, status, tags, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)
	`, a.ID, a.Name, a.Hostname, a.IPAddress, a.OS, a.AgentVersion,
		a.APIKeyID, string(a.Status), a.Tags, now)
	return err
}

// FindByID returns an agent by its UUID.
func (r *AgentRepo) FindByID(ctx context.Context, id uuid.UUID) (*agententity.Agent, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, name, hostname, ip_address, os, agent_version, api_key_id,
		       status, last_seen_at, tags, created_at, updated_at
		FROM scan.agents WHERE id = $1
	`, id)
	return scanAgentRow(row)
}

// FindByAPIKeyID returns an agent by its associated API key UUID.
func (r *AgentRepo) FindByAPIKeyID(ctx context.Context, apiKeyID uuid.UUID) (*agententity.Agent, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, name, hostname, ip_address, os, agent_version, api_key_id,
		       status, last_seen_at, tags, created_at, updated_at
		FROM scan.agents WHERE api_key_id = $1
	`, apiKeyID)
	return scanAgentRow(row)
}

// UpdateLastSeen sets last_seen_at = NOW() and status = 'active' for the agent.
func (r *AgentRepo) UpdateLastSeen(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		UPDATE scan.agents
		SET last_seen_at = $2, status = 'active', updated_at = $2
		WHERE id = $1
	`, id, now)
	return err
}

// UpdateStatus changes the agent status.
func (r *AgentRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status agententity.AgentStatus) error {
	_, err := r.db.Exec(ctx, `
		UPDATE scan.agents SET status = $2, updated_at = NOW() WHERE id = $1
	`, id, string(status))
	return err
}

// List returns a paginated list of agents with optional filtering.
func (r *AgentRepo) List(ctx context.Context, filter agentrepo.AgentFilter) ([]*agententity.Agent, int64, error) {
	page, pageSize := filter.Page, filter.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	args := []any{}
	where := "WHERE 1=1"
	if filter.Status != nil {
		args = append(args, string(*filter.Status))
		where += " AND status = $" + itoa(len(args))
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		where += " AND (name ILIKE $" + itoa(len(args)) +
			" OR hostname ILIKE $" + itoa(len(args)) + ")"
	}

	var total int64
	countArgs := append([]any{}, args...)
	if err := r.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM scan.agents "+where, countArgs...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, offset)
	rows, err := r.db.Query(ctx, `
		SELECT id, name, hostname, ip_address, os, agent_version, api_key_id,
		       status, last_seen_at, tags, created_at, updated_at
		FROM scan.agents `+where+`
		ORDER BY created_at DESC
		LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)),
		args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var agents []*agententity.Agent
	for rows.Next() {
		a, err := scanAgentRow(rows)
		if err != nil {
			continue
		}
		agents = append(agents, a)
	}
	return agents, total, rows.Err()
}

// Delete removes an agent record.
func (r *AgentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM scan.agents WHERE id = $1`, id)
	return err
}

// scanAgentRow scans one agent row from either pgx.Row or pgx.Rows.
func scanAgentRow(row interface{ Scan(...any) error }) (*agententity.Agent, error) {
	a := &agententity.Agent{}
	var status string
	err := row.Scan(
		&a.ID, &a.Name, &a.Hostname, &a.IPAddress,
		&a.OS, &a.AgentVersion, &a.APIKeyID,
		&status, &a.LastSeenAt, &a.Tags,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	a.Status = agententity.AgentStatus(status)
	return a, nil
}

// itoa converts an int to its string decimal representation (avoids fmt import).
func itoa(i int) string {
	const digits = "0123456789"
	if i < 10 {
		return string(digits[i])
	}
	// Simple two-digit case (sufficient for SQL arg indices)
	return string(digits[i/10]) + string(digits[i%10])
}
