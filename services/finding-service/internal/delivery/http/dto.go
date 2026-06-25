package http

import "time"

// FindingListItem is a summary representation of a finding for the list view
type FindingListItem struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	CveID       *string  `json:"cve_id"`
	Severity    string   `json:"severity"`
	CVSSv3      *float64 `json:"cvss_v3"`
	EPSSScore   *float64 `json:"epss_score"` // NEW
	IsKEV       bool     `json:"is_kev"`     // NEW

	// Status (derived from bool flags)
	Status string `json:"status"` // "new"|"confirmed"|"in_progress"|"resolved"|"accepted"|"false_positive"|"duplicate"

	// Flags
	IsDuplicate bool    `json:"is_duplicate"`
	DupOfID     *string `json:"duplicate_finding_id"`

	// Hierarchy
	ProductID    string `json:"product_id"`
	ProductName  string `json:"product_name"` // NEW via JOIN
	EngagementID string `json:"engagement_id"`
	TestID       string `json:"test_id"`

	// Asset
	AssetIP          *string `json:"asset_ip"`
	AssetHostname    *string `json:"asset_hostname"`
	ComponentName    *string `json:"component_name"`
	ComponentVersion *string `json:"component_version"`

	// SLA
	SLAExpiry   *string `json:"sla_expiration_date"`
	SLAStatus   string  `json:"sla_status"`    // NEW: "ok"|"at_risk"|"breached"
	SLADaysLeft *int    `json:"sla_days_left"` // NEW: computed

	// Meta
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	MitigatedAt *string `json:"mitigated_at"`
	AssignedTo  *string `json:"assigned_to"` // NEW
	CreatedBy   *string `json:"created_by"`  // NEW

	// JIRA integration
	JiraIssueKey *string `json:"jira_issue_key"` // NEW via LEFT JOIN
	JiraURL      *string `json:"jira_url"`       // NEW constructed
}

// FindingListResponse is the paginated findings response
type FindingListResponse struct {
	Findings   []FindingListItem `json:"findings"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	BySeverity map[string]int    `json:"by_severity"` // NEW
	ByStatus   map[string]int    `json:"by_status"`   // NEW
	SLAStats   *SLASummary       `json:"sla_stats"`   // NEW
}

type SLASummary struct {
	Breached int `json:"breached"`
	AtRisk   int `json:"at_risk"`
	OK       int `json:"ok"`
}

// computeSLAStatus derives SLA status from expiration date
func computeSLAStatus(slaExpiry *time.Time) (status string, daysLeft *int) {
	if slaExpiry == nil {
		return "ok", nil
	}
	d := int(time.Until(*slaExpiry).Hours() / 24)
	switch {
	case d < 0:
		abs := -d
		return "breached", &abs
	case d <= 7:
		return "at_risk", &d
	default:
		return "ok", &d
	}
}

// deriveStatus maps boolean flags to status string.
// Values must match test FINDING_STATUS_VALUES:
// "active", "mitigated", "false_positive", "risk_accepted", "out_of_scope", "duplicate"
func deriveStatus(mitigated, fp, riskAccepted, outOfScope, duplicate bool) string {
	switch {
	case duplicate:
		return "duplicate"
	case fp:
		return "false_positive"
	case riskAccepted:
		return "risk_accepted"
	case mitigated:
		return "mitigated"
	case outOfScope:
		return "out_of_scope"
	default:
		return "active"
	}
}

type ProductListItem struct {
    ID            string          `json:"id"`
    Name          string          `json:"name"`
    Description   string          `json:"description"`
    Type          string          `json:"type"`
    Criticality   string          `json:"criticality"`
    Lifecycle     string          `json:"lifecycle"`
    Grade         string          `json:"grade"`           // NEW
    Score         int             `json:"score"`           // NEW 0-100
    FindingSummary *FindingSummary `json:"finding_summary"` // NEW
    Tags          []string        `json:"tags"`
    CreatedAt     string          `json:"created_at"`
}

type FindingSummary struct {
    Critical    int `json:"critical"`
    High        int `json:"high"`
    Medium      int `json:"medium"`
    Low         int `json:"low"`
    TotalActive int `json:"total_active"`
}

type ProductGradeDTO struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    Grade         string `json:"grade"`
    Score         int    `json:"score"`
    CriticalCount int    `json:"critical_count"`
    HighCount     int    `json:"high_count"`
    TotalActive   int    `json:"total_active"`
    Trend         string `json:"trend"` // "improving" | "worsening" | "stable"
}

type ProductGradesResponse struct {
    Products     []ProductGradeDTO `json:"products"`
    OverallGrade string            `json:"overall_grade"`
    OverallScore int               `json:"overall_score"`
}
