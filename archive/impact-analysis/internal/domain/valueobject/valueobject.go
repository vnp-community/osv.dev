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

// Package valueobject holds value objects for the Impact Analysis domain.
package valueobject

import (
	"fmt"
	"regexp"
	"strings"
)

// CommitHash is a validated, normalised SHA-1 or SHA-256 hex string.
type CommitHash struct {
	value string
}

var (
	sha1Re  = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
	sha256R = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

// NewCommitHash validates and creates a CommitHash.
func NewCommitHash(raw string) (CommitHash, error) {
	h := strings.ToLower(strings.TrimSpace(raw))
	if !sha1Re.MatchString(h) && !sha256R.MatchString(h) {
		return CommitHash{}, fmt.Errorf("invalid commit hash %q: must be SHA-1 (40) or SHA-256 (64) hex", raw)
	}
	return CommitHash{value: h}, nil
}

// String returns the normalised hex string.
func (c CommitHash) String() string { return c.value }

// IsZero reports whether the hash is the zero value.
func (c CommitHash) IsZero() bool { return c.value == "" }

// Short returns the first 12 hex digits (for logging).
func (c CommitHash) Short() string {
	if len(c.value) < 12 {
		return c.value
	}
	return c.value[:12]
}

// RangeEventType mirrors OSV range event types.
type RangeEventType string

const (
	RangeEventIntroduced   RangeEventType = "introduced"
	RangeEventFixed        RangeEventType = "fixed"
	RangeEventLimit        RangeEventType = "limit"
	RangeEventLastAffected RangeEventType = "last_affected"
)

// VersionRangeEvent represents a single introduced/fixed/limit/last_affected event.
type VersionRangeEvent struct {
	Type    RangeEventType
	Version string // raw version string (e.g. "1.2.3") or commit hash
}

// EcosystemName is the normalised ecosystem identifier (e.g. "PyPI", "Go").
type EcosystemName struct {
	value string
}

// NewEcosystemName validates and creates an EcosystemName.
func NewEcosystemName(raw string) (EcosystemName, error) {
	if strings.TrimSpace(raw) == "" {
		return EcosystemName{}, fmt.Errorf("ecosystem name must not be empty")
	}
	return EcosystemName{value: strings.TrimSpace(raw)}, nil
}

// String returns the ecosystem name.
func (e EcosystemName) String() string { return e.value }

// ContentHash is a SHA-256 hex digest used for staleness detection.
type ContentHash struct {
	value string
}

// NewContentHash creates a ContentHash from a raw hex string.
func NewContentHash(raw string) (ContentHash, error) {
	if !sha256R.MatchString(strings.ToLower(raw)) {
		return ContentHash{}, fmt.Errorf("invalid content hash %q", raw)
	}
	return ContentHash{value: strings.ToLower(raw)}, nil
}

// String returns the hash hex.
func (c ContentHash) String() string { return c.value }
