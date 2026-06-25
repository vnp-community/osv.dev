# TASK-SCAN-001 — Nmap Scanner: Subprocess Wrapper + XML Parser

| Field | Value |
|-------|-------|
| **Task ID** | T-SCAN-001 |
| **Service** | `scan-service` |
| **Status** | ✅ Completed |
| **Solution Ref** | SOL-OVS-001 §4.1 Nmap Full Scan Flow |
| **Priority** | 🔴 High |
| **Depends On** | — |
| **Estimated** | 4h |

---

## Context

`scan-service` đã tồn tại tại `services/scan-service/`. Domain entity `scan.go` đã có. Cần implement scanner layer:

- **NmapScanner** — wraps `nmap` binary subprocess, parses XML output
- **CVE extraction** — regex extract từ nmap `vulners` script output
- **Cancel support** — qua `context.Context` cancellation

---

## Goal

Implement `NmapScanner` struct với các phương thức:
1. `FullScan(ctx, targets) → []ScanFinding` — network + service + vuln detection
2. `DiscoveryScan(ctx, targets) → []DiscoveryHost` — host discovery only (-sn)

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/scan-service/internal/scanner/nmap/scanner.go` |
| CREATE | `services/scan-service/internal/scanner/nmap/parser.go` |
| CREATE | `services/scan-service/internal/scanner/nmap/scanner_test.go` |

---

## Implementation

### File 1: `services/scan-service/internal/scanner/nmap/scanner.go`

```go
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
```

### File 2: `services/scan-service/internal/scanner/nmap/parser.go`

```go
package nmap

import (
    "encoding/xml"
    "fmt"
    "regexp"
    "strconv"
    "strings"
)

// cveRegex matches CVE IDs in nmap vulners script output
var cveRegex = regexp.MustCompile(`CVE-\d{4}-\d{4,}`)

// nmapRun is the top-level XML structure from nmap -oX
type nmapRun struct {
    XMLName xml.Name  `xml:"nmaprun"`
    Hosts   []nmapHost `xml:"host"`
}

type nmapHost struct {
    Status    nmapStatus    `xml:"status"`
    Addresses []nmapAddress `xml:"address"`
    Hostnames []nmapHostname `xml:"hostnames>hostname"`
    OS        nmapOS        `xml:"os"`
    Ports     nmapPorts     `xml:"ports"`
}

type nmapStatus struct {
    State string `xml:"state,attr"` // "up" | "down"
}

type nmapAddress struct {
    Addr     string `xml:"addr,attr"`
    AddrType string `xml:"addrtype,attr"` // "ipv4" | "ipv6" | "mac"
}

type nmapHostname struct {
    Name string `xml:"name,attr"`
    Type string `xml:"type,attr"` // "user" | "PTR"
}

type nmapOS struct {
    Matches []nmapOSMatch `xml:"osmatch"`
}

type nmapOSMatch struct {
    Name     string `xml:"name,attr"`
    Accuracy string `xml:"accuracy,attr"`
}

type nmapPorts struct {
    Ports []nmapPort `xml:"port"`
}

type nmapPort struct {
    Protocol string      `xml:"protocol,attr"` // "tcp" | "udp"
    PortID   string      `xml:"portid,attr"`
    State    nmapState   `xml:"state"`
    Service  nmapService `xml:"service"`
    Scripts  []nmapScript `xml:"script"`
}

type nmapState struct {
    State string `xml:"state,attr"` // "open" | "closed" | "filtered"
}

type nmapService struct {
    Name    string `xml:"name,attr"`
    Product string `xml:"product,attr"`
    Version string `xml:"version,attr"`
}

type nmapScript struct {
    ID     string `xml:"id,attr"`
    Output string `xml:"output,attr"`
}

// ParseXMLOutput parses nmap XML output into ScanFinding slice
func ParseXMLOutput(data []byte) ([]*ScanFinding, error) {
    var run nmapRun
    if err := xml.Unmarshal(data, &run); err != nil {
        return nil, fmt.Errorf("parse XML: %w", err)
    }

    findings := make([]*ScanFinding, 0, len(run.Hosts))
    for _, h := range run.Hosts {
        f := convertHost(h)
        findings = append(findings, f)
    }

    return findings, nil
}

