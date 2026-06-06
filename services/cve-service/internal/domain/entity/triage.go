package entity

// Remarks represents the triage/review status of a CVE finding.
// Mirrors the Python cve-bin-tool Remarks OrderedEnum exactly.
type Remarks int

const (
	RemarksNewFound      Remarks = 1 // Just discovered, unreviewed
	RemarksUnexplored    Remarks = 2 // Under investigation
	RemarksConfirmed     Remarks = 3 // Confirmed as applicable
	RemarksMitigated     Remarks = 4 // Mitigated or patched
	RemarksFalsePositive Remarks = 5 // Confirmed false positive
	RemarksNotAffected   Remarks = 6 // Vendor confirmed not affected (VEX)
)

// String returns the human-readable name of this Remarks value.
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

// VEXStatusToRemarks maps VEX statement status strings to Remarks values.
func VEXStatusToRemarks(status string) Remarks {
	switch status {
	case "affected":
		return RemarksConfirmed
	case "not_affected":
		return RemarksNotAffected
	case "fixed":
		return RemarksMitigated
	case "under_investigation":
		return RemarksUnexplored
	default:
		return RemarksNewFound
	}
}

// TriageData maps CVE numbers to their triage decisions.
// Loaded from VEX files and applied during CVE lookup.
type TriageData map[string]TriageEntry

// TriageEntry holds the triage decision for a single CVE.
type TriageEntry struct {
	Remarks       Remarks  `json:"remarks"`
	Comments      string   `json:"comments,omitempty"`
	Response      []string `json:"response,omitempty"`
	Justification string   `json:"justification,omitempty"`
}

// ShouldFilter returns true if this CVE should be hidden from results
// (i.e. not actually vulnerable in this context).
func (e TriageEntry) ShouldFilter() bool {
	return e.Remarks == RemarksMitigated ||
		e.Remarks == RemarksFalsePositive ||
		e.Remarks == RemarksNotAffected
}
