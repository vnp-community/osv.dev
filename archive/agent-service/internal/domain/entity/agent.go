package entity

import (
	"time"
	"github.com/google/uuid"
)

// AgentStatus describes the liveness state of an agent.
type AgentStatus string
const (
	AgentStatusActive   AgentStatus = "active"
	AgentStatusInactive AgentStatus = "inactive"
	AgentStatusUnknown  AgentStatus = "unknown"
)

// Agent represents a deployed OVS scanning agent.
type Agent struct {
	ID           uuid.UUID   `db:"id"`
	Name         string      `db:"name"`
	Hostname     string      `db:"hostname"`
	IPAddress    string      `db:"ip_address"`
	OS           string      `db:"os"`
	AgentVersion string      `db:"agent_version"`
	APIKeyID     uuid.UUID   `db:"api_key_id"`
	Status       AgentStatus `db:"status"`
	LastSeenAt   *time.Time  `db:"last_seen_at"`
	Tags         []string    `db:"tags"`
	CreatedAt    time.Time   `db:"created_at"`
	UpdatedAt    time.Time   `db:"updated_at"`
}

// IsActive returns true if the agent checked in within the last 24 hours.
func (a *Agent) IsActive() bool {
	if a.LastSeenAt == nil { return false }
	return time.Since(*a.LastSeenAt) < 24*time.Hour
}

// PackageEcosystem identifies the package manager.
type PackageEcosystem string
const (
	EcosystemDebian  PackageEcosystem = "debian"
	EcosystemRPM     PackageEcosystem = "rpm"
	EcosystemHomebrew PackageEcosystem = "homebrew"
	EcosystemPyPI    PackageEcosystem = "pypi"
	EcosystemNPM     PackageEcosystem = "npm"
	EcosystemGo      PackageEcosystem = "go"
)

// PackageCVE is a pre-enriched CVE entry for a package.
type PackageCVE struct {
	CVEID    string  `json:"cve_id"`
	Severity string  `json:"severity"`
	CVSS     float64 `json:"cvss"`
}

// Package is a software package installed on an agent host.
type Package struct {
	ID           uuid.UUID        `db:"id"`
	ReportID     uuid.UUID        `db:"report_id"`
	Name         string           `db:"name"`
	Version      string           `db:"version"`
	Ecosystem    PackageEcosystem `db:"ecosystem"`
	Architecture string           `db:"architecture"`
	CVEs         []PackageCVE     `db:"-"`
}

// AgentReport is a data snapshot submitted by an agent.
type AgentReport struct {
	ID            uuid.UUID  `db:"id"`
	AgentID       uuid.UUID  `db:"agent_id"`
	Hostname      string     `db:"hostname"`
	IPAddress     string     `db:"ip_address"`
	OSInfo        string     `db:"os_info"`
	KernelVersion string     `db:"kernel_version"`
	Packages      []Package  `db:"-"`
	PackageCount  int        `db:"package_count"`
	CVECount      int        `db:"cve_count"`
	ReportedAt    time.Time  `db:"reported_at"`
	ProcessedAt   *time.Time `db:"processed_at"`
	CreatedAt     time.Time  `db:"created_at"`
}
