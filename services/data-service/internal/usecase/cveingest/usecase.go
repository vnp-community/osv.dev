// Package cveingest implements SEED-004 use cases for custom CVE creation and triage.
// It allows clients to seed CVE data into the data-service without going through
// the external fetcher pipeline.
package cveingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/osv/data-service/internal/domain/entity"
	"github.com/osv/data-service/internal/domain/repository"
)

// ── Input / Output types ──────────────────────────────────────────────────────

// CustomCVEInput is the input for creating a custom/internal CVE.
type CustomCVEInput struct {
	ID          string    `json:"id"`           // e.g. "CVE-2024-XXXX" or "INTERNAL-2024-001"
	Summary     string    `json:"summary"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`     // critical|high|medium|low|none
	Cvss3       *float64  `json:"cvss3,omitempty"`
	Cvss3Vector string    `json:"cvss3_vector,omitempty"`
	Published   time.Time `json:"published"`
	Source      string    `json:"source"`       // "INTERNAL" | "CERT-VN" | custom
	Vendors     []string  `json:"vendors,omitempty"`
	Products    []string  `json:"products,omitempty"`
	References  []string  `json:"references,omitempty"`
	IsKEV       bool      `json:"is_kev,omitempty"`
	IsExploit   bool      `json:"is_exploit,omitempty"`
}

// BulkImportResult is the result of a bulk CVE import.
type BulkImportResult struct {
	ImportedCount int                    `json:"imported_count"`
	SkippedCount  int                    `json:"skipped_count"`
	FailedCount   int                    `json:"failed_count"`
	Results       []CVEImportItemResult  `json:"results"`
}

// CVEImportItemResult is the per-CVE result from a bulk import.
type CVEImportItemResult struct {
	CVEID   string `json:"cve_id"`
	Status  string `json:"status"`  // "created" | "updated" | "skipped" | "error"
	Message string `json:"message,omitempty"`
}

// BulkTriageResult is the result of a bulk triage operation.
type BulkTriageResult struct {
	ProcessedCount int                        `json:"processed_count"`
	FailedCount    int                        `json:"failed_count"`
	Results        []repository.TriageResult  `json:"results"`
}

// ── UseCase ───────────────────────────────────────────────────────────────────

// UseCase orchestrates custom CVE creation and triage operations.
type UseCase struct {
	cveRepo    repository.CVERepository
	triageRepo repository.TriageRepository
}

// New creates a new cveingest UseCase.
func New(cveRepo repository.CVERepository, triageRepo repository.TriageRepository) *UseCase {
	return &UseCase{
		cveRepo:    cveRepo,
		triageRepo: triageRepo,
	}
}

// CreateCustomCVE creates a single custom CVE record.
// If a CVE with the same ID already exists and source != "INTERNAL", returns conflict error.
// Supports overwrite for INTERNAL source CVEs.
func (uc *UseCase) CreateCustomCVE(ctx context.Context, in CustomCVEInput) (*entity.CVE, error) {
	if in.ID == "" {
		return nil, fmt.Errorf("CVE ID is required")
	}

	// Check for existing CVE
	existing, err := uc.cveRepo.FindByCVEID(ctx, in.ID)
	if err == nil && existing != nil {
		// Allow overwrite only for INTERNAL source
		if existing.Source != "INTERNAL" {
			return nil, fmt.Errorf("conflict: CVE %s already exists with source %s", in.ID, existing.Source)
		}
	}

	source := in.Source
	if source == "" {
		source = "INTERNAL"
	}

	published := in.Published
	if published.IsZero() {
		published = time.Now().UTC()
	}

	sev := entity.Severity(in.Severity)
	if sev == "" {
		if in.Cvss3 != nil {
			sev = entity.SeverityFromCVSS(*in.Cvss3)
		} else {
			sev = entity.SeverityNone
		}
	}

	// Build references as string slice
	refs := make([]string, 0, len(in.References))
	for _, r := range in.References {
		refs = append(refs, r)
	}

	cve := &entity.CVE{
		ID:         in.ID,
		Summary:    in.Summary + " " + in.Description, // data-service uses Summary, not Details
		Severity:   sev,
		Published:  published,
		Modified:   time.Now().UTC(),
		Source:     source,
		IsKEV:      in.IsKEV,
		IsExploit:  in.IsExploit,
		References: refs,
	}
	if in.Cvss3 != nil {
		cve.CVSS3 = *in.Cvss3
		cve.CVSS3Vector = in.Cvss3Vector
	}

	if err := uc.cveRepo.Upsert(ctx, cve); err != nil {
		return nil, fmt.Errorf("upsert CVE: %w", err)
	}

	return cve, nil
}

