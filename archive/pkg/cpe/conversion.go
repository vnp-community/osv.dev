// Package cpe provides CPE 2.2 URI ↔ CPE 2.3 Formatted String (FS) conversion.
// Ported from Python lib/cpe_conversion.py of the cve-search project.
//
// CPE format reference:
//   - CPE 2.2 URI: cpe:/a:apache:log4j:2.14.1
//   - CPE 2.3 FS:  cpe:2.3:a:apache:log4j:2.14.1:*:*:*:*:*:*:*
package cpe

import (
	"fmt"
	"net/url"
	"strings"
)

const wildcard = "*"

// WFN (Well-Formed Name) represents a parsed CPE attribute set.
// Fields are stored in their logical form (e.g. "*" for wildcard, "-" for N/A).
type WFN struct {
	Part      string // "a" (application), "o" (OS), "h" (hardware)
	Vendor    string
	Product   string
	Version   string
	Update    string
	Edition   string
	Language  string
	SWEdition string
	TargetSW  string
	TargetHW  string
	Other     string
}

// ParseFS parses a CPE 2.3 Formatted String into a WFN.
// Example: "cpe:2.3:a:apache:log4j:2.14.1:*:*:*:*:*:*:*"
func ParseFS(cpeStr string) (*WFN, error) {
	parts := splitFS(cpeStr)
	if len(parts) < 13 {
		return nil, fmt.Errorf("CPE 2.3 FS must have 13 components, got %d in %q", len(parts), cpeStr)
	}
	if parts[0] != "cpe" || parts[1] != "2.3" {
		return nil, fmt.Errorf("not a CPE 2.3 FS (must start with 'cpe:2.3:'): %q", cpeStr)
	}
	return &WFN{
		Part:      normalizeFS(parts[2]),
		Vendor:    normalizeFS(parts[3]),
		Product:   normalizeFS(parts[4]),
		Version:   normalizeFS(parts[5]),
		Update:    normalizeFS(parts[6]),
		Edition:   normalizeFS(parts[7]),
		Language:  normalizeFS(parts[8]),
		SWEdition: normalizeFS(parts[9]),
		TargetSW:  normalizeFS(parts[10]),
		TargetHW:  normalizeFS(parts[11]),
		Other:     normalizeFS(parts[12]),
	}, nil
}

// ToFS converts a WFN to a CPE 2.3 Formatted String.
// Empty string fields are treated as wildcard "*".
func (w *WFN) ToFS() string {
	fields := []string{
		w.Part, w.Vendor, w.Product, w.Version, w.Update,
		w.Edition, w.Language, w.SWEdition, w.TargetSW, w.TargetHW, w.Other,
	}
	for i, f := range fields {
		if f == "" {
			fields[i] = wildcard
		}
	}
	return "cpe:2.3:" + strings.Join(fields, ":")
}

// ParseURI parses a CPE 2.2 URI into a WFN.
// Example: "cpe:/a:apache:log4j:2.14.1"
func ParseURI(uri string) (*WFN, error) {
	uri = strings.TrimPrefix(uri, "cpe:/")
	parts := strings.Split(uri, ":")
	// Pad to 7 components: part vendor product version update edition language
	for len(parts) < 7 {
		parts = append(parts, "")
	}

	w := &WFN{
		Part:    pctDecode(parts[0]),
		Vendor:  pctDecode(parts[1]),
		Product: pctDecode(parts[2]),
		Version: pctDecode(parts[3]),
		Update:  pctDecode(parts[4]),
	}

	// Edition may be packed: "~edition~sw_edition~target_sw~target_hw~other"
	edition := pctDecode(parts[5])
	if strings.HasPrefix(edition, "~") {
		packed := strings.Split(edition[1:], "~")
		for len(packed) < 5 {
			packed = append(packed, "")
		}
		w.Edition   = packed[0]
		w.SWEdition = packed[1]
		w.TargetSW  = packed[2]
		w.TargetHW  = packed[3]
		w.Other     = packed[4]
	} else {
		w.Edition = edition
	}

	if len(parts) > 6 {
		w.Language = pctDecode(parts[6])
	}
	return w, nil
}

