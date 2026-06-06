// Package sbom provides SBOM generator implementations.
package sbom

import (
	"context"

	"github.com/osv/sbomvex/internal/domain/entity"
)

// ScanResult represents a component found during a scan.
type ScanResult struct {
	Vendor  string
	Product string
	Version string
	PURL    string
	CPE     string
	Type    string
}

// SBOMOptions configures SBOM generation.
type SBOMOptions struct {
	PackageName     string
	PackageVersion  string
	PackageSupplier string
	StripScanDir    string
	SubFormat       string // for SPDX: "tv"|"json"
}

// SBOMGenerator is the interface for SBOM generators.
type SBOMGenerator interface {
	// Generate creates an SBOM document from a list of scan results.
	Generate(ctx context.Context, results []ScanResult, opts SBOMOptions) ([]byte, error)
	// Format returns the primary SBOM format name.
	Format() string
	// ContentType returns the MIME type of the output.
	ContentType() string
}

// ToSBOMComponent converts a ScanResult to an entity.SBOMComponent.
func ToSBOMComponent(r ScanResult) entity.SBOMComponent {
	return entity.SBOMComponent{
		Name:    r.Product,
		Version: r.Version,
		PURL:    r.PURL,
		CPE:     r.CPE,
		Type:    r.Type,
		Vendor:  r.Vendor,
	}
}
