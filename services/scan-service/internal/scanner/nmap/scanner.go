package nmap

import (
    "bytes"
    "context"
    "fmt"
    "os/exec"
    "sync"
    "time"

    "github.com/rs/zerolog"
)

const (
    defaultNmapPath    = "/usr/bin/nmap"
    defaultTimeout     = 300 * time.Second
    defaultIntensity   = "4" // -T4
)

// ScanFinding represents a discovered host with its findings
type ScanFinding struct {
    IPAddress   string
    Hostname    string
    OS          string
    Status      string // "up" | "down"
    OpenPorts   []Port
    Services    []Service
    CVEIDs      []string // Extracted from vulners script
    RawNmapHost string   // Raw XML for audit
}

// DiscoveryHost represents a host found during ping scan
type DiscoveryHost struct {
    IPAddress string
    Hostname  string
    Status    string // "up" | "down"
}

// Port represents an open port
type Port struct {
    Port     int    `json:"port"`
    Protocol string `json:"protocol"` // "tcp" | "udp"
    State    string `json:"state"`    // "open" | "closed" | "filtered"
}

// Service represents a detected network service
type Service struct {
    Port     int    `json:"port"`
    Protocol string `json:"protocol"`
    Name     string `json:"name"`    // e.g., "http", "ssh"
    Product  string `json:"product"` // e.g., "Apache httpd"
    Version  string `json:"version"` // e.g., "2.4.51"
}

// Scanner wraps the nmap binary for vulnerability scanning
type Scanner struct {
    nmapPath   string
    timeout    time.Duration
    intensity  string
    activeCmds sync.Map // scanID → *exec.Cmd for cancellation
    logger     zerolog.Logger
}

// New creates a NmapScanner
func New(nmapPath string, timeout time.Duration, logger zerolog.Logger) *Scanner {
    if nmapPath == "" {
        nmapPath = defaultNmapPath
    }
    if timeout == 0 {
        timeout = defaultTimeout
    }
    return &Scanner{
        nmapPath:  nmapPath,
        timeout:   timeout,
        intensity: defaultIntensity,
        logger:    logger,
    }
}

// FullScan performs a comprehensive network vulnerability scan
// Nmap flags used:
//   -sV: Service/version detection
//   -O: OS detection (requires root/cap NET_RAW)
//   --script=vulners: CVE lookup via vulners NSE script
//   -oX -: XML output to stdout
//   -T4: Aggressive timing
func (s *Scanner) FullScan(ctx context.Context, scanID string, targets []string) ([]*ScanFinding, error) {
    args := buildFullScanArgs(targets, s.intensity)

    s.logger.Info().
        Str("scan_id", scanID).
        Strs("targets", targets).
        Strs("args", args).
        Msg("starting nmap full scan")

    start := time.Now()
    xmlOutput, err := s.runNmap(ctx, scanID, args)
    if err != nil {
        return nil, fmt.Errorf("nmap execution: %w", err)
    }

    s.logger.Info().
        Str("scan_id", scanID).
        Dur("duration", time.Since(start)).
        Int("output_bytes", len(xmlOutput)).
        Msg("nmap scan completed")

    findings, err := ParseXMLOutput(xmlOutput)
    if err != nil {
        return nil, fmt.Errorf("parse nmap output: %w", err)
    }

    return findings, nil
}

// DiscoveryScan performs a ping scan to discover live hosts
// Nmap flags: -sn (ping scan, no port scan)
func (s *Scanner) DiscoveryScan(ctx context.Context, scanID string, targets []string) ([]*DiscoveryHost, error) {
    args := append([]string{"-sn", "-oX", "-"}, targets...)

    xmlOutput, err := s.runNmap(ctx, scanID, args)
    if err != nil {
        return nil, fmt.Errorf("nmap discovery: %w", err)
    }

    return ParseDiscoveryXML(xmlOutput)
}

// runNmap executes the nmap binary and returns stdout
func (s *Scanner) runNmap(ctx context.Context, scanID string, args []string) ([]byte, error) {
    // Wrap with timeout
    ctx, cancel := context.WithTimeout(ctx, s.timeout)
    defer cancel()

    cmd := exec.CommandContext(ctx, s.nmapPath, args...)

    // Store cmd reference for external cancellation
    s.activeCmds.Store(scanID, cmd)
    defer s.activeCmds.Delete(scanID)

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        if ctx.Err() != nil {
            return nil, fmt.Errorf("scan cancelled or timed out: %w", ctx.Err())
        }
        return nil, fmt.Errorf("nmap failed: %w (stderr: %s)", err, stderr.String())
    }

    return stdout.Bytes(), nil
}

// Cancel kills the nmap process for a given scanID
func (s *Scanner) Cancel(scanID string) bool {
    v, ok := s.activeCmds.Load(scanID)
    if !ok {
        return false
    }
    cmd := v.(*exec.Cmd)
    if cmd.Process != nil {
        _ = cmd.Process.Kill()
        return true
    }
    return false
}

// buildFullScanArgs constructs the nmap command arguments
func buildFullScanArgs(targets []string, intensity string) []string {
    args := []string{
        "-sV",                    // Service version detection
        "-O",                     // OS detection
        "--script=vulners",       // CVE lookup (NSE script)
        "--script-args=mincvss=0", // Include all CVSSs
        "-oX", "-",               // XML output to stdout
        "-T" + intensity,         // Timing template
    }
    return append(args, targets...)
}