// ToURI converts a WFN to a CPE 2.2 URI string.
// Extended fields (SWEdition, TargetSW, TargetHW, Other) are packed into Edition
// only when they carry non-wildcard, non-empty values.
func (w *WFN) ToURI() string {
	// Helper: treat "*" and "" as empty (not set)
	notSet := func(s string) bool { return s == "" || s == wildcard }

	edition := w.Edition
	if edition == wildcard {
		edition = ""
	}

	// Pack extended fields only when any carry a real value
	if !notSet(w.SWEdition) || !notSet(w.TargetSW) || !notSet(w.TargetHW) || !notSet(w.Other) {
		ed := edition
		sw := w.SWEdition
		tw := w.TargetSW
		th := w.TargetHW
		ot := w.Other
		// Normalize wildcards to empty for packing
		for _, p := range []*string{&sw, &tw, &th, &ot} {
			if *p == wildcard {
				*p = ""
			}
		}
		edition = fmt.Sprintf("~%s~%s~%s~%s~%s", ed, sw, tw, th, ot)
	}

	// Normalize wildcard components to empty string for URI encoding
	norm := func(s string) string {
		if s == wildcard {
			return ""
		}
		return s
	}

	components := []string{
		norm(w.Part), norm(w.Vendor), norm(w.Product),
		norm(w.Version), norm(w.Update), edition, norm(w.Language),
	}
	encoded := make([]string, len(components))
	for i, c := range components {
		encoded[i] = pctEncode(c)
	}
	// Trim trailing empty colons (e.g. "::::")
	joined := strings.TrimRight(strings.Join(encoded, ":"), ":")
	return "cpe:/" + joined
}


// URIToFS converts a CPE 2.2 URI to a CPE 2.3 Formatted String.
func URIToFS(uri string) (string, error) {
	w, err := ParseURI(uri)
	if err != nil {
		return "", err
	}
	return w.ToFS(), nil
}

// FSToURI converts a CPE 2.3 Formatted String to a CPE 2.2 URI.
func FSToURI(fs string) (string, error) {
	w, err := ParseFS(fs)
	if err != nil {
		return "", err
	}
	return w.ToURI(), nil
}

// VendorProduct extracts (vendor, product) from any CPE string (2.2 or 2.3).
// Returns empty strings if parsing fails.
func VendorProduct(cpeStr string) (vendor, product string) {
	if strings.HasPrefix(cpeStr, "cpe:2.3:") {
		parts := splitFS(cpeStr)
		if len(parts) >= 5 {
			return normalizeFS(parts[3]), normalizeFS(parts[4])
		}
	} else if strings.HasPrefix(cpeStr, "cpe:/") {
		raw := strings.TrimPrefix(cpeStr, "cpe:/")
		// Skip the part prefix (e.g. "a:")
		if len(raw) > 2 {
			raw = raw[2:]
		}
		parts := strings.SplitN(raw, ":", 3)
		if len(parts) >= 2 {
			return pctDecode(parts[0]), pctDecode(parts[1])
		}
	}
	return "", ""
}

// IsFS returns true if s is a CPE 2.3 Formatted String.
func IsFS(s string) bool { return strings.HasPrefix(s, "cpe:2.3:") }

// IsURI returns true if s is a CPE 2.2 URI.
func IsURI(s string) bool { return strings.HasPrefix(s, "cpe:/") }

// ── helpers ───────────────────────────────────────────────────────────────────

// splitFS splits a CPE 2.3 FS on ":" while handling the fact that colons may
// appear legitimately within version strings. Since CPE 2.3 has exactly 13
// fixed components, we split on the first 12 colons only.
func splitFS(cpe string) []string {
	// A CPE 2.3 FS has exactly 13 components: "cpe", "2.3", then 11 attribute fields.
	// We split at most 12 times to produce 13 parts, preserving colons in the last field.
	parts := strings.SplitN(cpe, ":", 13)
	return parts
}

// normalizeFS converts a CPE 2.3 component to its logical value.
// "*" means any (wildcard), "-" means not applicable.
func normalizeFS(s string) string {
	// Unescape backslash-escaped colons in CPE 2.3 FS
	return strings.ReplaceAll(s, `\:`, ":")
}

// pctDecode applies URL percent-decoding for CPE 2.2 URI components.
func pctDecode(s string) string {
	if s == "" || s == wildcard || s == "-" {
		return s
	}
	decoded, err := url.QueryUnescape(s)
	if err != nil {
		return s // return as-is on decode error
	}
	return decoded
}

// pctEncode applies URL percent-encoding for CPE 2.2 URI components.
func pctEncode(s string) string {
	if s == wildcard || s == "" || s == "-" {
		return s
	}
	return url.QueryEscape(s)
}
