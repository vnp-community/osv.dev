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

// Package repository defines repository interfaces for the Impact Analysis domain.
package repository

import (
	"context"
	"time"

	"github.com/osv/impact-service/internal/domain/impact/entity"
)

// LogOptions configures git log behaviour.
type LogOptions struct {
	AllBranches bool
	MaxCount    int
}

// GitRepo is a handle to a locally cached bare git repository.
type GitRepo interface {
	// LogBetween returns commits reachable from fixed but not from introduced.
	// If fixed is "", returns commits from introduced to HEAD.
	LogBetween(ctx context.Context, introduced, fixed string, opts LogOptions) ([]*entity.Commit, error)
	// ResolveRef converts a ref name or short SHA to a full commit SHA.
	ResolveRef(ctx context.Context, ref string) (string, error)
	// URL returns the remote URL of this repository.
	URL() string
}

// GitRepoCache provides cached access to remote git repositories.
type GitRepoCache interface {
	// GetOrClone returns a GitRepo handle, cloning from remote if not cached.
	GetOrClone(ctx context.Context, repoURL string) (GitRepo, error)
	// Evict removes a repository from the local cache.
	Evict(ctx context.Context, repoURL string) error
	// Stats returns cache hit/miss statistics.
	Stats() CacheStats
}

// CacheStats holds snapshot statistics for the repo cache.
type CacheStats struct {
	Hits    int64
	Misses  int64
	Evicted int64
	// TotalSizeBytes is the total disk usage of all cached repos.
	TotalSizeBytes int64
}

// EcosystemVersionFetcher fetches available versions from a package registry.
type EcosystemVersionFetcher interface {
	// FetchVersions returns all published versions for the given package.
	FetchVersions(ctx context.Context, ecosystem, packageName string) ([]string, error)
	// LastFetchedAt returns when the version list was last fetched (for caching).
	LastFetchedAt(ctx context.Context, ecosystem, packageName string) (time.Time, error)
}
