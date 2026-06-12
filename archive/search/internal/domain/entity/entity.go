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

// Package entity holds domain entities for the Search service.
package entity

import "time"

// SearchDocument is the indexed representation of a vulnerability in OpenSearch.
//
// OpenSearch mapping schema (add to existing index via migration):
//
//	"kev":               { "type": "boolean" }
//	"kev_date_added":    { "type": "date" }
//	"epss_score":        { "type": "float" }
//	"epss_percentile":   { "type": "float" }
//	"tags":              { "type": "keyword" }
//	"cwe_ids":           { "type": "keyword" }
//	"exploit_available": { "type": "boolean" }
//	"embedding":         { "type": "knn_vector", "dimension": 1536, "method": { "name": "hnsw", "space_type": "cosinesimil", "engine": "nmslib" } }
type SearchDocument struct {
	VulnID       string    `json:"vuln_id"`
	Summary      string    `json:"summary"`
	Details      string    `json:"details"`
	Ecosystems   []string  `json:"ecosystems"`
	Packages     []string  `json:"packages"`
	PURLs        []string  `json:"purls"`
	Aliases      []string  `json:"aliases"`
	Source       string    `json:"source"`
	SeverityType string    `json:"severity_type,omitempty"`
	CVSSScore    float64   `json:"cvss_score,omitempty"`
	CVSSSeverity string    `json:"cvss_severity,omitempty"`
	Published    time.Time `json:"published,omitempty"`
	Modified     time.Time `json:"modified"`
	IsWithdrawn  bool      `json:"is_withdrawn"`

	// ── TASK-07-01: Enrichment fields ─────────────────────────────────────────

	// KEV (CISA Known Exploited Vulnerability) fields
	KEV         bool   `json:"kev,omitempty"`
	KEVDateAdded string `json:"kev_date_added,omitempty"` // RFC3339 date

	// EPSS (Exploit Prediction Scoring System) fields
	EPSSScore      float64 `json:"epss_score,omitempty"`
	EPSSPercentile float64 `json:"epss_percentile,omitempty"`

	// Taxonomy fields
	Tags   []string `json:"tags,omitempty"`   // e.g. ["attack:network", "impact:dos", "status:kev"]
	CWEIDs []string `json:"cwe_ids,omitempty"` // e.g. ["CWE-400", "CWE-835"]

	// Exploit availability
	ExploitAvailable bool `json:"exploit_available,omitempty"`

	// ── TASK-07-02: Semantic search embedding ─────────────────────────────────

	// DescriptionEmbedding is the knn_vector field.
	// Dimension 1536 (text-embedding-3-small) or 768 (text-embedding-gecko).
	// Set only when semantic indexing is enabled.
	DescriptionEmbedding []float32 `json:"description_embedding,omitempty"`
}

// SearchResult is a single search hit returned to callers.
type SearchResult struct {
	VulnID     string    `json:"vuln_id"`
	Summary    string    `json:"summary"`
	Ecosystem  string    `json:"ecosystem"`
	Severity   string    `json:"severity,omitempty"`
	Modified   time.Time `json:"modified"`
	Score      float32   `json:"_score"`

	// Enrichment data surfaced in search results
	KEV            bool    `json:"kev,omitempty"`
	EPSSScore      float64 `json:"epss_score,omitempty"`
	EPSSPercentile float64 `json:"epss_percentile,omitempty"`
	Tags           []string `json:"tags,omitempty"`

	Highlights []Highlight `json:"highlights,omitempty"`

	// Similarity score for semantic search (0.0–1.0)
	Similarity float32 `json:"similarity,omitempty"`
}

// Highlight is a matched text fragment.
type Highlight struct {
	Field   string `json:"field"`
	Snippet string `json:"snippet"`
}

// Facet is a faceted aggregation bucket.
type Facet struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// FacetedSearchResult groups hits with aggregation buckets.
type FacetedSearchResult struct {
	Hits   []*SearchResult `json:"hits"`
	Total  int64           `json:"total"`
	Facets FacetMap        `json:"aggregations,omitempty"`
}

// FacetMap maps facet dimension names to their buckets.
type FacetMap map[string][]Facet

// OpenSearchIndexMapping returns the partial mapping JSON to add enrichment
// fields to an existing OpenSearch index. Apply via PUT /{index}/_mapping.
const OpenSearchIndexMapping = `{
  "properties": {
    "kev": { "type": "boolean" },
    "kev_date_added": { "type": "date" },
    "epss_score": { "type": "float" },
    "epss_percentile": { "type": "float" },
    "tags": { "type": "keyword" },
    "cwe_ids": { "type": "keyword" },
    "exploit_available": { "type": "boolean" },
    "description_embedding": {
      "type": "knn_vector",
      "dimension": 1536,
      "method": {
        "name": "hnsw",
        "space_type": "cosinesimil",
        "engine": "nmslib"
      }
    }
  }
}`
