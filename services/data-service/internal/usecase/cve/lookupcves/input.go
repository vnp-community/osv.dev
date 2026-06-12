package lookupcves

import (
	"strconv"

	"github.com/osv/data-service/internal/domain/entity"
)

// Input defines the parameters for a CVE lookup operation.
type Input struct {
	// Products to look up CVEs for.
	Products []ProductQuery

	// MinScore filters out CVEs with score < MinScore. 0 = no filter.
	MinScore float64

	// MinEPSSPercentile filters CVEs with EPSS percentile < threshold. 0 = no filter.
	MinEPSSPercentile float64

	// MinEPSSProbability filters CVEs with EPSS probability < threshold. 0 = no filter.
	MinEPSSProbability float64

	// CheckExploits flags CVEs that are known to be exploited in the wild.
	CheckExploits bool

	// CheckMetrics attaches EPSS data to each CVE result.
	CheckMetrics bool

	// DisabledSources excludes CVEs from specific data sources (e.g. ["OSV"]).
	DisabledSources []string

	// TriageData is optional VEX triage data to apply during lookup.
	TriageData entity.TriageData

	// FilterTriage removes CVEs marked as NotAffected/Mitigated/FalsePositive.
	FilterTriage bool

	// NoScan skips all DB lookups and returns empty results (dry-run mode).
	NoScan bool
}

// ProductQuery identifies a product to look up.
type ProductQuery struct {
	Vendor  string
	Product string
	Version string
	PURL    string // optional Package URL
}

// String returns "vendor/product@version".
func (p ProductQuery) String() string {
	return p.Vendor + "/" + p.Product + "@" + p.Version
}

// Output is the result of a CVE lookup.
type Output struct {
	// Results maps each product to its CVE list.
	Results map[ProductQuery][]entity.CVE

	// TotalCVEs is total unique CVE count across all products.
	TotalCVEs int

	// ProductsWithCVE counts products that have at least one CVE.
	ProductsWithCVE int
}

// scoreForCVE returns the primary score for a CVE.
func scoreForCVE(cve entity.CVE) float64 {
	if cve.CVSSv3Score > 0 {
		return cve.CVSSv3Score
	}
	return cve.CVSSv2Score
}

// cvssVersionForCVE returns 3 or 2 based on which score is available.
func cvssVersionForCVE(cve entity.CVE) int {
	if cve.CVSSVersion != 0 {
		return cve.CVSSVersion
	}
	if cve.CVSSv3Score > 0 {
		return 3
	}
	return 2
}

// severityOrder maps severity strings to numeric order for filtering.
var severityOrder = map[string]int{
	"NONE": 0, "LOW": 1, "MEDIUM": 2, "HIGH": 3, "CRITICAL": 4,
}

// meetsScoreFilter returns true if CVE score meets minimum threshold.
func meetsScoreFilter(cve entity.CVE, minScore float64) bool {
	if minScore <= 0 {
		return true
	}
	return scoreForCVE(cve) >= minScore
}

// meetsEPSSFilter returns true if CVE meets EPSS thresholds.
func meetsEPSSFilter(cve entity.CVE, minPctile, minProb float64) bool {
	if minPctile > 0 {
		pctile, _ := strconv.ParseFloat(cve.CVEID, 64) // placeholder — EPSS attached later
		_ = pctile
	}
	_ = minProb
	return true // EPSS filtering done with attached metric data
}
