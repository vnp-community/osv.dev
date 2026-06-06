// Package cpe provides CPE (Common Platform Enumeration) parsing and
// version range detection for the converter service.
// Ported from vulnfeeds/conversion/versions.go — ExtractVersionsFromCPEs, ParseCPE.
package cpe

import (
	"fmt"
	"slices"
	"strings"
)

// CPEString represents a parsed CPE 2.3 formatted string.
type CPEString struct {
	CPEVersion string
	Part       string // "a" = application, "o" = operating system, "h" = hardware
	Vendor     string
	Product    string
	Version    string
	Update     string
	Edition    string
	Language   string
	SWEdition  string
	TargetSW   string
	TargetHW   string
	Other      string
}

// VersionSource indicates where a version range was derived from.
type VersionSource int

const (
	VersionSourceCPERange  VersionSource = iota // from CPE versionStartIncluding/versionEndExcluding
	VersionSourceCPEString                      // from CPE version field directly
	VersionSourceText                           // from free-text description
)

func (vs VersionSource) String() string {
	switch vs {
	case VersionSourceCPERange:
		return "cpe_range"
	case VersionSourceCPEString:
		return "cpe_string"
	case VersionSourceText:
		return "text"
	default:
		return "unknown"
	}
}

// CPEMatch represents a CPE match node from NVD configuration data.
type CPEMatch struct {
	Criteria              string  // CPE 2.3 formatted string
	Vulnerable            bool
	VersionStartIncluding *string
	VersionStartExcluding *string
	VersionEndIncluding   *string
	VersionEndExcluding   *string
}

// ConfigNode represents a node in NVD configuration (AND/OR operator).
type ConfigNode struct {
	Operator string // "OR" or "AND"
	CPEMatch []CPEMatch
}

// Configuration represents an NVD CPE configuration.
type Configuration struct {
	Nodes []ConfigNode
}

// VersionRange represents a version range derived from CPE data.
type VersionRange struct {
	Introduced   string
	Fixed        string
	LastAffected string
	Source       VersionSource
	CPECriteria  string // raw CPE string that generated this range
	Vendor       string
	Product      string
}

// IsEmpty returns true if this range has no useful version information.
func (vr VersionRange) IsEmpty() bool {
	return vr.Introduced == "" && vr.Fixed == "" && vr.LastAffected == ""
}

// ParseCPE parses a well-formed CPE 2.3 formatted string.
// Format: cpe:2.3:part:vendor:product:version:update:edition:language:sw_edition:target_sw:target_hw:other
func ParseCPE(formattedString string) (*CPEString, error) {
	if !strings.HasPrefix(formattedString, "cpe:") {
		return nil, fmt.Errorf("%q does not have expected 'cpe:' prefix", formattedString)
	}

	parts := strings.Split(formattedString, ":")
	// CPE 2.3 has 13 components separated by ":"
	// cpe:2.3:part:vendor:product:version:update:edition:language:sw_edition:target_sw:target_hw:other
	if len(parts) < 13 {
		return nil, fmt.Errorf("CPE string %q has too few components: got %d, want 13", formattedString, len(parts))
	}

	return &CPEString{
		CPEVersion: parts[1],
		Part:       removeQuoting(parts[2]),
		Vendor:     removeQuoting(parts[3]),
		Product:    removeQuoting(parts[4]),
		Version:    removeQuoting(parts[5]),
		Update:     parts[6],
		Edition:    removeQuoting(parts[7]),
		Language:   removeQuoting(parts[8]),
		SWEdition:  removeQuoting(parts[9]),
		TargetSW:   removeQuoting(parts[10]),
		TargetHW:   removeQuoting(parts[11]),
		Other:      removeQuoting(parts[12]),
	}, nil
}

// removeQuoting handles CPE quoting rules per NISTIR 7695 §5.3.2.
func removeQuoting(s string) string {
	return strings.ReplaceAll(s, "\\", "")
}

// cleanVersion trims trailing punctuation from version strings.
func cleanVersion(version string) string {
	return strings.TrimRight(version, ":")
}

// hasVersion checks whether a version exists in the valid versions list.
// If validVersions is empty, all versions are considered valid.
func hasVersion(validVersions []string, version string) bool {
	if len(validVersions) == 0 {
		return true
	}
	return slices.Contains(validVersions, version)
}

// nextVersion returns the version immediately after the given version in the
// sorted validVersions list.
func nextVersion(validVersions []string, version string) (string, error) {
	idx := -1
	for i, v := range validVersions {
		if v == version {
			idx = i
			break
		}
	}
	if idx == -1 {
		return "", fmt.Errorf("version %q is not in valid versions list", version)
	}
	idx++
	if idx >= len(validVersions) {
		return "", fmt.Errorf("version %q has no successor in valid versions list", version)
	}
	return validVersions[idx], nil
}

