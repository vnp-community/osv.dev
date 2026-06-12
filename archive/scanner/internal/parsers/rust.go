package parsers

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/osv/scanner/internal/domain/entity"
)

// RustParser handles Cargo.lock and Cargo.toml.
type RustParser struct{}

func (p *RustParser) Supports(filename string) bool {
	base := filepath.Base(filename)
	return base == "Cargo.lock" || base == "Cargo.toml"
}

func (p *RustParser) Parse(ctx context.Context, filePath string) ([]entity.ProductInfo, error) {
	base := filepath.Base(filePath)
	if base == "Cargo.lock" {
		return parseCargoLock(ctx, filePath)
	}
	return parseCargoToml(ctx, filePath)
}

// parseCargoLock parses Cargo.lock (TOML-like, exact versions guaranteed).
// Format:
//
//	[[package]]
//	name = "openssl"
//	version = "0.10.55"
func parseCargoLock(_ context.Context, filePath string) ([]entity.ProductInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("rust parser: %w", err)
	}
	defer f.Close() //nolint:errcheck

	var results []entity.ProductInfo
	var curName, curVersion string

	flush := func() {
		if curName != "" && curVersion != "" {
			results = append(results, entity.ProductInfo{
				Vendor:  "crates-io",
				Product: strings.ToLower(curName),
				Version: curVersion,
				PURL:    fmt.Sprintf("pkg:cargo/%s@%s", curName, curVersion),
				Source:  "parser",
			})
		}
		curName, curVersion = "", ""
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[[package]]" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "name = ") {
			curName = strings.Trim(strings.TrimPrefix(line, "name = "), `"`)
		} else if strings.HasPrefix(line, "version = ") {
			curVersion = strings.Trim(strings.TrimPrefix(line, "version = "), `"`)
		}
	}
	flush()

	return results, scanner.Err()
}

// parseCargoToml parses [dependencies] in Cargo.toml (may have ranges).
func parseCargoToml(_ context.Context, filePath string) ([]entity.ProductInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("rust parser: %w", err)
	}
	defer f.Close() //nolint:errcheck

	var results []entity.ProductInfo
	inDeps := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[dependencies]" || line == "[dev-dependencies]" {
			inDeps = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inDeps = false
			continue
		}
		if !inDeps || line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Format: name = "1.2.3" OR name = { version = "1.2.3", ... }
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		valueStr := strings.TrimSpace(parts[1])

		version := ""
		if strings.HasPrefix(valueStr, `"`) {
			// Simple string version
			version = strings.Trim(valueStr, `"`)
		} else if strings.Contains(valueStr, `version`) {
			// Inline table: { version = "1.2.3" }
			if idx := strings.Index(valueStr, `version = "`); idx >= 0 {
				rest := valueStr[idx+len(`version = "`):]
				if end := strings.Index(rest, `"`); end >= 0 {
					version = rest[:end]
				}
			}
		}

		// Strip range specifiers
		version = strings.TrimLeft(version, "^~>=<")
		if version == "" || strings.ContainsAny(version, "><,") {
			continue
		}

		results = append(results, entity.ProductInfo{
			Vendor:  "crates-io",
			Product: strings.ToLower(name),
			Version: version,
			PURL:    fmt.Sprintf("pkg:cargo/%s@%s", name, version),
			Source:  "parser",
		})
	}

	return results, scanner.Err()
}
