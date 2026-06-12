// Package csaf implements the CSAF VEX parser.
package csaf

import (
	"encoding/json"
	"strings"

	"github.com/osv/sbomvex/internal/domain/entity"
)

// csafDoc is the CSAF JSON structure.
type csafDoc struct {
	Vulnerabilities []csafVuln `json:"vulnerabilities"`
}

type csafVuln struct {
	CVE   string      `json:"cve"`
	Flags []csafFlag  `json:"flags"`
	Notes []csafNote  `json:"notes"`
}

type csafFlag struct {
	Label string `json:"label"`
}

type csafNote struct {
	Text     string `json:"text"`
	Category string `json:"category"`
}

// Parser parses CSAF VEX JSON documents.
type Parser struct{}

// New creates a CSAF VEX parser.
func New() *Parser { return &Parser{} }

// Format returns VEXFormatCSAF.
func (p *Parser) Format() entity.VEXFormat { return entity.VEXFormatCSAF }

// Parse parses CSAF VEX JSON content.
func (p *Parser) Parse(content []byte) (*entity.VEXDocument, error) {
	var raw csafDoc
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, err
	}

	doc := &entity.VEXDocument{Format: entity.VEXFormatCSAF}
	for _, v := range raw.Vulnerabilities {
		stmt := entity.VEXStatement{
			CVENumber: v.CVE,
		}
		// First flag label → justification / status
		if len(v.Flags) > 0 {
			label := v.Flags[0].Label
			stmt.Justification = label
			// Map common CSAF flags to VEX status
			stmt.Status = csafFlagToStatus(label)
		}
		// Collect notes text
		var notes []string
		for _, n := range v.Notes {
			if n.Text != "" {
				notes = append(notes, n.Text)
			}
		}
		stmt.Comments = strings.Join(notes, "; ")
		doc.Statements = append(doc.Statements, stmt)
	}
	return doc, nil
}

// csafFlagToStatus maps CSAF flag labels to VEX status strings.
func csafFlagToStatus(label string) string {
	switch label {
	case "component_not_present", "vulnerable_code_not_present",
		"vulnerable_code_not_in_execute_path", "inline_mitigations_already_exist":
		return "not_affected"
	case "vulnerable_code_cannot_be_controlled_by_adversary":
		return "not_affected"
	default:
		return "under_investigation"
	}
}
