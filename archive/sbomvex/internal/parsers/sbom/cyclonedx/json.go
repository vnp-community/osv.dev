// Package cyclonedx provides a CycloneDX JSON SBOM parser.
package cyclonedx

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/osv/sbomvex/internal/domain/entity"
)

// cdxDoc is the CycloneDX JSON structure.
type cdxDoc struct {
	BOMFormat   string    `json:"bomFormat"`
	SpecVersion string    `json:"specVersion"`
	Metadata    *cdxMeta  `json:"metadata"`
	Components  []cdxComp `json:"components"`
}

type cdxMeta struct {
	Timestamp string `json:"timestamp"`
	Component *cdxComp `json:"component"`
}

type cdxComp struct {
	Type     string   `json:"type"`
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	PURL     string   `json:"purl"`
	CPE      string   `json:"cpe"`
	Supplier *struct {
		Name string `json:"name"`
	} `json:"supplier"`
}

// JSONParser parses CycloneDX JSON (bom.json) SBOM files.
type JSONParser struct{}

// New creates a new CycloneDX JSONParser.
func New() *JSONParser { return &JSONParser{} }

// Format returns SBOMFormatCycloneDX.
func (p *JSONParser) Format() entity.SBOMFormat { return entity.SBOMFormatCycloneDX }

// Parse parses CycloneDX JSON content.
func (p *JSONParser) Parse(content []byte) (*entity.SBOMDocument, error) {
	var raw cdxDoc
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, err
	}

	doc := &entity.SBOMDocument{
		Format:  entity.SBOMFormatCycloneDX,
		Version: raw.SpecVersion,
	}

	if raw.Metadata != nil {
		if raw.Metadata.Timestamp != "" {
			ts, _ := time.Parse(time.RFC3339, raw.Metadata.Timestamp)
			doc.Created = ts
		}
		if raw.Metadata.Component != nil {
			doc.Name = raw.Metadata.Component.Name
		}
	}

	for _, c := range raw.Components {
		comp := entity.SBOMComponent{
			Name:    c.Name,
			Version: c.Version,
			PURL:    c.PURL,
			CPE:     c.CPE,
			Type:    strings.ToLower(c.Type),
		}
		if c.Supplier != nil {
			comp.Vendor = c.Supplier.Name
		}
		doc.Components = append(doc.Components, comp)
	}
	return doc, nil
}
