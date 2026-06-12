package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/ingestion-service/internal/adapter/external/sources"
	"github.com/osv/ingestion-service/internal/domain/service"
)

// mockDS is a test DataSource.
type mockDS struct {
	name    string
	data    sources.CVEData
	err     error
	latency time.Duration
}

func (m *mockDS) Name() string { return m.name }
func (m *mockDS) FetchCVEData(ctx context.Context) (sources.CVEData, error) {
	if m.latency > 0 {
		select {
		case <-time.After(m.latency):
		case <-ctx.Done():
			return sources.CVEData{}, ctx.Err()
		}
	}
	return m.data, m.err
}

func TestSyncOrchestrator_FetchAll_Parallel(t *testing.T) {
	srcs := map[string]service.DataSource{
		"A": &mockDS{name: "A", latency: 50 * time.Millisecond, data: sources.CVEData{Source: "A"}},
		"B": &mockDS{name: "B", latency: 50 * time.Millisecond, data: sources.CVEData{Source: "B"}},
		"C": &mockDS{name: "C", latency: 50 * time.Millisecond, data: sources.CVEData{Source: "C"}},
	}
	orch := service.NewSyncOrchestrator(srcs, zerolog.Nop())

	start := time.Now()
	results := orch.FetchAll(context.Background(), nil)
	elapsed := time.Since(start)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	// Parallel execution: should be ~50ms, not 150ms
	if elapsed > 200*time.Millisecond {
		t.Errorf("took too long (%v): sources not running in parallel", elapsed)
	}
}

func TestSyncOrchestrator_FetchAll_DisabledSources(t *testing.T) {
	srcs := map[string]service.DataSource{
		"NVD": &mockDS{name: "NVD", data: sources.CVEData{Source: "NVD", Severities: []sources.CVESeverityRow{{CVENumber: "CVE-001"}}}},
		"OSV": &mockDS{name: "OSV", data: sources.CVEData{Source: "OSV", Severities: []sources.CVESeverityRow{{CVENumber: "CVE-002"}}}},
		"GAD": &mockDS{name: "GAD", data: sources.CVEData{Source: "GAD"}},
	}
	orch := service.NewSyncOrchestrator(srcs, zerolog.Nop())

	results := orch.FetchAll(context.Background(), []string{"GAD"})
	if len(results) != 2 {
		t.Errorf("expected 2 results (GAD disabled), got %d", len(results))
	}
	for _, r := range results {
		if r.SourceName == "GAD" {
			t.Error("GAD should be disabled")
		}
	}
}

func TestSyncOrchestrator_FetchAll_ErrorHandled(t *testing.T) {
	fetchErr := errors.New("network timeout")
	srcs := map[string]service.DataSource{
		"NVD": &mockDS{name: "NVD", err: fetchErr},
		"OSV": &mockDS{name: "OSV", data: sources.CVEData{Source: "OSV"}},
	}
	orch := service.NewSyncOrchestrator(srcs, zerolog.Nop())

	results := orch.FetchAll(context.Background(), nil)
	if len(results) != 2 {
		t.Errorf("expected 2 results even with error, got %d", len(results))
	}

	var nvdResult *service.FetchResult
	for i := range results {
		if results[i].SourceName == "NVD" {
			nvdResult = &results[i]
		}
	}
	if nvdResult == nil || !errors.Is(nvdResult.Error, fetchErr) {
		t.Errorf("expected NVD result with error, got %v", nvdResult)
	}
}

func TestSyncOrchestrator_ContextCancellation(t *testing.T) {
	srcs := map[string]service.DataSource{
		"SLOW": &mockDS{name: "SLOW", latency: 10 * time.Second},
	}
	orch := service.NewSyncOrchestrator(srcs, zerolog.Nop())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	results := orch.FetchAll(ctx, nil)
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Errorf("context cancellation not respected: elapsed %v", elapsed)
	}
	if len(results) == 1 && results[0].Error == nil {
		t.Error("expected error from slow source after context cancellation")
	}
}

func TestSyncStateManager_NeedsSync(t *testing.T) {
	mgr := service.NewSyncStateManager(t.TempDir())

	// Never synced → needs sync
	if !mgr.NeedsSync("NVD", 24*time.Hour) {
		t.Error("expected NeedsSync=true for never-synced source")
	}

	// Mark synced
	if err := mgr.MarkSynced("NVD"); err != nil {
		t.Fatalf("MarkSynced: %v", err)
	}

	// Just synced → does NOT need sync
	if mgr.NeedsSync("NVD", 24*time.Hour) {
		t.Error("expected NeedsSync=false right after sync")
	}

	// With very short maxAge → needs sync again
	if !mgr.NeedsSync("NVD", 0) {
		t.Error("expected NeedsSync=true with 0 maxAge")
	}
}

func TestSyncStateManager_LastSyncTime(t *testing.T) {
	mgr := service.NewSyncStateManager(t.TempDir())

	// Never synced → zero time
	if !mgr.LastSyncTime("OSV").IsZero() {
		t.Error("expected zero time for never-synced source")
	}

	before := time.Now().Add(-time.Second)
	mgr.MarkSynced("OSV") //nolint:errcheck
	after := time.Now().Add(time.Second)

	last := mgr.LastSyncTime("OSV")
	if last.Before(before) || last.After(after) {
		t.Errorf("LastSyncTime %v not in range [%v, %v]", last, before, after)
	}
}
