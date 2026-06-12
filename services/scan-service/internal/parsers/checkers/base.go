// Package checkers implements the checker framework: declarative definitions,
// pattern compilation, and the global auto-registration registry.
package checkers

import (
	"fmt"
	"regexp"
	"strings"

	parserentity "github.com/osv/scan-service/internal/parsers/entity"
)

// CheckerDef is a declarative, uncompiled checker definition.
// Populated by each checker file's init() function via Register().
type CheckerDef struct {
	// Name is the checker identifier (e.g. "openssl"). Must be unique.
	Name string

	// ContainsPatterns are regex patterns that must appear in file content.
	// At least one must match for the checker to trigger.
	ContainsPatterns []string

	// VersionPatterns are regex patterns with exactly one capture group
	// that extracts the version string.
	VersionPatterns []string

	// FilenamePatterns are regex patterns matched against the filename (basename).
	// If any matches, the content check is performed.
	FilenamePatterns []string

	// VendorProduct lists the CPE vendor/product pairs this checker detects.
	// All values must be lowercase.
	VendorProduct []parserentity.VendorProduct

	// IgnorePatterns are patterns that, if found in content, suppress the match.
	// Used to avoid false positives from source code comments and header files.
	IgnorePatterns []string
}

// Build compiles all regex patterns in the definition and returns a *parserentity.Checker.
// Returns an error if:
//   - Name is empty
//   - VendorProduct is empty
//   - Any vendor or product is not lowercase
//   - Any regex pattern is invalid
func (def CheckerDef) Build() (*parserentity.Checker, error) {
	if def.Name == "" {
		return nil, fmt.Errorf("checker name must not be empty")
	}
	if len(def.VendorProduct) == 0 {
		return nil, fmt.Errorf("checker %q: VendorProduct must not be empty", def.Name)
	}
	// Validate VendorProduct entries
	seen := make(map[string]struct{})
	for _, vp := range def.VendorProduct {
		if vp.Vendor != strings.ToLower(vp.Vendor) {
			return nil, fmt.Errorf("checker %q: vendor %q must be lowercase", def.Name, vp.Vendor)
		}
		if vp.Product != strings.ToLower(vp.Product) {
			return nil, fmt.Errorf("checker %q: product %q must be lowercase", def.Name, vp.Product)
		}
		key := vp.Vendor + "/" + vp.Product
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("checker %q: duplicate vendor/product %q", def.Name, key)
		}
		seen[key] = struct{}{}
	}

	containsRe, err := compileAll(def.Name, "ContainsPatterns", def.ContainsPatterns)
	if err != nil {
		return nil, err
	}
	versionRe, err := compileAll(def.Name, "VersionPatterns", def.VersionPatterns)
	if err != nil {
		return nil, err
	}
	filenameRe, err := compileAll(def.Name, "FilenamePatterns", def.FilenamePatterns)
	if err != nil {
		return nil, err
	}
	ignoreRe, err := compileAll(def.Name, "IgnorePatterns", def.IgnorePatterns)
	if err != nil {
		return nil, err
	}

	return parserentity.NewChecker(def.Name, containsRe, versionRe, filenameRe, def.VendorProduct, ignoreRe), nil
}

// compileAll compiles a list of regex patterns.
func compileAll(checkerName, fieldName string, patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("checker %q: %s: invalid regex %q: %w", checkerName, fieldName, p, err)
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}
