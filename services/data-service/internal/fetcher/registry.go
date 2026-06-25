// Package fetcher — registry.go
// Registry provides a central store for all registered Fetchers.
// It enables runtime fetcher registration and lookup without hardcoded switch statements.
//
// Usage:
//
//	reg := fetcher.NewRegistry()
//	reg.Register(fetcher.NewNVDCVEFetcher(db, apiKey, 2002))
//	reg.Register(fetcher.NewCIRCLFetcher(db))
//
//	// Run all enabled fetchers
//	for _, f := range reg.All() { ... }
package fetcher

import (
	"fmt"
	"sort"
	"sync"
)

// Registry manages all registered Fetchers.
// Thread-safe for concurrent reads/writes.
type Registry struct {
	mu       sync.RWMutex
	fetchers map[string]Fetcher
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		fetchers: make(map[string]Fetcher),
	}
}

// Register adds a Fetcher to the registry.
// Panics if a fetcher with the same name is already registered.
func (r *Registry) Register(f Fetcher) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := f.Name()
	if _, exists := r.fetchers[name]; exists {
		panic(fmt.Sprintf("fetcher registry: duplicate registration for %q", name))
	}
	r.fetchers[name] = f
}

// RegisterOrReplace adds or replaces a Fetcher in the registry.
// Unlike Register, does NOT panic on duplicate — overwrites silently.
func (r *Registry) RegisterOrReplace(f Fetcher) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fetchers[f.Name()] = f
}

// Get returns the Fetcher for the given name.
// Returns (nil, false) if not found.
func (r *Registry) Get(name string) (Fetcher, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.fetchers[name]
	return f, ok
}

// MustGet returns the Fetcher for the given name.
// Panics if not found.
func (r *Registry) MustGet(name string) Fetcher {
	f, ok := r.Get(name)
	if !ok {
		panic(fmt.Sprintf("fetcher registry: fetcher %q not found", name))
	}
	return f
}

// All returns all registered fetchers in deterministic alphabetical order by name.
func (r *Registry) All() []Fetcher {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.fetchers))
	for name := range r.fetchers {
		names = append(names, name)
	}
	sort.Strings(names)

	fetchers := make([]Fetcher, 0, len(names))
	for _, name := range names {
		fetchers = append(fetchers, r.fetchers[name])
	}
	return fetchers
}

// Names returns all registered fetcher names in alphabetical order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.fetchers))
	for name := range r.fetchers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Enabled returns all fetchers not in the disabled list.
func (r *Registry) Enabled(disabled []string) []Fetcher {
	disabledSet := make(map[string]bool, len(disabled))
	for _, d := range disabled {
		disabledSet[d] = true
	}

	all := r.All()
	result := make([]Fetcher, 0, len(all))
	for _, f := range all {
		if !disabledSet[f.Name()] {
			result = append(result, f)
		}
	}
	return result
}

// Len returns the number of registered fetchers.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.fetchers)
}
