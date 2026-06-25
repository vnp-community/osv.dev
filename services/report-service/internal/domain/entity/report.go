package entity

import (
    "time"

    "github.com/google/uuid"
)

// OutputFormat identifies the report output format
type OutputFormat string

const (
    FormatPDF     OutputFormat = "pdf"
    FormatHTML    OutputFormat = "html"
    FormatCSV     OutputFormat = "csv"
    FormatExcel   OutputFormat = "excel"
    FormatJSON    OutputFormat = "json"
    FormatConsole OutputFormat = "console"
)

// Theme controls the HTML report color scheme
type Theme string

const (
    ThemeLight Theme = "light"
    ThemeDark  Theme = "dark"
)

// ReportRunStatus tracks async report generation progress
type ReportRunStatus string

const (
    StatusPending    ReportRunStatus = "pending"
    StatusGenerating ReportRunStatus = "generating"
    StatusCompleted  ReportRunStatus = "completed"
    StatusFailed     ReportRunStatus = "failed"
)

// ReportInput is the input data for report generation
type ReportInput struct {
    ScanID      *uuid.UUID
    ProductID   *uuid.UUID
    ScanTarget  string
    GeneratedAt time.Time
    Theme       Theme
    MinSeverity string  // "critical" | "high" | "medium" | "low" | ""
    MinScore    *float64

    Findings []*ReportFinding
    Stats    ScanStats
    Products []*ProductSection
}

// ReportArtifact represents a generated report file
type ReportArtifact struct {
    Format      OutputFormat
    StoragePath string  // S3/MinIO path
    SizeBytes   int64
    ContentType string
    URL         string  // Presigned download URL
    URLExpiresAt *time.Time
}

// ReportRun represents an async report generation job
type ReportRun struct {
    ID          uuid.UUID
    ScanID      *uuid.UUID
    ProductID   *uuid.UUID
    Formats     []OutputFormat
    MinSeverity string
    MinScore    *float64
    Theme       Theme
    Status      ReportRunStatus
    ExitCode    *int  // 0=no issues, 1=issues found
    ErrorMsg    string
    CreatedBy   uuid.UUID
    CreatedAt   time.Time
    CompletedAt *time.Time
    Artifacts   []*ReportArtifact
}
