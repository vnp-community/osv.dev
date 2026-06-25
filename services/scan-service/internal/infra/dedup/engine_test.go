package dedupinfra

import (
	"context"
	"testing"

	importuc "github.com/osv/scan-service/internal/usecase/import"
)

type mockFindingLookupClient struct {
	hashes map[string][]string
	unique map[string]string
	status map[string]struct{
		active    bool
		mitigated bool
	}
	reactivated []string
}

func newMockLookupClient() *mockFindingLookupClient {
	return &mockFindingLookupClient{
		hashes: make(map[string][]string),
		unique: make(map[string]string),
		status: make(map[string]struct{ active bool; mitigated bool }),
	}
}

func (m *mockFindingLookupClient) FindByHashCode(ctx context.Context, hashCode, productID string, engagementID string) ([]string, error) {
	return m.hashes[hashCode], nil
}

func (m *mockFindingLookupClient) FindByUniqueID(ctx context.Context, uniqueID, productID string) (string, bool, error) {
	if id, ok := m.unique[uniqueID]; ok {
		return id, true, nil
	}
	return "", false, nil
}

func (m *mockFindingLookupClient) ExistsFalsePositiveByHash(ctx context.Context, hashCode, productID string) (bool, error) {
	return false, nil
}

func (m *mockFindingLookupClient) GetFindingStatus(ctx context.Context, findingID string) (active bool, mitigated bool, err error) {
	s := m.status[findingID]
	return s.active, s.mitigated, nil
}

func (m *mockFindingLookupClient) ReactivateFinding(ctx context.Context, findingIDs []string) error {
	m.reactivated = append(m.reactivated, findingIDs...)
	return nil
}

func TestDeduplicate(t *testing.T) {
	client := newMockLookupClient()
	engine := NewEngine(client)

	ctx := context.Background()
	dc := &importuc.DedupContext{
		TestID:       "t1",
		EngagementID: "e1",
		ProductID:    "p1",
		OnEngagement: true,
		ScanType:     "Snyk Scan",
	}

	findings := []*importuc.ParsedFinding{
		{VulnIDFromTool: "snyk-123", Severity: "High", Title: "Snyk Vuln"},
		{Severity: "Medium", Title: "Nmap Vuln"}, // No VulnIDFromTool
	}

	// Calculate hashes manually or let engine do it
	findings[0].HashCode = ComputeHashCode(findings[0])
	findings[1].HashCode = ComputeHashCode(findings[1])

	// Setup mock data
	client.unique["snyk-123"] = "finding-1"
	client.status["finding-1"] = struct{ active bool; mitigated bool }{active: false, mitigated: true}

	res, err := engine.Deduplicate(ctx, findings, dc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Expect 1 reactivated, 1 new finding
	if len(res.Reactivated) != 1 {
		t.Errorf("expected 1 reactivated, got %d", len(res.Reactivated))
	}
	if len(res.NewFindings) != 1 {
		t.Errorf("expected 1 new finding, got %d", len(res.NewFindings))
	}
	if res.NewFindings[0].Title != "Nmap Vuln" {
		t.Errorf("expected new finding to be Nmap Vuln")
	}
}
