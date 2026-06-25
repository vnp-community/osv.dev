// Package repository — triage_repository.go
// SEED-004: TriageRepository interface for persistent CVE triage decisions.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TriageRecord is the persistent triage record stored in osv_cves.triage_entries.
// Distinct from entity.TriageEntry which is ephemeral/in-memory from VEX files.
type TriageRecord struct {
	ID            uuid.UUID
	CVEID         string
	Remarks       string // "NewFound"|"Unexplored"|"Confirmed"|"Mitigated"|"FalsePositive"|"NotAffected"
	Comments      string
	Justification string
	Response      []string
	TriagedBy     uuid.UUID
	TriagedAt     time.Time
	UpdatedAt     time.Time
}

// TriageUpsertInput is the input for creating/updating a triage record.
type TriageUpsertInput struct {
	CVEID         string
	Remarks       string
	Comments      string
	Justification string
	Response      []string
	TriagedBy     uuid.UUID
}

// TriageResult is the per-item result for bulk triage operations.
type TriageResult struct {
	CVEID   string `json:"cve_id"`
	Status  string `json:"status"`  // "triaged" | "not_found" | "error"
	Message string `json:"message,omitempty"`
}

// ValidRemarks is the set of allowed remarks values.
var ValidRemarks = map[string]bool{
	"NewFound": true, "Unexplored": true, "Confirmed": true,
	"Mitigated": true, "FalsePositive": true, "NotAffected": true,
}

// TriageRepository defines persistence operations for CVE triage decisions.
type TriageRepository interface {
	// Upsert creates or updates a triage record. ON CONFLICT (cve_id) DO UPDATE.
	Upsert(ctx context.Context, input TriageUpsertInput) (*TriageRecord, error)

	// FindByCVEID returns the triage record for a CVE, or ErrNotFound.
	FindByCVEID(ctx context.Context, cveID string) (*TriageRecord, error)

	// BulkUpsert applies triage to multiple CVE IDs with the same remarks.
	// Partial failures are captured as TriageResult{Status: "error"}.
	BulkUpsert(ctx context.Context, cveIDs []string, remarks, comments, justification string, triagedBy uuid.UUID) ([]TriageResult, error)
}
