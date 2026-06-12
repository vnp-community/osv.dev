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

// Package gogit provides a go-git backed implementation of domain repository interfaces.
package gogit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/osv/impact-service/internal/domain/impact/entity"
	"github.com/osv/impact-service/internal/domain/impact/repository"
)

const (
	defaultCloneTimeout = 30 * time.Minute
	defaultFetchTimeout = 5 * time.Minute
	maxRepoSizeMB       = 10_000
)

// LocalRepoCache implements repository.GitRepoCache using local bare clones on disk.
type LocalRepoCache struct {
	cacheDir string
	mu       sync.Mutex
	repos    map[string]*cachedRepo
	hits     int64
	misses   int64
}

type cachedRepo struct {
	repo       *gogit.Repository
	url        string
	clonedAt   time.Time
	lastUsedAt time.Time
}

// NewLocalRepoCache creates a cache that stores bare repos under cacheDir.
func NewLocalRepoCache(cacheDir string) (*LocalRepoCache, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("repo cache mkdir %s: %w", cacheDir, err)
	}
	return &LocalRepoCache{
		cacheDir: cacheDir,
		repos:    make(map[string]*cachedRepo),
	}, nil
}

// GetOrClone returns a GitRepo handle, cloning if not already cached.
func (c *LocalRepoCache) GetOrClone(ctx context.Context, repoURL string) (repository.GitRepo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cr, ok := c.repos[repoURL]; ok {
		cr.lastUsedAt = time.Now()
		c.hits++
		return &goGitRepo{repo: cr.repo, url: repoURL}, nil
	}
	c.misses++

	// Clone bare repo.
	repoDir := filepath.Join(c.cacheDir, sanitize(repoURL))
	cloneCtx, cancel := context.WithTimeout(ctx, defaultCloneTimeout)
	defer cancel()

	r, err := gogit.PlainCloneContext(cloneCtx, repoDir, true, &gogit.CloneOptions{
		URL:          repoURL,
		Tags:         gogit.AllTags,
		SingleBranch: false,
	})
	if err != nil {
		// If the directory already exists (partial clone), try to open it.
		if r2, openErr := gogit.PlainOpen(repoDir); openErr == nil {
			r = r2
		} else {
			return nil, fmt.Errorf("repo cache clone %q: %w", repoURL, err)
		}
	}

	c.repos[repoURL] = &cachedRepo{
		repo:       r,
		url:        repoURL,
		clonedAt:   time.Now(),
		lastUsedAt: time.Now(),
	}
	return &goGitRepo{repo: r, url: repoURL}, nil
}

// Evict removes a repository from the in-memory cache.
func (c *LocalRepoCache) Evict(_ context.Context, repoURL string) error {
	c.mu.Lock()
	delete(c.repos, repoURL)
	c.mu.Unlock()
	return nil
}

// Stats returns cache statistics.
func (c *LocalRepoCache) Stats() repository.CacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return repository.CacheStats{Hits: c.hits, Misses: c.misses}
}

// sanitize converts a URL to a safe directory name.
func sanitize(url string) string {
	safe := make([]byte, len(url))
	for i, ch := range url {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			safe[i] = byte(ch)
		} else {
			safe[i] = '_'
		}
	}
	return string(safe)
}

// ─────────────────────────────────────────────
// goGitRepo — implements repository.GitRepo
// ─────────────────────────────────────────────

type goGitRepo struct {
	repo *gogit.Repository
	url  string
}

func (g *goGitRepo) URL() string { return g.url }

// LogBetween walks commits from introduced to fixed (exclusive).
// If fixed == "", walks to HEAD.
func (g *goGitRepo) LogBetween(ctx context.Context, introduced, fixed string, opts repository.LogOptions) ([]*entity.Commit, error) {
	// Resolve introduced hash.
	startHash, err := g.resolveHash(introduced)
	if err != nil {
		return nil, fmt.Errorf("resolve introduced %q: %w", introduced, err)
	}

	// Collect all commits reachable from introduced.
	logOpts := &gogit.LogOptions{
		From: startHash,
		All:  opts.AllBranches,
	}
	iter, err := g.repo.Log(logOpts)
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	defer iter.Close()

	// Optionally resolve fixed boundary.
	var fixedHash plumbing.Hash
	hasBoundary := false
	if fixed != "" {
		fixedHash, err = g.resolveHash(fixed)
		if err != nil {
			return nil, fmt.Errorf("resolve fixed %q: %w", fixed, err)
		}
		hasBoundary = true
	}

	maxCount := opts.MaxCount
	if maxCount == 0 {
		maxCount = 50_000
	}

	var commits []*entity.Commit
	count := 0
	err = iter.ForEach(func(c *object.Commit) error {
		if count >= maxCount {
			return fmt.Errorf("max commit count reached")
		}
		// Stop when we reach the fixed boundary.
		if hasBoundary && c.Hash == fixedHash {
			return fmt.Errorf("stop")
		}
		commits = append(commits, &entity.Commit{
			Hash:      c.Hash.String(),
			Author:    c.Author.Name,
			Message:   c.Message,
			Timestamp: c.Author.When,
		})
		count++
		return nil
	})
	// "stop" and "max commit count reached" are sentinels, not real errors.
	if err != nil && err.Error() != "stop" && err.Error() != "max commit count reached" {
		return nil, fmt.Errorf("git log walk: %w", err)
	}
	return commits, nil
}

// ResolveRef converts a ref name or short SHA to a full commit SHA.
func (g *goGitRepo) ResolveRef(ctx context.Context, ref string) (string, error) {
	h, err := g.resolveHash(ref)
	if err != nil {
		return "", err
	}
	return h.String(), nil
}

func (g *goGitRepo) resolveHash(refOrHash string) (plumbing.Hash, error) {
	// Try direct hash first.
	if len(refOrHash) == 40 {
		return plumbing.NewHash(refOrHash), nil
	}
	// Try as a reference name.
	ref, err := g.repo.Reference(plumbing.NewRemoteReferenceName("origin", refOrHash), true)
	if err == nil {
		return ref.Hash(), nil
	}
	// Fallback: tag or branch.
	ref2, err := g.repo.Reference(plumbing.NewTagReferenceName(refOrHash), true)
	if err == nil {
		return ref2.Hash(), nil
	}
	return plumbing.ZeroHash, fmt.Errorf("cannot resolve ref %q", refOrHash)
}
