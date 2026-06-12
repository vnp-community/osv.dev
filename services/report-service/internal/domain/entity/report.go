// Package entity defines domain entities for the report service.
package entity

import "time"

// OutputFormat represents a supported report output format.
type OutputFormat string

const (
	FormatConsole OutputFormat = "console"
	FormatCSV     OutputFormat = "csv"
	FormatJSON    OutputFormat = "json"
	FormatJSON2   OutputFormat = "json2"
	FormatHTML    OutputFormat = "html"
	FormatPDF     OutputFormat = "pdf"
)

// AllFormats returns all supported output formats.
func AllFormats() []OutputFormat {
	return []OutputFormat{
		FormatConsole, FormatCSV, FormatJSON, FormatJSON2, FormatHTML, FormatPDF,
	}
}

// IsValid returns true if the format is one of the supported formats.
func (f OutputFormat) IsValid() bool {
	for _, valid := range AllFormats() {
		if f == valid {
			return true
		}
	}
	return false
}

// ProductInfo identifies a software product.
type ProductInfo struct {
	Vendor  string
	Product string
	Version string
	PURL    string
}

// CVEData contains details about a single CVE finding.
type CVEData struct {
	CVENumber     string
	Severity      string  // UNKNOWN|LOW|MEDIUM|HIGH|CRITICAL
	Remarks       int     // 0=unset, 1=notaffected, 2=affected, 3=fixed, 4=investigating
	Score         float64
	CVSSVersion   int    // 2 or 3
	CVSSVector    string
	DataSource    string
	IsExploit     bool
	Justification string
	Response      []string
	EPSS          float64
}

// ReportInput contains all data needed to generate a report.
type ReportInput struct {
	CVEData     map[ProductInfo][]CVEData
	ScanTarget  string
	GeneratedAt time.Time
	Formats     []OutputFormat
	MinSeverity string
	MinScore    float64
	Theme       string // "light"|"dark"

	// DefectDojo-specific fields (for Excel formatter)
	Findings []Finding
	Target   string
}

// ReportOutput contains the generated report bytes per format.
type ReportOutput struct {
	Reports  map[OutputFormat][]byte
	ExitCode int // 0=clean, 1=CVEs found
}

// AvailableFixResult describes a CVE with a known fix in a Linux distro.
type AvailableFixResult struct {
	CVENumber   string
	Product     ProductInfo
	Distro      string
	FixVersion  string
	IsAvailable bool
}

// Finding represents a single DefectDojo vulnerability finding for Excel reports.
type Finding struct {
	ID               string
	Title            string
	Severity         string
	CVE              string
	CWE              int32
	Status           string
	Component        string
	ComponentVersion string
	FilePath         string
	Line             int32
	HashCode         string
	Date             string    // RFC3339 formatted
	FoundDate        time.Time // date the finding was found
	SLAExpiration    string // RFC3339 formatted, optional
	ProductID        string
	TestID           string
}
