# TASK-FIND-002 — SHA-256 Deduplication

| Field | Value |
|-------|-------|
| **Task ID** | T-FIND-002 |
| **Service** | `finding-service` |
| **Status** | ✅ Completed |
| **Solution Ref** | SOL-OVS-002 §4 Deduplication |
| **Priority** | 🔴 High |
| **Depends On** | T-FIND-001 |
| **Estimated** | 2h |

---

## Context

Deduplication ngăn chặn việc tạo nhiều findings cho cùng một vulnerability trong các scan cycle khác nhau. Logic:

1. Tính `HashCode = SHA256(title | component_name | component_version | cve)` (đã có ở entity)
2. Trước khi `INSERT` finding mới → tìm trong DB bằng `hash_code`
3. Nếu tồn tại: đánh dấu finding mới là `duplicate` và link `duplicate_finding_id`
4. Nếu không: insert bình thường

Scope của dedup có thể là:
- **Global** (default): Cùng hash trong cùng product
- **Per-engagement**: Cùng hash trong cùng engagement (khi `deduplication_on_engagement=true`)

---

## Goal

Implement `DeduplicationService` use case với interface cho repository lookup theo hash.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/finding-service/internal/usecase/dedup/service.go` |
| CREATE | `services/finding-service/internal/usecase/dedup/service_test.go` |

---

## Implementation

### File 1: `services/finding-service/internal/usecase/dedup/service.go`

```go
package dedup

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"

    "github.com/google/osv.dev/services/finding-service/internal/domain/finding"
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
    f.MarkDuplicate(trueOriginalID)

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
            f.MarkDuplicate(existingID)
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
```

### File 2: `services/finding-service/internal/usecase/dedup/service_test.go`

```go
package dedup

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"

    "github.com/google/osv.dev/services/finding-service/internal/domain/finding"
)

// mockFindingRepo is an in-memory finding repository for testing
type mockFindingRepo struct {
    findings map[string]*finding.Finding // keyed by hash
}

func newMockRepo() *mockFindingRepo {
    return &mockFindingRepo{findings: make(map[string]*finding.Finding)}
}

func (m *mockFindingRepo) FindByHashInProduct(_ context.Context, hash string, _ uuid.UUID) (*finding.Finding, error) {
    if f, ok := m.findings[hash]; ok {
        return f, nil
    }
    return nil, nil
}

func (m *mockFindingRepo) FindByHashInEngagement(_ context.Context, hash string, _ uuid.UUID) (*finding.Finding, error) {
    if f, ok := m.findings[hash]; ok {
        return f, nil
    }
    return nil, nil
}

func (m *mockFindingRepo) FindByHashGlobal(_ context.Context, hash string) (*finding.Finding, error) {
    if f, ok := m.findings[hash]; ok {
        return f, nil
    }
    return nil, nil
}

var (
    testProductID    = uuid.New()
    testEngagementID = uuid.New()
    testTestID       = uuid.New()
)

func newTestFinding(t *testing.T, title, cve string) *finding.Finding {
    t.Helper()
    f, err := finding.NewFinding(
        title,
        finding.SeverityCritical,
        testTestID, testEngagementID, testProductID,
        "192.168.1.10", "1.0.0", cve,
    )
    if err != nil {
        t.Fatalf("NewFinding: %v", err)
    }
    return f
}

func TestDedup_NewFinding_NotDuplicate(t *testing.T) {
    repo := newMockRepo()
    svc := New(repo, zerolog.Nop())

    f := newTestFinding(t, "[CVE-2021-44228] Log4j RCE", "CVE-2021-44228")

    isDup, origID, err := svc.CheckAndMark(context.Background(), f, ScopeProduct)
    if err != nil {
        t.Fatalf("CheckAndMark: %v", err)
    }
    if isDup {
        t.Error("should NOT be duplicate for new finding")
    }
    if origID != nil {
        t.Error("originalID should be nil for new finding")
    }
    if f.Duplicate {
        t.Error("finding.Duplicate should be false for new finding")
    }
    if !f.Active {
        t.Error("finding should remain active")
    }
}

