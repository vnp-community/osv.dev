// Package entity contains the SyncJob domain entity.
// Adapted from ingestion-service/internal/domain/entity/sync_job.go.
package entity

import (
	"time"

	// Re-export SourceName from the canonical location
	cveentity "github.com/globalcve/mono/internal/cvesearch/domain/entity"
)

// SourceName is a type alias — re-exported from cvesearch/domain/entity for convenience.
type SourceName = cveentity.SourceName

// Source name constants — re-exported from cvesearch/domain/entity.
const (
	SourceNameNVD       = cveentity.SourceNameNVD
	SourceNameCIRCL     = cveentity.SourceNameCIRCL
	SourceNameJVN       = cveentity.SourceNameJVN
	SourceNameExploitDB = cveentity.SourceNameExploitDB
	SourceNameCVEOrg    = cveentity.SourceNameCVEOrg
	SourceNameEPSS      = cveentity.SourceNameEPSS
	SourceNameNVDCPE    = cveentity.SourceNameNVDCPE
	SourceNameCAPEC     = cveentity.SourceNameCAPEC
	SourceNameCWE       = cveentity.SourceNameCWE
)

// AllCVESources lists all CVE data source names.
var AllCVESources = []SourceName{
	SourceNameNVD,
	SourceNameCIRCL,
	SourceNameJVN,
	SourceNameExploitDB,
	SourceNameCVEOrg,
}

// SyncStatus represents the current state of a sync job.
type SyncStatus string

const (
	SyncStatusPending   SyncStatus = "PENDING"
	SyncStatusRunning   SyncStatus = "RUNNING"
	SyncStatusCompleted SyncStatus = "COMPLETED"
	SyncStatusFailed    SyncStatus = "FAILED"
)

// SyncJob tracks a single sync operation for one data source.
type SyncJob struct {
	ID          int64
	Source      SourceName
	Status      SyncStatus
	StartedAt   time.Time
	CompletedAt *time.Time
	Synced      int    // CVEs upserted
	Skipped     int    // Duplicates skipped
	Errors      int    // Error count
	ErrorMsg    string
}

// SyncStats holds the results of a single sync operation.
type SyncStats struct {
	Synced   int
	Skipped  int
	Errors   int
	ErrorMsg string
}
