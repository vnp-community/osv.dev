// Package agent provides the agent management use case for scan-service.
// Implements the agentUseCase interface expected by the delivery/http layer.
package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	agententity "github.com/osv/scan-service/internal/domain/agent/entity"
	agentrepo "github.com/osv/scan-service/internal/domain/agent/repository"
	pgadapter "github.com/osv/scan-service/internal/adapters/repository/postgres"
	deliveryhttp "github.com/osv/scan-service/internal/delivery/http"
)

// UseCase implements delivery/http.agentUseCase backed by a real PostgreSQL repository.
type UseCase struct {
	repo      agentrepo.AgentRepository
	apiKeyRepo APIKeyCreator
}

// APIKeyCreator creates an API key record and returns its ID + plaintext.
// Decoupled so identity-service can own key storage.
type APIKeyCreator interface {
	CreateAgentKey(ctx context.Context, agentID uuid.UUID, name string) (keyID uuid.UUID, plaintext string, err error)
}

// simpleKeyGen is a fallback APIKeyCreator that generates a random key
// without persisting it to identity-service (used in embedded mode).
type simpleKeyGen struct{}

func (s *simpleKeyGen) CreateAgentKey(_ context.Context, _ uuid.UUID, _ string) (uuid.UUID, string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return uuid.Nil, "", fmt.Errorf("rand read: %w", err)
	}
	return uuid.New(), "ak_live_" + hex.EncodeToString(b), nil
}

// NewUseCase creates an agent UseCase. pool must not be nil.
// In embedded mode pass nil for apiKeyRepo to use the built-in simple generator.
func NewUseCase(pool *pgxpool.Pool, apiKeyRepo APIKeyCreator) *UseCase {
	if apiKeyRepo == nil {
		apiKeyRepo = &simpleKeyGen{}
	}
	return &UseCase{
		repo:      pgadapter.NewAgentRepo(pool),
		apiKeyRepo: apiKeyRepo,
	}
}

// Register creates a new agent and returns its DTO + plaintext API key.
// Implements agentUseCase.Register expected by delivery/http.AgentHandler.
func (uc *UseCase) Register(ctx context.Context, req deliveryhttp.AgentRegisterReq) (deliveryhttp.AgentDto, string, error) {
	// Generate API key first (key ID stored on agent; plaintext returned once)
	keyID, plaintext, err := uc.apiKeyRepo.CreateAgentKey(ctx, uuid.Nil, req.Name)
	if err != nil {
		return deliveryhttp.AgentDto{}, "", fmt.Errorf("create api key: %w", err)
	}

	a := &agententity.Agent{
		ID:         uuid.New(),
		Name:       req.Name,
		Hostname:   req.Hostname,
		IPAddress:  req.IPAddress,
		OS:         req.OS,
		Tags:       req.Tags,
		APIKeyID:   keyID,
		Status:     agententity.AgentStatusInactive,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := uc.repo.Create(ctx, a); err != nil {
		return deliveryhttp.AgentDto{}, "", fmt.Errorf("persist agent: %w", err)
	}

	return agentToDTO(a), plaintext, nil
}

// List returns all agents, paginated at up to 200 for the handler stub.
func (uc *UseCase) List(ctx context.Context) ([]deliveryhttp.AgentDto, error) {
	agents, _, err := uc.repo.List(ctx, agentrepo.AgentFilter{Page: 1, PageSize: 200})
	if err != nil {
		return nil, err
	}
	dtos := make([]deliveryhttp.AgentDto, 0, len(agents))
	for _, a := range agents {
		dtos = append(dtos, agentToDTO(a))
	}
	return dtos, nil
}

// SubmitReport is a stub — report processing is handled by the submit_report use case
// invoked from the NATS subscriber. Here we just acknowledge receipt.
func (uc *UseCase) SubmitReport(ctx context.Context, agentID uuid.UUID, _ deliveryhttp.AgentReportPayload) (deliveryhttp.ReportResultDto, error) {
	// Update agent last-seen timestamp on report submission
	_ = uc.repo.UpdateLastSeen(ctx, agentID)
	return deliveryhttp.ReportResultDto{ID: uuid.New()}, nil
}

func agentToDTO(a *agententity.Agent) deliveryhttp.AgentDto {
	return deliveryhttp.AgentDto{
		ID:        a.ID,
		Name:      a.Name,
		Hostname:  a.Hostname,
		IPAddress: a.IPAddress,
		OS:        a.OS,
		Tags:      a.Tags,
		Status:    string(a.Status),
		CreatedAt: a.CreatedAt,
	}
}
