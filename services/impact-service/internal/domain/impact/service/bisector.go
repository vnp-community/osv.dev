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

// Package service contains core domain services for Impact Analysis.
package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/osv/impact-service/internal/domain/impact/entity"
	"github.com/osv/impact-service/internal/domain/impact/repository"
	"github.com/osv/impact-service/internal/domain/impact/valueobject"
)

// BisectionOptions configures git bisection behaviour.
type BisectionOptions struct {
	DetectCherryPicks  bool
	ConsiderAllBranches bool
	MaxCommits         int // safety cap, 0 = default 50000
}

// BisectionResult is returned by GitBisector.Bisect.
type BisectionResult struct {
	AffectedCommits []string
	HasChanges      bool
}

// GitBisector finds commits affected by a vulnerability's GIT ranges.
type GitBisector struct {
	repoCache repository.GitRepoCache
}

// NewGitBisector creates a new GitBisector.
func NewGitBisector(cache repository.GitRepoCache) *GitBisector {
	return &GitBisector{repoCache: cache}
}

// Bisect resolves git commit ranges for a vulnerability GIT range event list.
//
// Algorithm:
//  1. Open repo from cache (clone on miss, fetch to update).
//  2. For each pair of (introduced, fixed) events: git log between them.
//  3. Deduplicate commits across pairs.
//  4. Optionally detect cherry-picks.
func (b *GitBisector) Bisect(
	ctx context.Context,
	repoURL string,
	events []valueobject.VersionRangeEvent,
	opts BisectionOptions,
) (*BisectionResult, error) {
	if repoURL == "" {
		return nil, fmt.Errorf("git bisector: repoURL must not be empty")
	}

	repo, err := b.repoCache.GetOrClone(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("git bisector: open repo %q: %w", repoURL, err)
	}

	// Parse event list into (introduced, fixed) windows.
	windows := parseWindows(events)
	if len(windows) == 0 {
		return &BisectionResult{}, nil
	}

	maxCommits := opts.MaxCommits
	if maxCommits == 0 {
		maxCommits = 50_000
	}

	seen := make(map[string]struct{})
	var affected []string

	for _, w := range windows {
		commits, err := repo.LogBetween(ctx, w.introduced, w.fixed, repository.LogOptions{
			AllBranches: opts.ConsiderAllBranches,
			MaxCount:    maxCommits,
		})
		if err != nil {
			return nil, fmt.Errorf("git bisector: log between %s..%s: %w", w.introduced, w.fixed, err)
		}
		for _, c := range commits {
			if _, dup := seen[c.Hash]; !dup {
				seen[c.Hash] = struct{}{}
				affected = append(affected, c.Hash)
			}
		}
	}

	sort.Strings(affected)
	return &BisectionResult{
		AffectedCommits: affected,
		HasChanges:      len(affected) > 0,
	}, nil
}

// window is a half-open [introduced, fixed) range.
type window struct {
	introduced string
	fixed      string // "" means "open ended (to HEAD)"
}

// parseWindows converts the flat event list into (introduced, fixed) pairs.
func parseWindows(events []valueobject.VersionRangeEvent) []window {
	var windows []window
	var currentIntroduced string

	for _, ev := range events {
		switch ev.Type {
		case valueobject.RangeEventIntroduced:
			currentIntroduced = ev.Version
		case valueobject.RangeEventFixed, valueobject.RangeEventLimit, valueobject.RangeEventLastAffected:
			if currentIntroduced != "" {
				windows = append(windows, window{
					introduced: currentIntroduced,
					fixed:      ev.Version,
				})
				currentIntroduced = ""
			}
		}
	}
	// Open-ended range (no fixed event)
	if currentIntroduced != "" {
		windows = append(windows, window{introduced: currentIntroduced, fixed: ""})
	}
	return windows
}

// ─────────────────────────────────────────────
// VersionEnumerator
// ─────────────────────────────────────────────

