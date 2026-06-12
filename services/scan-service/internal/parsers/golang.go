package parsers

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	parserentity "github.com/osv/scan-service/internal/parsers/entity"
)

// GolangParser parses go.mod files.
type GolangParser struct{}

func (p *GolangParser) Supports(filename string) bool {
	return filepath.Base(filename) == "go.mod"
}

func (p *GolangParser) Parse(ctx context.Context, filePath string) ([]parserentity.ProductInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("golang parser: %w", err)
	}
	defer f.Close() //nolint:errcheck

	var results []parserentity.ProductInfo
	inRequireBlock := false
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		line := strings.TrimSpace(scanner.Text())

		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}

		// Single-line require
		if strings.HasPrefix(line, "require ") {
			line = strings.TrimPrefix(line, "require ")
			line = strings.TrimSpace(line)
			if pi, ok := parseGoRequire(line); ok {
				results = append(results, pi)
			}
			continue
		}

		if inRequireBlock && line != "" && !strings.HasPrefix(line, "//") {
			if pi, ok := parseGoRequire(line); ok {
				results = append(results, pi)
			}
		}
	}

	return results, scanner.Err()
}

// parseGoRequire parses "github.com/user/repo v1.2.3" → ProductInfo
func parseGoRequire(line string) (parserentity.ProductInfo, bool) {
	// Remove inline comments
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return parserentity.ProductInfo{}, false
	}

	module := parts[0]
	version := strings.TrimPrefix(parts[1], "v")

	// Extract vendor (host/owner) and product (repo name)
	segments := strings.Split(module, "/")
	if len(segments) < 2 {
		return parserentity.ProductInfo{}, false
	}

	product := strings.ToLower(segments[len(segments)-1])
	vendor := strings.ToLower(strings.Join(segments[:len(segments)-1], "/"))

	purl := fmt.Sprintf("pkg:golang/%s@%s", module, version)

	return parserentity.ProductInfo{
		Vendor:  vendor,
		Product: product,
		Version: version,
		PURL:    purl,
		Source:  "parser",
	}, true
}
