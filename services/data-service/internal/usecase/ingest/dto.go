// Package ingest — dto.go
// Data transfer objects for the OSV ingest pipeline.
package ingest

import (
	"fmt"
	"time"
)

// IngestRequest is the input for the OSV ingest pipeline.
type IngestRequest struct {
	RawRecord []byte    // Raw JSON bytes of an OSV/CVE record
	Source    string    // Source identifier: "nvd", "ghsa", "pypi", "go-vulndb", etc.
	SourceURL string    // URL where the record was fetched from (for audit logging)
	FetchedAt time.Time // When the record was fetched from the source
}

// IngestResult reports the outcome of processing one record.
type IngestResult struct {
	CVEID  string `json:"cve_id"`  // Normalized CVE/GHSA/OSV ID
	Action string `json:"action"`  // "created" | "updated" | "skipped"
	Source string `json:"source"`
}

// IngestError captures per-record errors with source context.
type IngestError struct {
	Source string
	Err    error
}

func (e *IngestError) Error() string {
	return fmt.Sprintf("ingest[%s]: %v", e.Source, e.Err)
}

func (e *IngestError) Unwrap() error { return e.Err }
