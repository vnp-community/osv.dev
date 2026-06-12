// Package entity defines SBOM domain entities.
package entity

import (
	"strings"
	"time"
)

// SBOMFormat identifies an SBOM document format.
type SBOMFormat string

const (
	SBOMFormatSPDX      SBOMFormat = "spdx"
	SBOMFormatCycloneDX SBOMFormat = "cyclonedx"
	SBOMFormatSWID      SBOMFormat = "swid"
	SBOMFormatUnknown   SBOMFormat = "unknown"
)

// SBOMDocument is a parsed SBOM.
type SBOMDocument struct {
	Format     SBOMFormat
	Version    string
	Name       string
	Created    time.Time
	Components []SBOMComponent
}

// SBOMComponent is an individual software component from an SBOM.
type SBOMComponent struct {
	Name    string
	Version string
	PURL    string
	CPE     string
	Type    string // library|framework|application|os
	Vendor  string
}

// ProductInfo is the normalized product identity used downstream.
type ProductInfo struct {
	Vendor  string
	Product string
	Version string
	PURL    string
}

// ToProductInfo converts an SBOMComponent to a ProductInfo.
// Vendor is extracted from CPE, then PURL, then from the Vendor field.
func (c SBOMComponent) ToProductInfo() ProductInfo {
	vendor := c.Vendor
	if vendor == "" {
		vendor = extractVendorFromCPE(c.CPE)
	}
	if vendor == "" {
		vendor = extractVendorFromPURL(c.PURL)
	}
	return ProductInfo{
		Vendor:  strings.ToLower(vendor),
		Product: strings.ToLower(c.Name),
		Version: c.Version,
		PURL:    c.PURL,
	}
}

// extractVendorFromCPE parses cpe:2.3:a:{vendor}:{product}:... and returns vendor.
func extractVendorFromCPE(cpe string) string {
	// cpe:2.3:a:openssl:openssl:1.1.1k:...
	if cpe == "" {
		return ""
	}
	parts := strings.Split(cpe, ":")
	// cpe:2.3:a:{vendor}:{product}:...  →  index 3
	if len(parts) >= 5 && (parts[0] == "cpe" || parts[1] == "2.3") {
		v := parts[3]
		if v != "*" && v != "" {
			return v
		}
	}
	return ""
}

// extractVendorFromPURL parses pkg:{ecosystem}/{namespace}/{name}@... and returns namespace.
func extractVendorFromPURL(purl string) string {
	// pkg:generic/openssl@1.1.1k  or  pkg:npm/lodash@4.17.21  or  pkg:golang/golang.org/x/sys@v0.1.0
	if purl == "" {
		return ""
	}
	// Remove pkg: prefix
	rest := strings.TrimPrefix(purl, "pkg:")
	// Remove version (@...)
	if idx := strings.IndexByte(rest, '@'); idx >= 0 {
		rest = rest[:idx]
	}
	// Remove qualifiers (?...)
	if idx := strings.IndexByte(rest, '?'); idx >= 0 {
		rest = rest[:idx]
	}
	// Split: ecosystem/namespace/name
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) >= 2 {
		// parts[1] is namespace or name if no namespace
		ns := parts[1]
		if ns != "" {
			return ns
		}
	}
	return ""
}
