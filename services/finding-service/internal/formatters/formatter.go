// Package formatters defines the Formatter interface for report output.
package formatters

import (
	"context"

	"github.com/osv/finding-service/internal/domain/entity"
)

// Formatter produces a report in a specific format.
type Formatter interface {
	// Format generates report bytes from the input data.
	Format(ctx context.Context, data entity.ReportInput, opts FormatOptions) ([]byte, error)
	// ContentType returns the MIME type of the output (e.g. "text/html").
	ContentType() string
	// FileExtension returns the recommended file extension (e.g. ".html").
	FileExtension() string
}

// FormatOptions contains optional rendering configuration.
type FormatOptions struct {
	Theme        string // "light" | "dark"
	StripScanDir string // prefix to strip from file paths
	Quiet        bool   // suppress informational output
}

// Registry maps OutputFormat → Formatter.
type Registry map[entity.OutputFormat]Formatter

// Get returns the formatter for a given format, or false if not found.
func (r Registry) Get(f entity.OutputFormat) (Formatter, bool) {
	fm, ok := r[f]
	return fm, ok
}
