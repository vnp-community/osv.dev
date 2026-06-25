package formatters

import (
    "context"

    "github.com/google/osv.dev/services/report-service/internal/domain/entity"
)

// Formatter defines the interface for report format generators
type Formatter interface {
    // Format generates the report bytes for the given input
    Format(ctx context.Context, input *entity.ReportInput) ([]byte, error)

    // ContentType returns the MIME type for this format
    ContentType() string

    // FileExtension returns the file extension (without dot)
    FileExtension() string
}

// MeetsFilter checks if a finding passes the severity/score filter
func MeetsFilter(f *entity.ReportFinding, minSeverity string, minScore *float64) bool {
    // Score filter takes priority
    if minScore != nil && f.CVSSv3Score != nil && *f.CVSSv3Score < *minScore {
        return false
    }

    if minSeverity == "" {
        return true
    }

    severityOrder := map[string]int{
        "critical": 4,
        "high":     3,
        "medium":   2,
        "low":      1,
        "info":     0,
    }

    findingSeverityNum, _ := severityOrder[normalizeSeverity(f.Severity)]
    minSeverityNum, _ := severityOrder[normalizeSeverity(minSeverity)]

    return findingSeverityNum >= minSeverityNum
}

func normalizeSeverity(s string) string {
    switch {
    case len(s) == 0:
        return ""
    default:
        // Case-insensitive comparison
        r := []byte(s)
        for i, c := range r {
            if c >= 'A' && c <= 'Z' {
                r[i] = c + 32
            }
        }
        return string(r)
    }
}
