// Package sbom provides SPDX SBOM generation.
package sbom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SPDXTagValueGenerator generates SPDX-2.3 tag-value documents.
type SPDXTagValueGenerator struct{}

// NewSPDX creates an SPDXTagValueGenerator.
func NewSPDX() *SPDXTagValueGenerator { return &SPDXTagValueGenerator{} }

// Format returns "spdx".
func (g *SPDXTagValueGenerator) Format() string { return "spdx" }

// ContentType returns "text/spdx".
func (g *SPDXTagValueGenerator) ContentType() string { return "text/spdx" }

// Generate produces an SPDX-2.3 tag-value document.
func (g *SPDXTagValueGenerator) Generate(_ context.Context, results []ScanResult, opts SBOMOptions) ([]byte, error) {
	var buf bytes.Buffer
	ns := "https://cve-bin-tool.example.com/sbom/" + sanitize(opts.PackageName) + "-" + time.Now().Format("20060102")

	buf.WriteString("SPDXVersion: SPDX-2.3\n")
	buf.WriteString("DataLicense: CC0-1.0\n")
	buf.WriteString("SPDXID: SPDXRef-DOCUMENT\n")
	buf.WriteString("DocumentName: " + opts.PackageName + "\n")
	buf.WriteString("DocumentNamespace: " + ns + "\n")
	buf.WriteString("Creator: Tool: cve-bin-tool\n")
	buf.WriteString("Created: " + time.Now().UTC().Format(time.RFC3339) + "\n\n")

	for i, r := range results {
		spdxid := fmt.Sprintf("SPDXRef-Package-%d-%s", i, sanitize(r.Product))
		buf.WriteString("PackageName: " + r.Product + "\n")
		buf.WriteString("SPDXID: " + spdxid + "\n")
		buf.WriteString("PackageVersion: " + r.Version + "\n")
		if r.Vendor != "" {
			buf.WriteString("PackageSupplier: Organization: " + r.Vendor + "\n")
		}
		if r.CPE != "" {
			buf.WriteString("ExternalRef: SECURITY cpe23Type " + r.CPE + "\n")
		}
		if r.PURL != "" {
			buf.WriteString("ExternalRef: PACKAGE-MANAGER purl " + r.PURL + "\n")
		}
		buf.WriteString("FilesAnalyzed: false\n\n")
	}
	return buf.Bytes(), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// CycloneDX JSON Generator
// ─────────────────────────────────────────────────────────────────────────────

// cdxOutput is the CycloneDX JSON output structure.
type cdxOutput struct {
	BOMFormat    string       `json:"bomFormat"`
	SpecVersion  string       `json:"specVersion"`
	SerialNumber string       `json:"serialNumber"`
	Metadata     cdxOutMeta   `json:"metadata"`
	Components   []cdxOutComp `json:"components"`
}

type cdxOutMeta struct {
	Timestamp string      `json:"timestamp"`
	Tools     []cdxTool   `json:"tools"`
}

type cdxTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type cdxOutComp struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
	PURL    string `json:"purl,omitempty"`
	CPE     string `json:"cpe,omitempty"`
}

// CycloneDXGenerator generates CycloneDX 1.4 JSON SBOM documents.
type CycloneDXGenerator struct{}

// NewCycloneDX creates a CycloneDXGenerator.
func NewCycloneDX() *CycloneDXGenerator { return &CycloneDXGenerator{} }

// Format returns "cyclonedx".
func (g *CycloneDXGenerator) Format() string { return "cyclonedx" }

// ContentType returns "application/vnd.cyclonedx+json".
func (g *CycloneDXGenerator) ContentType() string { return "application/vnd.cyclonedx+json" }

// Generate produces a CycloneDX 1.4 JSON SBOM.
func (g *CycloneDXGenerator) Generate(_ context.Context, results []ScanResult, _ SBOMOptions) ([]byte, error) {
	comps := make([]cdxOutComp, 0, len(results))
	for _, r := range results {
		compType := r.Type
		if compType == "" {
			compType = "library"
		}
		comps = append(comps, cdxOutComp{
			Type:    compType,
			Name:    r.Product,
			Version: r.Version,
			PURL:    r.PURL,
			CPE:     r.CPE,
		})
	}

	out := cdxOutput{
		BOMFormat:    "CycloneDX",
		SpecVersion:  "1.4",
		SerialNumber: "urn:uuid:00000000-0000-0000-0000-000000000000",
		Metadata: cdxOutMeta{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Tools:     []cdxTool{{Name: "cve-bin-tool", Version: "2.0"}},
		},
		Components: comps,
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

// sanitize replaces non-alphanumeric chars with dashes.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
}
