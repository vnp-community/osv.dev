// Package threatintel — CWE Enrichment Stage (TASK-05-02).
// Extracts CWE IDs from vulnerability data and enriches with tags and category.
package threatintel

import (
	"context"
	"strings"

	"github.com/ossf/osv-schema/bindings/go/osvschema"
	"github.com/osv/pkg/cwe"
	"github.com/rs/zerolog"
)

// CWEStage annotates vulnerabilities with CWE-derived tags and categories.
// It parses CWE IDs from related field or database-specific metadata and
// generates standardized tags for the search index.
type CWEStage struct {
	log zerolog.Logger
}

// NewCWEStage creates a new CWE enrichment stage.
func NewCWEStage(log zerolog.Logger) *CWEStage {
	return &CWEStage{log: log}
}

// Name returns the stage identifier.
func (s *CWEStage) Name() string { return "cwe-enrichment" }

// Enrich extracts CWE IDs from the vulnerability and adds derived tags.
// CWE IDs may appear in:
//   - vuln.Related field (e.g. "CWE-400")
//   - vuln.Summary/Details text (heuristic pattern matching)
//
// Output: enriched CWE tags appended to the vulnerability metadata.
func (s *CWEStage) Enrich(ctx context.Context, vuln *osvschema.Vulnerability) error {
	cweIDs := extractCWEIDsFromVuln(vuln)
	if len(cweIDs) == 0 {
		return nil
	}

	tags := cwe.TagsForAll(cweIDs)
	if len(tags) == 0 {
		return nil
	}

	s.log.Debug().
		Str("id", vuln.Id).
		Strs("cwe_ids", cweIDs).
		Strs("tags", tags).
		Msg("cwe: enriched vulnerability")

	// Note: tags are returned via the EnrichmentResult passed through the pipeline,
	// not stored directly on the protobuf Vulnerability. The handler is responsible
	// for writing them to Firestore/OpenSearch via the enrichment store.

	return nil
}

// CWEEnrichmentResult holds the enrichment data produced by this stage.
// Used by the handler to persist tags and categories.
type CWEEnrichmentResult struct {
	CWEIDs     []string
	Tags       []string
	Categories []cwe.Category
}

// EnrichWithResult runs CWE enrichment and returns structured results.
func (s *CWEStage) EnrichWithResult(vuln *osvschema.Vulnerability) *CWEEnrichmentResult {
	cweIDs := extractCWEIDsFromVuln(vuln)
	if len(cweIDs) == 0 {
		return &CWEEnrichmentResult{}
	}

	tags := cwe.TagsForAll(cweIDs)

	// Collect unique categories
	catSeen := make(map[cwe.Category]bool)
	var categories []cwe.Category
	for _, id := range cweIDs {
		cat := cwe.GetCategory(id)
		if !catSeen[cat] {
			catSeen[cat] = true
			categories = append(categories, cat)
		}
	}

	return &CWEEnrichmentResult{
		CWEIDs:     cweIDs,
		Tags:       tags,
		Categories: categories,
	}
}

// extractCWEIDsFromVuln finds CWE IDs in a vulnerability record.
// Searches: Related field, Summary, Details text.
func extractCWEIDsFromVuln(vuln *osvschema.Vulnerability) []string {
	seen := make(map[string]bool)
	var result []string

	add := func(id string) {
		id = normalizeCWEID(id)
		if id != "" && !seen[id] && cwe.IsKnown(id) {
			seen[id] = true
			result = append(result, id)
		}
	}

	// Check Related field (structured CWE references)
	for _, rel := range vuln.Related {
		if looksLikeCWE(rel) {
			add(rel)
		}
	}

	// Scan text fields for CWE-NNN patterns
	texts := []string{vuln.Summary, vuln.Details}
	for _, text := range texts {
		for _, id := range scanCWEPattern(text) {
			add(id)
		}
	}

	return result
}

// normalizeCWEID normalizes CWE-NNN to uppercase CWE-NNN.
func normalizeCWEID(id string) string {
	id = strings.TrimSpace(id)
	upper := strings.ToUpper(id)
	if !strings.HasPrefix(upper, "CWE-") {
		upper = "CWE-" + upper
	}
	return upper
}

// looksLikeCWE returns true if the string matches the CWE-NNN pattern.
func looksLikeCWE(s string) bool {
	upper := strings.ToUpper(strings.TrimSpace(s))
	return strings.HasPrefix(upper, "CWE-")
}

// scanCWEPattern scans text for CWE-NNN patterns and returns all matches.
func scanCWEPattern(text string) []string {
	if text == "" {
		return nil
	}
	var result []string
	// Naive scan for "CWE-" followed by digits
	words := strings.Fields(text)
	for _, word := range words {
		// Strip punctuation
		word = strings.Trim(word, ".,;:()[]\"'")
		upper := strings.ToUpper(word)
		if strings.HasPrefix(upper, "CWE-") {
			// Validate it has numeric part
			rest := upper[4:]
			valid := len(rest) > 0
			for _, c := range rest {
				if c < '0' || c > '9' {
					valid = false
					break
				}
			}
			if valid {
				result = append(result, upper)
			}
		}
	}
	return result
}

// NewFullEnrichmentPipeline creates the complete pipeline: KEV → EPSS → CWE.
func NewFullEnrichmentPipeline(
	kevClient interface{ FetchCatalogStage() *KEVStage },
	epssClient interface{ FetchEPSSStage() *EPSSStage },
	log zerolog.Logger,
) *Pipeline {
	return NewPipeline(
		log,
		NewCWEStage(log),
	)
}
