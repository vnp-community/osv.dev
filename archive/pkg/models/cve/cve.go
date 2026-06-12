// Package cve contains the shared CVE data model used across GlobalCVE services.
// This is a read/transfer model — not a DDD aggregate.
package cve

import "time"

// CVE is the canonical CVE record shared across services (search, sync, notification).
type CVE struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`    // CRITICAL|HIGH|MEDIUM|LOW|UNKNOWN
	Published   time.Time `json:"published"`
	Source      string    `json:"source"`      // NVD|CIRCL|JVN|EXPLOITDB|CVE.ORG|ARCHIVE
	IsKEV       bool      `json:"kev"`
	Link        string    `json:"link,omitempty"`
	CVSSScore   *float64  `json:"cvss,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EventType is the type of a CVE-related domain event.
type EventType string

const (
	// EventCVECreated fires when a new CVE is inserted into the database.
	EventCVECreated EventType = "cve.created"
	// EventCVEUpdated fires when an existing CVE record changes.
	EventCVEUpdated EventType = "cve.updated"
	// EventKEVAdded fires when a CVE is added to the CISA KEV catalog.
	EventKEVAdded EventType = "kev.added"
	// EventKEVRemoved fires when a CVE is removed from the KEV catalog.
	EventKEVRemoved EventType = "kev.removed"
	// EventSyncCompleted fires when a source sync job completes.
	EventSyncCompleted EventType = "sync.completed"
)

// SyncEvent is the event payload published when a CVE is created or updated.
// Used by notification-service to dispatch webhook calls.
type SyncEvent struct {
	EventID   string    `json:"event_id"`
	EventType EventType `json:"event_type"`
	CVE       *CVE      `json:"cve"`
	Source    string    `json:"source"` // which sync source triggered this
	Timestamp time.Time `json:"timestamp"`
}

// KEVEvent is the event payload published when a KEV catalog entry changes.
type KEVEvent struct {
	EventID   string    `json:"event_id"`
	EventType EventType `json:"event_type"` // kev.added or kev.removed
	CVEID     string    `json:"cve_id"`
	Timestamp time.Time `json:"timestamp"`
}

// Source constants — valid values for CVE.Source field.
const (
	SourceNVD       = "NVD"
	SourceCIRCL     = "CIRCL"
	SourceJVN       = "JVN"
	SourceExploitDB = "EXPLOITDB"
	SourceCVEOrg    = "CVE.ORG"
	SourceArchive   = "ARCHIVE"
)
