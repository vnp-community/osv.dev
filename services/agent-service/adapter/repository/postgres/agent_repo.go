package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/agent-service/internal/domain/entity"
	"github.com/osv/agent-service/internal/domain/repository"
)

// AgentRepo implements repository.AgentRepository.
type AgentRepo struct{ db *pgxpool.Pool }

func NewAgentRepo(db *pgxpool.Pool) *AgentRepo { return &AgentRepo{db: db} }

func (r *AgentRepo) Create(ctx context.Context, a *entity.Agent) error {
	if a.ID == uuid.Nil { a.ID = uuid.New() }
	a.CreatedAt = time.Now().UTC(); a.UpdatedAt = a.CreatedAt
	tags, _ := json.Marshal(a.Tags)
	_, err := r.db.Exec(ctx, `
		INSERT INTO agent.agents (id,name,hostname,ip_address,os,agent_version,api_key_id,status,tags,created_at,updated_at)
		VALUES ($1,$2,$3,$4::inet,$5,$6,$7,$8,$9::text[],$10,$10)`,
		a.ID, a.Name, a.Hostname, a.IPAddress, a.OS, a.AgentVersion, a.APIKeyID, a.Status, tags, a.CreatedAt,
	)
	return err
}

func (r *AgentRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.Agent, error) {
	return r.scanOne(ctx, `SELECT id,name,hostname,ip_address,os,agent_version,api_key_id,status,last_seen_at,tags,created_at,updated_at FROM agent.agents WHERE id=$1`, id)
}

func (r *AgentRepo) FindByAPIKeyID(ctx context.Context, apiKeyID uuid.UUID) (*entity.Agent, error) {
	return r.scanOne(ctx, `SELECT id,name,hostname,ip_address,os,agent_version,api_key_id,status,last_seen_at,tags,created_at,updated_at FROM agent.agents WHERE api_key_id=$1`, apiKeyID)
}

func (r *AgentRepo) scanOne(ctx context.Context, query string, arg any) (*entity.Agent, error) {
	var a entity.Agent
	var tags []byte; var ipStr *string
	err := r.db.QueryRow(ctx, query, arg).Scan(
		&a.ID, &a.Name, &a.Hostname, &ipStr, &a.OS, &a.AgentVersion,
		&a.APIKeyID, &a.Status, &a.LastSeenAt, &tags, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil { return nil, err }
	if ipStr != nil { a.IPAddress = *ipStr }
	json.Unmarshal(tags, &a.Tags)
	return &a, nil
}

func (r *AgentRepo) UpdateLastSeen(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE agent.agents SET last_seen_at=$2, status='active', updated_at=$2 WHERE id=$1`, id, now)
	return err
}

func (r *AgentRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.AgentStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE agent.agents SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

func (r *AgentRepo) List(ctx context.Context, f repository.AgentFilter) ([]*entity.Agent, int64, error) {
	page := f.Page; if page < 1 { page = 1 }
	ps := f.PageSize; if ps < 1 { ps = 20 }
	rows, err := r.db.Query(ctx,
		`SELECT id,name,hostname,ip_address,status,last_seen_at,created_at FROM agent.agents
		 ORDER BY created_at DESC LIMIT $1 OFFSET $2`, ps, (page-1)*ps)
	if err != nil { return nil, 0, err }
	defer rows.Close()
	var agents []*entity.Agent
	for rows.Next() {
		var a entity.Agent; var ipStr *string
		rows.Scan(&a.ID, &a.Name, &a.Hostname, &ipStr, &a.Status, &a.LastSeenAt, &a.CreatedAt)
		if ipStr != nil { a.IPAddress = *ipStr }
		agents = append(agents, &a)
	}
	return agents, int64(len(agents)), rows.Err()
}

func (r *AgentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM agent.agents WHERE id=$1`, id)
	return err
}

// ReportRepo implements repository.AgentReportRepository.
type ReportRepo struct{ db *pgxpool.Pool }

func NewReportRepo(db *pgxpool.Pool) *ReportRepo { return &ReportRepo{db: db} }

