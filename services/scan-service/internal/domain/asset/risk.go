package asset

// FindingStats contains aggregated counts of active findings by severity
type FindingStats struct {
    Active   int // Total active findings
    Critical int
    High     int
    Medium   int
    Low      int
}

// computeRiskScore calculates a risk score 0.0–10.0 based on finding severity.
//
// Risk scoring:
//   - Any Critical finding → 10.0 (maximum risk)
//   - High findings: 8.0 base + up to 2.0 for volume
//   - Medium findings: 5.0 base + up to 3.0 for volume
//   - Low findings: up to 5.0 based on count
//   - No findings → 0.0
func computeRiskScore(stats FindingStats) float64 {
    if stats.Critical > 0 {
        return 10.0
    }

    if stats.High > 0 {
        extra := float64(minInt(stats.High, 5)) * 0.4
        return min(10.0, 8.0+extra)
    }

    if stats.Medium > 0 {
        extra := float64(minInt(stats.Medium, 5)) * 0.6
        return min(10.0, 5.0+extra)
    }

    if stats.Low > 0 {
        return min(5.0, float64(stats.Low)*1.0)
    }

    return 0.0
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
    if a < b {
        return a
    }
    return b
}

// minInt returns the minimum of two int values
func minInt(a, b int) int {
    if a < b {
        return a
    }
    return b
}
