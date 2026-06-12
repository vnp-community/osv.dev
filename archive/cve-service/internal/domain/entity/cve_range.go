package entity

// CVERange represents a version range for a CVE affecting a specific product.
// Mirrors the NVD CPE match / OSV affected version range structures.
type CVERange struct {
	CVENumber             string `db:"cve_number"`
	Vendor                string `db:"vendor"`   // lowercase CPE vendor
	Product               string `db:"product"`  // lowercase CPE product
	Version               string `db:"version"`  // "*" = any version (use range bounds); exact version otherwise
	VersionStartIncluding string `db:"version_start_including"` // product >= this
	VersionStartExcluding string `db:"version_start_excluding"` // product > this
	VersionEndIncluding   string `db:"version_end_including"`   // product <= this
	VersionEndExcluding   string `db:"version_end_excluding"`   // product < this
	DataSource            string `db:"data_source"` // NVD|OSV|GAD|...
}

// IsExactMatch returns true when this range entry represents an exact version match
// (no range bounds — just a specific version number).
func (r CVERange) IsExactMatch() bool {
	return r.Version != "" && r.Version != "*" &&
		r.VersionStartIncluding == "" && r.VersionStartExcluding == "" &&
		r.VersionEndIncluding == "" && r.VersionEndExcluding == ""
}

// IsWildcard returns true when Version=="*" with no bounds (affects all versions).
func (r CVERange) IsWildcard() bool {
	return r.Version == "*" &&
		r.VersionStartIncluding == "" && r.VersionStartExcluding == "" &&
		r.VersionEndIncluding == "" && r.VersionEndExcluding == ""
}

// CVESeverity holds severity and scoring for a CVE.
type CVESeverity struct {
	CVENumber    string  `db:"cve_number"`
	Severity     string  `db:"severity"`      // CRITICAL|HIGH|MEDIUM|LOW|NONE
	Description  string  `db:"description"`
	Score        float64 `db:"score"`         // CVSS base score
	CVSSVersion  int     `db:"cvss_version"`  // 2 or 3
	CVSSVector   string  `db:"cvss_vector"`   // e.g. CVSS:3.1/AV:N/...
	DataSource   string  `db:"data_source"`
	LastModified string  `db:"last_modified"` // RFC3339
}

// PURL2CPE maps a Package URL to one or more CPE strings.
type PURL2CPE struct {
	PURL string `db:"purl"`
	CPE  string `db:"cpe"`
}

// DBState holds metadata about the local CVE database.
type DBState struct {
	SchemaVersion string `db:"schema_version"`
	LastUpdated   string `db:"last_updated"` // RFC3339
	CVECount      int64  `db:"cve_count"`
	RangeCount    int64  `db:"range_count"`
}
