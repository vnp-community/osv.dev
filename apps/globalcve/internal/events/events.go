// Package events defines NATS event types for inter-service communication.
package events

import "time"

// Subject constants for NATS JetStream.
const (
	// SubjectCVESynced is published when new CVEs are synced.
	SubjectCVESynced = "cve.synced"

	// SubjectKEVUpdated is published when the KEV catalog is updated.
	SubjectKEVUpdated = "kev.updated"

	// SubjectAlertTriggered is published when a new critical CVE alert fires.
	SubjectAlertTriggered = "alert.triggered"
)

// CVESyncedEvent is published by CVE Sync service after a successful sync.
type CVESyncedEvent struct {
	Source    string    `json:"source"`
	Synced    int       `json:"synced"`
	SyncedAt  time.Time `json:"synced_at"`
}

// KEVUpdatedEvent is published by KEV service after catalog sync.
type KEVUpdatedEvent struct {
	Total     int       `json:"total"`
	Inserted  int       `json:"inserted"`
	Updated   int       `json:"updated"`
	SyncedAt  time.Time `json:"synced_at"`
	// CVE IDs that were newly added to KEV (for marking is_kev in CVEs table)
	NewKEVIDs []string `json:"new_kev_ids,omitempty"`
}

// AlertEvent is published when a new critical CVE is detected.
type AlertEvent struct {
	CVEID       string    `json:"cve_id"`
	Severity    string    `json:"severity"`
	CVSS3Score  float64   `json:"cvss3_score"`
	Description string    `json:"description"`
	PublishedAt time.Time `json:"published_at"`
}
