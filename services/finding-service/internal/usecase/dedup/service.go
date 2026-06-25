package dedup

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/rs/zerolog"

    "github.com/osv/finding-service/internal/domain/finding"
)

// DedupScope determines how wide the deduplication search is
type DedupScope string

const (
    ScopeProduct    DedupScope = "product"    // Check within same product (default)
    ScopeEngagement DedupScope = "engagement" // Check only within same engagement
    ScopeGlobal     DedupScope = "global"     // Check across all products
)

// FindingRepository defines the storage interface needed by dedup
type FindingRepository interface {
    FindByHashInProduct(ctx context.Context, hashCode string, productID uuid.UUID) (*finding.Finding, error)
    FindByHashInEngagement(ctx context.Context, hashCode string, engagementID uuid.UUID) (*finding.Finding, error)
    FindByHashGlobal(ctx context.Context, hashCode string) (*finding.Finding, error)
}

// DeduplicationStats tracks dedup results for reporting
type DeduplicationStats struct {
    TotalProcessed int
    NewFindings    int
    Duplicates     int
}

// Service handles finding deduplication logic
type Service struct {
    repo   FindingRepository
    logger zerolog.Logger
}

// New creates a DeduplicationService
func New(repo FindingRepository, logger zerolog.Logger) *Service {
    return &Service{repo: repo, logger: logger}
}

// CheckAndMark checks if a finding is a duplicate and marks it accordingly.
// Returns: (isDuplicate bool, originalFindingID *uuid.UUID, error)
//
// Process:
//  1. Lookup by hash_code in the appropriate scope
//  2. If found and is not itself a duplicate → mark current as duplicate, link to original
//  3. If found but IS itself a duplicate → link to the ORIGINAL's original (follow chain)
//  4. If not found → new finding, no action needed
func (s *Service) CheckAndMark(
    ctx context.Context,
    f *finding.Finding,
    scope DedupScope,
) (isDuplicate bool, originalID *uuid.UUID, err error) {
    existing, err := s.lookup(ctx, f, scope)
    if err != nil {
        return false, nil, fmt.Errorf("dedup lookup: %w", err)
    }
    if existing == nil {
        // Not a duplicate — this is a new finding
        s.logger.Debug().
            Str("hash", f.HashCode).
            Str("scope", string(scope)).
            Msg("dedup: new finding")
        return false, nil, nil
    }

    // Determine the "true original" — if existing is itself a duplicate, follow the chain
    trueOriginalID := existing.ID
    if existing.DuplicateFindingID != nil {
        trueOriginalID = *existing.DuplicateFindingID
    }

    // Mark this finding as duplicate
    trueOrigStr := trueOriginalID.String()
    f.MarkDuplicate(&trueOrigStr)

    s.logger.Info().
        Str("finding_hash", f.HashCode).
        Str("original_id", trueOriginalID.String()).
        Str("scope", string(scope)).
        Msg("dedup: duplicate finding detected")

    return true, &trueOriginalID, nil
}

// ProcessBatch processes multiple findings with dedup, returning stats.
// This is used by the CI/CD orchestrator when importing scan results.
func (s *Service) ProcessBatch(
    ctx context.Context,
    findings []*finding.Finding,
    scope DedupScope,
) (*DeduplicationStats, error) {
    stats := &DeduplicationStats{TotalProcessed: len(findings)}

    // Track hashes seen in this batch (within-batch dedup)
    batchHashSet := make(map[string]uuid.UUID)

    for _, f := range findings {
        // Check within-batch first (same scan, multiple ports with same CVE)
        if existingID, exists := batchHashSet[f.HashCode]; exists {
            existingIDStr := existingID.String()
        f.MarkDuplicate(&existingIDStr)
            stats.Duplicates++
            continue
        }

        // Check against persistent storage
        isDuplicate, originalID, err := s.CheckAndMark(ctx, f, scope)
        if err != nil {
            s.logger.Error().Err(err).
                Str("finding_title", f.Title).
                Msg("dedup check failed, treating as new")
            stats.NewFindings++
            continue
        }

        if isDuplicate && originalID != nil {
            stats.Duplicates++
        } else {
            stats.NewFindings++
            // Add to batch hash set for within-batch dedup
            batchHashSet[f.HashCode] = f.ID
        }
    }

    return stats, nil
}

// lookup finds an existing non-duplicate finding with the same hash
func (s *Service) lookup(
    ctx context.Context,
    f *finding.Finding,
    scope DedupScope,
) (*finding.Finding, error) {
    switch scope {
    case ScopeEngagement:
        return s.repo.FindByHashInEngagement(ctx, f.HashCode, f.EngagementID)
    case ScopeGlobal:
        return s.repo.FindByHashGlobal(ctx, f.HashCode)
    default: // ScopeProduct
        return s.repo.FindByHashInProduct(ctx, f.HashCode, f.ProductID)
    }
}

// DedupScopeFor determines scope from engagement settings
func DedupScopeFor(deduplicationOnEngagement bool) DedupScope {
    if deduplicationOnEngagement {
        return ScopeEngagement
    }
    return ScopeProduct
}

// BuildHashCode computes SHA-256 hash for deduplication.
// This is a package-level function for use outside entity (e.g., in queries).
func BuildHashCode(title, componentName, componentVersion, cve string) string {
    f := &finding.Finding{
        Title:            title,
        ComponentName:    componentName,
        ComponentVersion: componentVersion,
        CVE:              cve,
    }
    return f.ComputeHash()
}
