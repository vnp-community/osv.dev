// Package swid provides an XXE-safe SWID XML SBOM parser.
package swid

import (
	"encoding/xml"

	"github.com/osv/sbomvex/internal/domain/entity"
	"github.com/osv/sbomvex/internal/infrastructure/validator"
)

// swidDoc represents a SWID tag-value XML document.
type swidDoc struct {
	XMLName xml.Name  `xml:"SoftwareIdentity"`
	Name    string    `xml:"name,attr"`
	Version string    `xml:"version,attr"`
	Tagid   string    `xml:"tagId,attr"`
	Entities []swidEntity `xml:"Entity"`
}

type swidEntity struct {
	Name string `xml:"name,attr"`
	Role string `xml:"role,attr"`
}

// XMLParser parses SWID XML SBOM files with XXE protection.
type XMLParser struct{}

// New creates a new SWID XMLParser.
func New() *XMLParser { return &XMLParser{} }

// Format returns SBOMFormatSWID.
func (p *XMLParser) Format() entity.SBOMFormat { return entity.SBOMFormatSWID }

// Parse parses SWID XML content safely (no XXE).
func (p *XMLParser) Parse(content []byte) (*entity.SBOMDocument, error) {
	var raw swidDoc
	if err := validator.SafeParseXML(content, &raw); err != nil {
		return nil, err
	}

	doc := &entity.SBOMDocument{
		Format:  entity.SBOMFormatSWID,
		Version: "1.0",
		Name:    raw.Name,
	}

	comp := entity.SBOMComponent{
		Name:    raw.Name,
		Version: raw.Version,
	}
	// Extract vendor from Entity with role=softwareCreator or tagCreator
	for _, e := range raw.Entities {
		if e.Role == "softwareCreator" || e.Role == "tagCreator" {
			comp.Vendor = e.Name
			break
		}
	}
	if comp.Name != "" {
		doc.Components = []entity.SBOMComponent{comp}
	}

	return doc, nil
}
