// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Package cve5 parses the CVE Record Format (CVE v5.0/v5.1) and converts it to OSV format.
//
// CVE JSON 5.0 spec: https://github.com/CVEProject/cvelistV5
// OSV format spec:   https://ossf.github.io/osv-schema/
package cve5

import (
	"fmt"
	"strings"
	"time"

	"github.com/ossf/osv-schema/bindings/go/osvschema"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CVERecord represents the top-level structure of a CVE v5 JSON record.
type CVERecord struct {
	DataType    string       `json:"dataType"`
	DataVersion string       `json:"dataVersion"`
	CVEMetadata *CVEMetadata `json:"cveMetadata"`
	Containers  *Containers  `json:"containers"`
}

// CVEMetadata holds the metadata fields of a CVE record.
type CVEMetadata struct {
	CVEID             string    `json:"cveId"`
	AssignerOrgID     string    `json:"assignerOrgId"`
	AssignerShortName string    `json:"assignerShortName"`
	RequesterUserID   string    `json:"requesterUserId"`
	Serial            int       `json:"serial"`
	State             string    `json:"cveState"` // "PUBLISHED", "REJECTED"
	DatePublished     time.Time `json:"datePublished"`
	DateUpdated       time.Time `json:"dateUpdated"`
}

// Containers wraps the CNA and ADP containers.
type Containers struct {
	CNA *CNA   `json:"cna"`
	ADP []*ADP `json:"adp,omitempty"`
}

// CNA represents the CNA (CVE Numbering Authority) container.
type CNA struct {
	ProviderMetadata *ProviderMetadata `json:"providerMetadata"`
	Descriptions     []*Description    `json:"descriptions"`
	Affected         []*AffectedEntry  `json:"affected"`
	References       []*Reference      `json:"references"`
	ProblemTypes     []*ProblemType    `json:"problemTypes,omitempty"`
	Metrics          []*Metric         `json:"metrics,omitempty"`
	DatePublic       *time.Time        `json:"datePublic,omitempty"`
}

// ADP represents an ADP (Authorized Data Publisher) container.
type ADP struct {
	ProviderMetadata *ProviderMetadata `json:"providerMetadata"`
	Descriptions     []*Description    `json:"descriptions,omitempty"`
	Affected         []*AffectedEntry  `json:"affected,omitempty"`
	Metrics          []*Metric         `json:"metrics,omitempty"`
}

// ProviderMetadata identifies the data provider.
type ProviderMetadata struct {
	OrgID       string `json:"orgId"`
	ShortName   string `json:"shortName"`
	DateUpdated string `json:"dateUpdated,omitempty"`
}

// Description holds a text description in a given language.
type Description struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

// AffectedEntry describes affected versions of a product.
type AffectedEntry struct {
	Vendor   string     `json:"vendor"`
	Product  string     `json:"product"`
	Versions []*Version `json:"versions"`
}

// Version represents an affected version range or specific version.
type Version struct {
	Version    string `json:"version"`
	Status     string `json:"status"` // "affected", "unaffected"
	LessThan   string `json:"lessThan,omitempty"`
	LessThanOrEqual string `json:"lessThanOrEqual,omitempty"`
	VersionType string `json:"versionType,omitempty"` // "semver", "git", "custom"
}

// Reference is an external reference for the CVE.
type Reference struct {
	URL  string   `json:"url"`
	Name string   `json:"name,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

// ProblemType represents a CWE or similar vulnerability type.
type ProblemType struct {
	Descriptions []*ProblemTypeDescription `json:"descriptions"`
}

// ProblemTypeDescription is an individual CWE entry.
type ProblemTypeDescription struct {
	Lang        string `json:"lang"`
	Description string `json:"description"`
	CWEID       string `json:"cweId,omitempty"`
	Type        string `json:"type,omitempty"`
}

// Metric holds CVSS or other severity metrics.
type Metric struct {
	Format   string `json:"format"` // "CVSS"
	Scenarios []*Scenario `json:"scenarios,omitempty"`
	CVSSv30  *CVSSData `json:"cvssV3_0,omitempty"`
	CVSSv31  *CVSSData `json:"cvssV3_1,omitempty"`
	CVSSv40  *CVSSData `json:"cvssV4_0,omitempty"`
}

// Scenario is a conditional scenario for a metric.
type Scenario struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

// CVSSData holds a CVSS vector string and base score.
type CVSSData struct {
	VectorString string  `json:"vectorString"`
	BaseScore    float64 `json:"baseScore"`
	BaseSeverity string  `json:"baseSeverity"`
}

// ConvertToOSV converts a CVE v5 record to OSV vulnerability format.
func ConvertToOSV(record *CVERecord) (*osvschema.Vulnerability, error) {
	if record == nil {
		return nil, fmt.Errorf("cve5: nil record")
	}
	if record.CVEMetadata == nil {
		return nil, fmt.Errorf("cve5: missing cveMetadata")
	}
	if record.Containers == nil || record.Containers.CNA == nil {
		return nil, fmt.Errorf("cve5: missing CNA container")
	}

	meta := record.CVEMetadata
	cna := record.Containers.CNA

	vuln := &osvschema.Vulnerability{
		SchemaVersion: "1.7.0",
		Id:            meta.CVEID,
	}

	// Timestamps
	if !meta.DateUpdated.IsZero() {
		vuln.Modified = timestamppb.New(meta.DateUpdated)
	}
	if !meta.DatePublished.IsZero() {
		vuln.Published = timestamppb.New(meta.DatePublished)
	}

	// Description: prefer English
	vuln.Details = selectDescription(cna.Descriptions, "en")
	if idx := strings.Index(vuln.Details, "\n"); idx > 0 {
		vuln.Summary = vuln.Details[:idx]
	} else if len(vuln.Details) > 200 {
		vuln.Summary = vuln.Details[:200] + "..."
	} else {
		vuln.Summary = vuln.Details
	}

	// References
	for _, ref := range cna.References {
		vuln.References = append(vuln.References, &osvschema.Reference{
			Type: classifyReference(ref),
			Url:  ref.URL,
		})
	}

	// Severity from CVSS
	for _, metric := range cna.Metrics {
		if metric.CVSSv31 != nil {
			vuln.Severity = append(vuln.Severity, &osvschema.Severity{
				Type:  osvschema.Severity_CVSS_V3,
				Score: metric.CVSSv31.VectorString,
			})
		} else if metric.CVSSv40 != nil {
			vuln.Severity = append(vuln.Severity, &osvschema.Severity{
				Type:  osvschema.Severity_CVSS_V4,
				Score: metric.CVSSv40.VectorString,
			})
		}
	}

	// Affected packages
	for _, aff := range cna.Affected {
		osvAffected := convertAffected(aff)
		if osvAffected != nil {
			vuln.Affected = append(vuln.Affected, osvAffected)
		}
	}

	// Add CVE ID as alias
	if meta.CVEID != "" {
		vuln.Aliases = []string{meta.CVEID}
	}

	return vuln, nil
}

func selectDescription(descriptions []*Description, preferLang string) string {
	var fallback string
	for _, d := range descriptions {
		if strings.HasPrefix(strings.ToLower(d.Lang), preferLang) {
			return d.Value
		}
		fallback = d.Value
	}
	return fallback
}

func classifyReference(ref *Reference) osvschema.Reference_Type {
	lower := strings.ToLower(ref.URL)
	for _, tag := range ref.Tags {
		switch strings.ToLower(tag) {
		case "patch":
			return osvschema.Reference_FIX
		case "vendor-advisory":
			return osvschema.Reference_ADVISORY
		case "exploit":
			return osvschema.Reference_EVIDENCE
		case "issue-tracking":
			return osvschema.Reference_REPORT
		}
	}
	if strings.Contains(lower, "github.com") && strings.Contains(lower, "/issues/") {
		return osvschema.Reference_REPORT
	}
	if strings.Contains(lower, "github.com") && strings.Contains(lower, "/commit/") {
		return osvschema.Reference_FIX
	}
	if strings.Contains(lower, "nvd.nist.gov") {
		return osvschema.Reference_ADVISORY
	}
	return osvschema.Reference_WEB
}

func convertAffected(aff *AffectedEntry) *osvschema.Affected {
	if aff == nil {
		return nil
	}

	osvAff := &osvschema.Affected{
		Package: &osvschema.Package{
			Name: aff.Product,
		},
	}

	// Convert version ranges
	var events []*osvschema.Event
	for _, v := range aff.Versions {
		if v.Status == "affected" {
			events = append(events, &osvschema.Event{Introduced: v.Version})
			if v.LessThan != "" {
				events = append(events, &osvschema.Event{Fixed: v.LessThan})
			} else if v.LessThanOrEqual != "" {
				events = append(events, &osvschema.Event{LastAffected: v.LessThanOrEqual})
			}
		}
	}

	if len(events) > 0 {
		osvAff.Ranges = []*osvschema.Range{
			{
				Type:   osvschema.Range_ECOSYSTEM,
				Events: events,
			},
		}
	}

	return osvAff
}
