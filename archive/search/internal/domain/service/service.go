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

// Package service contains domain services for the Search service.
package service

import (
	"strings"
	"time"

	"github.com/osv/search/internal/domain/entity"
	"github.com/osv/search/internal/domain/valueobject"
)

// QueryParser parses a raw user query string into a structured SearchQuery.
type QueryParser struct{}

// NewQueryParser creates a new QueryParser.
func NewQueryParser() *QueryParser { return &QueryParser{} }

// Parse extracts keywords, ecosystem hints, and severity filters from a raw query.
//
// Supported syntax:
//   - "ecosystem:PyPI" → sets Ecosystem filter
//   - "severity:CRITICAL" → sets Severity filter
//   - remaining tokens → Keywords
func (p *QueryParser) Parse(raw string) *valueobject.SearchQuery {
	q := &valueobject.SearchQuery{Raw: raw}
	var keywords []string

	for _, token := range strings.Fields(raw) {
		lower := strings.ToLower(token)
		switch {
		case strings.HasPrefix(lower, "ecosystem:"):
			q.Ecosystems = append(q.Ecosystems, strings.TrimPrefix(token, strings.ToLower("ecosystem:")))
		case strings.HasPrefix(lower, "severity:"):
			q.Severities = append(q.Severities, strings.ToUpper(strings.TrimPrefix(token, strings.ToLower("severity:"))))
		default:
			keywords = append(keywords, token)
		}
	}
	q.Keywords = strings.Join(keywords, " ")
	return q
}

// DocumentMapper maps vulnerability data to a SearchDocument.
type DocumentMapper struct{}

// NewDocumentMapper creates a new DocumentMapper.
func NewDocumentMapper() *DocumentMapper { return &DocumentMapper{} }

// MapVulnImportedEvent maps minimal event payload to a SearchDocument.
// Full document fields are populated from the event.
func (m *DocumentMapper) Map(v *VulnPayload) *entity.SearchDocument {
	doc := &entity.SearchDocument{
		VulnID:    v.ID,
		Summary:   v.Summary,
		Details:   v.Details,
		Source:    v.Source,
		Aliases:   v.Aliases,
		Modified:  v.Modified,
		Published: v.Published,
	}

	seen := make(map[string]struct{})
	for _, aff := range v.Affected {
		eco := aff.Ecosystem
		pkg := aff.Package
		if _, ok := seen[eco]; !ok {
			doc.Ecosystems = append(doc.Ecosystems, eco)
			seen[eco] = struct{}{}
		}
		if pkg != "" {
			doc.Packages = append(doc.Packages, pkg)
		}
		if aff.PURL != "" {
			doc.PURLs = append(doc.PURLs, aff.PURL)
		}
	}

	if v.CVSS != nil {
		doc.CVSSScore = v.CVSS.Score
		doc.CVSSSeverity = v.CVSS.Severity
		doc.SeverityType = v.CVSS.Type
	}
	return doc
}

// VulnPayload is the minimal vulnerability payload used for document mapping.
type VulnPayload struct {
	ID        string
	Summary   string
	Details   string
	Source    string
	Aliases   []string
	Published time.Time
	Modified  time.Time
	Affected  []AffectedPkg
	CVSS      *CVSSInfo
}

// AffectedPkg is a single affected package entry.
type AffectedPkg struct {
	Ecosystem string
	Package   string
	PURL      string
}

// CVSSInfo holds CVSS scoring data.
type CVSSInfo struct {
	Type     string
	Score    float64
	Severity string
}
