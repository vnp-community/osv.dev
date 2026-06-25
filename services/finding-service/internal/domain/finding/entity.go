package finding

import (
    "crypto/sha256"
    "fmt"
    "strings"
    "time"

    "github.com/google/uuid"
)

// Severity represents the criticality level of a finding
type Severity string

const (
    SeverityCritical Severity = "Critical"
    SeverityHigh     Severity = "High"
    SeverityMedium   Severity = "Medium"
    SeverityLow      Severity = "Low"
    SeverityInfo     Severity = "Info"
)

// NumericalSeverity maps severity strings to numbers for sorting/comparison
func (s Severity) Numerical() int {
    switch s {
    case SeverityCritical:
        return 4
    case SeverityHigh:
        return 3
    case SeverityMedium:
        return 2
    case SeverityLow:
        return 1
    default:
        return 0
    }
}

// Finding represents a vulnerability discovered during a security test.
// It tracks the full lifecycle from discovery through remediation.
type Finding struct {
    ID                uuid.UUID
    Title             string
    Description       string
    Mitigation        string
    Impact            string
    References        string
    Severity          Severity
    NumericalSeverity int
    CVE               string    // e.g., "CVE-2021-44228"
    CWE               int       // e.g., 79
    VulnIDFromTool    string    // e.g., "nmap-vulners", "zap-active-scan"
    CVSSv3            string    // Vector string
    CVSSv3Score       *float64
    CVSSv4            string
    CVSSv4Score       *float64
    EPSSScore         *float64 // NEW
    IsKEV             bool     // NEW

    // ---- State flags ----
    // These flags are NOT mutually exclusive — CurrentState() determines display priority
    Active        bool // false when closed/resolved
    Verified      bool // Manually confirmed by analyst
    FalsePositive bool // Tool false alarm
    Duplicate     bool // Same issue already tracked (via hash)
    OutOfScope    bool // Not in scope of this engagement
    IsMitigated   bool // Patched/remediated
    RiskAccepted  bool // Risk accepted by stakeholder

    // ---- Timestamps ----
    Date              time.Time
    MitigatedAt       *time.Time
    MitigatedByID     *uuid.UUID
    LastReviewed      *time.Time
    LastStatusUpdate  *time.Time
    SLAExpirationDate *time.Time
    AssignedTo        *string // NEW

    // ---- Context ----
    TestID       uuid.UUID // Link to product-service Test
    EngagementID uuid.UUID // Link to product-service Engagement
    ProductID    uuid.UUID // Link to product-service Product
    CreatedBy    *string   // NEW


    // ---- Deduplication ----
    DuplicateFindingID *uuid.UUID // Points to the original if this is a duplicate
    HashCode           string     // SHA-256 for dedup: title|component|version|cve

    // ---- Location ----
    ComponentName    string // IP, package name, or filename
    ComponentVersion string
    Service          string
    FilePath         string
    LineNumber       *int
    AssetIP          *string // NEW
    AssetHostname    *string // NEW

    // ---- Tags ----
    Tags         []string
    InheritedTags []string // From product/engagement

    CreatedAt time.Time
    UpdatedAt time.Time
}

// NewFinding creates a Finding with validation and computed fields
func NewFinding(
    title string,
    severity Severity,
    testID, engagementID, productID uuid.UUID,
    componentName, componentVersion, cve string,
) (*Finding, error) {
    if strings.TrimSpace(title) == "" {
        return nil, ErrTitleRequired
    }
    if severity != SeverityCritical && severity != SeverityHigh &&
        severity != SeverityMedium && severity != SeverityLow && severity != SeverityInfo {
        return nil, ErrInvalidSeverity
    }
    if testID == uuid.Nil {
        return nil, ErrTestIDRequired
    }
    if engagementID == uuid.Nil {
        return nil, ErrEngagementIDRequired
    }
    if productID == uuid.Nil {
        return nil, ErrProductIDRequired
    }

    now := time.Now().UTC()
    f := &Finding{
        ID:                uuid.New(),
        Title:             strings.TrimSpace(title),
        Severity:          severity,
        NumericalSeverity: severity.Numerical(),
        CVE:               strings.ToUpper(strings.TrimSpace(cve)),
        TestID:            testID,
        EngagementID:      engagementID,
        ProductID:         productID,
        ComponentName:     componentName,
        ComponentVersion:  componentVersion,
        Active:            true,
        Date:              now,
        Tags:              []string{},
        InheritedTags:     []string{},
        CreatedAt:         now,
        UpdatedAt:         now,
    }
    f.HashCode = f.ComputeHash()
    return f, nil
}

// ComputeHash generates the SHA-256 deduplication key for this finding.
// Format: SHA256(title | component_name | component_version | cve)
// This matches DefectDojo's deduplication algorithm.
func (f *Finding) ComputeHash() string {
    parts := []string{
        strings.TrimSpace(f.Title),
        strings.TrimSpace(f.ComponentName),
        strings.TrimSpace(f.ComponentVersion),
        strings.ToUpper(strings.TrimSpace(f.CVE)),
    }
    input := strings.Join(parts, "|")
    hash := sha256.Sum256([]byte(input))
    return fmt.Sprintf("%x", hash[:])
}


// IsSLABreached returns true if the finding is overdue its SLA deadline
func (f *Finding) IsSLABreached() bool {
    if !f.Active || f.SLAExpirationDate == nil {
        return false
    }
    return time.Now().UTC().After(*f.SLAExpirationDate)
}

// DaysUntilSLA returns the number of days until SLA expires (negative if breached)
func (f *Finding) DaysUntilSLA() *int {
    if f.SLAExpirationDate == nil {
        return nil
    }
    days := int(time.Until(*f.SLAExpirationDate).Hours() / 24)
    return &days
}
