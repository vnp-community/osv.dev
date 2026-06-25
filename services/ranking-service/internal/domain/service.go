package domain

import "strings"

// SplitCPEParts splits a CPE string into meaningful matchable parts.
// Used for loosy ranking lookup: tries each part from most-specific to least.
//
// Mirrors Python: lib/CVEs.py::getranking() loosy CPE algorithm in cve-search
//
// Examples:
//
//	"cpe:2.3:a:sap:netweaver:7.5:*:*:*:*:*:*:*" → ["a", "sap", "netweaver", "7.5"]
//	"cpe:2.2:/a:apache:log4j:2.14.1"             → ["a", "apache", "log4j", "2.14.1"]
//	"sap:netweaver"                               → ["sap", "netweaver"]
func SplitCPEParts(cpe string) []string {
	// Skip these tokens — they're not meaningful for ranking lookup
	skip := map[string]bool{
		"cpe": true, "2.3": true, "2.2": true,
		"*": true, "-": true, "": true,
	}

	parts := strings.Split(cpe, ":")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Strip leading slash (cpe:2.2:/a format)
		p = strings.TrimPrefix(p, "/")
		if !skip[p] && len(p) >= 2 { // min 2 chars to avoid noise
			result = append(result, p)
		}
	}
	return result
}
