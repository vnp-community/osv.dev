// domain/valueobject/vuln_id.go
package valueobject

import (
	"errors"
	"strings"
)

// VulnID is a validated vulnerability identifier.
type VulnID struct {
	value string
}

// Common vuln ID prefixes in priority order for canonical selection.
var canonicalPrefixes = []string{"CVE-", "GHSA-", "OSV-"}

var ErrInvalidVulnID = errors.New("invalid vuln ID: must be non-empty")

// NewVulnID creates a validated VulnID.
func NewVulnID(id string) (VulnID, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return VulnID{}, ErrInvalidVulnID
	}
	return VulnID{value: id}, nil
}

// MustVulnID panics if id is invalid — use only in tests/init.
func MustVulnID(id string) VulnID {
	v, err := NewVulnID(id)
	if err != nil {
		panic(err)
	}
	return v
}

func (v VulnID) String() string { return v.value }

// IsEmpty returns true if the VulnID was not initialized.
func (v VulnID) IsEmpty() bool { return v.value == "" }

// CanonicalPriority returns the canonical prefix priority (lower = higher priority).
// Returns len(canonicalPrefixes) if no known prefix matches.
func (v VulnID) CanonicalPriority() int {
	for i, prefix := range canonicalPrefixes {
		if strings.HasPrefix(v.value, prefix) {
			return i
		}
	}
	return len(canonicalPrefixes)
}
