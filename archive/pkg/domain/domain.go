// Package domain provides shared domain types used across CVE Binary Tool services.
// These types represent the core vocabulary for cross-service communication.
package domain

import "time"

// ProductInfo identifies a software product with vendor, name and version.
// Used by Scanner (output), CVEDB (input for lookup), and Gateway (orchestration).
type ProductInfo struct {
	Vendor  string `json:"vendor"`
	Product string `json:"product"`
	Version string `json:"version"`
	PURL    string `json:"purl,omitempty"`
}

// String returns a human-readable product identifier.
func (p ProductInfo) String() string {
	if p.Vendor != "" {
		return p.Vendor + "/" + p.Product + "@" + p.Version
	}
	return p.Product + "@" + p.Version
}

// ScanInfo is a scanner output: product detected in a file.
type ScanInfo struct {
	Product  ProductInfo `json:"product"`
	FilePath string      `json:"file_path"`
}

// CVE represents the full CVE data returned by the CVEDB service.
type CVE struct {
	CVENumber     string    `json:"cve_number"`
	Severity      string    `json:"severity"`       // CRITICAL|HIGH|MEDIUM|LOW|NONE
	Remarks       Remarks   `json:"remarks"`
	Description   string    `json:"description"`
	Score         float64   `json:"score"`
	CVSSVersion   int       `json:"cvss_version"`   // 2 or 3
	CVSSVector    string    `json:"cvss_vector"`
	DataSource    string    `json:"data_source"`
	IsExploit     bool      `json:"is_exploit"`     // Listed in KEV
	Justification string    `json:"justification,omitempty"`
	Response      []string  `json:"response,omitempty"`
	EPSS          float64   `json:"epss,omitempty"`          // EPSS probability
	EPSSPctile    float64   `json:"epss_percentile,omitempty"` // EPSS percentile
	LastModified  time.Time `json:"last_modified,omitempty"`
}

// Remarks represents the triage/review status of a CVE finding.
// Maps to Python Remarks OrderedEnum.
type Remarks int

const (
	RemarksNewFound      Remarks = 1 // Just discovered, unreviewed
	RemarksUnexplored    Remarks = 2 // Under investigation
	RemarksConfirmed     Remarks = 3 // Confirmed as applicable
	RemarksMitigated     Remarks = 4 // Mitigated/patched
	RemarksFalsePositive Remarks = 5 // Confirmed false positive
	RemarksNotAffected   Remarks = 6 // Vendor confirmed not affected
)

// String returns the string name of a Remarks value.
func (r Remarks) String() string {
	switch r {
	case RemarksNewFound:
		return "NewFound"
	case RemarksUnexplored:
		return "Unexplored"
	case RemarksConfirmed:
		return "Confirmed"
	case RemarksMitigated:
		return "Mitigated"
	case RemarksFalsePositive:
		return "FalsePositive"
	case RemarksNotAffected:
		return "NotAffected"
	default:
		return "Unknown"
	}
}

// TriageData maps CVE numbers to their triage entries.
// Loaded from VEX files and applied during CVEDB lookup.
type TriageData map[string]TriageEntry

// TriageEntry holds the triage decision for a single CVE.
type TriageEntry struct {
	Remarks       Remarks  `json:"remarks"`
	Comments      string   `json:"comments,omitempty"`
	Response      []string `json:"response,omitempty"`
	Justification string   `json:"justification,omitempty"`
}

// ProductCVEs groups CVE results by product.
type ProductCVEs struct {
	Product ProductInfo `json:"product"`
	CVEs    []CVE       `json:"cves"`
}

// DataSource names constants.
const (
	DataSourceNVD      = "NVD"
	DataSourceOSV      = "OSV"
	DataSourceGAD      = "GAD"
	DataSourceRedHat   = "REDHAT"
	DataSourceCURL     = "CURL"
	DataSourceEPSS     = "EPSS"
	DataSourcePURL2CPE = "PURL2CPE"
	DataSourceCIRCL    = "CIRCL"
	DataSourceJVN      = "JVN"
)

// AllSources returns all known data source names.
func AllSources() []string {
	return []string{
		DataSourceNVD,
		DataSourceOSV,
		DataSourceGAD,
		DataSourceRedHat,
		DataSourceCURL,
		DataSourceEPSS,
		DataSourcePURL2CPE,
	}
}
