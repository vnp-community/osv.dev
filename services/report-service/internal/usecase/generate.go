package usecase

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"

    "github.com/google/osv.dev/services/report-service/internal/formatters"
    "github.com/google/osv.dev/services/report-service/internal/formatters/pdf"
    "github.com/google/osv.dev/services/report-service/internal/storage"
)

// ReportRun tracks an async report generation job.
type ReportRun struct {
    ID          uuid.UUID
    ScanID      uuid.UUID
    Status      string    // pending|running|completed|failed
    Formats     []string
    Artifacts   map[string]string // format → object key in MinIO
    ExitCode    int               // 0=clean, 1=CVEs found
    Error       string
    CreatedAt   time.Time
    CompletedAt *time.Time
}

// ReportRunRepository stores report runs.
type ReportRunRepository interface {
    Save(ctx context.Context, run *ReportRun) error
    Update(ctx context.Context, run *ReportRun) error
    FindByID(ctx context.Context, id uuid.UUID) (*ReportRun, error)
}

// FindingSource retrieves findings for a scan.
type FindingSource interface {
    FindByScanID(ctx context.Context, scanID uuid.UUID) ([]*formatters.Finding, error)
}

// GenerateInput is the input for report generation.
type GenerateInput struct {
    ScanID      uuid.UUID
    Formats     []string // "html"|"pdf"|"csv"|"json"|"excel"
    MinSeverity string
    MinScore    float64
    Theme       string  // "light"|"dark"
}

// GenerateOutput is the output from report generation.
type GenerateOutput struct {
    RunID    uuid.UUID
    ExitCode int
}

// GenerateUseCase orchestrates multi-format report generation.
type GenerateUseCase struct {
    runRepo     ReportRunRepository
    findingSrc  FindingSource
    htmlFmt     interface{ Format(context.Context, []*formatters.Finding) ([]byte, error) }
    pdfFmt      *pdf.PDFFormatter
    csvFmt      *formatters.CSVFormatter
    jsonFmt     *formatters.JSONFormatter
    excelFmt    *formatters.ExcelFormatter
    store       *storage.MinIOStorage
    logger      zerolog.Logger
}

// New creates the GenerateUseCase.
func New(
    runRepo ReportRunRepository,
    findingSrc FindingSource,
    store *storage.MinIOStorage,
    logger zerolog.Logger,
) *GenerateUseCase {
    return &GenerateUseCase{
        runRepo:    runRepo,
        findingSrc: findingSrc,
        csvFmt:     formatters.NewCSVFormatter(),
        jsonFmt:    formatters.NewJSONFormatter(),
        excelFmt:   formatters.NewExcelFormatter(),
        pdfFmt:     pdf.New(""),
        store:      store,
        logger:     logger,
    }
}

// Execute starts async report generation.
func (uc *GenerateUseCase) Execute(ctx context.Context, in GenerateInput) (*GenerateOutput, error) {
    run := &ReportRun{
        ID:        uuid.New(),
        ScanID:    in.ScanID,
        Status:    "pending",
        Formats:   in.Formats,
        Artifacts: make(map[string]string),
        CreatedAt: time.Now().UTC(),
    }

    if err := uc.runRepo.Save(ctx, run); err != nil {
        return nil, fmt.Errorf("save report run: %w", err)
    }

    // Run async
    go uc.runGeneration(context.Background(), run, in)

    return &GenerateOutput{RunID: run.ID}, nil
}

// runGeneration is the async worker that generates all report formats.
func (uc *GenerateUseCase) runGeneration(ctx context.Context, run *ReportRun, in GenerateInput) {
    run.Status = "running"
    _ = uc.runRepo.Update(ctx, run)

    // Fetch findings
    findings, err := uc.findingSrc.FindByScanID(ctx, in.ScanID)
    if err != nil {
        uc.failRun(ctx, run, fmt.Sprintf("fetch findings: %v", err))
        return
    }

    // Determine exit code
    run.ExitCode = 0
    for _, f := range findings {
        if f.Severity == "Critical" || f.Severity == "High" {
            run.ExitCode = 1
            break
        }
    }

    // Generate each format in parallel
    var mu sync.Mutex
    var wg sync.WaitGroup

    for _, format := range in.Formats {
        wg.Add(1)
        go func(fmt string) {
            defer wg.Done()

            data, err := uc.generateFormat(ctx, fmt, findings)
            if err != nil {
                uc.logger.Error().Err(err).Str("format", fmt).Msg("format generation failed")
                return
            }

            // Upload to MinIO
            key, err := uc.store.Upload(ctx, run.ID.String(), fmt, data)
            if err != nil {
                uc.logger.Error().Err(err).Str("format", fmt).Msg("upload failed")
                return
            }

            mu.Lock()
            run.Artifacts[fmt] = key
            mu.Unlock()
        }(format)
    }

    wg.Wait()

    // Mark complete
    now := time.Now().UTC()
    run.Status = "completed"
    run.CompletedAt = &now
    _ = uc.runRepo.Update(ctx, run)

    uc.logger.Info().
        Str("run_id", run.ID.String()).
        Int("exit_code", run.ExitCode).
        Int("formats", len(run.Artifacts)).
        Msg("report generation completed")
}

// generateFormat generates a specific report format.
func (uc *GenerateUseCase) generateFormat(ctx context.Context, format string, findings []*formatters.Finding) ([]byte, error) {
    switch format {
    case "csv":
        return uc.csvFmt.Format(ctx, findings)
    case "json":
        return uc.jsonFmt.Format(ctx, findings)
    case "excel":
        return uc.excelFmt.Format(ctx, findings)
    case "pdf":
        return uc.pdfFmt.Format(ctx, &pdf.ReportInput{
            HTMLContent: []byte{},
            GeneratedAt: time.Now().UTC(),
            Title:       "OpenVulnScan Vulnerability Report",
        })
    default:
        return nil, fmt.Errorf("unsupported format: %s", format)
    }
}

func (uc *GenerateUseCase) failRun(ctx context.Context, run *ReportRun, reason string) {
    run.Status = "failed"
    run.Error = reason
    _ = uc.runRepo.Update(ctx, run)
}
