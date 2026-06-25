package entity

import "time"

// CWEEntry represents a CWE weakness from MITRE.
type CWEEntry struct {
	ID          string    `db:"id"          json:"id"`           // "CWE-89"
	Name        string    `db:"name"        json:"name"`
	Description string    `db:"description" json:"description"`
	Abstraction string    `db:"abstraction" json:"abstraction"`  // "Base"|"Class"|"Variant"
	Status      string    `db:"status"      json:"status"`
	CAPECIDs    []string  `db:"-"           json:"capec_ids,omitempty"`
	Mitigations []string  `db:"-"           json:"mitigations"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`
}

// CAPECEntry represents a CAPEC attack pattern from MITRE.
type CAPECEntry struct {
	ID          string    `db:"id"          json:"id"`           // "CAPEC-66"
	Name        string    `db:"name"        json:"name"`
	Description string    `db:"description" json:"description"`
	Likelihood  string    `db:"likelihood"  json:"likelihood"`   // "High"|"Medium"|"Low"
	Severity    string    `db:"severity"    json:"severity"`
	CWEIDs      []string  `db:"-"           json:"cwe_ids,omitempty"`
	Mitigations []string  `db:"-"           json:"mitigations"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`
}
