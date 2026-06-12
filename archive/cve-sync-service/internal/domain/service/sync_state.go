// Package service — sync state tracker for 24h schedule enforcement.
package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const syncStateFile = "last_sync.json"

// SyncState tracks the last sync timestamps per source.
type SyncState struct {
	LastSync string            `json:"last_sync"` // RFC3339 global last sync
	Sources  map[string]string `json:"sources"`   // source → RFC3339 timestamp
}

// SyncStateManager manages the last-sync state file.
type SyncStateManager struct {
	path string
	mu   sync.Mutex
}

// NewSyncStateManager creates a SyncStateManager using the given cache directory.
func NewSyncStateManager(cacheDir string) *SyncStateManager {
	if cacheDir == "" {
		home, _ := os.UserHomeDir()
		cacheDir = filepath.Join(home, ".cache", "cve-bin-tool", "datasync")
	}
	return &SyncStateManager{
		path: filepath.Join(cacheDir, syncStateFile),
	}
}

// NeedsSync returns true if the source has not been synced within maxAge.
// Returns true (needs sync) if the state file doesn't exist or is unreadable.
func (m *SyncStateManager) NeedsSync(sourceName string, maxAge time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.load()
	if state == nil || len(state.Sources) == 0 {
		return true
	}

	lastStr, ok := state.Sources[sourceName]
	if !ok {
		return true
	}

	lastSync, err := time.Parse(time.RFC3339, lastStr)
	if err != nil {
		return true
	}

	return time.Since(lastSync) >= maxAge
}

// MarkSynced records the current time as the last sync time for a source.
func (m *SyncStateManager) MarkSynced(sourceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.load()
	if state == nil {
		state = &SyncState{Sources: make(map[string]string)}
	}
	if state.Sources == nil {
		state.Sources = make(map[string]string)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	state.Sources[sourceName] = now
	state.LastSync = now

	return m.save(state)
}

// LastSyncTime returns the last sync time for a source (zero value if never synced).
func (m *SyncStateManager) LastSyncTime(sourceName string) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.load()
	if state == nil {
		return time.Time{}
	}
	lastStr, ok := state.Sources[sourceName]
	if !ok {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, lastStr)
	return t
}

func (m *SyncStateManager) load() *SyncState {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return nil
	}
	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	return &state
}

func (m *SyncStateManager) save(state *SyncState) error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0o640)
}
