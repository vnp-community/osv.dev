package savedsearch_test

import (
	"context"
	"testing"

	"github.com/osv/shared/pkg/search/savedsearch"
)

// ---- Mock SearchExecutor ----

type mockExecutor struct {
	results []savedsearch.SearchResult
}

func (m *mockExecutor) Execute(_ context.Context, _ string) ([]savedsearch.SearchResult, error) {
	return m.results, nil
}

// ---- Mock AlertPublisher ----

type mockPublisher struct {
	events []savedsearch.AlertEvent
}

func (m *mockPublisher) Publish(_ context.Context, event savedsearch.AlertEvent) error {
	m.events = append(m.events, event)
	return nil
}

// ---- Tests ----

func TestService_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	repo := savedsearch.NewInMemoryRepository()
	svc := savedsearch.NewService(repo, nil, nil)

	ss, err := svc.Create(ctx, &savedsearch.SavedSearch{
		Name:        "High CVSS CVEs",
		OwnerID:     "user-1",
		SearchQuery: `{"severity":"CRITICAL"}`,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ss.ID == "" {
		t.Error("ID should be auto-generated")
	}
	if ss.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	got, err := svc.Get(ctx, ss.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "High CVSS CVEs" {
		t.Errorf("Name: got %q", got.Name)
	}
}

func TestService_CreateValidation(t *testing.T) {
	ctx := context.Background()
	repo := savedsearch.NewInMemoryRepository()
	svc := savedsearch.NewService(repo, nil, nil)

	_, err := svc.Create(ctx, &savedsearch.SavedSearch{SearchQuery: "q"})
	if err == nil {
		t.Error("expected error: missing name")
	}

	_, err = svc.Create(ctx, &savedsearch.SavedSearch{Name: "no-query"})
	if err == nil {
		t.Error("expected error: missing query")
	}
}

func TestService_ListByOwner(t *testing.T) {
	ctx := context.Background()
	repo := savedsearch.NewInMemoryRepository()
	svc := savedsearch.NewService(repo, nil, nil)

	svc.Create(ctx, &savedsearch.SavedSearch{Name: "s1", OwnerID: "u1", SearchQuery: "q1"})
	svc.Create(ctx, &savedsearch.SavedSearch{Name: "s2", OwnerID: "u1", SearchQuery: "q2"})
	svc.Create(ctx, &savedsearch.SavedSearch{Name: "s3", OwnerID: "u2", SearchQuery: "q3"})

	searches, err := svc.List(ctx, "u1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(searches) != 2 {
		t.Errorf("expected 2 searches for u1, got %d", len(searches))
	}
}

func TestService_Delete(t *testing.T) {
	ctx := context.Background()
	repo := savedsearch.NewInMemoryRepository()
	svc := savedsearch.NewService(repo, nil, nil)

	ss, _ := svc.Create(ctx, &savedsearch.SavedSearch{Name: "to-delete", OwnerID: "u1", SearchQuery: "q"})
	if err := svc.Delete(ctx, ss.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := svc.Get(ctx, ss.ID)
	if err == nil {
		t.Error("expected not found after delete")
	}
}

func TestService_Execute_WithResults(t *testing.T) {
	ctx := context.Background()
	repo := savedsearch.NewInMemoryRepository()
	executor := &mockExecutor{results: []savedsearch.SearchResult{
		{CVEID: "CVE-2024-1234", CVSSScore: 9.8, IsKEV: true},
		{CVEID: "CVE-2024-5678", CVSSScore: 7.5, IsKEV: false},
	}}
	svc := savedsearch.NewService(repo, nil, executor)

	ss, _ := svc.Create(ctx, &savedsearch.SavedSearch{
		Name:        "test search",
		OwnerID:     "u1",
		SearchQuery: `{"type":"CRITICAL"}`,
	})

	_, results, err := svc.Execute(ctx, ss.ID)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// LastResultCount should be updated
	updated, _ := svc.Get(ctx, ss.ID)
	if updated.LastResultCount != 2 {
		t.Errorf("LastResultCount: got %d, want 2", updated.LastResultCount)
	}
}

func TestService_AlertThreshold_MinResults(t *testing.T) {
	ctx := context.Background()
	repo := savedsearch.NewInMemoryRepository()
	pub := &mockPublisher{}
	executor := &mockExecutor{results: []savedsearch.SearchResult{
		{CVEID: "CVE-1"}, {CVEID: "CVE-2"}, {CVEID: "CVE-3"},
	}}
	svc := savedsearch.NewService(repo, pub, executor)

	ss, _ := svc.Create(ctx, &savedsearch.SavedSearch{
		Name:        "alert search",
		OwnerID:     "u1",
		SearchQuery: "q",
		Alert:       &savedsearch.AlertThreshold{MinResults: 2},
	})

	svc.Execute(ctx, ss.ID)

	if len(pub.events) != 1 {
		t.Errorf("expected 1 alert event, got %d", len(pub.events))
	}
	if pub.events[0].ResultCount != 3 {
		t.Errorf("alert ResultCount: got %d, want 3", pub.events[0].ResultCount)
	}
}

func TestService_AlertThreshold_CVSS(t *testing.T) {
	ctx := context.Background()
	repo := savedsearch.NewInMemoryRepository()
	pub := &mockPublisher{}
	executor := &mockExecutor{results: []savedsearch.SearchResult{
		{CVEID: "CVE-CRITICAL", CVSSScore: 9.8},
		{CVEID: "CVE-LOW", CVSSScore: 3.0},
	}}
	svc := savedsearch.NewService(repo, pub, executor)

	ss, _ := svc.Create(ctx, &savedsearch.SavedSearch{
		Name:        "cvss alert",
		OwnerID:     "u1",
		SearchQuery: "q",
		Alert:       &savedsearch.AlertThreshold{MaxCVSSScore: 9.0},
	})
	svc.Execute(ctx, ss.ID)

	if len(pub.events) == 0 {
		t.Fatal("expected alert for high CVSS")
	}
	if len(pub.events[0].HighCVSS) != 1 || pub.events[0].HighCVSS[0] != "CVE-CRITICAL" {
		t.Errorf("HighCVSS: %v", pub.events[0].HighCVSS)
	}
}

func TestService_AlertThreshold_KEV(t *testing.T) {
	ctx := context.Background()
	repo := savedsearch.NewInMemoryRepository()
	pub := &mockPublisher{}
	executor := &mockExecutor{results: []savedsearch.SearchResult{
		{CVEID: "CVE-KEV-1", IsKEV: true},
		{CVEID: "CVE-NORMAL", IsKEV: false},
	}}
	svc := savedsearch.NewService(repo, pub, executor)

	ss, _ := svc.Create(ctx, &savedsearch.SavedSearch{
		Name: "kev alert", OwnerID: "u1", SearchQuery: "q",
		Alert: &savedsearch.AlertThreshold{RequireKEV: true},
	})
	svc.Execute(ctx, ss.ID)

	if len(pub.events) == 0 {
		t.Fatal("expected KEV alert")
	}
	if len(pub.events[0].KEVEntries) != 1 {
		t.Errorf("KEVEntries: %v", pub.events[0].KEVEntries)
	}
}

func TestService_NoAlertBelowThreshold(t *testing.T) {
	ctx := context.Background()
	repo := savedsearch.NewInMemoryRepository()
	pub := &mockPublisher{}
	executor := &mockExecutor{results: []savedsearch.SearchResult{{CVEID: "CVE-1"}}}
	svc := savedsearch.NewService(repo, pub, executor)

	ss, _ := svc.Create(ctx, &savedsearch.SavedSearch{
		Name: "no alert", OwnerID: "u1", SearchQuery: "q",
		Alert: &savedsearch.AlertThreshold{MinResults: 5}, // threshold not reached
	})
	svc.Execute(ctx, ss.ID)

	if len(pub.events) != 0 {
		t.Error("should not alert when below threshold")
	}
}
