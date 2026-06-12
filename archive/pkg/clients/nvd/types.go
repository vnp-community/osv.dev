// Package nvd provides a client for the NVD (National Vulnerability Database) 2.0 API.
// Used by cve-service as a fallback when OSV API lacks data for a CVE.
//
// API docs: https://nvd.nist.gov/developers/vulnerabilities
// Rate limits:
//   - Without API key: 5 requests per 30 seconds
//   - With API key:    50 requests per 30 seconds
package nvd

import "time"

// NVDCVEItem represents a CVE entry returned by the NVD 2.0 API.
type NVDCVEItem struct {
	// ID is the CVE identifier, e.g. "CVE-2021-44228".
	ID string

	// Description is the English language description of the vulnerability.
	Description string

	// Published is when the CVE was first published in NVD.
	Published time.Time

	// Modified is the last time the NVD record was modified.
	Modified time.Time

	// Metrics holds CVSS scoring information.
	Metrics NVDMetrics

	// References is a list of external URLs related to this CVE.
	References []NVDReference

	// Weaknesses holds associated CWE IDs.
	Weaknesses []NVDWeakness
}

// NVDMetrics holds CVSS v2 and v3 scoring data.
type NVDMetrics struct {
	// CVSSv3Score is the CVSS 3.x base score (0.0-10.0).
	CVSSv3Score float64

	// CVSSv3Vector is the CVSS 3.x vector string, e.g. "CVSS:3.1/AV:N/AC:L/...".
	CVSSv3Vector string

	// CVSSv3Severity is the textual severity: CRITICAL|HIGH|MEDIUM|LOW|NONE.
	CVSSv3Severity string

	// CVSSv2Score is the CVSS 2.0 base score (legacy).
	CVSSv2Score float64

	// CVSSv2Vector is the CVSS 2.0 vector string (legacy).
	CVSSv2Vector string
}

// NVDReference is an external reference for a CVE.
type NVDReference struct {
	// URL is the reference URL.
	URL string

	// Source is the originating organization or vendor.
	Source string

	// Tags describes the reference type, e.g. ["Patch", "Vendor Advisory"].
	Tags []string
}

// NVDWeakness holds a CWE (Common Weakness Enumeration) entry.
type NVDWeakness struct {
	// CWEID is the identifier, e.g. "CWE-502".
	CWEID string

	// Description is the human-readable weakness name.
	Description string
}

// nvdAPIResponse matches the top-level NVD 2.0 API JSON response.
type nvdAPIResponse struct {
	ResultsPerPage  int           `json:"resultsPerPage"`
	StartIndex      int           `json:"startIndex"`
	TotalResults    int           `json:"totalResults"`
	Format          string        `json:"format"`
	Version         string        `json:"version"`
	Timestamp       string        `json:"timestamp"`
	Vulnerabilities []nvdVulnWrap `json:"vulnerabilities"`
}

type nvdVulnWrap struct {
	CVE nvdCVE `json:"cve"`
}

type nvdCVE struct {
	ID               string         `json:"id"`
	SourceIdentifier string         `json:"sourceIdentifier"`
	Published        string         `json:"published"`
	LastModified     string         `json:"lastModified"`
	VulnStatus       string         `json:"vulnStatus"`
	Descriptions     []nvdLangValue `json:"descriptions"`
	Metrics          nvdMetrics     `json:"metrics"`
	Weaknesses       []nvdWeakness  `json:"weaknesses"`
	References       []nvdReference `json:"references"`
}

type nvdLangValue struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

type nvdMetrics struct {
	CVSSMetricV31 []nvdCVSSv3 `json:"cvssMetricV31"`
	CVSSMetricV30 []nvdCVSSv3 `json:"cvssMetricV30"`
	CVSSMetricV2  []nvdCVSSv2 `json:"cvssMetricV2"`
}

type nvdCVSSv3 struct {
	Source   string    `json:"source"`
	Type     string    `json:"type"`
	CVSSData nvdCVSSv3Data `json:"cvssData"`
}

type nvdCVSSv3Data struct {
	Version      string  `json:"version"`
	VectorString string  `json:"vectorString"`
	BaseScore    float64 `json:"baseScore"`
	BaseSeverity string  `json:"baseSeverity"`
}

type nvdCVSSv2 struct {
	Source   string       `json:"source"`
	CVSSData nvdCVSSv2Data `json:"cvssData"`
}

type nvdCVSSv2Data struct {
	VectorString string  `json:"vectorString"`
	BaseScore    float64 `json:"baseScore"`
}

type nvdWeakness struct {
	Source      string         `json:"source"`
	Type        string         `json:"type"`
	Description []nvdLangValue `json:"description"`
}

type nvdReference struct {
	URL    string   `json:"url"`
	Source string   `json:"source"`
	Tags   []string `json:"tags"`
}
