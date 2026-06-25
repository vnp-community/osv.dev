package entity

import "time"

// ReportFinding is the finding data model used by the report service
// (simplified copy from finding-service, contains only fields needed for reports)
type ReportFinding struct {
    ID               string
    Title            string
    Description      string
    Mitigation       string
    Severity         string  // "Critical" | "High" | "Medium" | "Low" | "Info"
    CVE              string
    CWE              int
    CVSSv3Score      *float64
    EPSSScore        *float64
    IsExploit        bool
    IsInCISAKEV      bool
    Status           string  // "active" | "mitigated" | "false_positive" etc.
    ComponentName    string
    ComponentVersion string
    Date             time.Time
    SLAExpirationDate *time.Time
    DaysUntilSLA     *int
    DataSource       string  // "nmap" | "zap" | "agent" | "manual"
    ProductID        string
    ProductName      string
    EngagementID     string
    EngagementName   string
    Tags             []string
}

// ScanStats aggregates finding counts for the report header
type ScanStats struct {
    CriticalCount int
    HighCount     int
    MediumCount   int
    LowCount      int
    InfoCount     int
    TotalCount    int
}

// ProductSection groups findings by product for the HTML report
type ProductSection struct {
    ProductName    string
    ProductID      string
    TotalFindings  int
    Findings       []*ReportFinding
}
