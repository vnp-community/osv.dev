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

func ptrStr(s string) *string { return &s }

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
	if f.CurrentState() != StateActive {
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
	userID := uuid.New().String()

	if err := f.Close(&userID); err != nil {
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
	if f.CurrentState() != StateMitigated {
		t.Errorf("CurrentState = %s, want mitigated", f.CurrentState())
	}
}

func TestFinding_Close_Duplicate(t *testing.T) {
	f := newTestFinding(t)
	oid := uuid.New().String()
	f.MarkDuplicate(&oid)
	err := f.Close(nil)
	// Duplicate → cannot transition to mitigated
	if err != ErrInvalidTransition {
		t.Errorf("expected ErrInvalidTransition, got: %v", err)
	}
}

func TestFinding_Reopen(t *testing.T) {
	f := newTestFinding(t)
	userID := uuid.New().String()
	f.Close(&userID)

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
	oid := uuid.New().String()
	f.MarkDuplicate(&oid)
	err := f.Reopen()
	// Duplicate → cannot transition to active
	if err != ErrInvalidTransition {
		t.Errorf("expected ErrInvalidTransition, got: %v", err)
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
	if f.CurrentState() != StateFalsePositive {
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
	if f.CurrentState() != StateDuplicate {
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

func TestFinding_IsSLABreached_NoDate(t *testing.T) {
	f := newTestFinding(t)
	if f.IsSLABreached() {
		t.Error("IsSLABreached() = true, want false when no SLA date set")
	}
}
