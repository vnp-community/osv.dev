package entity

// MetricID identifies which metric type is stored.
type MetricID int

const (
	MetricUnknown MetricID = 0
	MetricEPSS    MetricID = 1 // Exploit Prediction Scoring System
	MetricCVSS2   MetricID = 2 // CVSS v2
	MetricCVSS3   MetricID = 3 // CVSS v3
)

// CVEMetric holds a single metric value for a CVE.
// Used primarily for EPSS (probability + percentile).
type CVEMetric struct {
	CVENumber   string   `db:"cve_number"`
	MetricID    MetricID `db:"metric_id"`
	MetricScore float64  `db:"metric_score"` // EPSS probability (0.0–1.0)
	MetricField string   `db:"metric_field"` // EPSS percentile as string (0.0–1.0)
}

// EPSSData is a convenience struct for EPSS probability and percentile.
type EPSSData struct {
	Probability float64
	Percentile  float64
}
