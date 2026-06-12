// Package entity defines VEX domain entities.
package entity

// VEXFormat identifies a VEX document format.
type VEXFormat string

const (
	VEXFormatOpenVEX   VEXFormat = "openvex"
	VEXFormatCycloneDX VEXFormat = "cyclonedx"
	VEXFormatCSAF      VEXFormat = "csaf"
	VEXFormatUnknown   VEXFormat = "unknown"
)

// VEXDocument is a parsed VEX document.
type VEXDocument struct {
	Format     VEXFormat
	Statements []VEXStatement
}

// VEXStatement is a single VEX entry mapping a CVE to a status.
type VEXStatement struct {
	CVENumber     string
	Status        string   // under_investigation|affected|not_affected|fixed
	Justification string
	Comments      string
	Response      []string
}

// TriageEntry is the domain triage data sent to CVEDB lookup.
type TriageEntry struct {
	Remarks       int    // 0=unset, 1=not_affected, 2=affected, 3=fixed, 4=investigating
	Justification string
	Comments      string
	Response      []string
}

// TriageData maps CVE number → TriageEntry.
type TriageData map[string]TriageEntry

// VEX status → Remarks integer mapping.
var statusToRemarks = map[string]int{
	"not_affected":         1,
	"affected":             2,
	"fixed":               3,
	"mitigated":           3,
	"under_investigation": 4,
}

// ToTriageData converts a VEXDocument to a TriageData map.
func (d *VEXDocument) ToTriageData() TriageData {
	td := make(TriageData, len(d.Statements))
	for _, s := range d.Statements {
		remarks := 0
		if r, ok := statusToRemarks[s.Status]; ok {
			remarks = r
		}
		td[s.CVENumber] = TriageEntry{
			Remarks:       remarks,
			Justification: s.Justification,
			Comments:      s.Comments,
			Response:      s.Response,
		}
	}
	return td
}
