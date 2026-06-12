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

// PythonParser handles requirements.txt, setup.cfg, Pipfile.
type PythonParser struct{}

func (p *PythonParser) Supports(filename string) bool {
	base := filepath.Base(filename)
	return base == "requirements.txt" ||
		base == "requirements-dev.txt" ||
		base == "requirements-test.txt" ||
		base == "Pipfile" ||
		(strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt"))
}

func (p *PythonParser) Parse(ctx context.Context, filePath string) ([]entity.ProductInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("python parser: %w", err)
	}
	defer f.Close() //nolint:errcheck

	var results []entity.ProductInfo
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		line := strings.TrimSpace(scanner.Text())

		// Skip empty, comments, options
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}

		if pi, ok := parsePythonRequirement(line); ok {
			results = append(results, pi)
		}
	}

	return results, scanner.Err()
}

// parsePythonRequirement parses "Django==4.2.0" or "requests>=2.28.0"
func parsePythonRequirement(line string) (entity.ProductInfo, bool) {
	// Strip inline comment
	if idx := strings.Index(line, " #"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}

	// Split on first operator
	var pkg, version, op string
	for _, sep := range []string{"==", "~=", ">=", "<=", "!=", ">", "<"} {
		if idx := strings.Index(line, sep); idx > 0 {
			pkg = strings.TrimSpace(line[:idx])
			op = sep
			rest := strings.TrimSpace(line[idx+len(sep):])
			// Take first version if multiple constraints
			parts := strings.Split(rest, ",")
			version = strings.TrimSpace(parts[0])
			break
		}
	}

	if pkg == "" {
		pkg = line // no version constraint
	}
	pkg = strings.ToLower(pkg)

	// Only emit if exact version (== or ~= with full version)
	exactVersion := ""
	switch op {
	case "==":
		exactVersion = version
	case "~=":
		// compatible release: take as minimum
		exactVersion = version
	default:
		return entity.ProductInfo{}, false
	}

	if exactVersion == "" {
		return entity.ProductInfo{}, false
	}

	purl := fmt.Sprintf("pkg:pypi/%s@%s", pkg, exactVersion)
	return entity.ProductInfo{
		Vendor:  "pypi",
		Product: pkg,
		Version: exactVersion,
		PURL:    purl,
		Source:  "parser",
	}, true
}
