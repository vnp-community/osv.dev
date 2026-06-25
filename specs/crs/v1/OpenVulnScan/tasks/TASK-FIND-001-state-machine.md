# TASK-FIND-001 — Finding State Machine + Domain Entity

| Field | Value |
|-------|-------|
| **Task ID** | T-FIND-001 |
| **Service** | `finding-service` |
| **Status** | ✅ Completed |
| **Solution Ref** | SOL-OVS-002 §3 Domain Model |
| **Priority** | 🔴 High |
| **Depends On** | — |
| **Estimated** | 3h |

---

## Context

`finding-service` đã tồn tại tại `services/finding-service/`. Task này tạo domain entity `Finding` với đầy đủ state machine cho finding lifecycle:

```
active ──── Close() ──── mitigated
       ──── MarkFalsePositive() ──── false_positive
       ──── AcceptRisk() ──── risk_accepted
       ──── MarkOutOfScope() ──── out_of_scope
mitigated ──── Reopen() ──── active (reset tất cả flags)
```

Duplicate là trạng thái đặc biệt: không thể Reopen, không thể Close.

---

## Goal

Implement `Finding` domain entity với:
1. Complete state machine (Close, Reopen, MarkFalsePositive, AcceptRisk, MarkOutOfScope)
2. `CurrentState()` method trả về string description
3. Validation in constructor
4. Hash computation cho deduplication (SHA-256)

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/finding-service/internal/domain/finding/entity.go` |
| CREATE | `services/finding-service/internal/domain/finding/errors.go` |
| CREATE | `services/finding-service/internal/domain/finding/entity_test.go` |

---

## Implementation

### File 1: `services/finding-service/internal/domain/finding/entity.go`

```go
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

    // ---- Context ----
    TestID       uuid.UUID // Link to product-service Test
    EngagementID uuid.UUID // Link to product-service Engagement
    ProductID    uuid.UUID // Link to product-service Product

    // ---- Deduplication ----
    DuplicateFindingID *uuid.UUID // Points to the original if this is a duplicate
    HashCode           string     // SHA-256 for dedup: title|component|version|cve

    // ---- Location ----
    ComponentName    string // IP, package name, or filename
    ComponentVersion string
    Service          string
    FilePath         string
    LineNumber       *int

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

// ---- State Transitions ----

// Close marks the finding as remediated.
// Only allowed when: active=true, not duplicate.
func (f *Finding) Close(mitigatedByID uuid.UUID) error {
    if f.Duplicate {
        return ErrCannotCloseDuplicate
    }
    if f.IsMitigated {
        return ErrAlreadyMitigated
    }
    if !f.Active {
        return ErrNotActive
    }

    now := time.Now().UTC()
    f.Active = false
    f.IsMitigated = true
    f.MitigatedAt = &now
    f.MitigatedByID = &mitigatedByID
    f.LastStatusUpdate = &now
    f.UpdatedAt = now
    return nil
}

// Reopen restores an active finding state.
// Not allowed for duplicates.
func (f *Finding) Reopen() error {
    if f.Duplicate {
        return ErrCannotReopenDuplicate
    }
    if f.Active {
        return ErrAlreadyActive
    }

    now := time.Now().UTC()
    // Reset ALL state flags
    f.Active = true
    f.IsMitigated = false
    f.FalsePositive = false
    f.RiskAccepted = false
    f.OutOfScope = false
    f.MitigatedAt = nil
    f.MitigatedByID = nil
    f.LastStatusUpdate = &now
    f.UpdatedAt = now
    return nil
}

// MarkFalsePositive flags this finding as a tool false alarm.
func (f *Finding) MarkFalsePositive() error {
    if f.Duplicate {
        return ErrCannotModifyDuplicate
    }
    if !f.Active {
        return ErrNotActive
    }

    now := time.Now().UTC()
    f.Active = false
    f.FalsePositive = true
    f.LastStatusUpdate = &now
    f.UpdatedAt = now
    return nil
}

