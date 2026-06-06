package entity

import (
	"time"
	"github.com/google/uuid"
)

// Service represents a detected network service on an asset.
type Service struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Name     string `json:"name"`
	Product  string `json:"product"`
	Version  string `json:"version"`
}

// WebTechnology represents a detected web framework/library.
type WebTechnology struct {
	Name       string   `json:"name"`
	Version    string   `json:"version,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

// Tag is a label that can be attached to assets.
type Tag struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	Color     string    `db:"color"` // hex color e.g. "#FF0000"
	CreatedAt time.Time `db:"created_at"`
}

// Severity levels for vulnerabilities.
type Severity string
const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityNone     Severity = "none"
)

// Vulnerability represents a CVE linked to an asset.
type Vulnerability struct {
	ID           uuid.UUID  `db:"id"`
	AssetID      uuid.UUID  `db:"asset_id"`
	CVEID        string     `db:"cve_id"`
	Summary      string     `db:"summary"`
	Severity     Severity   `db:"severity"`
	CVSS         float64    `db:"cvss"`
	ScanID       uuid.UUID  `db:"scan_id"`
	DetectedAt   time.Time  `db:"detected_at"`
	RemediatedAt *time.Time `db:"remediated_at"`
}

// VulnSummary is an aggregated count of vulnerabilities by severity for an asset.
type VulnSummary struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// Asset is a discovered network host/device with services and metadata.
type Asset struct {
	ID            uuid.UUID         `db:"id"`
	IPAddress     string            `db:"ip_address"`
	Hostname      string            `db:"hostname"`
	OS            string            `db:"os"`
	MACAddress    string            `db:"mac_address"`
	Services      []Service         `db:"services"`
	WebTech       []WebTechnology   `db:"web_tech"`
	Labels        map[string]string `db:"labels"`
	Tags          []Tag             `db:"-"`
	LastScannedAt *time.Time        `db:"last_scanned_at"`
	CreatedAt     time.Time         `db:"created_at"`
	UpdatedAt     time.Time         `db:"updated_at"`
	VulnSummary   *VulnSummary      `db:"-"` // populated from cache/query
}
