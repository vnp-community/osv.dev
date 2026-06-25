// Package report defines the Report domain entity for finding-service.
package report

import (
	"context"
	"time"
)

// ReportFormat identifies the output file format.
type ReportFormat string

const (
	FormatPDF  ReportFormat = "pdf"
	FormatXLSX ReportFormat = "xlsx"
	FormatJSON ReportFormat = "json"
	FormatCSV  ReportFormat = "csv"
)

// ReportStatus tracks the report generation lifecycle.
type ReportStatus string

const (
	StatusPending    ReportStatus = "pending"
	StatusGenerating ReportStatus = "generating"
	StatusCompleted  ReportStatus = "completed"
	StatusFailed     ReportStatus = "failed"
)

// RetentionDays is how long completed reports are kept before deletion.
const RetentionDays = 30

// Report is the aggregate root for report generation requests.
type Report struct {
	ID      string
	ProductID string
	Title   string
	Format  ReportFormat
	Status  ReportStatus

	// Scope filters
	EngagementID *string
	TestID       *string
	Severities   []string
	ActiveOnly   bool

	// Storage
	StorageKey    string // MinIO object key: "reports/<productID>/<reportID>.pdf"
	FileSizeBytes int64

	// Lifecycle
	GeneratedBy  string     // user ID
	GeneratedAt  *time.Time
	ExpiresAt    *time.Time // 30 days after generation
	ErrorMessage string
	CreatedAt    time.Time
}

// Repository is the persistence interface for reports.
type Repository interface {
	Save(ctx context.Context, r *Report) error
	FindByID(ctx context.Context, id string) (*Report, error)
	ListByProduct(ctx context.Context, productID, userID string, limit, offset int) ([]*Report, int, error)
	Delete(ctx context.Context, id string) error
	DeleteExpired(ctx context.Context) (int, error) // returns count deleted
}