// ParseDiscoveryXML parses nmap -sn XML output
func ParseDiscoveryXML(data []byte) ([]*DiscoveryHost, error) {
    var run nmapRun
    if err := xml.Unmarshal(data, &run); err != nil {
        return nil, fmt.Errorf("parse XML: %w", err)
    }

    hosts := make([]*DiscoveryHost, 0, len(run.Hosts))
    for _, h := range run.Hosts {
        dh := &DiscoveryHost{Status: h.Status.State}
        for _, addr := range h.Addresses {
            if addr.AddrType == "ipv4" || addr.AddrType == "ipv6" {
                dh.IPAddress = addr.Addr
            }
        }
        for _, hostname := range h.Hostnames {
            if hostname.Type == "PTR" || hostname.Type == "user" {
                dh.Hostname = hostname.Name
                break
            }
        }
        hosts = append(hosts, dh)
    }

    return hosts, nil
}

func convertHost(h nmapHost) *ScanFinding {
    f := &ScanFinding{Status: h.Status.State}

    // Extract IP and hostname
    for _, addr := range h.Addresses {
        if addr.AddrType == "ipv4" || addr.AddrType == "ipv6" {
            f.IPAddress = addr.Addr
        }
    }
    for _, hn := range h.Hostnames {
        if hn.Type == "PTR" || hn.Type == "user" {
            f.Hostname = hn.Name
            break
        }
    }

    // Extract OS
    if len(h.OS.Matches) > 0 {
        f.OS = h.OS.Matches[0].Name
    }

    // Extract ports, services, CVEs
    cveSet := make(map[string]struct{})
    for _, p := range h.Ports.Ports {
        portNum, _ := strconv.Atoi(p.PortID)

        f.OpenPorts = append(f.OpenPorts, Port{
            Port:     portNum,
            Protocol: p.Protocol,
            State:    p.State.State,
        })

        if p.State.State == "open" && p.Service.Name != "" {
            f.Services = append(f.Services, Service{
                Port:     portNum,
                Protocol: p.Protocol,
                Name:     p.Service.Name,
                Product:  p.Service.Product,
                Version:  p.Service.Version,
            })
        }

        // Extract CVE IDs from vulners script output
        for _, script := range p.Scripts {
            if script.ID == "vulners" {
                for _, cve := range extractCVEs(script.Output) {
                    cveSet[cve] = struct{}{}
                }
            }
        }
    }

    // Deduplicate CVE IDs
    for cve := range cveSet {
        f.CVEIDs = append(f.CVEIDs, cve)
    }

    return f
}

// extractCVEs finds all CVE IDs in a text string
func extractCVEs(text string) []string {
    matches := cveRegex.FindAllString(text, -1)
    // Deduplicate
    seen := make(map[string]struct{})
    result := make([]string, 0, len(matches))
    for _, m := range matches {
        upper := strings.ToUpper(m)
        if _, exists := seen[upper]; !exists {
            seen[upper] = struct{}{}
            result = append(result, upper)
        }
    }
    return result
}
```

### File 3: `services/scan-service/internal/scanner/nmap/scanner_test.go`

```go
package nmap

import (
    "strings"
    "testing"
)

// Sample nmap XML output for testing (minimal)
var sampleNmapXML = `<?xml version="1.0"?>
<nmaprun>
  <host>
    <status state="up"/>
    <address addr="192.168.1.10" addrtype="ipv4"/>
    <hostnames>
      <hostname name="web-server.local" type="PTR"/>
    </hostnames>
    <os>
      <osmatch name="Linux 5.15" accuracy="95"/>
    </os>
    <ports>
      <port protocol="tcp" portid="80">
        <state state="open"/>
        <service name="http" product="Apache httpd" version="2.4.51"/>
        <script id="vulners" output="
          CVE-2021-41773 7.5 https://vulners.com/cve/CVE-2021-41773
          CVE-2021-42013 9.8 https://vulners.com/cve/CVE-2021-42013
        "/>
      </port>
      <port protocol="tcp" portid="22">
        <state state="open"/>
        <service name="ssh" product="OpenSSH" version="8.4p1"/>
      </port>
    </ports>
  </host>
</nmaprun>`

