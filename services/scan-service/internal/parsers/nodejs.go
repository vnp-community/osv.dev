package parsers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	parserentity "github.com/osv/scan-service/internal/parsers/entity"
)

// NodeJSParser handles package.json and package-lock.json.
type NodeJSParser struct{}

func (p *NodeJSParser) Supports(filename string) bool {
	base := filepath.Base(filename)
	return base == "package.json" || base == "package-lock.json"
}

func (p *NodeJSParser) Parse(ctx context.Context, filePath string) ([]parserentity.ProductInfo, error) {
	base := filepath.Base(filePath)
	if base == "package-lock.json" {
		return parsePackageLock(ctx, filePath)
	}
	return parsePackageJSON(ctx, filePath)
}

// package-lock.json v2/v3 format
type packageLock struct {
	LockfileVersion int `json:"lockfileVersion"`
	Packages        map[string]struct {
		Version string `json:"version"`
		Dev     bool   `json:"dev"`
	} `json:"packages"`
	// v1 format
	Dependencies map[string]struct {
		Version  string `json:"version"`
		Resolved string `json:"resolved"`
	} `json:"dependencies"`
}

func parsePackageLock(_ context.Context, filePath string) ([]parserentity.ProductInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("nodejs parser: %w", err)
	}
	var lock packageLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("nodejs parser: decode: %w", err)
	}

	var results []parserentity.ProductInfo

	// v2/v3: packages map
	for key, pkg := range lock.Packages {
		if key == "" || pkg.Version == "" {
			continue
		}
		// key format: "node_modules/express" or "node_modules/foo/node_modules/bar"
		name := strings.TrimPrefix(key, "node_modules/")
		if last := strings.LastIndex(name, "node_modules/"); last >= 0 {
			name = name[last+len("node_modules/"):]
		}
		name = strings.ToLower(name)
		results = append(results, parserentity.ProductInfo{
			Vendor:  "npm",
			Product: name,
			Version: pkg.Version,
			PURL:    fmt.Sprintf("pkg:npm/%s@%s", name, pkg.Version),
			Source:  "parser",
		})
	}

	// v1 fallback: dependencies map
	if len(results) == 0 {
		for name, dep := range lock.Dependencies {
			if dep.Version == "" {
				continue
			}
			pkg := strings.ToLower(name)
			results = append(results, parserentity.ProductInfo{
				Vendor:  "npm",
				Product: pkg,
				Version: dep.Version,
				PURL:    fmt.Sprintf("pkg:npm/%s@%s", pkg, dep.Version),
				Source:  "parser",
			})
		}
	}

	return results, nil
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func parsePackageJSON(_ context.Context, filePath string) ([]parserentity.ProductInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("nodejs parser: %w", err)
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("nodejs parser: decode: %w", err)
	}

	var results []parserentity.ProductInfo

	add := func(name, constraint string) {
		name = strings.ToLower(name)
		// Strip range prefix for npm semver
		version := cleanNPMVersion(constraint)
		if version == "" {
			return
		}
		results = append(results, parserentity.ProductInfo{
			Vendor:  "npm",
			Product: name,
			Version: version,
			PURL:    fmt.Sprintf("pkg:npm/%s@%s", name, version),
			Source:  "parser",
		})
	}

	for k, v := range pkg.Dependencies {
		add(k, v)
	}

	return results, nil
}

// cleanNPMVersion converts npm semver ranges to exact versions.
// "^4.18.2" → "4.18.2", "4.17.21" → "4.17.21", ">=1.0.0" → ""
func cleanNPMVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "*" || v == "latest" {
		return ""
	}
	// Strip leading ^ ~ = prefixes
	for strings.HasPrefix(v, "^") || strings.HasPrefix(v, "~") || strings.HasPrefix(v, "=") {
		v = v[1:]
	}
	// If still has range operators, skip
	if strings.ContainsAny(v, "><| ") {
		return ""
	}
	return v
}
