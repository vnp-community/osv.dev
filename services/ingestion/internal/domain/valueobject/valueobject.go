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

// Package valueobject contains value objects for the Ingestion Service.
package valueobject

import (
	"fmt"
	"strings"
)

// VulnID is a validated vulnerability identifier (e.g., "OSV-2024-001", "CVE-2024-1234").
type VulnID struct {
	value string
}

// NewVulnID creates a new VulnID, validating the format.
func NewVulnID(id string) (VulnID, error) {
	if id == "" {
		return VulnID{}, fmt.Errorf("vuln ID must not be empty")
	}
	// Basic validation: must contain at least one hyphen separating prefix and number
	if !strings.Contains(id, "-") {
		return VulnID{}, fmt.Errorf("vuln ID %q is invalid: must contain '-' separator", id)
	}
	return VulnID{value: id}, nil
}

// String returns the string representation of the VulnID.
func (v VulnID) String() string { return v.value }

// SourceRef identifies a data source by name and path.
type SourceRef struct {
	Name string // e.g., "ghsa", "nvd-cve"
	Path string // e.g., "/advisories/github-reviewed"
}

// ContentHash is a SHA-256 hex-encoded hash of the vulnerability content.
type ContentHash string

// ImportFindingType describes the category of an import quality finding.
type ImportFindingType string

const (
	FindingInvalidJSON      ImportFindingType = "INVALID_JSON"
	FindingSchemaViolation  ImportFindingType = "SCHEMA_VIOLATION"
	FindingMissingID        ImportFindingType = "MISSING_ID"
	FindingUnknownEcosystem ImportFindingType = "UNKNOWN_ECOSYSTEM"
	FindingProcessingError  ImportFindingType = "PROCESSING_ERROR"
)
