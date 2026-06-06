// Package entity contains the SyncJob entity for tracking sync operations.
package entity

import "time"

// SyncStatus represents the lifecycle state of a sync job.
type SyncStatus string

const (
	SyncStatusPending   SyncStatus = "PENDING"
	SyncStatusRunning   SyncStatus = "RUNNING"
	SyncStatusCompleted SyncStatus = "COMPLETED"
	SyncStatusFailed    SyncStatus = "FAILED"
)

// SourceName is a typed source identifier.
type SourceName string

const (
	SourceNameNVD       SourceName = "NVD"
	SourceNameCIRCL     SourceName = "CIRCL"
	SourceNameJVN       SourceName = "JVN"
	SourceNameExploitDB SourceName = "EXPLOITDB"
	SourceNameCVEOrg    SourceName = "CVE.ORG"
)

// AllSources lists all supported source names.
var AllSources = []SourceName{
	SourceNameNVD, SourceNameCIRCL, SourceNameJVN, SourceNameExploitDB, SourceNameCVEOrg,
}

// SyncJob tracks a single sync operation for one data source.
type SyncJob struct {
	ID          int64
	Source      SourceName
	Status      SyncStatus
	StartedAt   time.Time
	CompletedAt *time.Time
	Synced      int
	Skipped     int
	Errors      int
	ErrorMsg    string
}

// SyncStats holds statistics for updating a sync job.
type SyncStats struct {
	Synced   int
	Skipped  int
	Errors   int
	ErrorMsg string
}