func TestParseXMLOutput_Basic(t *testing.T) {
    findings, err := ParseXMLOutput([]byte(sampleNmapXML))
    if err != nil {
        t.Fatalf("ParseXMLOutput() error: %v", err)
    }
    if len(findings) != 1 {
        t.Fatalf("expected 1 finding, got %d", len(findings))
    }

    f := findings[0]
    if f.IPAddress != "192.168.1.10" {
        t.Errorf("IP = %s, want 192.168.1.10", f.IPAddress)
    }
    if f.Hostname != "web-server.local" {
        t.Errorf("Hostname = %s, want web-server.local", f.Hostname)
    }
    if f.OS != "Linux 5.15" {
        t.Errorf("OS = %s, want 'Linux 5.15'", f.OS)
    }
    if f.Status != "up" {
        t.Errorf("Status = %s, want up", f.Status)
    }
    if len(f.OpenPorts) != 2 {
        t.Errorf("OpenPorts len = %d, want 2", len(f.OpenPorts))
    }
    if len(f.Services) != 2 {
        t.Errorf("Services len = %d, want 2", len(f.Services))
    }
}

func TestParseXMLOutput_CVEExtraction(t *testing.T) {
    findings, _ := ParseXMLOutput([]byte(sampleNmapXML))
    f := findings[0]

    if len(f.CVEIDs) != 2 {
        t.Errorf("CVEIDs len = %d, want 2", len(f.CVEIDs))
    }

    cveSet := make(map[string]bool)
    for _, cve := range f.CVEIDs {
        cveSet[cve] = true
    }
    if !cveSet["CVE-2021-41773"] {
        t.Error("CVE-2021-41773 should be in CVEIDs")
    }
    if !cveSet["CVE-2021-42013"] {
        t.Error("CVE-2021-42013 should be in CVEIDs")
    }
}

func TestExtractCVEs_Regex(t *testing.T) {
    text := `CVE-2021-44228 10.0 RCE\nCVE-2021-45105 7.5 DoS\nnot-a-cve ignored`
    cves := extractCVEs(text)
    if len(cves) != 2 {
        t.Errorf("expected 2 CVEs, got %d: %v", len(cves), cves)
    }
    for _, cve := range cves {
        if !strings.HasPrefix(cve, "CVE-") {
            t.Errorf("CVE %s should start with CVE-", cve)
        }
    }
}

func TestExtractCVEs_Deduplication(t *testing.T) {
    text := "CVE-2021-44228 CVE-2021-44228 CVE-2021-44228"
    cves := extractCVEs(text)
    if len(cves) != 1 {
        t.Errorf("expected 1 unique CVE, got %d", len(cves))
    }
}

func TestParseXMLOutput_InvalidXML(t *testing.T) {
    _, err := ParseXMLOutput([]byte("not valid xml <"))
    if err == nil {
        t.Error("expected error for invalid XML")
    }
}
```

---

## Verification

```bash
cd services/scan-service
go build ./internal/scanner/nmap/...
go test ./internal/scanner/nmap/... -v
```

**Expected**:
```
--- PASS: TestParseXMLOutput_Basic
--- PASS: TestParseXMLOutput_CVEExtraction
--- PASS: TestExtractCVEs_Regex
--- PASS: TestExtractCVEs_Deduplication
--- PASS: TestParseXMLOutput_InvalidXML
```

### Integration Test (requires nmap binary)

```bash
# Only run if nmap is installed
which nmap && go test ./internal/scanner/nmap/... -v -run Integration -tags integration
```

### Checklist

- [x] `ParseXMLOutput` parses IP, hostname, OS, open ports, services correctly
- [x] CVE IDs extracted from vulners script output (`CVE-\d{4}-\d{4,}` regex)
- [x] Duplicate CVE IDs are deduplicated
- [x] `DiscoveryScan` output includes status=up/down for each host
- [x] `Cancel(scanID)` kills nmap process
- [x] Context cancellation propagates → nmap process killed
- [x] Timeout wraps the scan (default 300s)

## Notes for AI

- `nmap -O` requires `NET_RAW` and `NET_ADMIN` Linux capabilities in Docker
- The `vulners` NSE script must be installed: `nmap --script-updatedb` (or included in Docker image)
- For CI/CD environments without nmap, mock the binary path in tests
