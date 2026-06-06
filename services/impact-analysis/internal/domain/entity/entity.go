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

// Package entity holds read-model entities for the Impact Analysis domain.
package entity

import "time"

// AffectedResult holds the result of analyzing a single OSV Affected entry.
type AffectedResult struct {
	Ecosystem string
	Package   string
	// AffectedCommits lists git commit SHAs in the affected range.
	AffectedCommits []string
	// AffectedVersions lists version strings that are affected.
	AffectedVersions []string
	// HasChanges reports whether any affected items were found.
	HasChanges bool
}

// Commit represents a single git commit with metadata.
type Commit struct {
	Hash      string
	Author    string
	Message   string
	Timestamp time.Time
	// IsCherryPick is true when the commit appears to be cherry-picked
	// from another branch (detected via commit message trailer).
	IsCherryPick bool
	// CherryPickSHA is the original commit SHA if IsCherryPick is true.
	CherryPickSHA string
}

// AnalysisStats holds operational metrics from a vulnerability analysis run.
type AnalysisStats struct {
	DurationMs        int64
	ReposFetched      int
	CommitsWalked     int
	VersionsEnumerated int
	CacheHits         int
	CacheMisses       int
}
