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

// Package classification provides CVE severity scoring, tier classification,
// and deterministic auto-tagging based on CVSS vectors and CVE descriptions.
//
// This package implements rule-based (fast, deterministic) classification.
// For AI/LLM-based classification, see services/ai-enrichment.
package classification

import (
	"strings"

	"github.com/ossf/osv-schema/bindings/go/osvschema"
)

// SeverityTier is a qualitative severity classification.
type SeverityTier string

const (
	TierCritical SeverityTier = "CRITICAL"
	TierHigh     SeverityTier = "HIGH"
	TierMedium   SeverityTier = "MEDIUM"
	TierLow      SeverityTier = "LOW"
	TierUnknown  SeverityTier = "UNKNOWN"
)

// CVSSTier returns a SeverityTier based on a CVSS v3 score (0.0–10.0).
func CVSSTier(score float64) SeverityTier {
	switch {
	case score >= 9.0:
		return TierCritical
	case score >= 7.0:
		return TierHigh
	case score >= 4.0:
		return TierMedium
	case score > 0:
		return TierLow
	default:
		return TierUnknown
	}
}

// Tag represents an auto-generated classification label.
type Tag string

// Attack vector tags.
const (
	TagNetworkBased  Tag = "attack:network"
	TagLocalAccess   Tag = "attack:local"
	TagAdjacentNet   Tag = "attack:adjacent"
	TagPhysical      Tag = "attack:physical"
)

// Impact type tags.
const (
	TagRCE           Tag = "impact:rce"
	TagPrivesc       Tag = "impact:privilege-escalation"
	TagDOS           Tag = "impact:dos"
	TagInfoDisc      Tag = "impact:info-disclosure"
	TagSQLi          Tag = "impact:sqli"
	TagXSS           Tag = "impact:xss"
	TagSSRF          Tag = "impact:ssrf"
	TagXXE           Tag = "impact:xxe"
	TagPathTraversal Tag = "impact:path-traversal"
	TagCSRF          Tag = "impact:csrf"
	TagMemorySafety  Tag = "impact:memory-safety"
	TagDeserialize   Tag = "impact:deserialization"
)

// Status tags.
const (
	TagHasFix    Tag = "status:has-fix"
	TagNoFix     Tag = "status:no-fix"
	TagWithdrawn Tag = "status:withdrawn"
	TagDisputed  Tag = "status:disputed"
	TagKEV       Tag = "status:kev"
)

// Severity tags.
const (
	TagSevCritical Tag = "severity:critical"
	TagSevHigh     Tag = "severity:high"
	TagSevMedium   Tag = "severity:medium"
	TagSevLow      Tag = "severity:low"
	TagSevUnknown  Tag = "severity:unknown"
)

