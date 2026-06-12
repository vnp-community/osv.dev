// Package parsers implements language-specific manifest file parsers.
// Each parser implements LanguageParser to detect products and versions.
package parsers

import (
	"context"

	"github.com/osv/scanner/internal/domain/entity"
)

// LanguageParser extracts product/version info from language manifest files.
type LanguageParser interface {
	// Supports returns true if this parser handles the given filename.
	Supports(filename string) bool
	// Parse reads the file and returns detected products.
	Parse(ctx context.Context, filePath string) ([]entity.ProductInfo, error)
}

// All returns all registered parsers.
func All() []LanguageParser {
	return []LanguageParser{
		&GolangParser{},
		&PythonParser{},
		&NodeJSParser{},
		&RustParser{},
		&JavaParser{},
		&RubyParser{},
		&ConanParser{},
	}
}

// FindParser returns the first parser that supports the given filename, or nil.
func FindParser(filename string) LanguageParser {
	for _, p := range All() {
		if p.Supports(filename) {
			return p
		}
	}
	return nil
}

// cleanVersion strips common range prefixes and returns an exact version.
// Returns "" if the version is a range that can't be made exact.
func cleanVersion(v string) string {
	if v == "" {
		return ""
	}
	// Remove common range operators
	for _, prefix := range []string{"==", ">=", "<=", "~=", "^", "~", ">", "<", "="} {
		if len(v) > len(prefix) && v[:len(prefix)] == prefix {
			v = v[len(prefix):]
		}
	}
	v = trimSpace(v)
	// If still contains range chars after stripping, skip
	for _, c := range ">,<~^" {
		for _, r := range v {
			if r == c {
				return ""
			}
		}
	}
	return v
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
