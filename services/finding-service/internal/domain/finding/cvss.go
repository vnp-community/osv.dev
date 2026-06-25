// Package finding — cvss.go
// CVSS score computation utilities for findings.
package finding

import (
	"regexp"
	"strings"
)

// cvssv3Regex validates CVSS v3.x vector format.
var cvssv3Regex = regexp.MustCompile(`^CVSS:3\.[01]/AV:[NALP]/AC:[LH]/PR:[NLH]/UI:[NR]/S:[UC]/C:[NLH]/I:[NLH]/A:[NLH]`)

// ValidateCVSSv3Vector returns true if the vector is a valid CVSS v3.x vector.
func ValidateCVSSv3Vector(vector string) bool {
	return cvssv3Regex.MatchString(vector)
}

// metricWeight maps CVSS v3 metric values to numeric weights.
// Source: CVSS v3.1 specification (https://www.first.org/cvss/specification-document)
var (
	avWeight = map[string]float64{"N": 0.85, "A": 0.62, "L": 0.55, "P": 0.20}
	acWeight = map[string]float64{"L": 0.77, "H": 0.44}
	prWeightU = map[string]float64{"N": 0.85, "L": 0.62, "H": 0.27}
	prWeightC = map[string]float64{"N": 0.85, "L": 0.68, "H": 0.50}
	uiWeight = map[string]float64{"N": 0.85, "R": 0.62}
	ciaWeight = map[string]float64{"N": 0.00, "L": 0.22, "H": 0.56}
)

// ComputeCVSSv3Score calculates a numerical CVSS v3.1 base score from a vector string.
// Vector format: CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H
// Returns 0.0 if the vector is invalid.
func ComputeCVSSv3Score(vector string) float64 {
	if !ValidateCVSSv3Vector(vector) {
		return 0
	}
	// Parse the vector into parts
	parts := strings.Split(vector, "/")
	if len(parts) < 9 {
		return 0
	}
	// parts[0] is "CVSS:3.1", parts[1..8] are AV, AC, PR, UI, S, C, I, A
	metrics := make(map[string]string)
	for _, p := range parts[1:] {
		kv := strings.SplitN(p, ":", 2)
		if len(kv) == 2 {
			metrics[kv[0]] = kv[1]
		}
	}

	av := avWeight[metrics["AV"]]
	ac := acWeight[metrics["AC"]]
	scope := metrics["S"]
	var pr float64
	if scope == "C" {
		pr = prWeightC[metrics["PR"]]
	} else {
		pr = prWeightU[metrics["PR"]]
	}
	ui := uiWeight[metrics["UI"]]
	conf := ciaWeight[metrics["C"]]
	integ := ciaWeight[metrics["I"]]
	avail := ciaWeight[metrics["A"]]

	// ISCBase
	iscBase := 1 - (1-conf)*(1-integ)*(1-avail)
	var impact float64
	if scope == "U" {
		impact = 6.42 * iscBase
	} else {
		impact = 7.52*(iscBase-0.029) - 3.25*pow(iscBase-0.02, 15)
	}

	exploitability := 8.22 * av * ac * pr * ui

	var baseScore float64
	if impact <= 0 {
		baseScore = 0
	} else if scope == "U" {
		baseScore = roundUp(min(impact+exploitability, 10))
	} else {
		baseScore = roundUp(min(1.08*(impact+exploitability), 10))
	}
	return baseScore
}

// ComputeSeverityFromCVSSv3Score maps a CVSS v3 score to a severity string.
func ComputeSeverityFromCVSSv3Score(score float64) string {
	switch {
	case score >= 9.0:
		return "Critical"
	case score >= 7.0:
		return "High"
	case score >= 4.0:
		return "Medium"
	case score >= 0.1:
		return "Low"
	default:
		return "Info"
	}
}

// ── Math helpers ──────────────────────────────────────────────────────────────

func roundUp(score float64) float64 {
	// Round up to 1 decimal place
	rounded := float64(int(score*10+0.999999)) / 10
	if rounded > 10 {
		return 10
	}
	return rounded
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// pow computes a^exp (integer exp, float base).
func pow(base float64, exp int) float64 {
	result := 1.0
	for i := 0; i < exp; i++ {
		result *= base
	}
	return result
}
