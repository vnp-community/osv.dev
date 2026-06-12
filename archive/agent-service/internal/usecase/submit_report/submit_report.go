// Package submitreport provides the submit agent report use case.
package submitreport

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/osv/agent-service/internal/domain/entity"
	"github.com/osv/agent-service/internal/domain/repository"
)

// Input is the agent report submission payload.
type Input struct {
	APIKeyID  uuid.UUID // from auth validation
	UserID    uuid.UUID // from auth validation
	Hostname  string
	IPAddress string
	OSInfo    string
	KernelVersion string
	Packages  []entity.Package
}

// Output is returned after report processing.
type Output struct {
	ReportID     uuid.UUID
	AgentID      uuid.UUID
	PackageCount int
	CVECount     int
	Message      string
}

// Publisher publishes agent-related events.
type Publisher interface {
	PublishReportSubmitted(ctx context.Context, agentID, reportID uuid.UUID, unenrichedPkgs []entity.Package) error
	PublishAgentRegistered(ctx context.Context, agent *entity.Agent) error
}

// UseCase orchestrates agent report submission.
type UseCase struct {
	agentRepo  repository.AgentRepository
	reportRepo repository.AgentReportRepository
	pkgRepo    repository.PackageRepository
	publisher  Publisher
}

// NewUseCase creates a SubmitReport use case.
func NewUseCase(
	agentRepo repository.AgentRepository,
	reportRepo repository.AgentReportRepository,
	pkgRepo repository.PackageRepository,
	publisher Publisher,
) *UseCase {
	return &UseCase{agentRepo: agentRepo, reportRepo: reportRepo, pkgRepo: pkgRepo, publisher: publisher}
}

// Execute processes a report submission from an agent.
func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	// 1. Find or auto-create agent
	agent, err := uc.agentRepo.FindByAPIKeyID(ctx, in.APIKeyID)
	isNew := false
	if err != nil {
		// Auto-register agent on first report
		agent = &entity.Agent{
			ID:        uuid.New(),
			Hostname:  in.Hostname,
			IPAddress: in.IPAddress,
			APIKeyID:  in.APIKeyID,
			Status:    entity.AgentStatusActive,
			CreatedAt: time.Now().UTC(),
		}
		if err := uc.agentRepo.Create(ctx, agent); err != nil {
			return nil, err
		}
		isNew = true
	}

	// 2. Create report record
	now := time.Now().UTC()
	report := &entity.AgentReport{
		ID:            uuid.New(),
		AgentID:       agent.ID,
		Hostname:      in.Hostname,
		IPAddress:     in.IPAddress,
		OSInfo:        in.OSInfo,
		KernelVersion: in.KernelVersion,
		Packages:      in.Packages,
		ReportedAt:    now,
		CreatedAt:     now,
	}
	if err := uc.reportRepo.Create(ctx, report); err != nil {
		return nil, err
	}

	// 3. Store packages
	var unenriched []entity.Package
	cveCount := 0
	for i := range report.Packages {
		report.Packages[i].ReportID = report.ID
		if len(report.Packages[i].CVEs) > 0 {
			cveCount += len(report.Packages[i].CVEs)
		} else {
			unenriched = append(unenriched, report.Packages[i])
		}
	}
	if len(in.Packages) > 0 {
		uc.pkgRepo.CreateBatch(ctx, report.Packages) //nolint:errcheck
	}

	// 4. Update agent last seen
	uc.agentRepo.UpdateLastSeen(ctx, agent.ID) //nolint:errcheck

	// 5. Publish events
	if isNew && uc.publisher != nil {
		uc.publisher.PublishAgentRegistered(ctx, agent) //nolint:errcheck
	}
	if uc.publisher != nil {
		uc.publisher.PublishReportSubmitted(ctx, agent.ID, report.ID, unenriched) //nolint:errcheck
	}

	return &Output{
		ReportID:     report.ID,
		AgentID:      agent.ID,
		PackageCount: len(in.Packages),
		CVECount:     cveCount,
		Message:      "report processed successfully",
	}, nil
}
