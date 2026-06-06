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

// Package ecosystem defines the EcosystemHelper and EcosystemRegistry interfaces
// used by OSV microservices to perform version comparison and enumeration.
package ecosystem

import "context"

// Helper provides version comparison and enumeration for a specific ecosystem.
type Helper interface {
	// Compare returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
	Compare(v1, v2 string) int

	// SortKey returns a lexicographically sortable key for the version,
	// suitable for database range queries.
	SortKey(version string) string

	// IsValid returns true if the version string is valid for this ecosystem.
	IsValid(version string) bool

	// EnumerateVersions returns all known versions of the given package,
	// sorted in ascending order.
	EnumerateVersions(ctx context.Context, packageName string) ([]string, error)

	// NextVersion returns the next released version after the given one,
	// or an empty string if none exists.
	NextVersion(ctx context.Context, packageName, version string) (string, error)
}

// Registry maps ecosystem names to their Helper implementations.
type Registry interface {
	// Get returns the Helper for the named ecosystem, or nil if unknown.
	Get(name string) Helper

	// List returns all registered ecosystem names.
	List() []string
}