// AcceptRisk marks the finding as risk-accepted by a stakeholder.
func (f *Finding) AcceptRisk() error {
    if f.Duplicate {
        return ErrCannotModifyDuplicate
    }
    if !f.Active {
        return ErrNotActive
    }

    now := time.Now().UTC()
    f.RiskAccepted = true
    f.LastStatusUpdate = &now
    f.UpdatedAt = now
    return nil
}

// MarkOutOfScope marks this finding as not in scope for this engagement.
func (f *Finding) MarkOutOfScope() error {
    if f.Duplicate {
        return ErrCannotModifyDuplicate
    }
    if !f.Active {
        return ErrNotActive
    }

    now := time.Now().UTC()
    f.Active = false
    f.OutOfScope = true
    f.LastStatusUpdate = &now
    f.UpdatedAt = now
    return nil
}

// MarkDuplicate sets this finding as a duplicate of an existing one.
// Duplicates are immediately marked inactive.
func (f *Finding) MarkDuplicate(originalID uuid.UUID) {
    now := time.Now().UTC()
    f.Duplicate = true
    f.Active = false
    f.DuplicateFindingID = &originalID
    f.LastStatusUpdate = &now
    f.UpdatedAt = now
}

// CurrentState returns the primary state string for display purposes.
// Priority: Duplicate > FalsePositive > OutOfScope > RiskAccepted > Mitigated > Active
func (f *Finding) CurrentState() string {
    switch {
    case f.Duplicate:
        return "duplicate"
    case f.FalsePositive:
        return "false_positive"
    case f.OutOfScope:
        return "out_of_scope"
    case f.RiskAccepted:
        return "risk_accepted"
    case f.IsMitigated:
        return "mitigated"
    default:
        return "active"
    }
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
```

### File 2: `services/finding-service/internal/domain/finding/errors.go`

```go
package finding

import "errors"

var (
    ErrTitleRequired          = errors.New("finding title is required")
    ErrInvalidSeverity        = errors.New("invalid severity: must be Critical, High, Medium, Low, or Info")
    ErrTestIDRequired         = errors.New("test ID is required")
    ErrEngagementIDRequired   = errors.New("engagement ID is required")
    ErrProductIDRequired      = errors.New("product ID is required")
    ErrNotActive              = errors.New("finding must be active to perform this action")
    ErrAlreadyActive          = errors.New("finding is already active")
    ErrAlreadyMitigated       = errors.New("finding is already mitigated")
    ErrCannotCloseDuplicate   = errors.New("duplicate findings cannot be closed directly")
    ErrCannotReopenDuplicate  = errors.New("duplicate findings cannot be reopened")
    ErrCannotModifyDuplicate  = errors.New("duplicate findings cannot be modified")
    ErrFindingNotFound        = errors.New("finding not found")
)
```

### File 3: `services/finding-service/internal/domain/finding/entity_test.go`

```go
package finding

import (
    "strings"
    "testing"

    "github.com/google/uuid"
)

var (
    testTestID       = uuid.New()
    testEngagementID = uuid.New()
    testProductID    = uuid.New()
)

func newTestFinding(t *testing.T) *Finding {
    t.Helper()
    f, err := NewFinding(
        "[CVE-2021-44228] Log4j RCE",
        SeverityCritical,
        testTestID, testEngagementID, testProductID,
        "192.168.1.10", "", "CVE-2021-44228",
    )
    if err != nil {
        t.Fatalf("NewFinding(): %v", err)
    }
    return f
}

func TestNewFinding_Valid(t *testing.T) {
    f := newTestFinding(t)
    if f.ID == uuid.Nil {
        t.Error("ID should be set")
    }
    if !f.Active {
        t.Error("new finding should be active")
    }
    if f.HashCode == "" {
        t.Error("hash should be computed")
    }
    if f.CurrentState() != "active" {
        t.Errorf("CurrentState = %s, want active", f.CurrentState())
    }
}

func TestNewFinding_EmptyTitle(t *testing.T) {
    _, err := NewFinding("", SeverityHigh, testTestID, testEngagementID, testProductID, "", "", "")
    if err != ErrTitleRequired {
        t.Errorf("expected ErrTitleRequired, got: %v", err)
    }
}

func TestNewFinding_InvalidSeverity(t *testing.T) {
    _, err := NewFinding("title", "Unknown", testTestID, testEngagementID, testProductID, "", "", "")
    if err != ErrInvalidSeverity {
        t.Errorf("expected ErrInvalidSeverity, got: %v", err)
    }
}

func TestFinding_Close(t *testing.T) {
    f := newTestFinding(t)
    userID := uuid.New()

    if err := f.Close(userID); err != nil {
        t.Fatalf("Close(): %v", err)
    }
    if f.Active {
        t.Error("Active should be false after Close")
    }
    if !f.IsMitigated {
        t.Error("IsMitigated should be true after Close")
    }
    if f.MitigatedAt == nil {
        t.Error("MitigatedAt should be set")
    }
    if f.CurrentState() != "mitigated" {
        t.Errorf("CurrentState = %s, want mitigated", f.CurrentState())
    }
}

func TestFinding_Close_Duplicate(t *testing.T) {
    f := newTestFinding(t)
    f.MarkDuplicate(uuid.New())
    err := f.Close(uuid.New())
    if err != ErrCannotCloseDuplicate {
        t.Errorf("expected ErrCannotCloseDuplicate, got: %v", err)
    }
}

func TestFinding_Reopen(t *testing.T) {
    f := newTestFinding(t)
    f.Close(uuid.New())

    if err := f.Reopen(); err != nil {
        t.Fatalf("Reopen(): %v", err)
    }
    if !f.Active {
        t.Error("Active should be true after Reopen")
    }
    if f.IsMitigated {
        t.Error("IsMitigated should be reset after Reopen")
    }
    if f.FalsePositive || f.RiskAccepted || f.OutOfScope {
        t.Error("All state flags should be reset after Reopen")
    }
}

func TestFinding_Reopen_Duplicate(t *testing.T) {
    f := newTestFinding(t)
    f.MarkDuplicate(uuid.New())
    err := f.Reopen()
    if err != ErrCannotReopenDuplicate {
        t.Errorf("expected ErrCannotReopenDuplicate, got: %v", err)
    }
}

func TestFinding_MarkFalsePositive(t *testing.T) {
    f := newTestFinding(t)
    if err := f.MarkFalsePositive(); err != nil {
        t.Fatalf("MarkFalsePositive(): %v", err)
    }
    if f.Active {
        t.Error("Active should be false after MarkFalsePositive")
    }
    if !f.FalsePositive {
        t.Error("FalsePositive should be true")
    }
    if f.CurrentState() != "false_positive" {
        t.Errorf("CurrentState = %s, want false_positive", f.CurrentState())
    }
}

func TestFinding_HashCode_Consistency(t *testing.T) {
    f1 := newTestFinding(t)
    f2 := newTestFinding(t)
    // Same inputs → same hash
    if f1.HashCode != f2.HashCode {
        t.Error("Same inputs should produce same HashCode")
    }
}

func TestFinding_HashCode_Dedup(t *testing.T) {
    f, _ := NewFinding("[CVE-2021-44228] Log4j RCE", SeverityCritical,
        testTestID, testEngagementID, testProductID,
        "192.168.1.10", "", "CVE-2021-44228")

    // Different IP = different hash
    f2, _ := NewFinding("[CVE-2021-44228] Log4j RCE", SeverityCritical,
        testTestID, testEngagementID, testProductID,
        "192.168.1.11", "", "CVE-2021-44228")

    if f.HashCode == f2.HashCode {
        t.Error("Different IPs should produce different HashCodes")
    }
}

func TestFinding_CurrentState_Priority(t *testing.T) {
    f := newTestFinding(t)
    f.Duplicate = true
    f.FalsePositive = true

    // Duplicate takes priority over FalsePositive
    if f.CurrentState() != "duplicate" {
        t.Errorf("CurrentState = %s, want duplicate (highest priority)", f.CurrentState())
    }
}

func TestSeverity_Numerical(t *testing.T) {
    tests := []struct {
        severity Severity
        expected int
    }{
        {SeverityCritical, 4},
        {SeverityHigh, 3},
        {SeverityMedium, 2},
        {SeverityLow, 1},
        {SeverityInfo, 0},
    }
    for _, tt := range tests {
        if got := tt.severity.Numerical(); got != tt.expected {
            t.Errorf("%s.Numerical() = %d, want %d", tt.severity, got, tt.expected)
        }
    }
}

func TestFinding_HashCode_CVEUpperCase(t *testing.T) {
    // CVE should be normalized to uppercase in hash
    f1, _ := NewFinding("Test", SeverityHigh, testTestID, testEngagementID, testProductID,
        "host", "", "cve-2021-44228")
    f2, _ := NewFinding("Test", SeverityHigh, testTestID, testEngagementID, testProductID,
        "host", "", "CVE-2021-44228")

    if f1.HashCode != f2.HashCode {
        t.Error("CVE case normalization should produce same hash")
    }
}

func TestFinding_HashCode_Contains_Expected(t *testing.T) {
    // Hash should be 64-char hex string (SHA-256)
    f := newTestFinding(t)
    if len(f.HashCode) != 64 {
        t.Errorf("HashCode length = %d, want 64 (SHA-256 hex)", len(f.HashCode))
    }
    if strings.ContainsAny(f.HashCode, "GHIJKLMNOPQRSTUVWXYZ ") {
        t.Error("HashCode should be lowercase hex only")
    }
}
```

---

## Verification

```bash
cd services/finding-service
go build ./internal/domain/finding/...
go test ./internal/domain/finding/... -v
```

**Expected**:
```
--- PASS: TestNewFinding_Valid
--- PASS: TestNewFinding_EmptyTitle
--- PASS: TestNewFinding_InvalidSeverity
--- PASS: TestFinding_Close
--- PASS: TestFinding_Close_Duplicate
--- PASS: TestFinding_Reopen
--- PASS: TestFinding_Reopen_Duplicate
--- PASS: TestFinding_MarkFalsePositive
--- PASS: TestFinding_HashCode_Consistency
--- PASS: TestFinding_HashCode_Dedup
--- PASS: TestFinding_CurrentState_Priority
--- PASS: TestSeverity_Numerical
--- PASS: TestFinding_HashCode_CVEUpperCase
--- PASS: TestFinding_HashCode_Contains_Expected
```

### Checklist

- [x] `NewFinding("")` → `ErrTitleRequired`
- [x] `NewFinding(..., "Unknown")` → `ErrInvalidSeverity`
- [x] `NewFinding(uuid.Nil, ...)` test/engagement/product → specific errors
- [x] `finding.Active == true` on creation
- [x] `HashCode` = 64 char hex (SHA-256)
- [x] Same title+component+version+cve → same HashCode (idempotent)
- [x] `Close()` → `active=false, is_mitigated=true, mitigated_at set`
- [x] `Close()` on duplicate → `ErrInvalidTransition`
- [x] `Reopen()` → all flags reset, `active=true`
- [x] `Reopen()` on duplicate → `ErrInvalidTransition`
- [x] `MarkFalsePositive()` → `active=false, false_positive=true`
- [x] `CurrentState()` priority: duplicate > false_positive > out_of_scope > risk_accepted > mitigated > active
- [x] `IsSLABreached()` = `false` when no SLA date set
