package asset

import (
    "context"
    "testing"

    "github.com/google/uuid"
    "github.com/rs/zerolog"

    assetdomain "github.com/osv/scan-service/internal/domain/asset"
)

// mockAssetRepo is an in-memory asset store for testing
type mockAssetRepo struct {
    assets  map[string]*assetdomain.Asset // keyed by IP string
    saves   int
    updates int
}

func newMockRepo() *mockAssetRepo {
    return &mockAssetRepo{assets: make(map[string]*assetdomain.Asset)}
}

func (m *mockAssetRepo) FindByIP(_ context.Context, ip string) (*assetdomain.Asset, error) {
    if a, ok := m.assets[ip]; ok {
        return a, nil
    }
    return nil, nil
}

func (m *mockAssetRepo) Save(_ context.Context, a *assetdomain.Asset) error {
    m.assets[a.IPString()] = a
    m.saves++
    return nil
}

func (m *mockAssetRepo) Update(_ context.Context, a *assetdomain.Asset) error {
    m.assets[a.IPString()] = a
    m.updates++
    return nil
}

func TestUpsert_NewAsset(t *testing.T) {
    repo := newMockRepo()
    uc := New(repo, zerolog.Nop())

    result, err := uc.Execute(context.Background(), UpsertInput{
        ScanID:    uuid.New(),
        IPAddress: "192.168.1.10",
        Hostname:  "web-server.local",
        OS:        "Linux",
        OSVersion: "5.15",
        Services: []assetdomain.NetworkService{
            {Port: 80, Protocol: "tcp", Name: "http"},
        },
    })
    if err != nil {
        t.Fatalf("Execute: %v", err)
    }
    if !result.Created {
        t.Error("should be Created=true for new IP")
    }
    if result.Reason != "created" {
        t.Errorf("Reason = %s, want created", result.Reason)
    }
    if repo.saves != 1 || repo.updates != 0 {
        t.Errorf("saves=%d updates=%d, want saves=1 updates=0", repo.saves, repo.updates)
    }
}

func TestUpsert_ExistingAsset(t *testing.T) {
    repo := newMockRepo()
    uc := New(repo, zerolog.Nop())
    scanID := uuid.New()

    // First upsert — creates
    _, _ = uc.Execute(context.Background(), UpsertInput{
        ScanID: scanID, IPAddress: "10.0.0.1", Hostname: "old-hostname",
    })

    // Second upsert — updates
    result, err := uc.Execute(context.Background(), UpsertInput{
        ScanID: uuid.New(), IPAddress: "10.0.0.1", Hostname: "new-hostname",
    })
    if err != nil {
        t.Fatalf("Execute: %v", err)
    }
    if result.Created {
        t.Error("should be Created=false for existing IP")
    }
    if result.Asset.Hostname != "new-hostname" {
        t.Errorf("Hostname = %s, want new-hostname", result.Asset.Hostname)
    }
    if repo.saves != 1 || repo.updates != 1 {
        t.Errorf("saves=%d updates=%d, want saves=1 updates=1", repo.saves, repo.updates)
    }
}

func TestUpsert_EmptyIP(t *testing.T) {
    uc := New(newMockRepo(), zerolog.Nop())
    _, err := uc.Execute(context.Background(), UpsertInput{ScanID: uuid.New()})
    if err == nil {
        t.Error("expected error for empty IP address")
    }
}

func TestUpsert_BatchProcessing(t *testing.T) {
    repo := newMockRepo()
    uc := New(repo, zerolog.Nop())
    scanID := uuid.New()

    inputs := []UpsertInput{
        {ScanID: scanID, IPAddress: "10.0.0.1"},
        {ScanID: scanID, IPAddress: "10.0.0.2"},
        {ScanID: scanID, IPAddress: "10.0.0.3"},
    }

    results, err := uc.ExecuteBatch(context.Background(), inputs)
    if err != nil {
        t.Fatalf("ExecuteBatch: %v", err)
    }
    if len(results) != 3 {
        t.Errorf("results = %d, want 3", len(results))
    }
    if repo.saves != 3 {
        t.Errorf("saves = %d, want 3", repo.saves)
    }
}

func TestRiskScore_Critical(t *testing.T) {
    stats := assetdomain.FindingStats{Active: 1, Critical: 1}
    a, _ := assetdomain.NewAsset("192.168.1.1", "", "", "")
    a.UpdateRiskScore(stats)
    if a.RiskScore != 10.0 {
        t.Errorf("Critical should give risk_score=10.0, got %.1f", a.RiskScore)
    }
}

func TestRiskScore_NoFindings(t *testing.T) {
    a, _ := assetdomain.NewAsset("192.168.1.1", "", "", "")
    a.UpdateRiskScore(assetdomain.FindingStats{})
    if a.RiskScore != 0.0 {
        t.Errorf("No findings should give risk_score=0.0, got %.1f", a.RiskScore)
    }
}

func TestAsset_Tags(t *testing.T) {
    a, _ := assetdomain.NewAsset("192.168.1.1", "", "", "")
    a.AddTag("production")
    a.AddTag("web")
    a.AddTag("PRODUCTION") // Duplicate (case-insensitive)

    if len(a.Tags) != 2 {
        t.Errorf("Tags len = %d, want 2 (dedup)", len(a.Tags))
    }

    a.RemoveTag("production")
    if len(a.Tags) != 1 {
        t.Errorf("After remove, Tags len = %d, want 1", len(a.Tags))
    }
}
