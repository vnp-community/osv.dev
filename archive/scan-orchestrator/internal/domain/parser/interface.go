// Package parser defines the Parser interface for security tool report parsers.
// Each of the 150+ supported scan types implements this interface.
package parser

import (
	"context"
	"io"
)

// Parser parses a security tool report and returns normalized findings.
type Parser interface {
	// ScanType returns the unique identifier for this parser (e.g., "Trivy Scan").
	ScanType() string
	// GetFindings reads the scanner report and returns parsed, normalized findings.
	GetFindings(ctx context.Context, file io.Reader, test *TestContext) ([]*ParsedFinding, error)
}

// TestContext provides metadata about the test being imported.
type TestContext struct {
	TestID       string
	EngagementID string
	ProductID    string
	ScanType     string
	Options      *ParserOptions
}

// ParserOptions controls parser behavior.
type ParserOptions struct {
	MinimumSeverity string // "Info" | "Low" | "Medium" | "High" | "Critical"
	Service         string
}

// ParsedFinding is the normalized output from a parser before deduplication and storage.
type ParsedFinding struct {
	Title            string
	Description      string
	Mitigation       string
	Impact           string
	References       string
	Severity         string   // "Critical" | "High" | "Medium" | "Low" | "Info"
	CVE              string
	CWE              int
	VulnIDFromTool   string
	CVSSv3           string
	CVSSv3Score      *float64
	CVSSv4           string
	CVSSv4Score      *float64
	Active           bool
	Verified         bool
	FalsePositive    bool
	Duplicate        bool
	ComponentName    string
	ComponentVersion string
	FilePath         string
	LineNumber       int
	Service          string
	Tags             []string
	Endpoints        []string
	HashCode         string // populated by hash code computation after parsing
}