// EcosystemHelper provides version enumeration for a specific ecosystem.
type EcosystemHelper interface {
	// EnumerateVersions returns all known versions for the given package.
	EnumerateVersions(ctx context.Context, packageName string) ([]string, error)
	// SortKey returns a comparable key for a version string.
	SortKey(version string) string
	// IsVersionValid reports whether a version string is valid for the ecosystem.
	IsVersionValid(version string) bool
}

// EcosystemRegistry maps ecosystem names to helpers.
type EcosystemRegistry interface {
	Get(ecosystem string) (EcosystemHelper, bool)
}

// VersionEnumerator enumerates versions affected by SEMVER/ECOSYSTEM ranges.
type VersionEnumerator struct {
	registry EcosystemRegistry
}

// NewVersionEnumerator creates a new VersionEnumerator.
func NewVersionEnumerator(registry EcosystemRegistry) *VersionEnumerator {
	return &VersionEnumerator{registry: registry}
}

// Enumerate returns version strings affected by the given event list.
func (e *VersionEnumerator) Enumerate(
	ctx context.Context,
	ecosystemName, packageName string,
	events []valueobject.VersionRangeEvent,
) ([]string, error) {
	helper, ok := e.registry.Get(ecosystemName)
	if !ok {
		return nil, fmt.Errorf("version enumerator: unknown ecosystem %q", ecosystemName)
	}

	allVersions, err := helper.EnumerateVersions(ctx, packageName)
	if err != nil {
		return nil, fmt.Errorf("version enumerator: enumerate %s/%s: %w", ecosystemName, packageName, err)
	}

	// Sort by SortKey for deterministic evaluation.
	sort.Slice(allVersions, func(i, j int) bool {
		return helper.SortKey(allVersions[i]) < helper.SortKey(allVersions[j])
	})

	var affected []string
	for _, v := range allVersions {
		if isVersionAffected(v, events, helper) {
			affected = append(affected, v)
		}
	}
	return affected, nil
}

// isVersionAffected walks the event list to determine if a version is affected.
// Algorithm: walk sorted events; introduced → inRange=true; fixed/limit/last_affected → inRange=false.
func isVersionAffected(version string, events []valueobject.VersionRangeEvent, helper EcosystemHelper) bool {
	inRange := false
	vKey := helper.SortKey(version)

	for _, ev := range events {
		evKey := helper.SortKey(ev.Version)
		switch ev.Type {
		case valueobject.RangeEventIntroduced:
			if ev.Version == "0" || vKey >= evKey {
				inRange = true
			}
		case valueobject.RangeEventFixed:
			if vKey >= evKey {
				inRange = false
			}
		case valueobject.RangeEventLimit:
			if vKey > evKey {
				inRange = false
			}
		case valueobject.RangeEventLastAffected:
			if vKey > evKey {
				inRange = false
			}
		}
	}
	return inRange
}

// ─────────────────────────────────────────────
// CherrypickDetector
// ─────────────────────────────────────────────

// CherrypickDetector identifies cherry-pick commits in a set.
type CherrypickDetector struct{}

// DetectCherryPicks scans commits for cherry-pick trailers in message.
func (d *CherrypickDetector) DetectCherryPicks(commits []*entity.Commit) []*entity.Commit {
	for _, c := range commits {
		sha := extractCherryPickSHA(c.Message)
		if sha != "" {
			c.IsCherryPick = true
			c.CherryPickSHA = sha
		}
	}
	return commits
}

// extractCherryPickSHA extracts the original SHA from a cherry-pick commit message.
// Looks for: "(cherry picked from commit <sha>)" or "Cherry-pick of <sha>"
func extractCherryPickSHA(message string) string {
	for _, line := range strings.Split(message, "\n") {
		line = strings.TrimSpace(line)
		const trailer = "(cherry picked from commit "
		if strings.HasPrefix(line, trailer) {
			sha := strings.TrimPrefix(line, trailer)
			sha = strings.TrimSuffix(sha, ")")
			sha = strings.TrimSpace(sha)
			if sha1Re.MatchString(sha) {
				return sha
			}
		}
	}
	return ""
}

var sha1Re = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
