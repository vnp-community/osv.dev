// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Package nvd converts NVD JSON v2.0 API responses to OSV format.
// NVD API v2 spec: https://nvd.nist.gov/developers/vulnerabilities
package nvd

import (
	"fmt"
	"strings"
	"time"

	"github.com/ossf/osv-schema/bindings/go/osvschema"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ─── NVD JSON v2 Types ────────────────────────────────────────────────────────

// NVDResponse is the top-level response from the NVD API v2.
type NVDResponse struct {
	ResultsPerPage  int               `json:"resultsPerPage"`
	StartIndex      int               `json:"startIndex"`
	TotalResults    int               `json:"totalResults"`
	Format          string            `json:"format"`
	Version         string            `json:"version"`
	Timestamp       string            `json:"timestamp"`
	Vulnerabilities []NVDVulnerability `json:"vulnerabilities"`
}

// NVDVulnerability wraps a single CVE entry.
type NVDVulnerability struct {
	CVE NVDCVE `json:"cve"`
}

// NVDCVE represents a single CVE entry in NVD JSON v2 format.
type NVDCVE struct {
	ID                    string          `json:"id"`
	SourceIdentifier      string          `json:"sourceIdentifier"`
	Published             time.Time       `json:"published"`
	LastModified          time.Time       `json:"lastModified"`
	VulnStatus            string          `json:"vulnStatus"` // "Analyzed", "Modified", "Awaiting Analysis"
	Descriptions          []LangString    `json:"descriptions"`
	Metrics               *NVDMetrics     `json:"metrics,omitempty"`
	Weaknesses            []NVDWeakness   `json:"weaknesses,omitempty"`
	Configurations        []Configuration `json:"configurations,omitempty"`
	References            []NVDReference  `json:"references"`
}

// LangString holds a language-tagged string.
type LangString struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

// NVDMetrics holds CVSS metrics.
type NVDMetrics struct {
	CVSSMetricV31 []CVSSMetricEntry `json:"cvssMetricV31,omitempty"`
	CVSSMetricV30 []CVSSMetricEntry `json:"cvssMetricV30,omitempty"`
	CVSSMetricV2  []CVSSMetricEntry `json:"cvssMetricV2,omitempty"`
}

// CVSSMetricEntry holds a CVSS score entry.
type CVSSMetricEntry struct {
	Source              string    `json:"source"`
	Type                string    `json:"type"` // "Primary", "Secondary"
	CVSSData            CVSSData  `json:"cvssData"`
	ExploitabilityScore float64   `json:"exploitabilityScore"`
	ImpactScore         float64   `json:"impactScore"`
}

// CVSSData holds CVSS score and vector.
type CVSSData struct {
	Version      string  `json:"version"`
	VectorString string  `json:"vectorString"`
	BaseScore    float64 `json:"baseScore"`
	BaseSeverity string  `json:"baseSeverity"`
}

// NVDWeakness holds CWE weakness information.
type NVDWeakness struct {
	Source      string       `json:"source"`
	Type        string       `json:"type"`
	Description []LangString `json:"description"`
}

// Configuration holds CPE configuration data.
type Configuration struct {
	Nodes []ConfigNode `json:"nodes"`
}

// ConfigNode holds CPE match criteria.
type ConfigNode struct {
	Operator string      `json:"operator"`
	Negate   bool        `json:"negate"`
	CPEMatch []CPEMatch  `json:"cpeMatch"`
}

// CPEMatch holds a CPE entry with version constraints.
type CPEMatch struct {
	Vulnerable            bool   `json:"vulnerable"`
	Criteria              string `json:"criteria"` // CPE 2.3 string
	MatchCriteriaID       string `json:"matchCriteriaId"`
	VersionStartIncluding string `json:"versionStartIncluding,omitempty"`
	VersionStartExcluding string `json:"versionStartExcluding,omitempty"`
	VersionEndIncluding   string `json:"versionEndIncluding,omitempty"`
	VersionEndExcluding   string `json:"versionEndExcluding,omitempty"`
}

// NVDReference is a single reference link.
type NVDReference struct {
	URL    string   `json:"url"`
	Source string   `json:"source"`
	Tags   []string `json:"tags,omitempty"`
}

// ─── Converter ────────────────────────────────────────────────────────────────

// ConversionResult holds the outcome of converting one NVD CVE.
type ConversionResult struct {
	CVEID    string
	OSV      *osvschema.Vulnerability
	Warnings []string
}

// ConvertNVDToOSV converts a single NVD CVE to OSV format.
func ConvertNVDToOSV(nvdCVE NVDCVE) (*ConversionResult, error) {
	if nvdCVE.ID == "" {
		return nil, fmt.Errorf("nvd: CVE ID is required")
	}

	result := &ConversionResult{CVEID: nvdCVE.ID}

	vuln := &osvschema.Vulnerability{
		SchemaVersion: "1.0.0",
		Id:            nvdCVE.ID,
		Published:     timestamppb.New(nvdCVE.Published),
		Modified:      timestamppb.New(nvdCVE.LastModified),
	}

	// Description
	for _, desc := range nvdCVE.Descriptions {
		if desc.Lang == "en" {
			vuln.Details = desc.Value
			break
		}
	}
	if vuln.Details == "" && len(nvdCVE.Descriptions) > 0 {
		vuln.Details = nvdCVE.Descriptions[0].Value
	}

	// Severity (CVSS)
	if nvdCVE.Metrics != nil {
		vuln.Severity = extractSeverity(nvdCVE.Metrics)
	}

	// References
	for _, ref := range nvdCVE.References {
		osvRef := &osvschema.Reference{
			Url:  ref.URL,
			Type: classifyNVDReference(ref.Tags),
		}
		vuln.References = append(vuln.References, osvRef)
	}

	// Aliases (self-referencing by convention in OSV)
	vuln.Aliases = []string{}

	// Database specific: CWE IDs and NVD vulnStatus
	cweIDs := extractCWEs(nvdCVE.Weaknesses)
	if len(cweIDs) > 0 || nvdCVE.VulnStatus != "" {
		cweValues := make([]interface{}, len(cweIDs))
		for i, c := range cweIDs {
			cweValues[i] = c
		}
		dbSpecific, err := structpb.NewStruct(map[string]interface{}{
			"nvd_published_at": nvdCVE.Published.Format(time.RFC3339),
			"cwe_ids":          cweValues,
			"severity":         extractBaseSeverity(nvdCVE.Metrics),
		})
		if err == nil {
			vuln.DatabaseSpecific = dbSpecific
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to create database_specific: %v", err))
		}
	}

	result.OSV = vuln
	return result, nil
}

// ConvertBatch converts all vulnerabilities in an NVD API response.
func ConvertBatch(resp *NVDResponse) ([]*ConversionResult, []error) {
	var results []*ConversionResult
	var errs []error

	for _, v := range resp.Vulnerabilities {
		r, err := ConvertNVDToOSV(v.CVE)
		if err != nil {
			errs = append(errs, fmt.Errorf("convert %s: %w", v.CVE.ID, err))
			continue
		}
		results = append(results, r)
	}
	return results, errs
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func extractSeverity(metrics *NVDMetrics) []*osvschema.Severity {
	var severities []*osvschema.Severity

	// Prefer CVSS v3.1 > v3.0 > v2
	for _, entries := range [][]CVSSMetricEntry{
		metrics.CVSSMetricV31,
		metrics.CVSSMetricV30,
		metrics.CVSSMetricV2,
	} {
		for _, entry := range entries {
			if entry.Type != "Primary" {
				continue
			}
			score := entry.CVSSData.VectorString
			if score == "" {
				continue
			}
			scoreType := osvschema.Severity_CVSS_V3
			if strings.HasPrefix(entry.CVSSData.Version, "2") {
				scoreType = osvschema.Severity_CVSS_V2
			}
			severities = append(severities, &osvschema.Severity{
				Type:  scoreType,
				Score: score,
			})
			return severities // Return first Primary match
		}
	}

	// Fallback: return secondary if no primary found
	for _, entries := range [][]CVSSMetricEntry{metrics.CVSSMetricV31, metrics.CVSSMetricV30, metrics.CVSSMetricV2} {
		for _, entry := range entries {
			if entry.CVSSData.VectorString != "" {
				scoreType := osvschema.Severity_CVSS_V3
				if strings.HasPrefix(entry.CVSSData.Version, "2") {
					scoreType = osvschema.Severity_CVSS_V2
				}
				severities = append(severities, &osvschema.Severity{
					Type:  scoreType,
					Score: entry.CVSSData.VectorString,
				})
				return severities
			}
		}
	}

	return nil
}

func extractBaseSeverity(metrics *NVDMetrics) string {
	if metrics == nil {
		return ""
	}
	for _, entries := range [][]CVSSMetricEntry{metrics.CVSSMetricV31, metrics.CVSSMetricV30} {
		for _, entry := range entries {
			if entry.Type == "Primary" && entry.CVSSData.BaseSeverity != "" {
				return entry.CVSSData.BaseSeverity
			}
		}
	}
	return ""
}

func extractCWEs(weaknesses []NVDWeakness) []string {
	var cwes []string
	seen := map[string]bool{}
	for _, w := range weaknesses {
		for _, desc := range w.Description {
			if strings.HasPrefix(desc.Value, "CWE-") && !seen[desc.Value] {
				cwes = append(cwes, desc.Value)
				seen[desc.Value] = true
			}
		}
	}
	return cwes
}

// classifyNVDReference maps NVD reference tags to OSV Reference_Type.
func classifyNVDReference(tags []string) osvschema.Reference_Type {
	for _, tag := range tags {
		switch strings.ToLower(tag) {
		case "patch":
			return osvschema.Reference_FIX
		case "exploit":
			return osvschema.Reference_EVIDENCE
		case "vendor advisory", "vendor-advisory":
			return osvschema.Reference_ADVISORY
		case "issue tracking":
			return osvschema.Reference_REPORT
		case "third party advisory":
			return osvschema.Reference_ADVISORY
		}
	}
	return osvschema.Reference_WEB
}
