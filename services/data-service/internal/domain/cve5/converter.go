// Package cve5 — functions for converting CVE v5 records to OSV format
// and extracting/merging ADP data.
package cve5

import (
	"fmt"
	"strings"

	"github.com/ossf/osv-schema/bindings/go/osvschema"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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

	// Description
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

	// Severity
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

	// Affected
	for _, aff := range cna.Affected {
		osvAff := convertAffected(aff)
		if osvAff != nil {
			vuln.Affected = append(vuln.Affected, osvAff)
		}
	}

	// Alias
	if meta.CVEID != "" {
		vuln.Aliases = []string{meta.CVEID}
	}

	return vuln, nil
}

// MergeADPContainers merges ADP data into a base OSV vulnerability.
func MergeADPContainers(vuln *osvschema.Vulnerability, record *CVERecord) {
	if record == nil || record.Containers == nil {
		return
	}
	wellKnownADPs := []string{"CISA-ADP", "NVD", "NVD-ADP", "Vulnogram"}
	for _, adp := range record.Containers.ADP {
		if adp == nil || adp.ProviderMetadata == nil {
			continue
		}
		shortName := adp.ProviderMetadata.ShortName
		// Merge severity from trusted ADPs
		upper := strings.ToUpper(shortName)
		trusted := false
		for _, known := range wellKnownADPs {
			if strings.HasPrefix(upper, known) {
				trusted = true
				break
			}
		}
		if trusted && len(adp.Metrics) > 0 {
			for _, metric := range adp.Metrics {
				if metric.CVSSv31 != nil && metric.CVSSv31.VectorString != "" {
					if !hasSeverityType(vuln, osvschema.Severity_CVSS_V3) {
						vuln.Severity = append(vuln.Severity, &osvschema.Severity{
							Type:  osvschema.Severity_CVSS_V3,
							Score: metric.CVSSv31.VectorString,
						})
					}
				}
				if metric.CVSSv40 != nil && metric.CVSSv40.VectorString != "" {
					if !hasSeverityType(vuln, osvschema.Severity_CVSS_V4) {
						vuln.Severity = append(vuln.Severity, &osvschema.Severity{
							Type:  osvschema.Severity_CVSS_V4,
							Score: metric.CVSSv40.VectorString,
						})
					}
				}
			}
		}
		// Merge affected (deduplicated)
		existingProducts := make(map[string]bool)
		for _, aff := range vuln.Affected {
			if aff.Package != nil {
				existingProducts[strings.ToLower(aff.Package.Name)] = true
			}
		}
		for _, aff := range adp.Affected {
			osvAff := convertAffected(aff)
			if osvAff == nil || osvAff.Package == nil {
				continue
			}
			key := strings.ToLower(osvAff.Package.Name)
			if !existingProducts[key] {
				existingProducts[key] = true
				vuln.Affected = append(vuln.Affected, osvAff)
			}
		}
	}
}

// ExtractCWEsFromRecord extracts all CWE IDs from CNA problemTypes.
func ExtractCWEsFromRecord(record *CVERecord) []string {
	if record == nil || record.Containers == nil || record.Containers.CNA == nil {
		return nil
	}
	seen := make(map[string]bool)
	var cweIDs []string
	for _, pt := range record.Containers.CNA.ProblemTypes {
		for _, desc := range pt.Descriptions {
			id := normalizeCWEID(desc.CWEID)
			if id != "" && !seen[id] {
				seen[id] = true
				cweIDs = append(cweIDs, id)
			}
		}
	}
	return cweIDs
}

// --- helpers ---

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
			{Type: osvschema.Range_ECOSYSTEM, Events: events},
		}
	}
	return osvAff
}

func hasSeverityType(vuln *osvschema.Vulnerability, t osvschema.Severity_Type) bool {
	for _, s := range vuln.Severity {
		if s.Type == t {
			return true
		}
	}
	return false
}

func normalizeCWEID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	upper := strings.ToUpper(id)
	if !strings.HasPrefix(upper, "CWE-") {
		upper = "CWE-" + upper
	}
	return upper
}
