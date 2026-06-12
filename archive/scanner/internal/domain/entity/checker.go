// Package entity contains core scanner domain entities.
package entity

import (
	"regexp"
	"strings"
)

// VendorProduct is a CPE vendor+product pair.
type VendorProduct struct {
	Vendor  string // lowercase CPE vendor (e.g. "openssl")
	Product string // lowercase CPE product (e.g. "openssl")
}

// Checker is a compiled binary checker for a specific software component.
// Instances are created via CheckerDef.Build() and are safe for concurrent use.
type Checker struct {
	name             string
	containsPatterns []*regexp.Regexp
	versionPatterns  []*regexp.Regexp
	filenamePatterns []*regexp.Regexp
	vendorProducts   []VendorProduct
	ignorePatterns   []*regexp.Regexp
}

// NewChecker creates a Checker with compiled patterns.
func NewChecker(
	name string,
	containsPatterns []*regexp.Regexp,
	versionPatterns []*regexp.Regexp,
	filenamePatterns []*regexp.Regexp,
	vendorProducts []VendorProduct,
	ignorePatterns []*regexp.Regexp,
) *Checker {
	return &Checker{
		name:             name,
		containsPatterns: containsPatterns,
		versionPatterns:  versionPatterns,
		filenamePatterns: filenamePatterns,
		vendorProducts:   vendorProducts,
		ignorePatterns:   ignorePatterns,
	}
}

// Name returns the checker's identifier.
func (c *Checker) Name() string { return c.name }

// VendorProducts returns the CPE vendor/product pairs this checker covers.
func (c *Checker) VendorProducts() []VendorProduct { return c.vendorProducts }

// MatchFilename returns true if the filename matches any filename pattern.
// Matching is case-insensitive (patterns should include (?i) flag).
func (c *Checker) MatchFilename(filename string) bool {
	base := filename
	// Use just the basename for matching
	if idx := strings.LastIndex(filename, "/"); idx >= 0 {
		base = filename[idx+1:]
	}
	for _, p := range c.filenamePatterns {
		if p.MatchString(base) {
			return true
		}
	}
	return false
}

// MatchContent returns true if the content satisfies containsPatterns
// AND none of the ignorePatterns match.
func (c *Checker) MatchContent(content string) bool {
	// Must match at least one contains pattern
	matched := false
	for _, p := range c.containsPatterns {
		if p.MatchString(content) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}

	// Must NOT match any ignore pattern
	for _, p := range c.ignorePatterns {
		if p.MatchString(content) {
			return false
		}
	}
	return true
}

// ExtractVersion extracts the version string from content using the first
// successful version pattern match. Returns "" if no pattern matches or
// the capture group is empty.
func (c *Checker) ExtractVersion(content string) string {
	for _, p := range c.versionPatterns {
		m := p.FindStringSubmatch(content)
		if len(m) >= 2 && m[1] != "" {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}
