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

// Package valueobject contains value objects for the Source Sync Service.
package valueobject

// SourceType identifies the type of external source being synchronized.
type SourceType string

const (
	SourceTypeGit          SourceType = "GIT"
	SourceTypeBucket       SourceType = "BUCKET"
	SourceTypeRESTEndpoint SourceType = "REST_ENDPOINT"
)

// IsValid returns true if the source type is a known value.
func (s SourceType) IsValid() bool {
	switch s {
	case SourceTypeGit, SourceTypeBucket, SourceTypeRESTEndpoint:
		return true
	}
	return false
}

// ChangeSet holds the result of a change detection run for a source.
type ChangeSet struct {
	Modified    []FileChange
	Deleted     []FileChange
	NewSyncHash string
	TotalCount  int
}

// FileChange represents a single modified or deleted file in a source.
type FileChange struct {
	Path    string
	Hash    string // SHA256 or git OID
	Content []byte // populated for small files (< 100KB)
	GCSPath string // GCS URI for large files, e.g. gs://bucket/path
}
