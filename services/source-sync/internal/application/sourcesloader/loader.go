// Package sourcesloader ports osv/sources.py — vulnerability file loading,
// parsing, writing, and source path utilities.
// TASK-10-02: Port osv/sources.py to Go.
package sourcesloader

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ---- Source ID Parsing (port of osv/sources.py:parse_source_id) ----

// SourceID represents a parsed source identifier.
type SourceID struct {
	Source string // e.g. "nvd", "github", "osv"
	ID     string // e.g. "CVE-2024-1234"
}

// ParseSourceID parses a source ID like "nvd:CVE-2024-1234".
func ParseSourceID(sourceID string) (SourceID, error) {
	parts := strings.SplitN(sourceID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return SourceID{}, fmt.Errorf("invalid source ID %q: expected format 'source:id'", sourceID)
	}
	return SourceID{Source: parts[0], ID: parts[1]}, nil
}

// String returns the canonical string representation of a SourceID.
func (s SourceID) String() string {
	return s.Source + ":" + s.ID
}

// ---- Vulnerability Loading (port of osv/sources.py:parse_vulnerability) ----

// ParsedVulnerability holds a raw OSV vulnerability document loaded from disk.
type ParsedVulnerability struct {
	// Raw is the parsed map representation.
	Raw map[string]interface{}
	// ID is the vulnerability identifier.
	ID string
	// ModifiedAt is the last modified timestamp.
	ModifiedAt time.Time
	// SourcePath is the file path the vulnerability was loaded from.
	SourcePath string
}

// ParseVulnerabilityFile loads an OSV vulnerability from a JSON or YAML file.
// Port of osv/sources.py:parse_vulnerability().
func ParseVulnerabilityFile(path string) (*ParsedVulnerability, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return ParseVulnerabilityBytes(data, path)
}

// ParseVulnerabilityBytes parses OSV vulnerability bytes (JSON or YAML).
// Port of osv/sources.py:parse_vulnerability_from_dict() / parse_vulnerabilities_from_data().
func ParseVulnerabilityBytes(data []byte, sourcePath string) (*ParsedVulnerability, error) {
	var raw map[string]interface{}

	// Try JSON first, fall back to YAML
	if err := json.Unmarshal(data, &raw); err != nil {
		if yamlErr := yaml.Unmarshal(data, &raw); yamlErr != nil {
			return nil, fmt.Errorf("parse %q: not valid JSON (%v) or YAML (%v)", sourcePath, err, yamlErr)
		}
	}

	id, _ := raw["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("vulnerability in %q has no 'id' field", sourcePath)
	}

	// Parse modified timestamp
	var modifiedAt time.Time
	if modStr, ok := raw["modified"].(string); ok && modStr != "" {
		if t, err := time.Parse(time.RFC3339, modStr); err == nil {
			modifiedAt = t
		}
	}

	return &ParsedVulnerability{
		Raw:        raw,
		ID:         id,
		ModifiedAt: modifiedAt,
		SourcePath: sourcePath,
	}, nil
}

// ParseVulnerabilitiesDir loads all OSV vulnerability files from a directory.
// Port of osv/sources.py:parse_vulnerabilities().
func ParseVulnerabilitiesDir(dir string) ([]*ParsedVulnerability, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", dir, err)
	}

	var vulns []*ParsedVulnerability
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") &&
			!strings.HasSuffix(name, ".yaml") &&
			!strings.HasSuffix(name, ".yml") {
			continue
		}
		v, err := ParseVulnerabilityFile(filepath.Join(dir, name))
		if err != nil {
			continue // non-fatal: skip bad files
		}
		vulns = append(vulns, v)
	}
	return vulns, nil
}

// ---- Vulnerability Writing (port of osv/sources.py:write_vulnerability) ----

// WriteVulnerabilityJSON writes an OSV vulnerability map as JSON to a file.
// Port of osv/sources.py:write_vulnerability() (JSON format).
func WriteVulnerabilityJSON(path string, vuln map[string]interface{}) error {
	data, err := json.MarshalIndent(vuln, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %q: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", filepath.Dir(path), err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// ---- Source Path Utilities (port of osv/sources.py:source_path) ----

// SourcePath returns the relative file path for a vulnerability.
// Port of osv/sources.py:source_path():
//   - CVE-YYYY-NNNN → YYYY/NxxxN/CVE-YYYY-NNNN.json
//   - Other IDs → PREFIX/ID.json
func SourcePath(vulnID string) string {
	if strings.HasPrefix(vulnID, "CVE-") {
		parts := strings.Split(vulnID, "-")
		if len(parts) == 3 {
			year := parts[1]
			seq := parts[2]
			bucket := cveSeqBucket(seq)
			return filepath.Join(year, bucket, vulnID+".json")
		}
	}
	parts := strings.SplitN(vulnID, "-", 2)
	if len(parts) >= 2 {
		return filepath.Join(strings.ToUpper(parts[0]), vulnID+".json")
	}
	return vulnID + ".json"
}

// cveSeqBucket returns the CVE sequence number bucket.
// e.g. "1234" → "1xxx", "12345" → "12xxx", "123" → "0xxx"
func cveSeqBucket(seq string) string {
	if len(seq) <= 3 {
		return "0xxx"
	}
	return seq[:len(seq)-3] + "xxx"
}

// ---- Checksum Utilities (port of osv/sources.py:sha256) ----

// SHA256File computes the SHA-256 hex digest of a file.
func SHA256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("sha256 read %q: %w", path, err)
	}
	return SHA256Bytes(data), nil
}

// SHA256Bytes computes the SHA-256 hex digest of a byte slice.
func SHA256Bytes(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// ---- Range Checking (port of osv/sources.py:vulnerability_has_range) ----

// VulnerabilityHasRange checks if a parsed vulnerability has a specific introduced+fixed range.
// Port of osv/sources.py:vulnerability_has_range().
func VulnerabilityHasRange(vuln map[string]interface{}, introduced, fixed string) bool {
	affected, ok := vuln["affected"].([]interface{})
	if !ok {
		return false
	}
	for _, aff := range affected {
		affMap, ok := aff.(map[string]interface{})
		if !ok {
			continue
		}
		ranges, ok := affMap["ranges"].([]interface{})
		if !ok {
			continue
		}
		for _, rng := range ranges {
			rngMap, ok := rng.(map[string]interface{})
			if !ok {
				continue
			}
			events, ok := rngMap["events"].([]interface{})
			if !ok {
				continue
			}
			var foundIntroduced, foundFixed bool
			for _, ev := range events {
				evMap, ok := ev.(map[string]interface{})
				if !ok {
					continue
				}
				if evMap["introduced"] == introduced {
					foundIntroduced = true
				}
				if evMap["fixed"] == fixed {
					foundFixed = true
				}
			}
			if foundIntroduced && foundFixed {
				return true
			}
		}
	}
	return false
}

// ---- Nested key access (port of osv/sources.py:_get_nested_vulnerability) ----

// GetNestedValue traverses a nested map using a dotted key path.
// Port of osv/sources.py:_get_nested_vulnerability().
// Example: GetNestedValue(data, "affected.0.package.name")
func GetNestedValue(data map[string]interface{}, keyPath string) (interface{}, bool) {
	if keyPath == "" {
		return data, true
	}
	keys := strings.SplitN(keyPath, ".", 2)
	val, ok := data[keys[0]]
	if !ok {
		return nil, false
	}
	if len(keys) == 1 {
		return val, true
	}
	nested, ok := val.(map[string]interface{})
	if !ok {
		return nil, false
	}
	return GetNestedValue(nested, keys[1])
}
