// Package check provides the bulk CVE ID KEV membership check use case.
package check

import (
	"context"
	"fmt"

	entity "github.com/osv/data-service/internal/domain/kev"
	domainerrors "github.com/osv/data-service/internal/domain/errors"
	"github.com/osv/data-service/internal/domain/repository"
)

const maxIDs = 500

// UseCase handles bulk CVE KEV membership checks.
type UseCase struct {
	kevRepo repository.KEVRepository
}

// New creates a check UseCase.
func New(kevRepo repository.KEVRepository) *UseCase {
	return &UseCase{kevRepo: kevRepo}
}

// CheckMany reports whether each supplied CVE ID is in the KEV catalog.
// At most 500 IDs are processed per call; excess IDs are silently truncated.
func (uc *UseCase) CheckMany(ctx context.Context, cveIDs []string) ([]*entity.BulkCheckResult, error) {
	if len(cveIDs) == 0 {
		return nil, domainerrors.ErrEmptyCVEIDs
	}
	if len(cveIDs) > maxIDs {
		cveIDs = cveIDs[:maxIDs]
	}

	kevMap, err := uc.kevRepo.CheckMany(ctx, cveIDs)
	if err != nil {
		return nil, fmt.Errorf("kev check: %w", err)
	}

	results := make([]*entity.BulkCheckResult, len(cveIDs))
	for i, id := range cveIDs {
		results[i] = &entity.BulkCheckResult{
			CVEID: id,
			IsKEV: kevMap[id],
		}
	}
	return results, nil
}
