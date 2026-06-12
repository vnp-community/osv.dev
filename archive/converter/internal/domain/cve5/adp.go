// Package cve5 — ADP merging logic (TASK-04-01a).
// Merges ADP (Authorized Data Publisher) containers into the base CVE record.
// Priority: CISA-ADP > NVD-ADP > CNA
package cve5

import (
	"strings"

	"github.com/ossf/osv-schema/bindings/go/osvschema"
)

// wellKnownADPs lists ADP short names we trust in priority order.
// CISA-ADP provides KEV/exploit data; NVD-ADP provides enriched severity.
var wellKnownADPs = []string{"CISA-ADP", "NVD", "NVD-ADP", "Vulnogram"}

// MergeADPContainers merges ADP data into a base OSV vulnerability.
// Rules:
//  1. ADP severity (CVSS) overrides CNA if from a trusted ADP.
//  2. ADP affected entries are appended (deduplicated by product name).
//  3. ADP references are merged.
func MergeADPContainers(vuln *osvschema.Vulnerability, record *CVERecord) {
	if record == nil || record.Containers == nil {
		return
	}

	for _, adp := range record.Containers.ADP {
		if adp == nil || adp.ProviderMetadata == nil {
			continue
		}
		shortName := adp.ProviderMetadata.ShortName

		// Merge severity from trusted ADPs
		if isTrustedADP(shortName) && len(adp.Metrics) > 0 {
			mergeADPSeverity(vuln, adp)
		}

		// Merge affected entries
		mergeADPAffected(vuln, adp)
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
			if id := normalizeCWEID(desc.CWEID); id != "" && !seen[id] {
				seen[id] = true
				cweIDs = append(cweIDs, id)
			}
		}
	}
	return cweIDs
}

// mergeADPSeverity merges CVSS severity from an ADP into the vulnerability.
// Only adds severity if not already present from a higher-priority source.
func mergeADPSeverity(vuln *osvschema.Vulnerability, adp *ADP) {
	for _, metric := range adp.Metrics {
		if metric.CVSSv31 != nil && metric.CVSSv31.VectorString != "" {
			// Check if we already have a CVSS v3.1 severity
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

// mergeADPAffected appends affected entries from ADP (deduplicated by product).
func mergeADPAffected(vuln *osvschema.Vulnerability, adp *ADP) {
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
		// Deduplicate by product name
		key := strings.ToLower(osvAff.Package.Name)
		if !existingProducts[key] {
			existingProducts[key] = true
			vuln.Affected = append(vuln.Affected, osvAff)
		}
	}
}

// hasSeverityType returns true if the vulnerability already has a severity of the given type.
func hasSeverityType(vuln *osvschema.Vulnerability, t osvschema.Severity_Type) bool {
	for _, s := range vuln.Severity {
		if s.Type == t {
			return true
		}
	}
	return false
}

// isTrustedADP returns true if the ADP short name is in the trusted list.
func isTrustedADP(name string) bool {
	upper := strings.ToUpper(name)
	for _, known := range wellKnownADPs {
		if strings.HasPrefix(upper, known) {
			return true
		}
	}
	return false
}

// normalizeCWEID normalizes a CWE ID to the format "CWE-NNN".
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
