package repository

import (
	"context"
	"github.com/google/uuid"
	"github.com/osv/agent-service/internal/domain/entity"
)

// AgentFilter defines filtering for listing agents.
type AgentFilter struct {
	Status   *entity.AgentStatus
	Search   string
	Page     int
	PageSize int
}

// AgentRepository defines persistence for agents.
type AgentRepository interface {
	Create(ctx context.Context, a *entity.Agent) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Agent, error)
	FindByAPIKeyID(ctx context.Context, apiKeyID uuid.UUID) (*entity.Agent, error)
	UpdateLastSeen(ctx context.Context, id uuid.UUID) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status entity.AgentStatus) error
	List(ctx context.Context, filter AgentFilter) ([]*entity.Agent, int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// AgentReportRepository defines persistence for agent reports.
type AgentReportRepository interface {
	Create(ctx context.Context, r *entity.AgentReport) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.AgentReport, error)
	FindByAgentID(ctx context.Context, agentID uuid.UUID, page, pageSize int) ([]*entity.AgentReport, int64, error)
	FindLatestByAgentID(ctx context.Context, agentID uuid.UUID) (*entity.AgentReport, error)
	MarkProcessed(ctx context.Context, id uuid.UUID) error
}

// PackageRepository defines persistence for installed packages.
type PackageRepository interface {
	CreateBatch(ctx context.Context, packages []entity.Package) error
	FindByReportID(ctx context.Context, reportID uuid.UUID) ([]entity.Package, error)
}
