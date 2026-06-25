// Package cve5 defines the domain types for CVE Record Format v5.0/v5.1.
// These types mirror the CVE JSON 5.0 specification from https://github.com/CVEProject/cvelistV5
package cve5

import "time"

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
	Version         string `json:"version"`
	Status          string `json:"status"` // "affected", "unaffected"
	LessThan        string `json:"lessThan,omitempty"`
	LessThanOrEqual string `json:"lessThanOrEqual,omitempty"`
	VersionType     string `json:"versionType,omitempty"` // "semver", "git", "custom"
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
	Format    string      `json:"format"` // "CVSS"
	Scenarios []*Scenario `json:"scenarios,omitempty"`
	CVSSv30   *CVSSData   `json:"cvssV3_0,omitempty"`
	CVSSv31   *CVSSData   `json:"cvssV3_1,omitempty"`
	CVSSv40   *CVSSData   `json:"cvssV4_0,omitempty"`
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
