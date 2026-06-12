package parsers

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	parserentity "github.com/osv/scan-service/internal/parsers/entity"
)

// JavaParser handles pom.xml and build.gradle.
type JavaParser struct{}

func (p *JavaParser) Supports(filename string) bool {
	base := filepath.Base(filename)
	return base == "pom.xml" || base == "build.gradle" || base == "build.gradle.kts"
}

func (p *JavaParser) Parse(ctx context.Context, filePath string) ([]parserentity.ProductInfo, error) {
	base := filepath.Base(filePath)
	if base == "pom.xml" {
		return parsePOM(ctx, filePath)
	}
	return parseGradle(ctx, filePath)
}

// Maven POM XML structures
type mavenPOM struct {
	GroupID    string        `xml:"groupId"`
	ArtifactID string        `xml:"artifactId"`
	Version    string        `xml:"version"`
	Parent     mavenParent   `xml:"parent"`
	Deps       []mavenDep    `xml:"dependencies>dependency"`
	DepMgmt    []mavenDep    `xml:"dependencyManagement>dependencies>dependency"`
}

type mavenParent struct {
	GroupID string `xml:"groupId"`
	Version string `xml:"version"`
}

type mavenDep struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
}

func parsePOM(_ context.Context, filePath string) ([]parserentity.ProductInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("java parser: %w", err)
	}

	var pom mavenPOM
	if err := xml.Unmarshal(data, &pom); err != nil {
		// Empty or malformed file → return no results without error
		if len(data) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("java parser: xml: %w", err)
	}

	var results []parserentity.ProductInfo

	addDep := func(groupID, artifactID, version string) {
		if groupID == "" || artifactID == "" || version == "" {
			return
		}
		// Skip property placeholders like ${project.version}
		if strings.Contains(version, "${") {
			return
		}
		// Skip ranges
		if strings.ContainsAny(version, "[](,") {
			return
		}
		groupID = strings.ToLower(groupID)
		artifactID = strings.ToLower(artifactID)
		purl := fmt.Sprintf("pkg:maven/%s/%s@%s", groupID, artifactID, version)
		results = append(results, parserentity.ProductInfo{
			Vendor:  groupID,
			Product: artifactID,
			Version: version,
			PURL:    purl,
			Source:  "parser",
		})
	}

	for _, dep := range pom.Deps {
		addDep(dep.GroupID, dep.ArtifactID, dep.Version)
	}
	for _, dep := range pom.DepMgmt {
		addDep(dep.GroupID, dep.ArtifactID, dep.Version)
	}

	return results, nil
}

// gradleDepRegex matches: implementation 'com.foo:bar:1.2.3' or implementation "com.foo:bar:1.2.3"
var gradleDepRegex = regexp.MustCompile(`(?:implementation|api|compile|runtimeOnly|testImplementation)\s+['"]([^'"]+):([^'"]+):([^'"]+)['"]`)

func parseGradle(_ context.Context, filePath string) ([]parserentity.ProductInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("java parser: %w", err)
	}

	var results []parserentity.ProductInfo
	matches := gradleDepRegex.FindAllStringSubmatch(string(data), -1)
	for _, m := range matches {
		if len(m) != 4 {
			continue
		}
		groupID := strings.ToLower(m[1])
		artifactID := strings.ToLower(m[2])
		version := m[3]
		if strings.ContainsAny(version, "+,[]()") {
			continue
		}
		purl := fmt.Sprintf("pkg:maven/%s/%s@%s", groupID, artifactID, version)
		results = append(results, parserentity.ProductInfo{
			Vendor:  groupID,
			Product: artifactID,
			Version: version,
			PURL:    purl,
			Source:  "parser",
		})
	}

	return results, nil
}

// RubyParser handles Gemfile.lock.
type RubyParser struct{}

func (p *RubyParser) Supports(filename string) bool {
	return filepath.Base(filename) == "Gemfile.lock"
}

func (p *RubyParser) Parse(ctx context.Context, filePath string) ([]parserentity.ProductInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("ruby parser: %w", err)
	}

	// Gemfile.lock format:
	// GEM
	//   specs:
	//     rails (7.0.4)
	//     rake (13.0.6)
	gemSpecRegex := regexp.MustCompile(`^\s{4}(\S+)\s+\(([\d.]+)\)`)
	var results []parserentity.ProductInfo

	for _, line := range strings.Split(string(data), "\n") {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		m := gemSpecRegex.FindStringSubmatch(line)
		if len(m) == 3 {
			name := strings.ToLower(m[1])
			version := m[2]
			results = append(results, parserentity.ProductInfo{
				Vendor:  "rubygems",
				Product: name,
				Version: version,
				PURL:    fmt.Sprintf("pkg:gem/%s@%s", name, version),
				Source:  "parser",
			})
		}
	}

	return results, nil
}

// ConanParser handles conanfile.txt (C/C++ package manager).
type ConanParser struct{}

func (p *ConanParser) Supports(filename string) bool {
	base := filepath.Base(filename)
	return base == "conanfile.txt" || base == "conanfile.py"
}

func (p *ConanParser) Parse(ctx context.Context, filePath string) ([]parserentity.ProductInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("conan parser: %w", err)
	}

	// conanfile.txt format:
	// [requires]
	// openssl/3.0.7
	// zlib/1.2.13
	conanDepRegex := regexp.MustCompile(`^([a-zA-Z0-9_\-]+)/([\d.]+)`)
	inRequires := false
	var results []parserentity.ProductInfo

	for _, line := range strings.Split(string(data), "\n") {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		line = strings.TrimSpace(line)
		if line == "[requires]" {
			inRequires = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inRequires = false
			continue
		}
		if !inRequires || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := conanDepRegex.FindStringSubmatch(line)
		if len(m) == 3 {
			name := strings.ToLower(m[1])
			version := m[2]
			results = append(results, parserentity.ProductInfo{
				Vendor:  "conan",
				Product: name,
				Version: version,
				PURL:    fmt.Sprintf("pkg:conan/%s@%s", name, version),
				Source:  "parser",
			})
		}
	}

	return results, nil
}
