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

// Package service contains core domain services for Version Index.
package service

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
)

// NumBuckets is the number of hash buckets in the index (must be 512).
const NumBuckets = 512

// BucketSet holds a file hash organised into 512 buckets.
type BucketSet struct {
	Buckets [NumBuckets][]string // sorted file hashes per bucket
	Bitmap  [NumBuckets]bool     // true = non-empty bucket
	// FileCount is the total number of files hashed.
	FileCount int
}

// BucketHasher implements the 512-bucket file hash algorithm.
type BucketHasher struct{}

// NewBucketHasher creates a new BucketHasher.
func NewBucketHasher() *BucketHasher { return &BucketHasher{} }

// Hash distributes file MD5 hashes into NumBuckets buckets.
//
// Algorithm for each file:
//   - Decode hex MD5 hash → raw bytes.
//   - bucketIdx = BigEndian.Uint16(bytes[0:2]) % NumBuckets
//   - Append hash to bs.Buckets[bucketIdx]
//   - Sort each bucket (deterministic order).
func (h *BucketHasher) Hash(fileHashes map[string]string) (*BucketSet, error) {
	bs := &BucketSet{FileCount: len(fileHashes)}

	for _, hash := range fileHashes {
		b, err := hex.DecodeString(hash)
		if err != nil || len(b) < 2 {
			return nil, fmt.Errorf("bucket hash: invalid MD5 %q: %w", hash, err)
		}
		idx := int(binary.BigEndian.Uint16(b[:2])) % NumBuckets
		bs.Buckets[idx] = append(bs.Buckets[idx], hash)
		bs.Bitmap[idx] = true
	}

	// Sort each bucket deterministically.
	for i := range bs.Buckets {
		sort.Strings(bs.Buckets[i])
	}
	return bs, nil
}

// ComputeBucketHash computes the MD5 of a sorted list of hashes in a bucket.
func (h *BucketHasher) ComputeBucketHash(hashes []string) string {
	hasher := md5.New()
	for _, hash := range hashes {
		hasher.Write([]byte(hash))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// ─────────────────────────────────────────────
// VersionScorer
// ─────────────────────────────────────────────

const (
	// MinScoreThreshold filters out poor matches.
	MinScoreThreshold = 0.05
	// MaxResults caps the number of VersionMatch results.
	MaxResults = 10
)

// ScoringInput holds the parameters for the log-formula scoring algorithm.
type ScoringInput struct {
	QueryFileCount int
	IndexFileCount int
	BucketMatches  int // buckets with identical hashes
	EmptyBuckets   int // buckets empty in both query and index
	MissedEmpty    int // empty in query, non-empty in index
	SkippedBuckets int // >100 matches, too noisy
}

// VersionMatch is a candidate result from DetermineVersion.
type VersionMatch struct {
	RepoURL              string
	Version              string
	Score                float64
	MinimumFileCountSeen int
}

// VersionScorer computes a similarity score using the OSV log-formula algorithm.
type VersionScorer struct{}

// NewVersionScorer creates a new VersionScorer.
func NewVersionScorer() *VersionScorer { return &VersionScorer{} }

// Score returns a [0,1] similarity score for a candidate version.
//
// Formula:
//
//	numBucketChange = NumBuckets - BucketMatches - EmptyBuckets + MissedEmpty - SkippedBuckets
//	fileDiff        = |QueryFileCount - IndexFileCount|
//	estimatedDiff   = estimateDiff(numBucketChange, fileDiff)
//	maxFiles        = max(QueryFileCount, IndexFileCount)
//	score           = max(0, (maxFiles - estimatedDiff) / maxFiles)
func (s *VersionScorer) Score(input ScoringInput) float64 {
	numBucketChange := NumBuckets - input.BucketMatches - input.EmptyBuckets + input.MissedEmpty - input.SkippedBuckets
	if numBucketChange < 0 {
		numBucketChange = 0
	}
	fileDiff := abs(input.QueryFileCount - input.IndexFileCount)
	estimatedDiff := estimateDiff(numBucketChange, fileDiff)

	maxFiles := max(input.QueryFileCount, input.IndexFileCount)
	if maxFiles == 0 {
		return 0
	}
	score := float64(maxFiles-estimatedDiff) / float64(maxFiles)
	if score < 0 {
		return 0
	}
	return score
}

// estimateDiff estimates the number of different files from bucket-change count.
func estimateDiff(numBucketChange, fileDiff int) int {
	if numBucketChange <= 0 {
		return fileDiff
	}
	estimate := float64(NumBuckets) * math.Log(float64(NumBuckets+1)/float64(NumBuckets-numBucketChange+1))
	additional := math.Max(estimate-float64(fileDiff), 0) / 2
	return fileDiff + int(math.Round(additional))
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