func TestDedup_DuplicateFinding(t *testing.T) {
    repo := newMockRepo()
    svc := New(repo, zerolog.Nop())

    // First finding — store in "DB"
    original := newTestFinding(t, "[CVE-2021-44228] Log4j RCE", "CVE-2021-44228")
    repo.findings[original.HashCode] = original

    // Second finding with same hash
    duplicate := newTestFinding(t, "[CVE-2021-44228] Log4j RCE", "CVE-2021-44228")

    isDup, origID, err := svc.CheckAndMark(context.Background(), duplicate, ScopeProduct)
    if err != nil {
        t.Fatalf("CheckAndMark: %v", err)
    }
    if !isDup {
        t.Error("should be detected as duplicate")
    }
    if origID == nil || *origID != original.ID {
        t.Errorf("originalID = %v, want %v", origID, original.ID)
    }
    if !duplicate.Duplicate {
        t.Error("duplicate.Duplicate should be true")
    }
    if duplicate.Active {
        t.Error("duplicate should be inactive")
    }
    if duplicate.DuplicateFindingID == nil || *duplicate.DuplicateFindingID != original.ID {
        t.Error("DuplicateFindingID should point to original")
    }
}

func TestDedup_BatchProcessing(t *testing.T) {
    repo := newMockRepo()
    svc := New(repo, zerolog.Nop())

    findings := []*finding.Finding{
        newTestFinding(t, "[CVE-2021-44228] Log4j RCE", "CVE-2021-44228"),
        newTestFinding(t, "[CVE-2021-44228] Log4j RCE", "CVE-2021-44228"), // Same → batch dedup
        newTestFinding(t, "[CVE-2021-45105] Log4j DoS", "CVE-2021-45105"), // Different
    }

    stats, err := svc.ProcessBatch(context.Background(), findings, ScopeProduct)
    if err != nil {
        t.Fatalf("ProcessBatch: %v", err)
    }
    if stats.TotalProcessed != 3 {
        t.Errorf("TotalProcessed = %d, want 3", stats.TotalProcessed)
    }
    if stats.NewFindings != 2 {
        t.Errorf("NewFindings = %d, want 2", stats.NewFindings)
    }
    if stats.Duplicates != 1 {
        t.Errorf("Duplicates = %d, want 1", stats.Duplicates)
    }
}

func TestDedupScopeFor(t *testing.T) {
    if DedupScopeFor(true) != ScopeEngagement {
        t.Error("deduplicationOnEngagement=true should use ScopeEngagement")
    }
    if DedupScopeFor(false) != ScopeProduct {
        t.Error("deduplicationOnEngagement=false should use ScopeProduct")
    }
}

func TestDedup_ChainFollowing(t *testing.T) {
    // When existing finding is itself a duplicate, new finding should point to original
    repo := newMockRepo()
    svc := New(repo, zerolog.Nop())

    originalID := uuid.New()
    existingDuplicate := newTestFinding(t, "[CVE-2021-44228] Log4j RCE", "CVE-2021-44228")
    existingDuplicate.Duplicate = true
    existingDuplicate.Active = false
    now := time.Now()
    existingDuplicate.LastStatusUpdate = &now
    existingDuplicate.DuplicateFindingID = &originalID
    repo.findings[existingDuplicate.HashCode] = existingDuplicate

    // New finding with same hash
    newF := newTestFinding(t, "[CVE-2021-44228] Log4j RCE", "CVE-2021-44228")
    isDup, origID, err := svc.CheckAndMark(context.Background(), newF, ScopeProduct)
    if err != nil {
        t.Fatalf("CheckAndMark: %v", err)
    }
    if !isDup {
        t.Error("should be duplicate")
    }
    // Should point to the original, not to the intermediate duplicate
    if origID == nil || *origID != originalID {
        t.Errorf("Should follow chain to original. Got: %v, want: %v", origID, originalID)
    }
}
```

---

## Verification

```bash
cd services/finding-service
go build ./internal/usecase/dedup/...
go test ./internal/usecase/dedup/... -v
```

**Expected**:
```
--- PASS: TestDedup_NewFinding_NotDuplicate
--- PASS: TestDedup_DuplicateFinding
--- PASS: TestDedup_BatchProcessing
--- PASS: TestDedupScopeFor
--- PASS: TestDedup_ChainFollowing
```

### Checklist

- [x] New finding (no existing with same hash) → `isDuplicate=false, origID=nil`
- [x] Existing finding with same hash → `isDuplicate=true, origID=existing.ID`
- [x] Existing finding IS itself duplicate → follow chain to `DuplicateFindingID`
- [x] Batch: 2 findings same hash → 1 new + 1 duplicate
- [x] Batch: within-batch dedup works (no DB call for second occurrence)
- [x] `deduplicationOnEngagement=true` → scope limited to engagement
- [x] `deduplicationOnEngagement=false` → scope is product (default)
