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

// Package semver provides SemVer normalization utilities for OSV services.
package semver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var semverRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9.\-]+))?(?:\+([a-zA-Z0-9.\-]+))?$`)

// Version represents a parsed semantic version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
	Raw        string
}

// Parse parses a semver string (with optional leading 'v').
// Returns an error if the string does not conform to semver.
func Parse(s string) (*Version, error) {
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return nil, fmt.Errorf("semver: %q is not a valid semantic version", s)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return &Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: m[4],
		Build:      m[5],
		Raw:        s,
	}, nil
}

// Normalize strips the leading 'v' and returns a canonical semver string.
func Normalize(s string) string {
	return strings.TrimPrefix(s, "v")
}

// SortKey returns a lexicographically comparable sort key for the version.
// Format: "MMMMMMMM.MMMMMMMM.PPPPPPPP" where each part is zero-padded to 8 digits.
func SortKey(s string) string {
	v, err := Parse(s)
	if err != nil {
		// Fallback: return raw string zero-padded as best effort
		return fmt.Sprintf("00000000.00000000.00000000")
	}
	return fmt.Sprintf("%08d.%08d.%08d", v.Major, v.Minor, v.Patch)
}

// Compare compares two semver strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func Compare(a, b string) int {
	va, err1 := Parse(a)
	vb, err2 := Parse(b)
	if err1 != nil || err2 != nil {
		return strings.Compare(Normalize(a), Normalize(b))
	}
	if va.Major != vb.Major {
		return cmp(va.Major, vb.Major)
	}
	if va.Minor != vb.Minor {
		return cmp(va.Minor, vb.Minor)
	}
	if va.Patch != vb.Patch {
		return cmp(va.Patch, vb.Patch)
	}
	// Pre-release: no pre-release > pre-release
	if va.Prerelease == vb.Prerelease {
		return 0
	}
	if va.Prerelease == "" {
		return 1
	}
	if vb.Prerelease == "" {
		return -1
	}
	return strings.Compare(va.Prerelease, vb.Prerelease)
}

func cmp(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
