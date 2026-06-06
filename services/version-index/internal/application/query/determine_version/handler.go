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

// Package determine_version implements the DetermineVersion query handler.
package determine_version

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/osv/version-index/internal/domain/repository"
	"github.com/osv/version-index/internal/domain/service"
)

const (
	maxConcurrentBuckets = 50
	maxBucketMatchCount  = 100 // buckets with > this count of matching versions are "noisy"
)

// Query is the input for DetermineVersionHandler.
type Query struct {
	// FileHashes maps file path → MD5 hex hash.
	FileHashes map[string]string
}

// Result is the output of DetermineVersionHandler.
type Result struct {
	Matches []*service.VersionMatch
}

// Handler implements the DetermineVersion query.
type Handler struct {
	bucketRepo repository.RepoIndexBucketRepo
	indexRepo  repository.RepoIndexRepo
	hasher     *service.BucketHasher
	scorer     *service.VersionScorer
}

// NewHandler creates a new DetermineVersionHandler.
func NewHandler(
	bucketRepo repository.RepoIndexBucketRepo,
	indexRepo repository.RepoIndexRepo,
) *Handler {
	return &Handler{
		bucketRepo: bucketRepo,
		indexRepo:  indexRepo,
		hasher:     service.NewBucketHasher(),
		scorer:     service.NewVersionScorer(),
	}
}

// Handle executes the DetermineVersion query.
//
// Algorithm:
//  1. Hash file set into BucketSet.
//  2. For each non-empty bucket (parallel, sem=50): query by bucket hash.
//  3. Aggregate: group by repoURL@version, count bucket matches.
//  4. For each candidate: fetch RepoIndex (file count), compute score.
//  5. Filter >= MinScoreThreshold, sort desc, top MaxResults.
func (h *Handler) Handle(ctx context.Context, q Query) (*Result, error) {
	if len(q.FileHashes) == 0 {
		return &Result{}, nil
	}

	bs, err := h.hasher.Hash(q.FileHashes)
	if err != nil {
		return nil, fmt.Errorf("determine version: bucket hash: %w", err)
	}

	// Fan-out bucket queries.
	type bucketResult struct {
		bucketIdx int
		bucketHash string
		matches   []*repository.BucketMatch
	}

	sem := make(chan struct{}, maxConcurrentBuckets)
	results := make(chan bucketResult, service.NumBuckets)
	var wg sync.WaitGroup

	for i := 0; i < service.NumBuckets; i++ {
		if !bs.Bitmap[i] {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			bHash := h.hasher.ComputeBucketHash(bs.Buckets[idx])
			matches, err := h.bucketRepo.QueryByBucketHash(ctx, idx, bHash, maxBucketMatchCount+1)
			if err != nil {
				return // best-effort
			}
			results <- bucketResult{bucketIdx: idx, bucketHash: bHash, matches: matches}
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Aggregate matches.
	type candidateKey struct{ repoURL, version string }
	type candidateData struct {
		bucketMatches  int
		skippedBuckets int
	}
	candidates := make(map[candidateKey]*candidateData)

	for res := range results {
		if len(res.matches) > maxBucketMatchCount {
			// Too noisy — skip.
			for _, m := range res.matches {
				k := candidateKey{m.RepoURL, m.Version}
				if _, ok := candidates[k]; !ok {
					candidates[k] = &candidateData{}
				}
				candidates[k].skippedBuckets++
			}
			continue
		}
		for _, m := range res.matches {
			k := candidateKey{m.RepoURL, m.Version}
			if _, ok := candidates[k]; !ok {
				candidates[k] = &candidateData{}
			}
			candidates[k].bucketMatches++
		}
	}

	// Score each candidate.
	var versionMatches []*service.VersionMatch
	for k, cd := range candidates {
		repoIndex, err := h.indexRepo.GetByRepoVersion(ctx, k.repoURL, k.version)
		if err != nil || repoIndex == nil {
			continue
		}

		// Count empty+missed buckets.
		emptyInQuery := 0
		missedEmpty := 0
		for i := 0; i < service.NumBuckets; i++ {
			if !bs.Bitmap[i] {
				emptyInQuery++
				if repoIndex.BucketBitmap[i] {
					missedEmpty++
				}
			}
		}

		input := service.ScoringInput{
			QueryFileCount: len(q.FileHashes),
			IndexFileCount: repoIndex.FileCount,
			BucketMatches:  cd.bucketMatches,
			EmptyBuckets:   emptyInQuery - missedEmpty,
			MissedEmpty:    missedEmpty,
			SkippedBuckets: cd.skippedBuckets,
		}
		score := h.scorer.Score(input)
		if score < service.MinScoreThreshold {
			continue
		}
		versionMatches = append(versionMatches, &service.VersionMatch{
			RepoURL:              k.repoURL,
			Version:              k.version,
			Score:                score,
			MinimumFileCountSeen: min(len(q.FileHashes), repoIndex.FileCount),
		})
	}

	// Sort descending by score, take top MaxResults.
	sort.Slice(versionMatches, func(i, j int) bool {
		return versionMatches[i].Score > versionMatches[j].Score
	})
	if len(versionMatches) > service.MaxResults {
		versionMatches = versionMatches[:service.MaxResults]
	}

	return &Result{Matches: versionMatches}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
