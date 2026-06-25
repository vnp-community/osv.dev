// Package service provides shared domain service types used across use cases.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/osv/data-service/internal/adapter/external/sources"
	"github.com/rs/zerolog"
)

// DataSource is the interface that all external CVE data source adapters must implement.
// It mirrors sources.DataSource but lives in the domain service layer.
type DataSource interface {
	Name() string
	FetchCVEData(ctx context.Context) (sources.CVEData, error)
}

// FetchResult holds the result of fetching a single data source.
type FetchResult struct {
	SourceName string
	Data       sources.CVEData
	Error      error
	Duration   time.Duration
}

// SyncOrchestrator coordinates fetching from multiple data sources in parallel.
type SyncOrchestrator struct {
	sources map[string]DataSource
	log     zerolog.Logger
}

// NewSyncOrchestrator creates a new SyncOrchestrator.
func NewSyncOrchestrator(sources map[string]DataSource, log zerolog.Logger) *SyncOrchestrator {
	return &SyncOrchestrator{
		sources: sources,
		log:     log.With().Str("component", "sync-orchestrator").Logger(),
	}
}

// SourceNames returns the names of all registered sources, sorted.
func (o *SyncOrchestrator) SourceNames() []string {
	names := make([]string, 0, len(o.sources))
	for name := range o.sources {
		names = append(names, name)
	}
	return names
}

// FetchAll fetches all sources concurrently, skipping those in the disabled list.
func (o *SyncOrchestrator) FetchAll(ctx context.Context, disabled []string) []FetchResult {
	disabledSet := make(map[string]bool, len(disabled))
	for _, d := range disabled {
		disabledSet[d] = true
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results []FetchResult
	)

	for name, src := range o.sources {
		if disabledSet[name] {
			o.log.Debug().Str("source", name).Msg("skipped (disabled)")
			continue
		}
		wg.Add(1)
		go func(name string, src DataSource) {
			defer wg.Done()
			start := time.Now()
			o.log.Info().Str("source", name).Msg("fetch start")
			data, err := src.FetchCVEData(ctx)
			dur := time.Since(start)
			if err != nil {
				o.log.Error().Err(err).Str("source", name).Dur("dur", dur).Msg("fetch failed")
			} else {
				o.log.Info().Str("source", name).Int("rows", len(data.Severities)).Dur("dur", dur).Msg("fetch done")
			}
			mu.Lock()
			results = append(results, FetchResult{SourceName: name, Data: data, Error: err, Duration: dur})
			mu.Unlock()
		}(name, src)
	}
	wg.Wait()
	return results
}

// syncState tracks the last successful sync time per source.
type syncState struct {
	LastSync time.Time `json:"last_sync"`
}

// SyncStateManager persists and queries the last sync time per source using JSON files.
type SyncStateManager struct {
	cacheDir string
	mu       sync.Mutex
	states   map[string]syncState
}

// NewSyncStateManager creates a new SyncStateManager using the given cache directory.
func NewSyncStateManager(cacheDir string) *SyncStateManager {
	m := &SyncStateManager{
		cacheDir: cacheDir,
		states:   make(map[string]syncState),
	}
	m.load()
	return m
}

// NeedsSync returns true if the source has not been synced within the given age.
func (m *SyncStateManager) NeedsSync(source string, age time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.states[source]
	if !ok {
		return true
	}
	return time.Since(s.LastSync) >= age
}

// MarkSynced records a successful sync for the given source.
func (m *SyncStateManager) MarkSynced(source string) error {
	m.mu.Lock()
	m.states[source] = syncState{LastSync: time.Now()}
	m.mu.Unlock()
	return m.save()
}

func (m *SyncStateManager) stateFile() string {
	return filepath.Join(m.cacheDir, "sync_state.json")
}

func (m *SyncStateManager) load() {
	b, err := os.ReadFile(m.stateFile())
	if err != nil {
		return // file not found — fresh start
	}
	_ = json.Unmarshal(b, &m.states)
}

func (m *SyncStateManager) save() error {
	if err := os.MkdirAll(m.cacheDir, 0755); err != nil {
		return fmt.Errorf("sync state: mkdir: %w", err)
	}
	b, err := json.Marshal(m.states)
	if err != nil {
		return fmt.Errorf("sync state: marshal: %w", err)
	}
	return os.WriteFile(m.stateFile(), b, 0644)
}
