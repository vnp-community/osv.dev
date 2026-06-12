// Package spdx provides SPDX tag-value and JSON SBOM parsers.
package spdx

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"

	"github.com/osv/sbomvex/internal/domain/entity"
)

// ─────────────────────────────────────────────────────────────────────────────
// SPDX Tag-Value Parser
// ─────────────────────────────────────────────────────────────────────────────

// TagValueParser parses SPDX 2.x tag-value (.spdx) files.
type TagValueParser struct{}

// NewTagValueParser creates a new TagValueParser.
func NewTagValueParser() *TagValueParser { return &TagValueParser{} }

// Format returns SBOMFormatSPDX.
func (p *TagValueParser) Format() entity.SBOMFormat { return entity.SBOMFormatSPDX }

// Parse parses SPDX tag-value content.
func (p *TagValueParser) Parse(content []byte) (*entity.SBOMDocument, error) {
	doc := &entity.SBOMDocument{Format: entity.SBOMFormatSPDX}
	var current *entity.SBOMComponent

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tag, value, ok := parseTagValue(line)
		if !ok {
			continue
		}

		switch tag {
		case "SPDXVersion":
			doc.Version = strings.TrimPrefix(value, "SPDX-")
		case "DocumentName":
			doc.Name = value
		case "PackageName":
			// Flush previous component
			if current != nil {
				doc.Components = append(doc.Components, *current)
			}
			current = &entity.SBOMComponent{Name: value}
		case "PackageVersion":
			if current != nil {
				current.Version = value
			}
		case "ExternalRef":
			if current != nil {
				parseExternalRef(value, current)
			}
		case "PackageSupplier":
			if current != nil && strings.HasPrefix(value, "Organization:") {
				current.Vendor = strings.TrimSpace(strings.TrimPrefix(value, "Organization:"))
			}
		}
	}
	// Flush last component
	if current != nil {
		doc.Components = append(doc.Components, *current)
	}
	return doc, scanner.Err()
}

// parseTagValue splits "Tag: Value" into tag and value.
func parseTagValue(line string) (tag, value string, ok bool) {
	idx := strings.IndexByte(line, ':')
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

// parseExternalRef parses SPDX ExternalRef values:
//
//	SECURITY cpe23Type cpe:2.3:a:openssl:openssl:1.1.1k:*:*:*:*:*:*:*
//	PACKAGE-MANAGER purl pkg:generic/openssl@1.1.1k
func parseExternalRef(value string, c *entity.SBOMComponent) {
	parts := strings.Fields(value)
	if len(parts) < 3 {
		return
	}
	ref := parts[2]
	switch {
	case strings.HasPrefix(ref, "cpe:"):
		c.CPE = ref
	case strings.HasPrefix(ref, "pkg:"):
		c.PURL = ref
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// SPDX JSON Parser
// ─────────────────────────────────────────────────────────────────────────────

// spdxJSON is the SPDX JSON document structure.
type spdxJSON struct {
	SpdxVersion string       `json:"spdxVersion"`
	Name        string       `json:"name"`
	Packages    []spdxPkg    `json:"packages"`
}

type spdxPkg struct {
	Name             string        `json:"name"`
	VersionInfo      string        `json:"versionInfo"`
	SPDXID           string        `json:"SPDXID"`
	Supplier         string        `json:"supplier"`
	ExternalRefs     []spdxExtRef  `json:"externalRefs"`
}

type spdxExtRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}

// JSONParser parses SPDX JSON (.spdx.json) files.
type JSONParser struct{}

// NewJSONParser creates a new SPDX JSONParser.
func NewJSONParser() *JSONParser { return &JSONParser{} }

// Format returns SBOMFormatSPDX.
func (p *JSONParser) Format() entity.SBOMFormat { return entity.SBOMFormatSPDX }

// Parse parses SPDX JSON content.
func (p *JSONParser) Parse(content []byte) (*entity.SBOMDocument, error) {
	var raw spdxJSON
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, err
	}

	doc := &entity.SBOMDocument{
		Format:  entity.SBOMFormatSPDX,
		Version: strings.TrimPrefix(raw.SpdxVersion, "SPDX-"),
		Name:    raw.Name,
	}

	for _, pkg := range raw.Packages {
		comp := entity.SBOMComponent{
			Name:    pkg.Name,
			Version: pkg.VersionInfo,
		}
		// Parse supplier
		if strings.HasPrefix(pkg.Supplier, "Organization:") {
			comp.Vendor = strings.TrimSpace(strings.TrimPrefix(pkg.Supplier, "Organization:"))
		}
		// Parse external refs
		for _, ref := range pkg.ExternalRefs {
			switch {
			case strings.HasPrefix(ref.ReferenceLocator, "cpe:"):
				comp.CPE = ref.ReferenceLocator
			case strings.HasPrefix(ref.ReferenceLocator, "pkg:"):
				comp.PURL = ref.ReferenceLocator
			}
		}
		doc.Components = append(doc.Components, comp)
	}
	return doc, nil
}
