// Package sources defines shared data structures and the DataSource interface
// used by all external CVE data source adapters.
package sources

import "context"

// CVESeverityRow represents a single CVE severity entry from a data source.
type CVESeverityRow struct {
	CVENumber   string  `json:"cve_number"`
	Severity    string  `json:"severity"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	CVSSVersion int     `json:"cvss_version"`
	CVSSVector  string  `json:"cvss_vector"`
	DataSource  string  `json:"data_source"`
}

// CVERangeRow represents a version range for a CVE.
type CVERangeRow struct {
	CVENumber             string `json:"cve_number"`
	Vendor                string `json:"vendor"`
	Product               string `json:"product"`
	Version               string `json:"version"` // exact version match
	VersionStartIncluding string `json:"version_start_including,omitempty"`
	VersionStartExcluding string `json:"version_start_excluding,omitempty"`
	VersionEndIncluding   string `json:"version_end_including,omitempty"`
	VersionEndExcluding   string `json:"version_end_excluding,omitempty"`
	DataSource            string `json:"data_source"`
}

// CVEData bundles all data from a single source fetch.
type CVEData struct {
	Source     string
	Severities []CVESeverityRow
	Ranges     []CVERangeRow
}

// DataSource is implemented by every external CVE data source adapter.
type DataSource interface {
	Name() string
	FetchCVEData(ctx context.Context) (CVEData, error)
}

// SeverityFromScore maps a CVSS score to a severity label.
func SeverityFromScore(score float64) string {
	switch {
	case score >= 9.0:
		return "CRITICAL"
	case score >= 7.0:
		return "HIGH"
	case score >= 4.0:
		return "MEDIUM"
	case score > 0:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}