// cvssV3VectorComponents parses a CVSS v3 vector string into a map of key→value.
// Example: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"
func cvssV3VectorComponents(vector string) map[string]string {
	result := make(map[string]string)
	// Remove "CVSS:3.x/" prefix
	for _, part := range strings.Split(vector, "/") {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

// TagsFromCVSSVector generates attack vector and impact tags from a CVSS v3 vector string.
func TagsFromCVSSVector(vector string) []Tag {
	if vector == "" {
		return nil
	}

	comps := cvssV3VectorComponents(vector)
	var tags []Tag

	// Attack Vector
	switch comps["AV"] {
	case "N":
		tags = append(tags, TagNetworkBased)
	case "A":
		tags = append(tags, TagAdjacentNet)
	case "L":
		tags = append(tags, TagLocalAccess)
	case "P":
		tags = append(tags, TagPhysical)
	}

	// Impact flags — Confidentiality:H + Integrity:H + Availability:H → potential RCE
	c, i, a := comps["C"], comps["I"], comps["A"]
	if c == "H" && i == "H" {
		// High C+I is often associated with code execution
		tags = append(tags, TagRCE)
	}
	if a == "H" && c != "H" && i != "H" {
		tags = append(tags, TagDOS)
	}
	if c == "H" && i == "N" && a == "N" {
		tags = append(tags, TagInfoDisc)
	}

	return tags
}

// TagsFromDescription generates impact tags based on keywords in the CVE description.
// This is a fast, rule-based approximation; AI enrichment provides more accuracy.
func TagsFromDescription(description string) []Tag {
	if description == "" {
		return nil
	}

	lower := strings.ToLower(description)
	var tags []Tag

	// Check each pattern
	for _, rule := range descriptionRules {
		for _, keyword := range rule.keywords {
			if strings.Contains(lower, keyword) {
				tags = append(tags, rule.tag)
				break // one match per rule is enough
			}
		}
	}

	return tags
}

// descriptionRule maps keywords to an impact tag.
type descriptionRule struct {
	tag      Tag
	keywords []string
}

// descriptionRules is the ordered list of keyword→tag mappings.
var descriptionRules = []descriptionRule{
	{TagSQLi, []string{"sql injection", "sqli", "sql query"}},
	{TagXSS, []string{"cross-site scripting", "xss", "stored xss", "reflected xss"}},
	{TagSSRF, []string{"server-side request forgery", "ssrf"}},
	{TagXXE, []string{"xml external entity", "xxe", "xml injection"}},
	{TagPathTraversal, []string{"path traversal", "directory traversal", "../"}},
	{TagCSRF, []string{"cross-site request forgery", "csrf"}},
	{TagMemorySafety, []string{"buffer overflow", "heap overflow", "stack overflow", "use-after-free", "out-of-bounds", "memory corruption", "double free"}},
	{TagDeserialize, []string{"deserialization", "deserialize", "object injection"}},
	{TagRCE, []string{"remote code execution", "rce", "arbitrary code execution", "command injection"}},
	{TagPrivesc, []string{"privilege escalation", "escalate privileges", "gain elevated", "root access"}},
	{TagDOS, []string{"denial of service", "dos", "resource exhaustion", "infinite loop"}},
	{TagInfoDisc, []string{"information disclosure", "sensitive information", "information leak", "data exposure"}},
}

// VulnerabilityResult holds the classification result for a single vulnerability.
type VulnerabilityResult struct {
	// SeverityTier is the computed severity classification.
	SeverityTier SeverityTier
	// Tags is the combined set of auto-generated tags (deduplicated).
	Tags []Tag
	// HasFix indicates whether a fixed version is specified.
	HasFix bool
	// IsWithdrawn indicates whether the vulnerability has been withdrawn.
	IsWithdrawn bool
}

// Classify performs fast, rule-based classification of an OSV vulnerability.
func Classify(v *osvschema.Vulnerability) *VulnerabilityResult {
	result := &VulnerabilityResult{
		SeverityTier: TierUnknown,
	}

	if v == nil {
		return result
	}

	// Check withdrawn
	if v.Withdrawn != nil && v.Withdrawn.Seconds != 0 {
		result.IsWithdrawn = true
		result.Tags = append(result.Tags, TagWithdrawn)
	}

	// Extract CVSS-based severity
	for _, sev := range v.Severity {
		if sev.Type == osvschema.Severity_CVSS_V3 || sev.Type == osvschema.Severity_CVSS_V4 {
			result.Tags = append(result.Tags, TagsFromCVSSVector(sev.Score)...)
		}
	}

	// Description-based tags
	result.Tags = append(result.Tags, TagsFromDescription(v.Details)...)
	result.Tags = append(result.Tags, TagsFromDescription(v.Summary)...)

	// Check has-fix across all affected packages
	for _, aff := range v.Affected {
		for _, r := range aff.Ranges {
			for _, e := range r.Events {
				if e.Fixed != "" || e.Limit != "" {
					result.HasFix = true
				}
			}
		}
	}

	if result.HasFix {
		result.Tags = append(result.Tags, TagHasFix)
	} else if !result.IsWithdrawn {
		result.Tags = append(result.Tags, TagNoFix)
	}

	// Severity tier from CVSS score
	for _, sev := range v.Severity {
		if sev.Score != "" {
			result.SeverityTier = cvssStringToTier(sev.Score)
			break
		}
	}

	// Severity tag
	result.Tags = append(result.Tags, severityTierToTag(result.SeverityTier))

	result.Tags = deduplicateTags(result.Tags)
	return result
}

func cvssStringToTier(cvssScore string) SeverityTier {
	// CVSS v3 score format: "CVSS:3.1/AV:N/AC:L/.../..."
	// Base score is not directly in the vector string; we approximate from impact flags
	comps := cvssV3VectorComponents(cvssScore)
	c, i, a := comps["C"], comps["I"], comps["A"]

	criticalCount := 0
	for _, v := range []string{c, i, a} {
		if v == "H" {
			criticalCount++
		}
	}

	switch {
	case criticalCount == 3:
		return TierCritical
	case criticalCount == 2:
		return TierHigh
	case criticalCount == 1:
		return TierMedium
	default:
		return TierLow
	}
}

func severityTierToTag(tier SeverityTier) Tag {
	switch tier {
	case TierCritical:
		return TagSevCritical
	case TierHigh:
		return TagSevHigh
	case TierMedium:
		return TagSevMedium
	case TierLow:
		return TagSevLow
	default:
		return TagSevUnknown
	}
}

func deduplicateTags(tags []Tag) []Tag {
	seen := make(map[Tag]bool, len(tags))
	result := make([]Tag, 0, len(tags))
	for _, t := range tags {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}
