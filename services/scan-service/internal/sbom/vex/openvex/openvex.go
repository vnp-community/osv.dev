// Package openvex implements the OpenVEX JSON parser.
package openvex

import (
	"encoding/json"

	"github.com/osv/scan-service/internal/sbom/entity"
)

// openvexDoc is the OpenVEX JSON document structure.
type openvexDoc struct {
	Context    string           `json:"@context"`
	ID         string           `json:"@id"`
	Statements []openvexStmt    `json:"statements"`
}

type openvexStmt struct {
	Vulnerability     openvexVuln   `json:"vulnerability"`
	Status            string        `json:"status"`
	Justification     string        `json:"justification"`
	ImpactStatement   string        `json:"impact_statement"`
	ActionStatement   string        `json:"action_statement"`
	Response          []string      `json:"response"`
}

type openvexVuln struct {
	Name string `json:"name"`
	ID   string `json:"@id"`
}

// Parser parses OpenVEX JSON documents.
type Parser struct{}

// New creates an OpenVEX parser.
func New() *Parser { return &Parser{} }

// Format returns VEXFormatOpenVEX.
func (p *Parser) Format() entity.VEXFormat { return entity.VEXFormatOpenVEX }

// Parse parses OpenVEX JSON content.
func (p *Parser) Parse(content []byte) (*entity.VEXDocument, error) {
	var raw openvexDoc
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, err
	}

	doc := &entity.VEXDocument{Format: entity.VEXFormatOpenVEX}
	for _, s := range raw.Statements {
		cveID := s.Vulnerability.Name
		if cveID == "" {
			cveID = s.Vulnerability.ID
		}
		comments := s.ImpactStatement
		if comments == "" {
			comments = s.ActionStatement
		}
		doc.Statements = append(doc.Statements, entity.VEXStatement{
			CVENumber:     cveID,
			Status:        s.Status,
			Justification: s.Justification,
			Comments:      comments,
			Response:      s.Response,
		})
	}
	return doc, nil
}
