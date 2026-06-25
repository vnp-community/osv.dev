package asset

import (
    "encoding/json"
    "net"
    "strings"
    "time"

    "github.com/google/uuid"
)

// AssetStatus tracks the operational state of an asset
type AssetStatus string

const (
    StatusActive   AssetStatus = "active"
    StatusInactive AssetStatus = "inactive"
    StatusUnknown  AssetStatus = "unknown"
)

// NetworkService represents a detected network service on an asset
type NetworkService struct {
    Port     int    `json:"port"`
    Protocol string `json:"protocol"` // "tcp" | "udp"
    Name     string `json:"name"`     // e.g., "http", "ssh", "mysql"
    Product  string `json:"product"`  // e.g., "Apache httpd"
    Version  string `json:"version"`  // e.g., "2.4.51"
    Banner   string `json:"banner,omitempty"`
}

// WebTechnology represents a web technology detected on an asset
type WebTechnology struct {
    Name       string   `json:"name"`
    Version    string   `json:"version,omitempty"`
    Categories []string `json:"categories"` // e.g., ["CMS", "PHP"]
}

// Asset represents a discovered IT asset in the network
type Asset struct {
    ID            uuid.UUID
    IPAddress     net.IP
    Hostname      string
    OS            string
    OSVersion     string
    MACAddress    string
    Status        AssetStatus
    Tags          []string
    Services      []NetworkService
    WebTech       []WebTechnology
    LastScanID    *uuid.UUID
    LastScannedAt *time.Time
    FindingCount  int     // Number of active findings
    RiskScore     float64 // 0.0 (safe) - 10.0 (critical)
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// NewAsset creates a new Asset from scan discovery data
func NewAsset(ipStr, hostname, os, osVersion string) (*Asset, error) {
    ip := net.ParseIP(strings.TrimSpace(ipStr))
    if ip == nil {
        return nil, ErrInvalidIPAddress
    }

    now := time.Now().UTC()
    return &Asset{
        ID:        uuid.New(),
        IPAddress: ip,
        Hostname:  strings.TrimSpace(hostname),
        OS:        os,
        OSVersion: osVersion,
        Status:    StatusActive,
        Tags:      []string{},
        Services:  []NetworkService{},
        WebTech:   []WebTechnology{},
        CreatedAt: now,
        UpdatedAt: now,
    }, nil
}

// IPString returns the IP address as a string
func (a *Asset) IPString() string {
    if a.IPAddress == nil {
        return ""
    }
    return a.IPAddress.String()
}

// AddTag adds a tag if not already present (case-insensitive dedup)
func (a *Asset) AddTag(tag string) {
    tag = strings.TrimSpace(tag)
    if tag == "" {
        return
    }
    for _, existing := range a.Tags {
        if strings.EqualFold(existing, tag) {
            return
        }
    }
    a.Tags = append(a.Tags, tag)
    a.UpdatedAt = time.Now().UTC()
}

// RemoveTag removes a tag (case-insensitive)
func (a *Asset) RemoveTag(tag string) {
    tag = strings.ToLower(strings.TrimSpace(tag))
    filtered := a.Tags[:0]
    for _, t := range a.Tags {
        if !strings.EqualFold(t, tag) {
            filtered = append(filtered, t)
        }
    }
    a.Tags = filtered
    a.UpdatedAt = time.Now().UTC()
}

// SetTags replaces all tags with the provided list
func (a *Asset) SetTags(tags []string) {
    a.Tags = make([]string, 0, len(tags))
    for _, t := range tags {
        t = strings.TrimSpace(t)
        if t != "" {
            a.Tags = append(a.Tags, t)
        }
    }
    a.UpdatedAt = time.Now().UTC()
}

// UpdateFromScan updates asset data from a new scan result
func (a *Asset) UpdateFromScan(scanID uuid.UUID, hostname, os, osVersion string, services []NetworkService) {
    now := time.Now().UTC()
    if hostname != "" {
        a.Hostname = hostname
    }
    if os != "" {
        a.OS = os
    }
    if osVersion != "" {
        a.OSVersion = osVersion
    }
    if len(services) > 0 {
        a.Services = services
    }
    a.LastScanID = &scanID
    a.LastScannedAt = &now
    a.Status = StatusActive
    a.UpdatedAt = now
}

// UpdateRiskScore recomputes the risk score based on finding stats
func (a *Asset) UpdateRiskScore(stats FindingStats) {
    a.FindingCount = stats.Active
    a.RiskScore = computeRiskScore(stats)
    a.UpdatedAt = time.Now().UTC()
}

// ServicesJSON serializes services for DB storage
func (a *Asset) ServicesJSON() ([]byte, error) {
    return json.Marshal(a.Services)
}

// WebTechJSON serializes web technologies for DB storage
func (a *Asset) WebTechJSON() ([]byte, error) {
    return json.Marshal(a.WebTech)
}