// BulkImportCVEs imports multiple custom CVEs.
// If overwrite=false, existing non-INTERNAL CVEs are skipped (not errored).
func (uc *UseCase) BulkImportCVEs(ctx context.Context, items []CustomCVEInput, overwrite bool) (*BulkImportResult, error) {
	if len(items) > 500 {
		return nil, fmt.Errorf("bulk limit exceeded: max 500 CVEs per request")
	}

	result := &BulkImportResult{Results: make([]CVEImportItemResult, 0, len(items))}

	for _, item := range items {
		existing, _ := uc.cveRepo.FindByCVEID(ctx, item.ID)
		if existing != nil && existing.Source != "INTERNAL" && !overwrite {
			result.SkippedCount++
			result.Results = append(result.Results, CVEImportItemResult{
				CVEID:   item.ID,
				Status:  "skipped",
				Message: "CVE already exists with source " + existing.Source,
			})
			continue
		}

		_, err := uc.CreateCustomCVE(ctx, item)
		if err != nil {
			result.FailedCount++
			result.Results = append(result.Results, CVEImportItemResult{
				CVEID:   item.ID,
				Status:  "error",
				Message: err.Error(),
			})
			continue
		}

		status := "created"
		if existing != nil {
			status = "updated"
		}
		result.ImportedCount++
		result.Results = append(result.Results, CVEImportItemResult{
			CVEID:  item.ID,
			Status: status,
		})
	}

	return result, nil
}

// ImportCVEsFromJSON parses a JSON array and calls BulkImportCVEs.
func (uc *UseCase) ImportCVEsFromJSON(ctx context.Context, r io.Reader, overwrite bool) (*BulkImportResult, error) {
	var items []CustomCVEInput
	if err := json.NewDecoder(r).Decode(&items); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	return uc.BulkImportCVEs(ctx, items, overwrite)
}

// UpsertTriage creates or updates a triage record for a CVE.
func (uc *UseCase) UpsertTriage(ctx context.Context, actorID uuid.UUID, cveID, remarks, comments, justification string, response []string) (*repository.TriageRecord, error) {
	if cveID == "" {
		return nil, fmt.Errorf("cve_id is required")
	}
	if !repository.ValidRemarks[remarks] {
		return nil, fmt.Errorf("invalid remarks: must be one of NewFound|Unexplored|Confirmed|Mitigated|FalsePositive|NotAffected")
	}

	return uc.triageRepo.Upsert(ctx, repository.TriageUpsertInput{
		CVEID:         cveID,
		Remarks:       remarks,
		Comments:      comments,
		Justification: justification,
		Response:      response,
		TriagedBy:     actorID,
	})
}

// BulkTriage applies the same triage decision to multiple CVEs.
// Returns 207-style results: per-CVE status.
func (uc *UseCase) BulkTriage(ctx context.Context, actorID uuid.UUID, cveIDs []string, remarks, comments, justification string) (*BulkTriageResult, error) {
	if len(cveIDs) == 0 {
		return nil, fmt.Errorf("cve_ids is required")
	}
	if len(cveIDs) > 200 {
		return nil, fmt.Errorf("bulk limit exceeded: max 200 CVEs per request")
	}
	if !repository.ValidRemarks[remarks] {
		return nil, fmt.Errorf("invalid remarks: must be one of NewFound|Unexplored|Confirmed|Mitigated|FalsePositive|NotAffected")
	}

	results, err := uc.triageRepo.BulkUpsert(ctx, cveIDs, remarks, comments, justification, actorID)
	if err != nil {
		return nil, fmt.Errorf("bulk triage: %w", err)
	}

	out := &BulkTriageResult{Results: results}
	for _, r := range results {
		if r.Status == "triaged" {
			out.ProcessedCount++
		} else {
			out.FailedCount++
		}
	}
	return out, nil
}
