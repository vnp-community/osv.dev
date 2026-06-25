// Package findingbulk provides the finding bulk-create and import use case for SEED-003.
// It tái dùng SHA-256 dedup pipeline (Finding.ComputeHash) và SLA auto-compute logic
// đã có sẵn trong finding domain.
package findingbulk

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/finding"
	"github.com/osv/finding-service/internal/domain/repository"
)

// ── Input / Output types ──────────────────────────────────────────────────────

// FindingCreateInput là input chuẩn hóa để tạo finding.
type FindingCreateInput struct {
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Mitigation       string     `json:"mitigation"`
	Severity         string     `json:"severity"` // Critical|High|Medium|Low|Info
	CVE              string     `json:"cve"`
	CWE              *int       `json:"cwe"` // nullable — some scanners omit CWE
	CVSSv3Score      *float64   `json:"cvss_v3_score,omitempty"`
	ComponentName    string     `json:"component_name"`
	ComponentVersion string     `json:"component_version"`
	Date             *time.Time `json:"date,omitempty"` // default: now
	Tags             []string   `json:"tags,omitempty"`
	TestID           uuid.UUID  `json:"test_id"`
}

// BulkCreateOptions là tùy chọn cho bulk operation.
type BulkCreateOptions struct {
	AutoCloseDuplicates bool   `json:"auto_close_duplicates"`
	AutoEnrichCVE       bool   `json:"auto_enrich_cve"` // future: hook AI-service
	ComputeSLA          bool   `json:"compute_sla"`
	MinimumSeverity     string `json:"minimum_severity"` // "Critical"|"High"|"Medium"|"Low"|""
}

// BulkFindingResult là per-item result.
type BulkFindingResult struct {
	Index    int        `json:"index"`
	Status   string     `json:"status"`             // "created" | "duplicate" | "skipped" | "error"
	ID       *uuid.UUID `json:"id,omitempty"`
	HashCode string     `json:"hash_code,omitempty"`
	Message  string     `json:"message,omitempty"`
}

// BulkCreateOutput is the response for a bulk create operation.
type BulkCreateOutput struct {
	CreatedCount   int                 `json:"created_count"`
	DuplicateCount int                 `json:"duplicate_count"`
	SkippedCount   int                 `json:"skipped_count"`
	FailedCount    int                 `json:"failed_count"`
	Results        []BulkFindingResult `json:"results"`
}

// ImportOutput wraps BulkCreateOutput with an import summary.
type ImportOutput struct {
	ImportedCount  int                 `json:"imported_count"`
	DuplicateCount int                 `json:"duplicate_count"`
	SkippedCount   int                 `json:"skipped_count"`
	FailedCount    int                 `json:"failed_count"`
	Results        []BulkFindingResult `json:"results"`
}

// ── UseCase ───────────────────────────────────────────────────────────────────

// UseCase orchestrates finding bulk-create and import operations.
type UseCase struct {
	findingRepo    finding.Repository
	testRepo       repository.TestRepository
	engagementRepo repository.EngagementRepository
}

// New creates a new findingbulk UseCase.
func New(findingRepo finding.Repository, testRepo repository.TestRepository, engagementRepo repository.EngagementRepository) *UseCase {
	return &UseCase{
		findingRepo:    findingRepo,
		testRepo:       testRepo,
		engagementRepo: engagementRepo,
	}
}

