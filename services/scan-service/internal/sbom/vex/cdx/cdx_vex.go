// Package cdx implements the CycloneDX VEX parser.
package cdx

import (
	"encoding/json"

	"github.com/osv/scan-service/internal/sbom/entity"
)

// cdxVEXDoc is the CycloneDX VEX document structure.
type cdxVEXDoc struct {
	BOMFormat       string         `json:"bomFormat"`
	Vulnerabilities []cdxVuln      `json:"vulnerabilities"`
}

type cdxVuln struct {
	ID       string       `json:"id"`
	Analysis *cdxAnalysis `json:"analysis"`
}

type cdxAnalysis struct {
	State         string   `json:"state"`
	Justification string   `json:"justification"`
	Response      []string `json:"response"`
	Detail        string   `json:"detail"`
}

// Parser parses CycloneDX VEX JSON documents.
type Parser struct{}

// New creates a CycloneDX VEX parser.
func New() *Parser { return &Parser{} }

// Format returns VEXFormatCycloneDX.
func (p *Parser) Format() entity.VEXFormat { return entity.VEXFormatCycloneDX }

// Parse parses CycloneDX VEX JSON content.
func (p *Parser) Parse(content []byte) (*entity.VEXDocument, error) {
	var raw cdxVEXDoc
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, err
	}

	doc := &entity.VEXDocument{Format: entity.VEXFormatCycloneDX}
	for _, v := range raw.Vulnerabilities {
		stmt := entity.VEXStatement{
			CVENumber: v.ID,
		}
		if v.Analysis != nil {
			stmt.Status = v.Analysis.State
			stmt.Justification = v.Analysis.Justification
			stmt.Comments = v.Analysis.Detail
			stmt.Response = v.Analysis.Response
		}
		doc.Statements = append(doc.Statements, stmt)
	}
	return doc, nil
}