func (r *ReportRepo) Create(ctx context.Context, rep *entity.AgentReport) error {
	if rep.ID == uuid.Nil { rep.ID = uuid.New() }
	rep.CreatedAt = time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		INSERT INTO agent.agent_reports (id,agent_id,hostname,ip_address,os_info,kernel_version,package_count,cve_count,reported_at,created_at)
		VALUES ($1,$2,$3,$4::inet,$5,$6,$7,$8,$9,$10)`,
		rep.ID, rep.AgentID, rep.Hostname, nilStr(rep.IPAddress),
		rep.OSInfo, rep.KernelVersion, rep.PackageCount, rep.CVECount,
		rep.ReportedAt, rep.CreatedAt,
	)
	return err
}

func (r *ReportRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.AgentReport, error) {
	var rep entity.AgentReport; var ipStr *string
	err := r.db.QueryRow(ctx,
		`SELECT id,agent_id,hostname,ip_address,os_info,kernel_version,package_count,cve_count,reported_at,processed_at,created_at
		 FROM agent.agent_reports WHERE id=$1`, id).Scan(
		&rep.ID, &rep.AgentID, &rep.Hostname, &ipStr,
		&rep.OSInfo, &rep.KernelVersion, &rep.PackageCount, &rep.CVECount,
		&rep.ReportedAt, &rep.ProcessedAt, &rep.CreatedAt,
	)
	if err != nil { return nil, err }
	if ipStr != nil { rep.IPAddress = *ipStr }
	return &rep, nil
}

func (r *ReportRepo) FindByAgentID(ctx context.Context, agentID uuid.UUID, page, pageSize int) ([]*entity.AgentReport, int64, error) {
	if page < 1 { page = 1 }; if pageSize < 1 { pageSize = 20 }
	rows, err := r.db.Query(ctx,
		`SELECT id,agent_id,hostname,package_count,cve_count,reported_at,created_at
		 FROM agent.agent_reports WHERE agent_id=$1 ORDER BY reported_at DESC LIMIT $2 OFFSET $3`,
		agentID, pageSize, (page-1)*pageSize)
	if err != nil { return nil, 0, err }
	defer rows.Close()
	var reps []*entity.AgentReport
	for rows.Next() {
		var rep entity.AgentReport
		rows.Scan(&rep.ID, &rep.AgentID, &rep.Hostname, &rep.PackageCount, &rep.CVECount, &rep.ReportedAt, &rep.CreatedAt)
		reps = append(reps, &rep)
	}
	return reps, int64(len(reps)), rows.Err()
}

func (r *ReportRepo) FindLatestByAgentID(ctx context.Context, agentID uuid.UUID) (*entity.AgentReport, error) {
	var rep entity.AgentReport
	err := r.db.QueryRow(ctx,
		`SELECT id,agent_id,hostname,package_count,cve_count,reported_at,created_at
		 FROM agent.agent_reports WHERE agent_id=$1 ORDER BY reported_at DESC LIMIT 1`, agentID).Scan(
		&rep.ID, &rep.AgentID, &rep.Hostname, &rep.PackageCount, &rep.CVECount, &rep.ReportedAt, &rep.CreatedAt)
	if err != nil { return nil, err }
	return &rep, nil
}

func (r *ReportRepo) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE agent.agent_reports SET processed_at=NOW() WHERE id=$1`, id)
	return err
}

// PackageRepo implements repository.PackageRepository.
type PackageRepo struct{ db *pgxpool.Pool }

func NewPackageRepo(db *pgxpool.Pool) *PackageRepo { return &PackageRepo{db: db} }

func (r *PackageRepo) CreateBatch(ctx context.Context, packages []entity.Package) error {
	for _, p := range packages {
		if p.ID == uuid.Nil { p.ID = uuid.New() }
		_, err := r.db.Exec(ctx, `
			INSERT INTO agent.packages (id,report_id,name,version,ecosystem,architecture)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			p.ID, p.ReportID, p.Name, p.Version, p.Ecosystem, p.Architecture,
		)
		if err != nil { return err }
	}
	return nil
}

func (r *PackageRepo) FindByReportID(ctx context.Context, reportID uuid.UUID) ([]entity.Package, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id,report_id,name,version,ecosystem,architecture FROM agent.packages WHERE report_id=$1`, reportID)
	if err != nil { return nil, err }
	defer rows.Close()
	var pkgs []entity.Package
	for rows.Next() {
		var p entity.Package
		rows.Scan(&p.ID, &p.ReportID, &p.Name, &p.Version, &p.Ecosystem, &p.Architecture)
		pkgs = append(pkgs, p)
	}
	return pkgs, rows.Err()
}

func nilStr(s string) *string {
	if s == "" { return nil }
	return &s
}