// BulkCreate creates multiple findings for a given test.
// It computes SHA-256 dedup hashes, filters by minimum severity, and handles duplicates.
func (uc *UseCase) BulkCreate(ctx context.Context, testID uuid.UUID, inputs []FindingCreateInput, opts BulkCreateOptions) (*BulkCreateOutput, error) {
	if len(inputs) == 0 {
		return &BulkCreateOutput{Results: []BulkFindingResult{}}, nil
	}
	if len(inputs) > 500 {
		return nil, fmt.Errorf("bulk limit exceeded: max 500 findings per request")
	}

	// Load test to get engagementID + productID
	t, err := uc.testRepo.FindByID(ctx, testID)
	if err != nil {
		return nil, fmt.Errorf("test not found: %w", err)
	}

	// Fetch engagement to get productID (Test only stores EngagementID)
	eng, err := uc.engagementRepo.FindByID(ctx, t.EngagementID)
	if err != nil {
		return nil, fmt.Errorf("engagement not found for test: %w", err)
	}

	minSevNum := severityNum(opts.MinimumSeverity)

	out := &BulkCreateOutput{Results: make([]BulkFindingResult, 0, len(inputs))}

	for i, in := range inputs {
		// Filter by minimum severity
		if minSevNum > 0 && severityNum(in.Severity) < minSevNum {
			out.SkippedCount++
			out.Results = append(out.Results, BulkFindingResult{
				Index:   i,
				Status:  "skipped",
				Message: fmt.Sprintf("severity %s below minimum %s", in.Severity, opts.MinimumSeverity),
			})
			continue
		}

		// Compute SHA-256 dedup hash (mirrors Finding.ComputeHash algorithm)
		hashCode := computeHash(in.Title, in.ComponentName, in.ComponentVersion, in.CVE)

		// Dedup check
		existingDup, _ := uc.findingRepo.FindByHashCode(ctx, hashCode, testID, false, nil, nil)
		if existingDup != nil {
			out.DuplicateCount++
			existingID := existingDup.ID
			out.Results = append(out.Results, BulkFindingResult{
				Index:    i,
				Status:   "duplicate",
				ID:       &existingID,
				HashCode: hashCode,
				Message:  "finding with same hash already exists",
			})
			continue
		}

		// Build Finding entity
		date := time.Now().UTC()
		if in.Date != nil {
			date = *in.Date
		}
		sev := finding.Severity(normalizeCapSeverity(in.Severity))

		tags := in.Tags
		if tags == nil {
			tags = []string{}
		}

		f := &finding.Finding{
			ID:                uuid.New(),
			Title:             in.Title,
			Description:       in.Description,
			Mitigation:        in.Mitigation,
			Impact:            "",
			References:        "",
			Severity:          sev,
			NumericalSeverity: sev.Numerical(),
			CVE:               in.CVE,
			CWE:               derefInt(in.CWE),
			VulnIDFromTool:    "",
			CVSSv3:            "",
			CVSSv3Score:       in.CVSSv3Score,
			Active:            true,
			Verified:          false,
			FalsePositive:     false,
			Duplicate:         false,
			OutOfScope:        false,
			IsMitigated:       false,
			RiskAccepted:      false,
			Date:              date,
			HashCode:          hashCode,
			TestID:            testID,
			EngagementID:      t.EngagementID,
			ProductID:         eng.ProductID,
			ComponentName:     in.ComponentName,
			ComponentVersion:  in.ComponentVersion,
			Service:           "",
			FilePath:          "",
			Tags:              tags,
			InheritedTags:     []string{},
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		}

		if err := uc.findingRepo.Create(ctx, f); err != nil {
			out.FailedCount++
			out.Results = append(out.Results, BulkFindingResult{
				Index:    i,
				Status:   "error",
				HashCode: hashCode,
				Message:  err.Error(),
			})
			continue
		}

		id := f.ID
		out.CreatedCount++
		out.Results = append(out.Results, BulkFindingResult{
			Index:    i,
			Status:   "created",
			ID:       &id,
			HashCode: hashCode,
		})
	}

	return out, nil
}

// ImportFromJSON parses a JSON array of FindingCreateInput and calls BulkCreate.
func (uc *UseCase) ImportFromJSON(ctx context.Context, testID uuid.UUID, r io.Reader, opts BulkCreateOptions) (*ImportOutput, error) {
	var inputs []FindingCreateInput
	if err := json.NewDecoder(r).Decode(&inputs); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	// Inject testID into each input
	for i := range inputs {
		inputs[i].TestID = testID
	}
	bulk, err := uc.BulkCreate(ctx, testID, inputs, opts)
	if err != nil {
		return nil, err
	}
	return &ImportOutput{
		ImportedCount:  bulk.CreatedCount,
		DuplicateCount: bulk.DuplicateCount,
		SkippedCount:   bulk.SkippedCount,
		FailedCount:    bulk.FailedCount,
		Results:        bulk.Results,
	}, nil
}

// ImportFromCSV parses CSV rows into FindingCreateInput and calls BulkCreate.
// Expected header (case-insensitive): title, severity, cve, cwe, description, mitigation, component_name, component_version
func (uc *UseCase) ImportFromCSV(ctx context.Context, testID uuid.UUID, r io.Reader, opts BulkCreateOptions) (*ImportOutput, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true

	headers, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	idx := make(map[string]int, len(headers))
	for i, h := range headers {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	col := func(row []string, name string) string {
		if i, ok := idx[name]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	var inputs []FindingCreateInput
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse CSV row: %w", err)
		}
		cwe, _ := strconv.Atoi(col(row, "cwe"))
		var cwePtr *int
		if cwe != 0 {
			cwePtr = &cwe
		}
		inputs = append(inputs, FindingCreateInput{
			Title:            col(row, "title"),
			Severity:         col(row, "severity"),
			CVE:              col(row, "cve"),
			CWE:              cwePtr,
			Description:      col(row, "description"),
			Mitigation:       col(row, "mitigation"),
			ComponentName:    col(row, "component_name"),
			ComponentVersion: col(row, "component_version"),
			TestID:           testID,
		})
	}

	bulk, err := uc.BulkCreate(ctx, testID, inputs, opts)
	if err != nil {
		return nil, err
	}
	return &ImportOutput{
		ImportedCount:  bulk.CreatedCount,
		DuplicateCount: bulk.DuplicateCount,
		SkippedCount:   bulk.SkippedCount,
		FailedCount:    bulk.FailedCount,
		Results:        bulk.Results,
	}, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// computeHash replicates Finding.ComputeHash to produce stable dedup hashes
// without needing a full Finding entity. Algorithm must match domain layer.
func computeHash(title, componentName, componentVersion, cve string) string {
	parts := []string{
		strings.TrimSpace(title),
		strings.TrimSpace(componentName),
		strings.TrimSpace(componentVersion),
		strings.ToUpper(strings.TrimSpace(cve)),
	}
	input := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:])
}

// derefInt safely dereferences a *int pointer, returning 0 for nil.
func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func severityNum(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "info", "informational":
		return 1
	default:
		return 0
	}
}

func normalizeCapSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return "Critical"
	case "high":
		return "High"
	case "medium":
		return "Medium"
	case "low":
		return "Low"
	default:
		return "Info"
	}
}
