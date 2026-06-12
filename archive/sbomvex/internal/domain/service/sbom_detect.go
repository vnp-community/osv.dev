// Package service contains domain services for sbomvex.
package service

import (
	"bytes"

	"github.com/osv/sbomvex/internal/domain/entity"
)

// SBOMDetectService auto-detects SBOM format from raw bytes.
type SBOMDetectService struct{}

// New creates a SBOMDetectService.
func New() *SBOMDetectService { return &SBOMDetectService{} }

// Detect returns the SBOMFormat of the given content.
// Detection order: CycloneDX → SPDX → SWID → Unknown.
func (s *SBOMDetectService) Detect(content []byte) entity.SBOMFormat {
	// CycloneDX JSON: must check before SPDX (no false positives)
	if bytes.Contains(content, []byte(`"bomFormat":"CycloneDX"`)) ||
		bytes.Contains(content, []byte(`"bomFormat": "CycloneDX"`)) {
		return entity.SBOMFormatCycloneDX
	}

	// SPDX tag-value or JSON
	if bytes.Contains(content, []byte("SPDXVersion:")) ||
		bytes.Contains(content, []byte(`"spdxVersion":`)) ||
		bytes.Contains(content, []byte("SPDXID:")) {
		return entity.SBOMFormatSPDX
	}

	// SWID XML
	if bytes.Contains(content, []byte("<SoftwareIdentity")) ||
		bytes.Contains(content, []byte("<SoftwareIdentity\n")) {
		return entity.SBOMFormatSWID
	}

	return entity.SBOMFormatUnknown
}
