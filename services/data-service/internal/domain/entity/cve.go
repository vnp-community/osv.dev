// Package entity — CVE domain entity.
// Supports both:
//   - PostgreSQL (CVE Binary Tool mode): db tags, UUID ID, CVEID etc.
//   - MongoDB (cve-search mode): bson tags, string ID, vendors/products etc.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// Reference is a CVE advisory or patch link.
type Reference struct {
	URL  string `bson:"url"  json:"url"`
	Type string `bson:"type" json:"type"` // ADVISORY|PATCH|WEB|REPORT
}

// AffectedPackage describes a package version affected by a CVE.
type AffectedPackage struct {
	Ecosystem    string   `bson:"ecosystem"             json:"ecosystem"`
	PackageName  string   `bson:"package_name"          json:"package_name"`
	Versions     []string `bson:"versions"              json:"versions"`
	FixedVersion string   `bson:"fixed_version,omitempty" json:"fixed_version,omitempty"`
}

// Severity levels for CVEs.
type Severity string

const (
	SeverityCritical Severity = "critical" // CVSS >= 9.0
	SeverityHigh     Severity = "high"     // 7.0-8.9
	SeverityMedium   Severity = "medium"   // 4.0-6.9
	SeverityLow      Severity = "low"      // 0.1-3.9
	SeverityNone     Severity = "none"
)

// SeverityFromCVSS derives severity from a CVSS v3 score.
func SeverityFromCVSS(score float64) Severity {
	switch {
	case score >= 9.0:
		return SeverityCritical
	case score >= 7.0:
		return SeverityHigh
	case score >= 4.0:
		return SeverityMedium
	case score > 0:
		return SeverityLow
	default:
		return SeverityNone
	}
}

// CVE is the canonical vulnerability record.
// Fields support both PostgreSQL (db tags) and MongoDB (bson tags) backends.
type CVE struct {
	// ── MongoDB / cve-search fields (bson tags) ───────────────────────────────
	// "id" = NVD CVE ID string (e.g. "CVE-2021-44228")
	ID          string    `bson:"id"          db:"-"            json:"id"`
	Published   time.Time `bson:"published"   db:"published_at"  json:"published"`
	Modified    time.Time `bson:"modified"    db:"updated_at"    json:"modified"`
	Summary     string    `bson:"summary"     db:"summary"       json:"summary"`
	Description string    `bson:"description" db:"description"   json:"description,omitempty"`
	Status      string    `bson:"status"      db:"-"             json:"status,omitempty"`
	Assigner    string    `bson:"assigner"    db:"-"             json:"assigner,omitempty"`

	// CVSS Scores (MongoDB names)
	CVSS        float64 `bson:"cvss"        db:"cvss_v2_score"  json:"cvss,omitempty"`
	CVSSVector  string  `bson:"cvssVector"  db:"cvss_v2_vector" json:"cvssVector,omitempty"`
	CVSS3       float64 `bson:"cvss3"       db:"cvss_v3_score"  json:"cvss3,omitempty"`
	CVSS3Vector string  `bson:"cvss3Vector" db:"cvss_v3_vector" json:"cvss3Vector,omitempty"`
	CVSS4       float64 `bson:"cvss4"       db:"-"              json:"cvss4,omitempty"`

	// EPSS
	EPSS           float64 `bson:"epss"           db:"epss"            json:"epss,omitempty"`
	EPSSPercentile float64 `bson:"epssPercentile" db:"epss_percentile" json:"epssPercentile,omitempty"`

	// Severity
	Severity Severity `bson:"severity" db:"severity" json:"severity"`

	// References & CWE
	References []string `bson:"references" db:"-" json:"references,omitempty"`
	CWE        []string `bson:"cwe"        db:"-" json:"cwe,omitempty"`

	// CPE / Vulnerable Configuration (cve-search MongoDB schema)
	Vendors                 []string `bson:"vendors"                  db:"-" json:"vendors,omitempty"`
	Products                []string `bson:"products"                 db:"-" json:"products,omitempty"`
	VulnerableConfiguration []string `bson:"vulnerable_configuration" db:"-" json:"vulnerable_configuration,omitempty"`
	VulnerableProduct       []string `bson:"vulnerable_product"       db:"-" json:"vulnerable_product,omitempty"`

	// Enrichment (not stored in DB)
	CAPEC   interface{} `bson:"-" json:"capec,omitempty"`
	Ranking interface{} `bson:"-" json:"ranking,omitempty"`

	// CR-GCV-001/CR-GCV-003: Source attribution and exploit enrichment
	Source    string `bson:"source,omitempty"     db:"source" json:"source,omitempty"`     // NVD|CIRCL|JVN|EXPLOITDB|CVE.ORG|CNNVD
	IsKEV     bool   `bson:"is_kev,omitempty"     db:"-" json:"is_kev,omitempty"`     // in CISA KEV catalog
	IsExploit bool   `bson:"is_exploit,omitempty" db:"is_exploit" json:"is_exploit,omitempty"` // has public exploit
	Link      string `bson:"link,omitempty"       db:"-"          json:"link,omitempty"`
	SourceJVN bool   `bson:"source_jvn,omitempty" db:"-" json:"-"`                    // cross-reference JVN
	JVNID     string `bson:"jvn_id,omitempty"     db:"-" json:"jvn_id,omitempty"`    // e.g. "JVNDB-2021-002374"

	// ── PostgreSQL / CVE Binary Tool fields (db tags, bson:"-") ──────────────
	// These are used by the existing binary-tool-oriented codebase.
	InternalID       uuid.UUID         `bson:"-" db:"id"                json:"-"`
	CVEID            string            `bson:"-" db:"cve_id"            json:"cve_id,omitempty"` // kept for PG compat
	CVSSv3Score      float64           `bson:"-" db:"cvss_v3_score"     json:"-"` // alias for CVSS3
	CVSSv3Vector     string            `bson:"-" db:"cvss_v3_vector"    json:"-"` // alias for CVSS3Vector
	CVSSv2Score      float64           `bson:"-" db:"cvss_v2_score"     json:"-"` // alias for CVSS
	CVSSVersion      int               `bson:"-" db:"cvss_version"      json:"-"`
	EPSSScore        float64           `bson:"-" db:"epss"              json:"-"`
	EPSSPctile       float64           `bson:"-" db:"epss_percentile"   json:"-"`
	Remediation      string            `bson:"-" db:"remediation"       json:"-"`
	AffectedPackages []AffectedPackage `bson:"-" db:"-"                 json:"affected_packages,omitempty"`
	PublishedAt      time.Time         `bson:"-" db:"published_at"      json:"-"`
	UpdatedAt        time.Time         `bson:"-" db:"updated_at"        json:"-"`
	LastFetchedAt    time.Time         `bson:"-" db:"last_fetched_at"   json:"-"`
	Sources          []string          `bson:"-" db:"sources"           json:"-"`
	Embedding        []float32         `bson:"-" db:"-"                 json:"-"`
	EmbeddingModel   string            `bson:"-" db:"embedding_model"   json:"-"`
	CreatedAt        time.Time         `bson:"-" db:"created_at"        json:"-"`

	// Triage/VEX fields
	Remarks       Remarks  `bson:"-" db:"-" json:"remarks,omitempty"`
	DataSource    string   `bson:"-" db:"-" json:"data_source,omitempty"`
	Justification string   `bson:"-" db:"-" json:"justification,omitempty"`
	Response      []string `bson:"-" db:"-" json:"response,omitempty"`
}
