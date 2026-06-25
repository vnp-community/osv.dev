// Package reportuc implements the report generation use case for finding-service.
package reportuc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/osv/finding-service/internal/domain/report"
)

// ─── Interfaces ────────────────────────────────────────────────────────────────

// FindingRepository streams findings for report generation.
type FindingRepository interface {
	ListForReport(ctx context.Context, opts *ListOptions) ([]*FindingRow, error)
}

// ListOptions controls which findings are included in the report.
type ListOptions struct {
	ProductID    string
	EngagementID *string
	Severities   []string
	ActiveOnly   bool
}

// FindingRow is a minimal finding record for report output.
type FindingRow struct {
	ID          string
	Title       string
	Severity    string
	Status      string
	CVE         string
	FilePath    string
	Description string
	FoundDate   time.Time
}

// ReportGenerator produces a file stream from a list of findings.
type ReportGenerator interface {
	Generate(ctx context.Context, findings []*FindingRow, meta *ReportMeta) (io.Reader, error)
}

// ReportMeta is metadata passed to the generator.
type ReportMeta struct {
	ProductID   string
	Title       string
	GeneratedAt time.Time
	Format      string
}

// Storage uploads report files to object storage (MinIO/S3).
type Storage interface {
	Upload(ctx context.Context, key string, r io.Reader) (int64, error)
	PresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// EventPublisher publishes NATS events.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload map[string]any) error
}

// ─── Generate ─────────────────────────────────────────────────────────────────

// GenerateInput is the request for POST /api/v2/reports.
type GenerateInput struct {
	ProductID    string
	Title        string
	Format       report.ReportFormat
	EngagementID *string
	Severities   []string
	ActiveOnly   bool
	RequesterID  string
}

// GenerateUseCase creates a report asynchronously.
type GenerateUseCase struct {
	reportRepo  report.Repository
	findingRepo FindingRepository
	generators  map[report.ReportFormat]ReportGenerator
	storage     Storage
	eventPub    EventPublisher
}

// NewGenerate creates a new GenerateUseCase.
func NewGenerate(
	rr report.Repository,
	fr FindingRepository,
	gen map[report.ReportFormat]ReportGenerator,
	st Storage,
	ep EventPublisher,
) *GenerateUseCase {
	return &GenerateUseCase{
		reportRepo:  rr,
		findingRepo: fr,
		generators:  gen,
		storage:     st,
		eventPub:    ep,
	}
}

// Execute creates a pending report record and starts async generation.
// Returns the Report immediately with status=pending.
func (uc *GenerateUseCase) Execute(ctx context.Context, in GenerateInput) (*report.Report, error) {
	// Validate format
	if _, ok := uc.generators[in.Format]; !ok {
		return nil, fmt.Errorf("unsupported report format: %s", in.Format)
	}

	now := time.Now().UTC()
	r := &report.Report{
		ID:           uuid.New().String(),
		ProductID:    in.ProductID,
		Title:        in.Title,
		Format:       in.Format,
		Status:       report.StatusPending,
		EngagementID: in.EngagementID,
		Severities:   in.Severities,
		ActiveOnly:   in.ActiveOnly,
		GeneratedBy:  in.RequesterID,
		CreatedAt:    now,
	}
	if err := uc.reportRepo.Save(ctx, r); err != nil {
		return nil, fmt.Errorf("saving report record: %w", err)
	}

	// Generate in background goroutine
	go uc.generate(r, in)

	return r, nil
}

// generate runs the full report generation pipeline (async).
func (uc *GenerateUseCase) generate(r *report.Report, in GenerateInput) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	slog.InfoContext(ctx, "starting report generation",
		"report_id", r.ID, "format", r.Format, "product_id", r.ProductID)

	// Mark as generating
	r.Status = report.StatusGenerating
	_ = uc.reportRepo.Save(ctx, r)

	// 1. Fetch findings
	findings, err := uc.findingRepo.ListForReport(ctx, &ListOptions{
		ProductID:    in.ProductID,
		EngagementID: in.EngagementID,
		Severities:   in.Severities,
		ActiveOnly:   in.ActiveOnly,
	})
	if err != nil {
		uc.markFailed(ctx, r, "failed to fetch findings: "+err.Error())
		return
	}

	// 2. Generate file
	generator := uc.generators[in.Format]
	now := time.Now().UTC()
	fileReader, err := generator.Generate(ctx, findings, &ReportMeta{
		ProductID:   in.ProductID,
		Title:       in.Title,
		GeneratedAt: now,
		Format:      string(in.Format),
	})
	if err != nil {
		uc.markFailed(ctx, r, "generation error: "+err.Error())
		return
	}

	// 3. Upload to MinIO
	key := fmt.Sprintf("reports/%s/%s.%s", in.ProductID, r.ID, string(in.Format))
	size, err := uc.storage.Upload(ctx, key, fileReader)
	if err != nil {
		uc.markFailed(ctx, r, "storage error: "+err.Error())
		return
	}

	// 4. Mark completed
	expires := now.AddDate(0, 0, report.RetentionDays)
	r.Status = report.StatusCompleted
	r.StorageKey = key
	r.FileSizeBytes = size
	r.GeneratedAt = &now
	r.ExpiresAt = &expires
	_ = uc.reportRepo.Save(ctx, r)

	_ = uc.eventPub.Publish(ctx, "defectdojo.report.generated", map[string]any{
		"report_id":  r.ID,
		"product_id": in.ProductID,
		"format":     string(in.Format),
		"_service":   "finding-service",
	})

	slog.InfoContext(ctx, "report generated",
		"report_id", r.ID, "size_bytes", size, "expires_at", expires)
}

func (uc *GenerateUseCase) markFailed(ctx context.Context, r *report.Report, msg string) {
	r.Status = report.StatusFailed
	r.ErrorMessage = msg
	_ = uc.reportRepo.Save(ctx, r)
	slog.ErrorContext(ctx, "report generation failed", "report_id", r.ID, "error", msg)
}
