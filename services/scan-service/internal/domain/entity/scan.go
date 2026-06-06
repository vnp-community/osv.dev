package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ScanType defines the type of scan to perform.
type ScanType string

const (
	ScanTypeFull      ScanType = "full"      // nmap -sV -O --script=vulners
	ScanTypeDiscovery ScanType = "discovery" // nmap -sn (host discovery only)
	ScanTypeWeb       ScanType = "web"       // OWASP ZAP active scan
	ScanTypeAgent     ScanType = "agent"     // triggered by agent report
)

// ScanStatus represents the lifecycle state of a scan.
type ScanStatus string

const (
	ScanStatusPending   ScanStatus = "pending"
	ScanStatusQueued    ScanStatus = "queued"
	ScanStatusRunning   ScanStatus = "running"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
	ScanStatusCancelled ScanStatus = "cancelled"
)

// CanTransitionTo validates state machine transitions.
// pending → queued → running → completed
//
//	                         ↘ failed
//	           ← cancelled (from queued or running)
func (s ScanStatus) CanTransitionTo(next ScanStatus) bool {
	transitions := map[ScanStatus][]ScanStatus{
		ScanStatusPending:   {ScanStatusQueued, ScanStatusCancelled},
		ScanStatusQueued:    {ScanStatusRunning, ScanStatusCancelled},
		ScanStatusRunning:   {ScanStatusCompleted, ScanStatusFailed, ScanStatusCancelled},
		ScanStatusCompleted: {},
		ScanStatusFailed:    {},
		ScanStatusCancelled: {},
	}
	for _, allowed := range transitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// ScanOptions holds configurable scan parameters.
type ScanOptions struct {
	Ports     string `json:"ports,omitempty"`     // e.g. "1-1024,8080,8443"
	Timeout   int    `json:"timeout,omitempty"`   // seconds
	Intensity int    `json:"intensity,omitempty"` // nmap -T1..-T5 (1=sneaky, 5=insane)
	MaxDepth  int    `json:"max_depth,omitempty"` // web crawl depth
	ZAPConfig struct {
		SpiderTimeout      int `json:"spider_timeout,omitempty"`
		ActiveScanTimeout  int `json:"active_scan_timeout,omitempty"`
	} `json:"zap_config,omitempty"`
}

// Scan represents a vulnerability scan job.
type Scan struct {
	ID           uuid.UUID       `db:"id"`
	UserID       uuid.UUID       `db:"user_id"`
	Targets      []string        `db:"targets"`     // IPs, CIDRs, hostnames, URLs
	ScanType     ScanType        `db:"scan_type"`
	Status       ScanStatus      `db:"status"`
	Priority     int             `db:"priority"`    // 1-10, higher = more urgent
	Options      ScanOptions     `db:"options"`
	ScheduledFor *time.Time      `db:"scheduled_for"`
	StartedAt    *time.Time      `db:"started_at"`
	CompletedAt  *time.Time      `db:"completed_at"`
	FailedAt     *time.Time      `db:"failed_at"`
	ErrorMsg     string          `db:"error_msg"`
	Progress     int             `db:"progress"` // 0-100 percentage
	FindingCount int             `db:"finding_count"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

// IsTerminal returns true if the scan is in a final state.
func (s *Scan) IsTerminal() bool {
	return s.Status == ScanStatusCompleted ||
		s.Status == ScanStatusFailed ||
		s.Status == ScanStatusCancelled
}

// Severity levels for findings and CVEs.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityNone     Severity = "none"
)

// SeverityFromCVSS derives severity from a CVSS v3 score.
func SeverityFromCVSS(score float64) Severity {
	switch {
	case score >= 9.0:
		return SeverityCritical
	case score >= 7.0:
		return SeverityHigh
	case score >= 4.0:
		return SeverityMedium
	case score > 0:
		return SeverityLow
	default:
		return SeverityNone
	}
}

// Port represents a single network port with its state.
type Port struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // tcp|udp
	State    string `json:"state"`    // open|closed|filtered
}

// Service represents a detected service on a port.
type Service struct {
	Port     int    `json:"port"`
	Name     string `json:"name"`
	Product  string `json:"product"`
	Version  string `json:"version"`
}

// WebTechnology represents a web technology detected on a target.
type WebTechnology struct {
	Name       string   `json:"name"`
	Version    string   `json:"version,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

// Finding represents a single host discovered during a scan.
type Finding struct {
	ID        uuid.UUID       `db:"id"`
	ScanID    uuid.UUID       `db:"scan_id"`
	IPAddress string          `db:"ip_address"`
	Hostname  string          `db:"hostname"`
	OS        string          `db:"os"`
	OpenPorts []Port          `db:"open_ports"`
	Services  []Service       `db:"services"`
	WebTech   []WebTechnology `db:"web_tech"`
	CVEIDs    []string        `db:"cve_ids"`
	Severity  Severity        `db:"severity"`
	RawData   json.RawMessage `db:"raw_data"` // nmap/zap raw output
	CreatedAt time.Time       `db:"created_at"`
}

// WebAlert represents a security alert from the ZAP web scanner.
type WebAlert struct {
	ID          uuid.UUID `db:"id"`
	ScanID      uuid.UUID `db:"scan_id"`
	TargetURL   string    `db:"target_url"`
	AlertName   string    `db:"alert_name"`
	Risk        string    `db:"risk"`       // High|Medium|Low|Informational
	Confidence  string    `db:"confidence"` // High|Medium|Low|False Positive
	Description string    `db:"description"`
	Solution    string    `db:"solution"`
	Reference   string    `db:"reference"`
	Evidence    string    `db:"evidence"`
	CreatedAt   time.Time `db:"created_at"`
}

// DiscoveryHost represents a host found during a discovery scan.
type DiscoveryHost struct {
	ID        uuid.UUID  `db:"id"`
	ScanID    uuid.UUID  `db:"scan_id"`
	IPAddress string     `db:"ip_address"`
	Hostname  string     `db:"hostname"`
	Status    string     `db:"status"` // up|down
	CreatedAt time.Time  `db:"created_at"`
}
