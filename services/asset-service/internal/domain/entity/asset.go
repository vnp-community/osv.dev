// Package entity defines asset domain entities for asset-service.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// AssetStatus represents the lifecycle state of an asset.
type AssetStatus string

const (
	AssetStatusActive           AssetStatus = "active"
	AssetStatusInactive         AssetStatus = "inactive"
	AssetStatusDecommissioned   AssetStatus = "decommissioned"
)

// ServicePort describes a network service detected on an asset.
type ServicePort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // tcp|udp
	Service  string `json:"service"`  // e.g. "ssh", "http", "mysql"
	Version  string `json:"version,omitempty"`
}

// Vulnerability is a CVE vulnerability associated with an asset.
type Vulnerability struct {
	ID         uuid.UUID `json:"id,omitempty"`
	AssetID    uuid.UUID `json:"asset_id"`
	CveID      string    `json:"cve_id"`
	Severity   string    `json:"severity"` // critical|high|medium|low|none
	Cvss       *float64  `json:"cvss,omitempty"`
	DetectedAt time.Time `json:"detected_at"`
}

// Asset represents a network asset (host, server, device).
type Asset struct {
	ID           uuid.UUID       `json:"id"`
	IPAddress    string          `json:"ip_address"`
	Hostname     string          `json:"hostname,omitempty"`
	OS           string          `json:"os,omitempty"`
	MACAddress   string          `json:"mac_address,omitempty"`
	Services     []ServicePort   `json:"services,omitempty"`
	Tags         []string        `json:"tags"`
	Labels       map[string]string `json:"labels,omitempty"`
	RiskScore    float64         `json:"risk_score"`
	FindingCount int             `json:"finding_count"`
	Status       AssetStatus     `json:"status"`
	LastSeenAt   *time.Time      `json:"last_seen_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// AssetFilter defines query parameters for listing assets.
type AssetFilter struct {
	Status    AssetStatus
	Tags      []string // filter by tag containment
	Tag       string
	OS        string
	Query     string
	HasPort   *int
	IPAddress string // exact or CIDR match
	Hostname  string // partial match
	Page      int
	Limit     int
}

// AssetCreateInput is the input for creating a new asset.
type AssetCreateInput struct {
	IPAddress  string            `json:"ip_address"`
	Hostname   string            `json:"hostname"`
	OS         string            `json:"os"`
	MACAddress string            `json:"mac_address"`
	Services   []ServicePort     `json:"services,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// BulkAssetResult is the per-item result of a bulk asset create operation.
type BulkAssetResult struct {
	IPAddress string     `json:"ip_address"`
	Status    string     `json:"status"`  // "created" | "updated" | "skipped" | "error"
	ID        *uuid.UUID `json:"id,omitempty"`
	Message   string     `json:"message,omitempty"`
}

// ScanSchedule defines a scheduled scan job for an asset.
type ScanSchedule struct {
	ID           uuid.UUID  `json:"id"`
	AssetID      uuid.UUID  `json:"asset_id"`
	ScanType     string     `json:"scan_type"` // nmap|zap|agent|manual
	ScheduleCron string     `json:"schedule_cron"`
	Enabled      bool       `json:"enabled"`
	LastRunAt    *time.Time `json:"last_run_at,omitempty"`
	NextRunAt    *time.Time `json:"next_run_at,omitempty"`
	CreatedBy    *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
