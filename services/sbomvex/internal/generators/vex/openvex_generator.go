// Package vex provides VEX generator implementations.
package vex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/osv/sbomvex/internal/domain/entity"
)

// ProductCVEs is a product with its associated CVEs.
type ProductCVEs struct {
	Vendor  string
	Product string
	Version string
	CVEs    []CVEEntry
}

// CVEEntry is a single CVE finding.
type CVEEntry struct {
	CVENumber  string
	Severity   string
	IsExploit  bool
	DataSource string
}

// VEXOptions configures VEX generation.
type VEXOptions struct {
	Format    string // "openvex"|"cyclonedx"|"csaf"
	Author    string
	ProductID string
}

// VEXGenerator is the interface for VEX generators.
type VEXGenerator interface {
	Generate(ctx context.Context, data []ProductCVEs, opts VEXOptions) ([]byte, error)
	Format() string
}

// ─────────────────────────────────────────────────────────────────────────────
// OpenVEX Generator
// ─────────────────────────────────────────────────────────────────────────────

// openVEXOut is the OpenVEX JSON output structure.
type openVEXOut struct {
	Context    string         `json:"@context"`
	ID         string         `json:"@id"`
	Author     string         `json:"author,omitempty"`
	Timestamp  string         `json:"timestamp"`
	Statements []openVEXStmt  `json:"statements"`
}

type openVEXStmt struct {
	Vulnerability openVEXVuln `json:"vulnerability"`
	Status        string      `json:"status"`
	Justification string      `json:"justification,omitempty"`
}

type openVEXVuln struct {
	Name string `json:"name"`
}

// OpenVEXGenerator generates OpenVEX JSON documents.
type OpenVEXGenerator struct{}

// New creates an OpenVEXGenerator.
func New() *OpenVEXGenerator { return &OpenVEXGenerator{} }

// Format returns "openvex".
func (g *OpenVEXGenerator) Format() string { return "openvex" }

// Generate produces an OpenVEX JSON document.
func (g *OpenVEXGenerator) Generate(_ context.Context, data []ProductCVEs, opts VEXOptions) ([]byte, error) {
	stmts := make([]openVEXStmt, 0)
	for _, pc := range data {
		for _, cve := range pc.CVEs {
			stmts = append(stmts, openVEXStmt{
				Vulnerability: openVEXVuln{Name: cve.CVENumber},
				Status:        "under_investigation",
			})
		}
	}

	productID := opts.ProductID
	if productID == "" {
		productID = fmt.Sprintf("https://cve-bin-tool.example.com/vex/%d", time.Now().UnixNano())
	}

	out := openVEXOut{
		Context:    "https://openvex.dev/ns/v0.2.0",
		ID:         productID,
		Author:     opts.Author,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Statements: stmts,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// ToTriageData converts entity.VEXDocument → TriageData (used by parsers).
func ToTriageData(doc *entity.VEXDocument) map[string]entity.TriageEntry {
	return doc.ToTriageData()
}
