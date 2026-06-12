// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ecosystem

import (
	"context"
	"sort"
	"sync"

	pkgerrors "github.com/osv/shared/pkg/errors"
)

// stubHelper is used for ecosystems that are registered but lack full implementation.
type stubHelper struct {
	name string
}

func (s stubHelper) Compare(_, _ string) int { return 0 }
func (s stubHelper) SortKey(v string) string  { return v }
func (s stubHelper) IsValid(_ string) bool    { return true }
func (s stubHelper) EnumerateVersions(_ context.Context, _ string) ([]string, error) {
	return nil, pkgerrors.ErrUnknownEcosystem
}
func (s stubHelper) NextVersion(_ context.Context, _, _ string) (string, error) {
	return "", pkgerrors.ErrUnknownEcosystem
}

// defaultRegistry is the in-memory registry of ecosystem helpers.
type defaultRegistry struct {
	mu       sync.RWMutex
	helpers  map[string]Helper
}

// NewRegistry creates a new empty Registry.
func NewRegistry() Registry {
	return &defaultRegistry{
		helpers: make(map[string]Helper),
	}
}

// Register adds a Helper for the given ecosystem name.
func (r *defaultRegistry) Register(name string, h Helper) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.helpers[name] = h
}

// RegisterStub registers a stub helper for ecosystems without full implementation.
func (r *defaultRegistry) RegisterStub(name string) {
	r.Register(name, stubHelper{name: name})
}

// Get returns the Helper for the given ecosystem, or nil if not found.
func (r *defaultRegistry) Get(name string) Helper {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.helpers[name]
}

// List returns all registered ecosystem names in sorted order.
func (r *defaultRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.helpers))
	for name := range r.helpers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Global is the default global registry populated at init time.
var Global = NewRegistry()