// ExtractVersionRangesFromCPEs extracts OSV-format version ranges from NVD CPE
// configuration data. This is the core algorithm ported from
// vulnfeeds/conversion/versions.go:ExtractVersionsFromCPEs.
//
// Parameters:
//   - configs:       NVD configuration nodes (CPE match criteria)
//   - validVersions: sorted list of known-valid versions (optional; nil means accept all)
//
// Returns a slice of VersionRange structs suitable for building OSV Affected entries.
func ExtractVersionRangesFromCPEs(configs []Configuration, validVersions []string) ([]VersionRange, []string) {
	var ranges []VersionRange
	var notes []string

	for _, config := range configs {
		for _, node := range config.Nodes {
			if node.Operator != "OR" {
				// AND nodes are complex compound conditions — skip for now
				continue
			}

			for _, match := range node.CPEMatch {
				if !match.Vulnerable {
					continue
				}

				introduced := ""
				fixed := ""
				lastAffected := ""
				source := VersionSourceCPERange

				// Determine introduced version
				if match.VersionStartIncluding != nil {
					introduced = cleanVersion(*match.VersionStartIncluding)
				} else if match.VersionStartExcluding != nil {
					var err error
					introduced, err = nextVersion(validVersions, cleanVersion(*match.VersionStartExcluding))
					if err != nil {
						notes = append(notes, fmt.Sprintf("nextVersion(%q): %v", *match.VersionStartExcluding, err))
					}
				}

				// Determine fixed / last_affected version
				if match.VersionEndExcluding != nil {
					fixed = cleanVersion(*match.VersionEndExcluding)
				} else if match.VersionEndIncluding != nil {
					var err error
					fixed, err = nextVersion(validVersions, cleanVersion(*match.VersionEndIncluding))
					if err != nil {
						notes = append(notes, fmt.Sprintf("nextVersion(%q): %v", *match.VersionEndIncluding, err))
						// Cannot infer fixed → use last_affected instead
						lastAffected = cleanVersion(*match.VersionEndIncluding)
					}
				}

				cpe, err := ParseCPE(match.Criteria)
				if err != nil {
					notes = append(notes, fmt.Sprintf("ParseCPE(%q): %v", match.Criteria, err))
					continue
				}

				// If no range bounds found, try inferring last_affected from the CPE version field
				if introduced == "" && fixed == "" && lastAffected == "" {
					// Only application ("a") and OS ("o") parts are meaningful
					if cpe.Part != "a" && cpe.Part != "o" {
						continue
					}
					if slices.Contains([]string{"NA", "ANY", "*"}, strings.ToUpper(cpe.Version)) {
						continue
					}
					lastAffected = cpe.Version
					if cpe.Update != "ANY" && cpe.Update != "*" && cpe.Update != "" {
						lastAffected += "-" + cpe.Update
					}
					source = VersionSourceCPEString
				}

				// Still nothing useful
				if introduced == "" && fixed == "" && lastAffected == "" {
					continue
				}

				// Ensure introduced is set
				if introduced == "" {
					introduced = "0"
				}

				// Validate version strings against known valid versions
				if introduced != "0" && !hasVersion(validVersions, introduced) {
					notes = append(notes, fmt.Sprintf("warning: introduced version %q not in valid versions", introduced))
				}
				if fixed != "" && !hasVersion(validVersions, fixed) {
					notes = append(notes, fmt.Sprintf("warning: fixed version %q not in valid versions", fixed))
				}

				vr := VersionRange{
					Introduced:   introduced,
					Fixed:        fixed,
					LastAffected: lastAffected,
					Source:       source,
					CPECriteria:  match.Criteria,
					Vendor:       cpe.Vendor,
					Product:      cpe.Product,
				}
				ranges = append(ranges, vr)
			}
		}
	}

	return ranges, notes
}

// CPEsFromConfigurations extracts all CPE criteria strings from NVD configurations.
func CPEsFromConfigurations(configs []Configuration) []string {
	var cpes []string
	for _, config := range configs {
		for _, node := range config.Nodes {
			for _, match := range node.CPEMatch {
				cpes = append(cpes, match.Criteria)
			}
		}
	}
	return cpes
}

// VendorProduct represents a vendor+product pair extracted from a CPE.
type VendorProduct struct {
	Vendor  string
	Product string
}

// VendorProductDenyList lists vendor/product pairs known to cause false associations.
// Ported from vulnfeeds/conversion/versions.go:VendorProductDenyList.
var VendorProductDenyList = []VendorProduct{
	{Vendor: "netapp", Product: ""},
	{Vendor: "oracle", Product: "zfs_storage_appliance_kit"},
	{Vendor: "gradle", Product: "enterprise"},
	{Vendor: "qualcomm", Product: ""},
	{Vendor: "linux", Product: "linux_kernel"},
}

// IsDenied returns true if the vendor/product pair is in the deny list.
func IsDenied(vendor, product string) bool {
	for _, d := range VendorProductDenyList {
		if d.Vendor == vendor && (d.Product == "" || d.Product == product) {
			return true
		}
	}
	return false
}

// DeduplicateVersionRanges removes exact duplicate version ranges from the list.
func DeduplicateVersionRanges(ranges []VersionRange) []VersionRange {
	seen := make(map[string]bool)
	var result []VersionRange
	for _, vr := range ranges {
		key := fmt.Sprintf("%s|%s|%s|%s", vr.Introduced, vr.Fixed, vr.LastAffected, vr.CPECriteria)
		if !seen[key] {
			seen[key] = true
			result = append(result, vr)
		}
	}
	return result
}
