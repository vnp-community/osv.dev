// Package test_usecase implements use cases for Test entities.
package test_usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/repository"
	testp "github.com/osv/finding-service/internal/domain/test"
)

// GetOrCreateTestInput is provided by Scan Orchestrator for auto-context creation.
type GetOrCreateTestInput struct {
	EngagementID uuid.UUID
	ScanType     string
	Title        string
	Version      string
	BuildID      string
	CommitHash   string
	BranchTag    string
}

// GetOrCreateTestUseCase finds or creates a Test for a given engagement + scan type.
type GetOrCreateTestUseCase struct {
	testRepo repository.TestRepository
}

func NewGetOrCreate(repo repository.TestRepository) *GetOrCreateTestUseCase {
	return &GetOrCreateTestUseCase{testRepo: repo}
}

// Execute finds an existing Test by engagement+scan-type or creates a new one.
// Returns (test, created, error).
func (uc *GetOrCreateTestUseCase) Execute(ctx context.Context, in GetOrCreateTestInput) (*testp.Test, bool, error) {
	existing, err := uc.testRepo.FindByEngagementAndType(ctx, in.EngagementID, in.ScanType)
	if err == nil && existing != nil {
		return existing, false, nil
	}

	t := testp.New(in.EngagementID, in.ScanType, in.Title)
	t.Version = in.Version
	t.BuildID = in.BuildID
	t.CommitHash = in.CommitHash
	t.BranchTag = in.BranchTag

	if err := uc.testRepo.Create(ctx, t); err != nil {
		return nil, false, fmt.Errorf("create test: %w", err)
	}

	return t, true, nil
}
