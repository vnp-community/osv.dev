// Package sbom provides SBOM parser implementations.
package sbom

import "github.com/osv/sbomvex/internal/domain/entity"

// SBOMParser is the interface implemented by all SBOM format parsers.
type SBOMParser interface {
	// Parse parses the raw SBOM bytes and returns a structured SBOMDocument.
	Parse(content []byte) (*entity.SBOMDocument, error)
	// Format returns the SBOMFormat this parser handles.
	Format() entity.SBOMFormat
}
