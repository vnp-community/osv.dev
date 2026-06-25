package dedup

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"

    "github.com/osv/finding-service/internal/domain/finding"
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
