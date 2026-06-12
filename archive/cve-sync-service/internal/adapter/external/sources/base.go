// Package sources defines the DataSource interface and shared data types
// used by all external CVE data source implementations in cve-sync-service.
package sources

import "context"

// DataSource is the common interface for all external CVE data sources.
// Each implementation fetches and normalizes CVE data for a specific source.
type DataSource interface {
	// Name returns the source identifier (e.g. "NVD", "OSV", "GAD").
	Name() string

	// FetchCVEData downloads and normalizes CVE data from the external source.
	// Implementations should respect ctx cancellation.
	FetchCVEData(ctx context.Context) (CVEData, error)
}

// CVEData holds normalized CVE data from a single source fetch.
// Consumers (CVEDB PopulateDB use case) will upsert all rows.
type CVEData struct {
	// Source is the data source name (matches DataSource.Name()).
	Source string

	// Severities maps CVE numbers to severity information.
	Severities []CVESeverityRow

	// Ranges describes version range constraints for affected products.
	Ranges []CVERangeRow

	// Metrics contains EPSS scores and other CVE metrics.
	Metrics []CVEMetricRow

	// PURL2CPEs maps Package URLs to CPE strings.
	PURL2CPEs []PURL2CPERow
}

// CVESeverityRow holds severity data for a single CVE.
type CVESeverityRow struct {
	CVENumber   string  // e.g. "CVE-2021-44228"
	Severity    string  // "CRITICAL"|"HIGH"|"MEDIUM"|"LOW"|"NONE"
	Description string
	Score       float64 // CVSS base score
	CVSSVersion int     // 2 or 3
	CVSSVector  string  // e.g. "CVSS:3.1/AV:N/AC:L/..."
	DataSource  string
}

// CVERangeRow describes a version range for an affected product.
type CVERangeRow struct {
	CVENumber             string
	Vendor                string // lowercase
	Product               string // lowercase
	Version               string // exact version (mutually exclusive with ranges)
	VersionStartIncluding string
	VersionStartExcluding string
	VersionEndIncluding   string
	VersionEndExcluding   string
	DataSource            string
}

// CVEMetricRow holds metric data (e.g. EPSS) for a CVE.
type CVEMetricRow struct {
	CVENumber   string
	MetricID    int    // 1=EPSS, 2=SSVC (reserved)
	MetricScore float64 // probability for EPSS
	MetricField string  // percentile for EPSS (as string)
	DataSource  string
}

// PURL2CPERow maps a Package URL to a CPE string.
type PURL2CPERow struct {
	PURL string
	CPE  string
}

// MetricIDEPSS is the MetricID for EPSS probability scores.
const MetricIDEPSS = 1

// SeverityFromScore maps a CVSS base score to a severity string.
// Follows NVD v3.1 rating scale.
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
		return "NONE"
	}
}
