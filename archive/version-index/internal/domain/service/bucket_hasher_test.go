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

package service_test

import (
	"testing"

	"github.com/osv/version-index/internal/domain/service"
)

func TestBucketHasher_Hash_Deterministic(t *testing.T) {
	h := service.NewBucketHasher()
	fileHashes := map[string]string{
		"main.go":   "d41d8cd98f00b204e9800998ecf8427e",
		"go.mod":    "7215ee9c7d9dc229d2921a40e899ec5f",
		"README.md": "acbd18db4cc2f85cedef654fccc4a4d8",
	}

	bs1, err := h.Hash(fileHashes)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	bs2, err := h.Hash(fileHashes)
	if err != nil {
		t.Fatalf("Hash (second): %v", err)
	}

	for i := range bs1.Buckets {
		if len(bs1.Buckets[i]) != len(bs2.Buckets[i]) {
			t.Errorf("bucket %d: mismatch length %d vs %d", i, len(bs1.Buckets[i]), len(bs2.Buckets[i]))
		}
	}
}

func TestBucketHasher_Hash_BucketAssignment(t *testing.T) {
	h := service.NewBucketHasher()
	// MD5 of "0" = cfcd208495d565ef66e7dff9f98764da
	// First 2 bytes: 0xcf, 0xcd → BigEndian uint16 = 0xcfcd = 53197 → 53197 % 512 = 141
	md5Zero := "cfcd208495d565ef66e7dff9f98764da"
	bs, err := h.Hash(map[string]string{"file": md5Zero})
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	expectedBucket := 141
	if !bs.Bitmap[expectedBucket] {
		t.Errorf("expected hash in bucket %d, got empty", expectedBucket)
	}
}

func TestVersionScorer_Score_PerfectMatch(t *testing.T) {
	s := service.NewVersionScorer()
	score := s.Score(service.ScoringInput{
		QueryFileCount: 100,
		IndexFileCount: 100,
		BucketMatches:  512,
		EmptyBuckets:   0,
		MissedEmpty:    0,
		SkippedBuckets: 0,
	})
	if score < 0.99 {
		t.Errorf("perfect match score = %f, want >= 0.99", score)
	}
}

func TestVersionScorer_Score_NoMatch(t *testing.T) {
	s := service.NewVersionScorer()
	score := s.Score(service.ScoringInput{
		QueryFileCount: 100,
		IndexFileCount: 100,
		BucketMatches:  0,
		EmptyBuckets:   0,
		MissedEmpty:    0,
		SkippedBuckets: 0,
	})
	if score > service.MinScoreThreshold {
		t.Errorf("no match score = %f, expected < %f", score, service.MinScoreThreshold)
	}
}
